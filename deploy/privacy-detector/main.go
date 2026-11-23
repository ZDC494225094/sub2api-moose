// Command privacy-detector exposes AstrLink's local privacy model over HTTP
// so sub2api can redact gateway log archives before uploading them. It runs
// AstrLink's privacy-worker processes and only detects: the caller decides
// what to replace.
//
//	POST /v1/detect  {"texts": ["..."]}  -> {"results": [[{start,end,label,score}]], "model": "..."}
//	GET  /healthz                         -> 200 once a worker is ready
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	listen       string
	apiKey       string
	allowNoAuth  bool
	workerBinary string
	modelDir     string
	modelID      string
	workers      int
	intraThreads int
	maxBodyBytes int64
	maxTexts     int
	frameTimeout time.Duration
}

func envInt(name string, def, lo, hi int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || v < lo || v > hi {
		return def
	}
	return v
}

func envString(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func loadConfig() (config, error) {
	cpus := runtime.GOMAXPROCS(0) // honors the container CPU limit
	workers := envInt("DETECTOR_WORKERS", max(1, min(4, cpus/2)), 1, 64)
	cfg := config{
		listen:       envString("DETECTOR_LISTEN", ":8090"),
		apiKey:       os.Getenv("DETECTOR_API_KEY"),
		allowNoAuth:  os.Getenv("DETECTOR_ALLOW_NO_AUTH") == "true",
		workerBinary: envString("DETECTOR_WORKER_BINARY", "/opt/astrlink/astrlink-privacy-worker"),
		modelDir:     envString("DETECTOR_MODEL_DIR", "/opt/astrlink/model"),
		modelID:      envString("DETECTOR_MODEL_ID", "QuantumNous/astrlink-guard@7216b6a3b897ef903d4b1ba367ae6fb95b0fa5e4#cpu_int8"),
		workers:      workers,
		intraThreads: envInt("DETECTOR_INTRA_THREADS", max(1, cpus/workers), 1, 64),
		maxBodyBytes: int64(envInt("DETECTOR_MAX_BODY_MB", 16, 1, 256)) << 20,
		maxTexts:     envInt("DETECTOR_MAX_TEXTS", 4096, 1, 1<<20),
		frameTimeout: time.Duration(envInt("DETECTOR_FRAME_TIMEOUT_SECONDS", 120, 5, 3600)) * time.Second,
	}
	if len(cfg.apiKey) < 16 && !cfg.allowNoAuth {
		return cfg, errors.New("DETECTOR_API_KEY must be at least 16 characters (or set DETECTOR_ALLOW_NO_AUTH=true on a private network)")
	}
	return cfg, nil
}

type server struct {
	cfg      config
	pool     *pool
	detector *detector
}

type detectRequest struct {
	Texts []string `json:"texts"`
}

type detectResponse struct {
	Model   string   `json:"model"`
	Results [][]Span `json:"results"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func (s *server) authorized(r *http.Request) bool {
	if s.cfg.apiKey == "" {
		return s.cfg.allowNoAuth
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.apiKey)) == 1
}

func (s *server) handleDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or wrong API key")
		return
	}
	var req detectRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, s.cfg.maxBodyBytes))
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", fmt.Sprintf("request body exceeds %d bytes", s.cfg.maxBodyBytes))
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be {\"texts\": [string]}")
		return
	}
	if len(req.Texts) > s.cfg.maxTexts {
		writeError(w, http.StatusRequestEntityTooLarge, "too_many_texts", fmt.Sprintf("at most %d texts per request", s.cfg.maxTexts))
		return
	}
	if ready, _ := s.pool.status(); ready == 0 {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "privacy model is not ready")
		return
	}
	results, err := s.detector.Detect(r.Context(), req.Texts)
	if err != nil {
		var rejected errWorkerRejected
		if errors.As(err, &rejected) {
			writeError(w, http.StatusUnprocessableEntity, "worker_rejected", rejected.code)
			return
		}
		if r.Context().Err() != nil {
			return // the client went away
		}
		writeError(w, http.StatusServiceUnavailable, "detection_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detectResponse{Model: s.cfg.modelID, Results: results})
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	ready, lastErr := s.pool.status()
	body := map[string]any{"ready_workers": ready, "workers": s.cfg.workers, "model": s.cfg.modelID}
	if ready == 0 {
		body["status"], body["last_error"] = "starting", lastErr
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}
	body["status"] = "ok"
	writeJSON(w, http.StatusOK, body)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/detect", s.handleDetect)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

// healthcheck lets the container probe itself without curl in the image.
func healthcheck(listen string) int {
	port := listen[strings.LastIndex(listen, ":")+1:]
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(envString("DETECTOR_LISTEN", ":8090")))
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal("privacy-detector: ", err)
	}
	p := newPool(workerSpec{binary: cfg.workerBinary, modelDir: cfg.modelDir, intraThreads: cfg.intraThreads, startTimeout: 2 * time.Minute}, cfg.workers)
	s := &server{cfg: cfg, pool: p, detector: &detector{
		runner: p, frameTimeout: cfg.frameTimeout, parallel: cfg.workers,
		// AstrLink Core sends 8 KiB pieces with 512 bytes of overlap; a frame
		// of 64 KiB stays far below the worker's 128K-token limit.
		chunk: chunker{pieceBytes: 8 << 10, overlapBytes: 512, frameBytes: 64 << 10},
	}}
	srv := &http.Server{Addr: cfg.listen, Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	log.Printf("privacy-detector: listening on %s with %d worker(s), %d thread(s) each, model %s", cfg.listen, cfg.workers, cfg.intraThreads, cfg.modelID)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("privacy-detector: ", err)
	}
}
