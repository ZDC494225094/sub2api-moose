package multigroupbilling

import (
	"context"
	"errors"
	"slices"
)

// ModuleID is the catalog entry that owns new multi-group key configuration.
const ModuleID = "multi-group-billing"

var (
	// ErrExtensionDisabled: the request needs extension routing while new
	// extension configuration is not admitted. The host maps it to its switch error.
	ErrExtensionDisabled = errors.New("multi-group key configuration is disabled")
	// ErrNativeKeyExtension: upstream-owned keys never acquire extension routing;
	// ownership is immutable, so a new key is required.
	ErrNativeKeyExtension = errors.New("upstream-routed API key cannot use multi-group configuration")
)

// EnabledFunc reads the shared switch. It is only invoked when the answer can
// change the outcome, so upstream-shaped edits do not depend on switch storage.
type EnabledFunc func(context.Context) (bool, error)

// ExtensionConfig reports whether a binding cannot be expressed by upstream's
// single-group key: two or more candidate groups, or subscription-first billing.
// Platform is not decisive; the host validates it against the chosen group.
func ExtensionConfig(groupIDs []int64, priority string) bool {
	return len(NormalizeGroupIDs(nil, groupIDs)) >= 2 || NormalizePriority(priority) == SubscriptionFirst
}

// DecideCreate chooses the immutable routing ownership of a new key. The switch
// is always consulted because the choice is durable: an unreadable state fails
// rather than guessing. Enabled keeps the historical multi-group behavior.
func DecideCreate(ctx context.Context, enabled EnabledFunc, groupIDs []int64, priority string) (string, error) {
	on := false
	if enabled != nil {
		var err error
		if on, err = enabled(ctx); err != nil {
			return "", err
		}
	}
	if on {
		return LegacyRouting, nil
	}
	if ExtensionConfig(groupIDs, priority) {
		return "", ErrExtensionDisabled
	}
	return NativeRouting, nil
}

// UpdateRequest is the post-plan view of a key edit. NextPriority is nil when
// the request does not carry a billing preference.
type UpdateRequest struct {
	Policy          string
	Current         Binding
	CurrentPriority string
	Plan            BindingPlan
	NextPriority    *string
}

// CheckUpdate admits an edit by ownership. Native keys stay upstream-shaped.
// Legacy keys keep their existing configuration while disabled: any subset of
// their groups, but no group they did not already have and no new preference.
func CheckUpdate(ctx context.Context, enabled EnabledFunc, req UpdateRequest) error {
	policy, err := RoutingPolicy(req.Policy)
	if err != nil {
		return err
	}
	groups := NormalizeGroupIDs(req.Current.Primary, req.Current.GroupIDs)
	if req.Plan.Changed && req.Plan.WriteGroups {
		groups = NormalizeGroupIDs(req.Plan.Primary, req.Plan.GroupIDs)
	}
	priority := NormalizePriority(req.CurrentPriority)
	if req.NextPriority != nil {
		priority = NormalizePriority(*req.NextPriority)
	}
	if policy == NativeRouting {
		if ExtensionConfig(groups, priority) {
			return ErrNativeKeyExtension
		}
		return nil
	}
	if !introducesExtension(req, groups, priority) {
		return nil
	}
	on := false
	if enabled != nil {
		if on, err = enabled(ctx); err != nil {
			return err
		}
	}
	if !on {
		return ErrExtensionDisabled
	}
	return nil
}

func introducesExtension(req UpdateRequest, groups []int64, priority string) bool {
	// Any subset of the key's existing groups, in any order, keeps its existing
	// configuration. Stored lists need not start with the primary, so order
	// cannot be a reliable signal; only groups the key never had are new.
	current := NormalizeGroupIDs(req.Current.Primary, req.Current.GroupIDs)
	if len(groups) >= 2 {
		for _, id := range groups {
			if !slices.Contains(current, id) {
				return true
			}
		}
	}
	// Returning to balance-first moves toward upstream and is always allowed.
	return priority == SubscriptionFirst && priority != NormalizePriority(req.CurrentPriority)
}
