package iqmonitor

import "time"

// Stat is a pass-rate sample over a window.
type Stat struct {
	Passed int     `json:"passed"`
	Total  int     `json:"total"`
	Rate   float64 `json:"rate"`
}

func (s *Stat) add(status string) {
	if !counted(status) {
		return
	}
	s.Total++
	if passed(status) {
		s.Passed++
	}
	s.Rate = float64(s.Passed) * 100 / float64(s.Total)
}

type kindStats map[string]*Stat

// statsSince aggregates finished results created at or after since, per
// history key and kind.
func statsSince(results []Result, since time.Time) map[historyKey]kindStats {
	out := map[historyKey]kindStats{}
	for _, r := range results {
		if r.CreatedAt.Before(since) {
			continue
		}
		k := historyKey{GroupID: r.GroupID, Model: r.Model, Effort: r.ReasoningEffort}
		if out[k] == nil {
			out[k] = kindStats{}
		}
		if out[k][r.Kind] == nil {
			out[k][r.Kind] = &Stat{}
		}
		out[k][r.Kind].add(r.Status)
	}
	return out
}

func (k kindStats) get(kind string) Stat {
	if k == nil || k[kind] == nil {
		return Stat{}
	}
	return *k[kind]
}

// healthOf is the model status: the primary test's 24h pass rate.
func healthOf(cfg Config, stats kindStats) (string, Stat) {
	s := stats.get(cfg.PrimaryKind())
	return cfg.Health(s.Rate, s.Total), s
}
