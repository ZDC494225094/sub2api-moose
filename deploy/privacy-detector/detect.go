package main

import (
	"context"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

// Span is one detected sensitive range. Start and End are UTF-8 byte offsets
// into the text the caller sent.
type Span struct {
	Start int     `json:"start"`
	End   int     `json:"end"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type chunker struct {
	pieceBytes   int // longest text sent as one worker input
	overlapBytes int // neighbors share this much, so a value cut at one edge is whole in the next
	frameBytes   int // text per worker frame; bounds the worker's token budget
}

type piece struct {
	text, start int // index into the request texts, byte offset in that text
	body        string
}

// cutBack moves i back to the start of a UTF-8 sequence.
func cutBack(s string, i int) int {
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

func (c chunker) pieces(texts []string) []piece {
	var out []piece
	for ti, text := range texts {
		if text == "" {
			continue
		}
		for start := 0; ; {
			end := len(text)
			if end-start > c.pieceBytes {
				end = cutBack(text, start+c.pieceBytes)
				if end <= start { // a single rune longer than a piece cannot happen with sane sizes
					end = start + c.pieceBytes
				}
			}
			out = append(out, piece{text: ti, start: start, body: text[start:end]})
			if end == len(text) {
				break
			}
			next := cutBack(text, end-c.overlapBytes)
			if next <= start {
				next = end
			}
			start = next
		}
	}
	return out
}

func (c chunker) frames(pieces []piece) [][]int {
	var frames [][]int
	var current []int
	size := 0
	for i, p := range pieces {
		if len(current) > 0 && size+len(p.body) > c.frameBytes {
			frames = append(frames, current)
			current, size = nil, 0
		}
		current = append(current, i)
		size += len(p.body)
	}
	if len(current) > 0 {
		frames = append(frames, current)
	}
	return frames
}

type frameRunner interface {
	run(ctx context.Context, timeout time.Duration, texts []frameText) ([]workerSpan, error)
}

type detector struct {
	runner       frameRunner
	chunk        chunker
	frameTimeout time.Duration
	parallel     int
}

// Detect returns the spans of every text, in request order. Frames run in
// parallel up to the worker count; the first failure cancels the rest.
func (d *detector) Detect(ctx context.Context, texts []string) ([][]Span, error) {
	pieces := d.chunk.pieces(texts)
	frames := d.chunk.frames(pieces)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]Span, len(texts))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, d.parallel))
	for _, frame := range frames {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(frame []int) {
			defer wg.Done()
			defer func() { <-sem }()
			inputs := make([]frameText, len(frame))
			for i, idx := range frame {
				inputs[i] = frameText{ID: uint32(i), Text: pieces[idx].body}
			}
			spans, err := d.runner.run(ctx, d.frameTimeout, inputs)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			for _, s := range spans {
				if int(s.TextID) >= len(frame) {
					continue
				}
				p := pieces[frame[s.TextID]]
				if s.Start < 0 || s.End <= s.Start || s.End > len(p.body) {
					continue
				}
				results[p.text] = append(results[p.text], Span{Start: p.start + s.Start, End: p.start + s.End, Label: s.Label, Score: s.Score})
			}
		}(frame)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range results {
		results[i] = mergeSpans(results[i])
	}
	return results, nil
}

// mergeSpans joins overlapping spans of the same label (overlapping pieces
// report a value twice) and keeps the higher score.
func mergeSpans(spans []Span) []Span {
	if len(spans) < 2 {
		if spans == nil {
			return []Span{}
		}
		return spans
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Label != spans[j].Label {
			return spans[i].Label < spans[j].Label
		}
		return spans[i].Start < spans[j].Start
	})
	out := spans[:1]
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.Label == last.Label && s.Start <= last.End {
			last.End = max(last.End, s.End)
			last.Score = max(last.Score, s.Score)
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].End < out[j].End
	})
	return out
}
