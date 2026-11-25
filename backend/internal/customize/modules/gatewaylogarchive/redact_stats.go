package gatewaylogarchive

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Replacement kinds from the built-in rules. Model findings use the
// detection service's labels (private_person, ...).
const (
	ruleEmail   = "rule:email"
	rulePhone   = "rule:phone"
	ruleIDCard  = "rule:id_card"
	ruleAccount = "rule:account" // labeled passport, bank card or other ID numbers
	rulePerson  = "rule:person"  // labeled names
	ruleAddress = "rule:address" // labeled addresses
	ruleField   = "rule:field"   // values of explicit personal fields
	modelPrefix = "model:"
)

// docCounts collects the statistics of one JSON document before they are
// merged, so a document takes the shared lock once.
type docCounts struct {
	kinds        map[string]int64
	toolSkipped  int64
	detectorWait time.Duration
}

func (c *docCounts) add(kind string) {
	if c == nil {
		return
	}
	if c.kinds == nil {
		c.kinds = map[string]int64{}
	}
	c.kinds[kind]++
}

// mergeDoc adds the counts of a nested JSON text to its document.
func (c *docCounts) mergeDoc(n *docCounts) {
	if c == nil {
		return
	}
	for k, v := range n.kinds {
		if c.kinds == nil {
			c.kinds = map[string]int64{}
		}
		c.kinds[k] += v
	}
	c.toolSkipped += n.toolSkipped
	c.detectorWait += n.detectorWait
}

// redactStats accumulates redaction work: for one upload, or for all uploads
// of this node since it started.
type redactStats struct {
	records      atomic.Int64 // documents (sessions) read
	changed      atomic.Int64 // documents with at least one replacement
	toolSkipped  atomic.Int64 // strings left unchanged as tool content
	detectorWait atomic.Int64 // time spent waiting for the detection service
	mu           sync.Mutex
	kinds        map[string]int64
}

func (s *redactStats) merge(c *docCounts) {
	if s == nil {
		return
	}
	s.records.Add(1)
	s.toolSkipped.Add(c.toolSkipped)
	s.detectorWait.Add(int64(c.detectorWait))
	if len(c.kinds) == 0 {
		return
	}
	s.changed.Add(1)
	s.mu.Lock()
	if s.kinds == nil {
		s.kinds = map[string]int64{}
	}
	for k, n := range c.kinds {
		s.kinds[k] += n
	}
	s.mu.Unlock()
}

// RedactionCounts is the API view of redactStats.
type RedactionCounts struct {
	Records      int64            `json:"records"`
	Changed      int64            `json:"changed"`
	ToolSkipped  int64            `json:"tool_skipped"`
	DetectorMS   int64            `json:"detector_ms"`
	Replacements map[string]int64 `json:"replacements"`
}

func (s *redactStats) view() RedactionCounts {
	v := RedactionCounts{Records: s.records.Load(), Changed: s.changed.Load(), ToolSkipped: s.toolSkipped.Load(),
		DetectorMS: time.Duration(s.detectorWait.Load()).Milliseconds()}
	s.mu.Lock()
	v.Replacements = maps.Clone(s.kinds)
	s.mu.Unlock()
	if v.Replacements == nil {
		v.Replacements = map[string]int64{}
	}
	return v
}

// networkClock measures wall time with at least one storage request in
// flight; parallel parts count once.
type networkClock struct {
	mu     sync.Mutex
	active int
	since  time.Time
	total  time.Duration
}

func (c *networkClock) begin() func() {
	if c == nil {
		return func() {}
	}
	c.mu.Lock()
	if c.active == 0 {
		c.since = time.Now()
	}
	c.active++
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.active--
		if c.active == 0 {
			c.total += time.Since(c.since)
		}
		c.mu.Unlock()
	}
}

func (c *networkClock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.total
	if c.active > 0 {
		d += time.Since(c.since)
	}
	return d
}

// networkSection marks a storage request of the current upload.
func networkSection(p *uploadProgress) func() {
	if p == nil {
		return func() {}
	}
	return p.network.begin()
}

// UploadRecord describes one finished upload attempt on this node.
type UploadRecord struct {
	FileName   string          `json:"file_name"`
	Bucket     string          `json:"bucket,omitempty"`
	Bytes      int64           `json:"bytes"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMS int64           `json:"duration_ms"`
	NetworkMS  int64           `json:"network_ms"`
	Redacted   bool            `json:"redacted"`
	OK         bool            `json:"ok"`
	Redaction  RedactionCounts `json:"redaction"`
}

// UploadTotals sums this node's finished uploads since it started.
type UploadTotals struct {
	Uploads    int64           `json:"uploads"`
	Failed     int64           `json:"failed"`
	Bytes      int64           `json:"bytes"`
	DurationMS int64           `json:"duration_ms"`
	NetworkMS  int64           `json:"network_ms"`
	Redaction  RedactionCounts `json:"redaction"`
	Since      time.Time       `json:"since"`
}

// UploadStats is the redaction and upload statistics view.
type UploadStats struct {
	Totals UploadTotals   `json:"totals"`
	Recent []UploadRecord `json:"recent"`
}

const recentUploads = 20

// uploadHistory keeps the totals and the latest attempts of this node. It
// lives in the runtime's stats, so it survives pipeline restarts.
type uploadHistory struct {
	once      sync.Once
	since     time.Time
	redaction redactStats // every document redacted on this node, uploads or staged fragments
	mu        sync.Mutex
	totals    UploadTotals
	recent    []UploadRecord // newest first
}

func (h *uploadHistory) start() {
	h.once.Do(func() { h.since = time.Now() })
}

func (h *uploadHistory) record(p *uploadProgress, ok bool) {
	if h == nil || p == nil {
		return
	}
	h.start()
	elapsed := time.Since(p.startedAt)
	r := UploadRecord{FileName: p.FileName, Bucket: p.bucket, Bytes: p.sent.Load(), StartedAt: p.startedAt, DurationMS: elapsed.Milliseconds(),
		NetworkMS: p.network.elapsed().Milliseconds(), Redacted: p.redacted, OK: ok, Redaction: p.redaction.view()}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recent = slices.Insert(h.recent, 0, r)
	if len(h.recent) > recentUploads {
		h.recent = h.recent[:recentUploads]
	}
	if !ok {
		h.totals.Failed++
		return
	}
	h.totals.Uploads++
	h.totals.Bytes += r.Bytes
	h.totals.DurationMS += r.DurationMS
	h.totals.NetworkMS += r.NetworkMS
}

func (h *uploadHistory) view() UploadStats {
	h.start()
	h.mu.Lock()
	v := UploadStats{Totals: h.totals, Recent: slices.Clone(h.recent)}
	h.mu.Unlock()
	v.Totals.Since = h.since
	v.Totals.Redaction = h.redaction.view()
	if v.Recent == nil {
		v.Recent = []UploadRecord{}
	}
	return v
}
