// Package subscriptionextensions owns durable issuance semantics, not historical billing.
package subscriptionextensions

import (
	"context"
	"fmt"
)

const (
	ModuleID    = "subscription-extensions"
	Independent = "independent-v1"
	Native      = "upstream-v1"
	SnapshotKey = "custom_subscription_policy"
)

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

// NewPolicy is consulted only at issuance/order/code creation, never fulfillment.
func NewPolicy(ctx context.Context, states StateReader) (string, error) {
	if states == nil {
		return Native, nil
	}
	enabled, err := states.Enabled(ctx, ModuleID)
	if err != nil {
		return "", err
	}
	if enabled {
		return Independent, nil
	}
	return Native, nil
}

// Resolve preserves grants issued before ownership was persisted.
func Resolve(policy string) (string, error) {
	switch policy {
	case "", Independent:
		return Independent, nil
	case Native:
		return Native, nil
	}
	return "", fmt.Errorf("unknown subscription issuance policy %q", policy)
}
func FromSnapshot(snapshot map[string]any) (string, error) {
	value, exists := snapshot[SnapshotKey]
	if !exists {
		return Independent, nil
	}
	policy, ok := value.(string)
	if !ok || policy == "" {
		return "", fmt.Errorf("invalid subscription issuance snapshot")
	}
	return Resolve(policy)
}
func WithSnapshot(snapshot map[string]any, policy string) (map[string]any, error) {
	resolved, err := Resolve(policy)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(snapshot)+1)
	for k, v := range snapshot {
		result[k] = v
	}
	result[SnapshotKey] = resolved
	return result, nil
}
