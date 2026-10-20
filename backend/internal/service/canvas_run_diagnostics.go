package service

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"
)

type CanvasRunDiagnostics struct {
	Phase          string
	UpstreamTaskID string
	Error          string
}

type CanvasRunDiagnosticsRepository interface {
	UpdateCanvasRunDiagnostics(context.Context, string, CanvasRunDiagnostics) error
}

type CanvasRunSettlementRepository interface {
	FinishCanvasChargeWithDiagnostics(context.Context, string, bool, CanvasRunDiagnostics) (*CanvasCharge, error)
}

func (s *CanvasBillingService) finishWithDiagnostics(id string, success bool, diagnostic CanvasRunDiagnostics) (*CanvasCharge, error) {
	diagnostic.Error = sanitizeCanvasRunError(diagnostic.Error)
	repo, ok := s.repo.(CanvasRunSettlementRepository)
	if !ok {
		return s.finish(id, success)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	charge, err := repo.FinishCanvasChargeWithDiagnostics(ctx, id, success, diagnostic)
	if err == nil {
		s.invalidate(ctx, charge)
	}
	return charge, err
}

type CanvasRunRecord struct {
	ID             string    `json:"id"`
	UserID         int64     `json:"user_id"`
	UserEmail      string    `json:"user_email"`
	Mode           string    `json:"mode"`
	Model          string    `json:"model"`
	Status         string    `json:"status"`
	Phase          string    `json:"phase"`
	UpstreamTaskID string    `json:"upstream_task_id"`
	Error          string    `json:"error"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CanvasRunPage struct {
	Items []CanvasRunRecord `json:"items"`
	Total int64             `json:"total"`
}

type CanvasRunLister interface {
	ListCanvasRuns(context.Context, int, int, string, string) (*CanvasRunPage, error)
}

var (
	canvasDiagnosticURL        = regexp.MustCompile(`(?i)(?:https?://|data:|blob:)[^\s<>"']+`)
	canvasDiagnosticSecret     = regexp.MustCompile(`(?i)\b(?:bearer\s+|sk-)[a-z0-9_./+\-=]+`)
	canvasDiagnosticCredential = regexp.MustCompile(`(?i)(["']?(?:authorization|api[_-]?key|access[_-]?key[_-]?id|secret[_-]?access[_-]?key|token|signature)["']?\s*[:=]\s*)[^\s,;]+`)
)

// Provider messages and transport errors can echo reference URLs or credentials.
func sanitizeCanvasRunError(message string) string {
	message = canvasDiagnosticURL.ReplaceAllString(message, "[media/url redacted]")
	message = canvasDiagnosticSecret.ReplaceAllString(message, "[credential redacted]")
	message = canvasDiagnosticCredential.ReplaceAllString(message, "$1[redacted]")
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > 2000 {
		message = string(runes[:2000]) + "..."
	}
	return message
}

func (s *PlaygroundRunService) writeCanvasDiagnostics(chargeID string, diagnostic CanvasRunDiagnostics) error {
	if s.canvasBilling == nil || chargeID == "" {
		return nil
	}
	repo, ok := s.canvasBilling.repo.(CanvasRunDiagnosticsRepository)
	if !ok {
		return nil
	}
	diagnostic.Error = sanitizeCanvasRunError(diagnostic.Error)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return repo.UpdateCanvasRunDiagnostics(ctx, chargeID, diagnostic)
}

func (s *PlaygroundRunService) setRunPhase(key, phase, upstreamTaskID string) {
	var chargeID string
	var diagnostic CanvasRunDiagnostics
	s.update(key, func(run *PlaygroundRun) {
		if isTerminalPlaygroundRunStatus(run.Status) {
			return
		}
		run.Phase = phase
		if upstreamTaskID != "" {
			id := []rune(sanitizeCanvasRunError(upstreamTaskID))
			if len(id) > 512 {
				id = id[:512]
			}
			run.UpstreamTaskID = string(id)
		}
		run.UpdatedAt = time.Now()
		chargeID = run.chargeID
		diagnostic = CanvasRunDiagnostics{Phase: run.Phase, UpstreamTaskID: run.UpstreamTaskID}
	})
	if err := s.writeCanvasDiagnostics(chargeID, diagnostic); err != nil {
		log.Printf("canvas diagnostics write failed run=%s phase=%s: %s", key, phase, sanitizeCanvasRunError(err.Error()))
	}
}

func (s *PlaygroundRunService) ListCanvasRuns(ctx context.Context, page, size int, search, status string) (*CanvasRunPage, error) {
	if s == nil || s.canvasBilling == nil {
		return nil, errors.New("canvas billing service is unavailable")
	}
	repo, ok := s.canvasBilling.repo.(CanvasRunLister)
	if !ok {
		return nil, errors.New("canvas task listing is unavailable")
	}
	return repo.ListCanvasRuns(ctx, page, size, search, status)
}
