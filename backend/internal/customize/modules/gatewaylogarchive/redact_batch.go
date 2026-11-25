package gatewaylogarchive

import (
	"context"
	"io"
	"strings"
	"time"
)

// lazy is a redacted string that may still wait for the detection service.
type lazy struct {
	s  string
	fn func() string
}

func static(s string) lazy { return lazy{s: s} }

func (l lazy) get() string {
	if l.fn == nil {
		return l.s
	}
	return l.fn()
}

func concatLazy(parts []lazy) lazy {
	waiting := false
	for _, p := range parts {
		waiting = waiting || p.fn != nil
	}
	join := func() string {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.get())
		}
		return b.String()
	}
	if !waiting {
		return static(join())
	}
	return lazy{fn: join}
}

// lazyOut is the parser output. Without pending values it writes straight
// through (the root) or collects bytes (nested JSON inside a string). Once a
// value waits for the service, later output is held in order until the batch
// returns and the root drains it.
type lazyOut struct {
	w    io.Writer // nil for nested documents
	segs []lazySeg
	held int // bytes held in segs
}

type lazySeg struct {
	b  []byte
	fn func() []byte
}

func (o *lazyOut) write(b []byte) error {
	if o.w != nil && len(o.segs) == 0 {
		_, err := o.w.Write(b)
		return err
	}
	// Callers may pass reader buffers; copy.
	if n := len(o.segs); n > 0 && o.segs[n-1].fn == nil {
		o.segs[n-1].b = append(o.segs[n-1].b, b...)
	} else {
		o.segs = append(o.segs, lazySeg{b: append([]byte(nil), b...)})
	}
	o.held += len(b)
	return nil
}

func (o *lazyOut) deferred(fn func() []byte) { o.segs = append(o.segs, lazySeg{fn: fn}) }

// drain writes everything held. Every pending value must be resolved.
func (o *lazyOut) drain() error {
	for _, s := range o.segs {
		b := s.b
		if s.fn != nil {
			b = s.fn()
		}
		if _, err := o.w.Write(b); err != nil {
			return err
		}
	}
	o.segs, o.held = o.segs[:0], 0
	return nil
}

// result renders a nested document as one string value.
func (o *lazyOut) result() lazy {
	segs := o.segs
	render := func() string {
		var b strings.Builder
		for _, s := range segs {
			if s.fn != nil {
				b.Write(s.fn())
			} else {
				b.Write(s.b)
			}
		}
		return b.String()
	}
	for _, s := range segs {
		if s.fn != nil {
			return lazy{fn: render}
		}
	}
	return static(render())
}

// Texts are sent once this much is pending. One batch is the unit of
// latency and of memory held back behind it.
const (
	detectBatchBytes = 256 << 10
	detectBatchTexts = 1024
)

// detectBatch collects texts for the detection service across one document,
// including JSON nested in strings, and resolves them together.
type detectBatch struct {
	ctx     context.Context
	det     textDetector
	root    *lazyOut
	pending []string
	ids     []int
	bytes   int
	next    int
	results map[int][]detectSpan
	err     error
	counts  *docCounts // the document's statistics
}

func (b *detectBatch) leaf(text string) lazy {
	id := b.next
	b.next++
	b.pending = append(b.pending, text)
	b.ids = append(b.ids, id)
	b.bytes += len(text)
	if b.bytes >= detectBatchBytes || len(b.pending) >= detectBatchTexts {
		_ = b.flush() // a failure is kept in b.err and stops the parser
	}
	return lazy{fn: func() string {
		spans := b.results[id]
		delete(b.results, id)
		// Counted when rendered, so text discarded with a nested JSON value
		// that failed to parse is not counted.
		for _, s := range spans {
			b.counts.add(modelPrefix + s.Label)
		}
		return applySpans(text, spans)
	}}
}

// flush resolves the pending texts and writes the root output held behind
// them. Output that was appended before the flush only references texts
// queued before it, so all of it can be rendered.
func (b *detectBatch) flush() error {
	if b.err != nil {
		return b.err
	}
	if len(b.pending) > 0 {
		started := time.Now()
		spans, err := b.det.Detect(b.ctx, b.pending)
		if b.counts != nil {
			b.counts.detectorWait += time.Since(started)
		}
		if err == nil && len(spans) != len(b.pending) {
			err = detectorError("返回的结果数量不符")
		}
		if err != nil {
			b.err = err
			return err
		}
		for i, id := range b.ids {
			if len(spans[i]) > 0 {
				b.results[id] = spans[i]
			}
		}
		b.pending, b.ids, b.bytes = b.pending[:0], b.ids[:0], 0
	}
	b.err = b.root.drain()
	return b.err
}
