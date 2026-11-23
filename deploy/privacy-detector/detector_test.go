package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as a fake privacy-worker: with FAKE_PRIVACY_WORKER set,
// the test binary speaks the frame protocol and reports every "@" address.
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_PRIVACY_WORKER") == "1" {
		fakeWorker()
		return
	}
	os.Exit(m.Run())
}

func fakeWorker() {
	out := bufio.NewWriter(os.Stdout)
	ready, _ := json.Marshal(map[string]any{"version": 1, "ready": true})
	_ = writeFrame(out, ready)
	_ = out.Flush()
	in := bufio.NewReader(os.Stdin)
	for {
		payload, err := readFrame(in)
		if err != nil {
			return
		}
		var req detectFrame
		_ = json.Unmarshal(payload, &req)
		spans := []workerSpan{}
		for _, t := range req.Texts {
			if strings.Contains(t.Text, "CRASH") {
				os.Exit(3)
			}
			for i := strings.Index(t.Text, "a@b.cn"); i >= 0; {
				spans = append(spans, workerSpan{TextID: t.ID, Label: "email", Start: i, End: i + 6, Score: 0.99})
				next := strings.Index(t.Text[i+1:], "a@b.cn")
				if next < 0 {
					break
				}
				i += 1 + next
			}
		}
		resp, _ := json.Marshal(map[string]any{"version": 1, "id": req.ID, "spans": spans})
		_ = writeFrame(out, resp)
		_ = out.Flush()
	}
}

func fakeSpec(t *testing.T) workerSpec {
	t.Setenv("FAKE_PRIVACY_WORKER", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return workerSpec{binary: exe, modelDir: t.TempDir(), startTimeout: 10 * time.Second}
}

func waitReady(t *testing.T, p *pool, n int) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ready, _ := p.status(); ready >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("workers did not become ready")
}

func TestChunkerCoversLongTextOnRuneBoundaries(t *testing.T) {
	c := chunker{pieceBytes: 10, overlapBytes: 4, frameBytes: 25}
	text := strings.Repeat("中文ab", 6) // 48 bytes, multi-byte runes
	pieces := c.pieces([]string{"", text, "short"})
	covered := 0
	for _, p := range pieces {
		if p.text == 1 {
			if p.start > covered {
				t.Fatalf("gap before %d", p.start)
			}
			if text[p.start:p.start+len(p.body)] != p.body {
				t.Fatal("piece does not match its offset")
			}
			if !strings.HasPrefix(text[p.start:], p.body) || len(p.body) > 10 {
				t.Fatalf("bad piece %q", p.body)
			}
			covered = max(covered, p.start+len(p.body))
		}
	}
	if covered != len(text) {
		t.Fatalf("covered %d of %d", covered, len(text))
	}
	for _, f := range c.frames(pieces) {
		size := 0
		for _, i := range f {
			size += len(pieces[i].body)
		}
		if size > 25 && len(f) > 1 {
			t.Fatalf("frame of %d bytes", size)
		}
	}
}

func TestDetectMapsOverlappingPiecesToTextOffsets(t *testing.T) {
	p := newPool(fakeSpec(t), 2)
	waitReady(t, p, 2)
	d := &detector{runner: p, frameTimeout: 5 * time.Second, parallel: 2, chunk: chunker{pieceBytes: 16, overlapBytes: 8, frameBytes: 40}}
	text := "联系 a@b.cn 或者 then more words a@b.cn end"
	results, err := d.Detect(context.Background(), []string{"nothing here", text})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || len(results[0]) != 0 {
		t.Fatalf("unexpected results %+v", results)
	}
	var got []string
	for _, s := range results[1] {
		got = append(got, text[s.Start:s.End])
	}
	if strings.Join(got, ",") != "a@b.cn,a@b.cn" {
		t.Fatalf("spans %+v -> %q", results[1], got)
	}
}

func TestCrashedWorkerFailsRequestAndIsReplaced(t *testing.T) {
	p := newPool(fakeSpec(t), 1)
	waitReady(t, p, 1)
	d := &detector{runner: p, frameTimeout: 5 * time.Second, parallel: 1, chunk: chunker{pieceBytes: 1024, overlapBytes: 64, frameBytes: 4096}}
	if _, err := d.Detect(context.Background(), []string{"CRASH"}); err == nil {
		t.Fatal("expected an error from a crashed worker")
	}
	waitReady(t, p, 1)
	results, err := d.Detect(context.Background(), []string{"x a@b.cn"})
	if err != nil || len(results[0]) != 1 {
		t.Fatalf("replacement worker: %v %+v", err, results)
	}
}

func TestMergeSpans(t *testing.T) {
	got := mergeSpans([]Span{{Start: 5, End: 9, Label: "email", Score: 0.8}, {Start: 0, End: 3, Label: "phone", Score: 0.9}, {Start: 7, End: 12, Label: "email", Score: 0.95}})
	if len(got) != 2 || got[0].Label != "phone" || got[1].Start != 5 || got[1].End != 12 || got[1].Score != 0.95 {
		t.Fatalf("%+v", got)
	}
	if mergeSpans(nil) == nil {
		t.Fatal("empty results must encode as []")
	}
}

func TestHTTPAuthAndLimits(t *testing.T) {
	p := newPool(fakeSpec(t), 1)
	waitReady(t, p, 1)
	s := &server{cfg: config{apiKey: "0123456789abcdef", maxBodyBytes: 1 << 10, maxTexts: 2, modelID: "m"}, pool: p,
		detector: &detector{runner: p, frameTimeout: 5 * time.Second, parallel: 1, chunk: chunker{pieceBytes: 512, overlapBytes: 32, frameBytes: 2048}}}
	h := s.routes()
	call := func(key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/detect", strings.NewReader(body))
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("", `{"texts":["a"]}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: %d", rec.Code)
	}
	if rec := call("wrong-key-wrong-key", `{"texts":["a"]}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: %d", rec.Code)
	}
	if rec := call("0123456789abcdef", `{"texts":["a","b","c"]}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too many texts: %d", rec.Code)
	}
	if rec := call("0123456789abcdef", `{"texts":["`+strings.Repeat("x", 2048)+`"]}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", rec.Code)
	}
	rec := call("0123456789abcdef", `{"texts":["mail a@b.cn",""]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("detect: %d %s", rec.Code, rec.Body.String())
	}
	var resp detectResponse
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil || resp.Model != "m" || len(resp.Results) != 2 || len(resp.Results[0]) != 1 || resp.Results[0][0].Start != 5 {
		t.Fatalf("response %s", rec.Body.String())
	}
	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health %d", health.Code)
	}
}
