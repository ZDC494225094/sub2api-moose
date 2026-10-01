// Package multigroupbilling owns custom key routing and billing preference rules.
// It has no dependency on host services, generated entities, or transport code.
package multigroupbilling

import (
	"context"
	"errors"
)

const (
	BalanceFirst      = "balance_first"
	SubscriptionFirst = "subscription_first"
)

var ErrNoUsableGroup = errors.New("no usable API key group is available")

func NormalizePriority(priority string) string {
	if priority == SubscriptionFirst {
		return SubscriptionFirst
	}
	return BalanceFirst
}

func NormalizeGroupIDs(primary *int64, ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids)+1)
	out := make([]int64, 0, len(ids)+1)
	if primary != nil && *primary > 0 {
		seen[*primary] = struct{}{}
		out = append(out, *primary)
	}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Key is a request-local policy view, never a persisted or shared auth-cache row.
// The host supplies normalized protocol names; protocol ownership stays upstream.
// InferredPlatform and ResolvedBalance are explicit updates for the host to apply,
// including when selection fails. Ordinary selection does not rewrite key data.
type Key struct {
	UserID           int64
	Primary          *int64
	GroupIDs         []int64
	Platform         string
	Priority         string
	CachedBalance    *float64
	InferredPlatform string
	ResolvedBalance  *float64
}

// Group preserves the actual host value rather than cloning its evolving DTO.
type Group[G any] struct {
	Value        G
	Platform     string
	Active       bool
	Subscription bool
}

type Host[G, S any] struct {
	ResolveGroup      func(context.Context, int64) (*Group[G], error)
	ListSubscriptions func(context.Context, int64, G) ([]S, error)
	Balance           func(context.Context, int64) (float64, error)
}

type Selection[G, S any] struct {
	Group        *Group[G]
	Subscription *S
}

// Select preserves the historical custom policy: prefer usable candidates,
// then let host preflight produce the original disabled/quota/subscription errors.
// It does not authorize a group or perform any debit or subscription consumption.
func Select[G, S any](ctx context.Context, key *Key, host Host[G, S]) (*Selection[G, S], error) {
	key.InferredPlatform = ""
	key.ResolvedBalance = nil
	ids := NormalizeGroupIDs(key.Primary, key.GroupIDs)
	if len(ids) == 0 {
		return &Selection[G, S]{}, nil
	}
	var balanceCandidates, subscriptionCandidates []Selection[G, S]
	var balanceFallbacks, subscriptionFallbacks, unavailableFallbacks []Selection[G, S]
	platform := key.Platform
	for _, id := range ids {
		group, err := host.ResolveGroup(ctx, id)
		if err != nil || group == nil {
			continue
		}
		if platform == "" {
			platform = group.Platform
			key.InferredPlatform = platform
		}
		if group.Platform != platform {
			continue
		}
		candidate := Selection[G, S]{Group: group}
		if !group.Active {
			unavailableFallbacks = append(unavailableFallbacks, candidate)
			continue
		}
		if group.Subscription {
			subscriptionFallbacks = append(subscriptionFallbacks, candidate)
			if host.ListSubscriptions == nil {
				continue
			}
			subs, err := host.ListSubscriptions(ctx, key.UserID, group.Value)
			if err != nil || len(subs) == 0 {
				continue
			}
			sub := subs[0]
			candidate.Subscription = &sub
			subscriptionCandidates = append(subscriptionCandidates, candidate)
			continue
		}
		balanceFallbacks = append(balanceFallbacks, candidate)
	}
	if len(balanceFallbacks) > 0 {
		// A live balance read is needed only when it can change subscription routing.
		if len(subscriptionFallbacks) == 0 || hasPositiveBalance(ctx, key, host.Balance) {
			balanceCandidates = append(balanceCandidates, balanceFallbacks...)
		}
	}
	ordered := prioritize(key.Priority, balanceCandidates, subscriptionCandidates)
	if len(ordered) == 0 {
		ordered = prioritize(key.Priority, balanceFallbacks, subscriptionFallbacks)
	}
	if len(ordered) == 0 {
		ordered = unavailableFallbacks
	}
	if len(ordered) == 0 {
		return nil, ErrNoUsableGroup
	}
	return &ordered[0], nil
}

func prioritize[G, S any](priority string, balance, subscription []Selection[G, S]) []Selection[G, S] {
	if NormalizePriority(priority) == SubscriptionFirst {
		return append(subscription, balance...)
	}
	return append(balance, subscription...)
}

func hasPositiveBalance(ctx context.Context, key *Key, resolve func(context.Context, int64) (float64, error)) bool {
	if resolve != nil && key.UserID > 0 {
		if balance, err := resolve(ctx, key.UserID); err == nil {
			key.ResolvedBalance = &balance
			return balance > 0
		}
	}
	// A missing user is not permission to bill: the host's original authentication
	// and billing checks still run. Preserve the previous routing fallback only.
	return key.CachedBalance == nil || *key.CachedBalance > 0
}
