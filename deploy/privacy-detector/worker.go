package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Frames follow AstrLink's privacy-worker protocol v1: a 4-byte big-endian
// length, then one JSON document, in both directions over stdin/stdout.
const (
	protocolVersion = 1
	maxFrameBytes   = 64 << 20
)

type frameText struct {
	ID   uint32 `json:"id"`
	Text string `json:"text"`
}

type detectFrame struct {
	Version uint8       `json:"version"`
	ID      uint64      `json:"id"`
	Texts   []frameText `json:"texts"`
}

type workerSpan struct {
	TextID uint32  `json:"text_id"`
	Label  string  `json:"label"`
	Start  int     `json:"start"`
	End    int     `json:"end"`
	Score  float64 `json:"score"`
}

type workerResponse struct {
	Version uint8        `json:"version"`
	ID      uint64       `json:"id"`
	Ready   bool         `json:"ready"`
	Spans   []workerSpan `json:"spans"`
	Error   *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) == 0 || len(payload) > maxFrameBytes {
		return errors.New("frame size out of range")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > maxFrameBytes {
		return nil, errors.New("worker sent an invalid frame length")
	}
	payload := make([]byte, n)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

// errWorkerRejected is a per-frame failure the worker reported; the process
// stays healthy and keeps serving.
type errWorkerRejected struct{ code string }

func (e errWorkerRejected) Error() string { return "privacy worker rejected the frame: " + e.code }

// workerProcess owns one privacy-worker child. The protocol is strictly
// sequential, so a frame that times out leaves the stream in an unknown
// state and the process is killed rather than reused.
type workerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	done   chan struct{}
	nextID uint64
}

type workerSpec struct {
	binary       string
	modelDir     string
	intraThreads int
	startTimeout time.Duration
}

func startWorker(spec workerSpec) (*workerProcess, error) {
	args := []string{"--model-dir", spec.modelDir}
	if spec.intraThreads > 0 {
		args = append(args, "--intra-threads", fmt.Sprint(spec.intraThreads))
	}
	cmd := exec.Command(spec.binary, args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	w := &workerProcess{cmd: cmd, stdin: stdin, stdout: bufio.NewReaderSize(stdout, 64<<10), done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(w.done) }()
	ready := make(chan error, 1)
	go func() {
		payload, err := readFrame(w.stdout)
		if err != nil {
			ready <- fmt.Errorf("privacy worker exited during startup: %w", err)
			return
		}
		var resp workerResponse
		if err := json.Unmarshal(payload, &resp); err != nil || !resp.Ready || resp.Version != protocolVersion {
			ready <- errors.New("privacy worker sent an unexpected ready frame")
			return
		}
		ready <- nil
	}()
	select {
	case err = <-ready:
	case <-time.After(spec.startTimeout):
		err = errors.New("privacy worker did not become ready in time")
	}
	if err != nil {
		w.kill()
		return nil, err
	}
	return w, nil
}

func (w *workerProcess) kill() {
	_ = w.stdin.Close()
	if w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
	<-w.done
}

func (w *workerProcess) alive() bool {
	select {
	case <-w.done:
		return false
	default:
		return true
	}
}

// detect sends one frame and waits for its response. A returned error other
// than errWorkerRejected means the process must be discarded.
func (w *workerProcess) detect(ctx context.Context, texts []frameText) ([]workerSpan, error) {
	w.nextID++
	payload, err := json.Marshal(detectFrame{Version: protocolVersion, ID: w.nextID, Texts: texts})
	if err != nil {
		return nil, err
	}
	type result struct {
		resp workerResponse
		err  error
	}
	out := make(chan result, 1)
	go func() {
		if err := writeFrame(w.stdin, payload); err != nil {
			out <- result{err: err}
			return
		}
		frame, err := readFrame(w.stdout)
		if err != nil {
			out <- result{err: err}
			return
		}
		var resp workerResponse
		err = json.Unmarshal(frame, &resp)
		out <- result{resp: resp, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.done:
		return nil, errors.New("privacy worker exited")
	case r := <-out:
		if r.err != nil {
			return nil, r.err
		}
		if r.resp.ID != w.nextID {
			return nil, errors.New("privacy worker answered a different frame")
		}
		if r.resp.Error != nil {
			return nil, errWorkerRejected{code: r.resp.Error.Code}
		}
		return r.resp.Spans, nil
	}
}

// pool hands out worker processes and replaces any that fail.
type pool struct {
	spec    workerSpec
	idle    chan *workerProcess
	mu      sync.Mutex
	ready   int
	lastErr string
}

func newPool(spec workerSpec, size int) *pool {
	p := &pool{spec: spec, idle: make(chan *workerProcess, size)}
	for range size {
		go p.replace()
	}
	return p
}

// replace starts a worker, retrying with backoff until one runs.
func (p *pool) replace() {
	delay := time.Second
	for {
		w, err := startWorker(p.spec)
		if err == nil {
			p.mu.Lock()
			p.ready++
			p.mu.Unlock()
			p.idle <- w
			return
		}
		p.mu.Lock()
		p.lastErr = err.Error()
		p.mu.Unlock()
		fmt.Fprintln(os.Stderr, "privacy-detector:", err)
		time.Sleep(delay)
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func (p *pool) status() (ready int, lastErr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ready, p.lastErr
}

func (p *pool) run(ctx context.Context, timeout time.Duration, texts []frameText) ([]workerSpan, error) {
	var w *workerProcess
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case w = <-p.idle:
	}
	frameCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	spans, err := w.detect(frameCtx, texts)
	var rejected errWorkerRejected
	if err == nil || errors.As(err, &rejected) && w.alive() {
		p.idle <- w
		return spans, err
	}
	p.mu.Lock()
	p.ready--
	p.lastErr = err.Error()
	p.mu.Unlock()
	go func() { w.kill(); p.replace() }()
	return nil, err
}
