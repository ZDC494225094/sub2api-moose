package iqmonitor

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// GroupOption is a host group as the target editor and public cards see it.
type GroupOption struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Platform       string  `json:"platform"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Status         string  `json:"status"`
}

// KeyOption is redacted API key metadata; key material never leaves the server.
type KeyOption struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	GroupIDs []int64 `json:"group_ids"`
	Status   string  `json:"status"`
}

type UserOption struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type BotOption struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Targets  int    `json:"targets"`
}

// EffortPreview is the value the gateway forwards for a requested effort.
type EffortPreview struct {
	Sent   string `json:"sent"`
	Denied bool   `json:"denied"`
	Note   string `json:"note,omitempty"`
}

// UnitCost is the observed average price per token of a key+model, taken
// from the probe account's own usage logs (actual charged cost).
type UnitCost struct {
	InputPerToken  float64
	OutputPerToken float64
	Samples        int64
}

// Host are the ports the wiring layer implements on the host database and
// services. Every function is required unless noted.
type Host struct {
	// APIKey returns the key material of an active key owned by userID.
	APIKey func(ctx context.Context, keyID, userID int64) (string, error)
	Groups func(ctx context.Context) ([]GroupOption, error)
	Keys   func(ctx context.Context, userID int64) ([]KeyOption, error)
	// Models lists the models a key can call (GET /v1/models through the
	// gateway); optional, the editor then takes free text only.
	Models func(ctx context.Context, keyID, userID int64) ([]string, error)
	Users  func(ctx context.Context, query string, ids []int64) ([]UserOption, error)
	// Effort previews the forwarded reasoning effort for a group and model.
	Effort func(ctx context.Context, groupID int64, model, effort string) EffortPreview
	// Bots and Notify are optional; without them notifications are off.
	Bots   func(ctx context.Context) ([]BotOption, error)
	Notify func(ctx context.Context, botIDs []int64, text string) error
	// Cost is optional; without it the estimate shows tokens only.
	Cost func(ctx context.Context, keyID int64, model string, since time.Time) (UnitCost, error)
}

// EffortOption is one selectable reasoning level for a model.
type EffortOption struct {
	Value   string        `json:"value"`
	Preview EffortPreview `json:"preview"`
}

var (
	geminiEfforts = []string{"minimal", "low", "medium", "high"}
	otherEfforts  = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
)

// EffortLevels lists the levels a model actually supports: Claude by its
// family catalog (low..max), Gemini up to high, every other provider the
// full OpenAI-style scale sent as is.
func EffortLevels(platform, model string) []string {
	name := strings.ToLower(model)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	isClaude := platform == "anthropic" || strings.HasPrefix(name, "claude")
	isGemini := platform == "gemini" || strings.HasPrefix(name, "gemini")
	switch {
	case isClaude:
		if levels := claude.EffortLevelsForModel(model); len(levels) > 0 {
			return levels
		}
		return []string{"low", "medium", "high", "max"}
	case isGemini:
		return append([]string(nil), geminiEfforts...)
	}
	return append([]string(nil), otherEfforts...)
}

func validEffort(platform, model, effort string) bool {
	if effort == "" {
		return true
	}
	for _, v := range EffortLevels(platform, model) {
		if v == effort {
			return true
		}
	}
	return false
}
