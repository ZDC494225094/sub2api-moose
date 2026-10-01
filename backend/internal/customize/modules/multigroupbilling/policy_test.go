package multigroupbilling

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestNormalizeGroupIDsPreservesPrimaryAndDoesNotMutateInput(t *testing.T) {
	ids := []int64{20, 10, 0, -1, 20, 30}
	original := append([]int64(nil), ids...)
	if got := NormalizeGroupIDs(ptr(int64(10)), ids); !reflect.DeepEqual(got, []int64{10, 20, 30}) {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(ids, original) {
		t.Fatal("input changed")
	}
	if got := NormalizeGroupIDs(ptr(int64(-1)), []int64{0, 3, 3}); !reflect.DeepEqual(got, []int64{3}) {
		t.Fatal(got)
	}
	for _, value := range []string{"", "invalid", "SUBSCRIPTION_FIRST", BalanceFirst} {
		if NormalizePriority(value) != BalanceFirst {
			t.Fatal(value)
		}
	}
	if NormalizePriority(SubscriptionFirst) != SubscriptionFirst {
		t.Fatal("subscription preference lost")
	}
}

func TestSelectionPreservesRoutingAndFallbackContract(t *testing.T) {
	cases := []struct {
		name              string
		ids               []int64
		priority          string
		platform          string
		cached            *float64
		live              *float64
		balanceError      bool
		noUser            bool
		noSubs            bool
		subscriptionError bool
		wantGroup         int64
		wantSub           int
		wantBalanceCalls  int
		wantErr           bool
		inferred          string
	}{
		{name: "no groups", wantGroup: 0},
		{name: "balance only even with zero", ids: []int64{10}, cached: ptr(0.0), live: ptr(0.0), wantGroup: 10, inferred: "anthropic"},
		{name: "subscription only", ids: []int64{20}, live: ptr(50.0), wantGroup: 20, wantSub: 201, inferred: "anthropic"},
		{name: "balance first positive", ids: []int64{20, 10}, live: ptr(1.0), wantGroup: 10, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "subscription first positive", ids: []int64{10, 20}, priority: SubscriptionFirst, live: ptr(1.0), wantGroup: 20, wantSub: 201, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "zero switches to subscription", ids: []int64{10, 20}, live: ptr(0.0), cached: ptr(50.0), wantGroup: 20, wantSub: 201, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "negative switches to subscription", ids: []int64{10, 20}, live: ptr(-1.0), wantGroup: 20, wantSub: 201, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "live error uses positive cache", ids: []int64{10, 20}, balanceError: true, cached: ptr(1.0), wantGroup: 10, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "live error uses empty cache", ids: []int64{10, 20}, balanceError: true, cached: ptr(0.0), wantGroup: 20, wantSub: 201, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "missing user retains balance routing", ids: []int64{10, 20}, wantGroup: 10, inferred: "anthropic"},
		{name: "zero user id avoids balance port", ids: []int64{10, 20}, noUser: true, live: ptr(0.0), wantGroup: 10, inferred: "anthropic"},
		{name: "unusable subscription balance first fallback", ids: []int64{20, 10}, noSubs: true, live: ptr(0.0), wantGroup: 10, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "unusable subscription subscription first fallback", ids: []int64{10, 20}, noSubs: true, priority: SubscriptionFirst, live: ptr(0.0), wantGroup: 20, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "subscription error falls back", ids: []int64{20, 10}, subscriptionError: true, live: ptr(0.0), priority: SubscriptionFirst, wantGroup: 20, wantBalanceCalls: 1, inferred: "anthropic"},
		{name: "inactive is last fallback", ids: []int64{30, 10}, wantGroup: 10, inferred: "anthropic"},
		{name: "all inactive stays host preflight responsibility", ids: []int64{30}, wantGroup: 30, inferred: "anthropic"},
		{name: "unknown group is skipped", ids: []int64{999, 20}, wantGroup: 20, wantSub: 201, inferred: "anthropic"},
		{name: "all missing", ids: []int64{999}, wantErr: true},
		{name: "known protocol excludes mismatches", ids: []int64{40, 10}, platform: "anthropic", wantGroup: 10},
		{name: "all protocol mismatches", ids: []int64{40}, platform: "anthropic", wantErr: true},
		{name: "inference locks first resolved protocol", ids: []int64{40, 10}, wantGroup: 40, inferred: "openai"},
		{name: "inactive protocol locks inference too", ids: []int64{50, 10}, wantGroup: 50, inferred: "openai"},
		{name: "unknown preference remains balance first", ids: []int64{20, 10}, priority: "future-value", live: ptr(1.0), wantGroup: 10, wantBalanceCalls: 1, inferred: "anthropic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, "request")
			calls := 0
			key := Key{UserID: 1, GroupIDs: tc.ids, Priority: tc.priority, Platform: tc.platform, CachedBalance: tc.cached}
			if tc.noUser {
				key.UserID = 0
			}
			host := Host[int64, int]{
				ResolveGroup: func(actual context.Context, id int64) (*Group[int64], error) {
					if actual != ctx {
						t.Fatal("lost context")
					}
					if id == 999 {
						return nil, errors.New("missing")
					}
					platform := "anthropic"
					if id == 40 || id == 50 {
						platform = "openai"
					}
					return &Group[int64]{Value: id, Platform: platform, Active: id != 30 && id != 50, Subscription: id == 20}, nil
				},
				ListSubscriptions: func(actual context.Context, userID int64, group int64) ([]int, error) {
					if actual != ctx || userID != key.UserID || group != 20 {
						t.Fatal("incorrect subscription port arguments")
					}
					if tc.subscriptionError {
						return nil, errors.New("unavailable")
					}
					return []int{201, 202}, nil
				},
			}
			if tc.noSubs {
				host.ListSubscriptions = nil
			}
			if tc.live != nil || tc.balanceError {
				host.Balance = func(actual context.Context, userID int64) (float64, error) {
					calls++
					if actual != ctx || userID != 1 {
						t.Fatal("incorrect balance port arguments")
					}
					if tc.balanceError {
						return 0, errors.New("unavailable")
					}
					return *tc.live, nil
				}
			}
			selected, err := Select(ctx, &key, host)
			if tc.wantErr {
				if !errors.Is(err, ErrNoUsableGroup) || selected != nil {
					t.Fatalf("selected=%+v err=%v", selected, err)
				}
			} else {
				if err != nil || selected == nil {
					t.Fatalf("selected=%+v err=%v", selected, err)
				}
				got := int64(0)
				if selected.Group != nil {
					got = selected.Group.Value
				}
				if got != tc.wantGroup {
					t.Fatalf("group %d want %d", got, tc.wantGroup)
				}
				sub := 0
				if selected.Subscription != nil {
					sub = *selected.Subscription
				}
				if sub != tc.wantSub {
					t.Fatalf("sub %d want %d", sub, tc.wantSub)
				}
			}
			if calls != tc.wantBalanceCalls {
				t.Fatalf("balance calls %d want %d", calls, tc.wantBalanceCalls)
			}
			if key.InferredPlatform != tc.inferred || key.Platform != tc.platform {
				t.Fatalf("platform updates %+v", key)
			}
			if calls > 0 && !tc.balanceError {
				if key.ResolvedBalance == nil || *key.ResolvedBalance != *tc.live {
					t.Fatal("live balance update lost")
				}
			} else if key.ResolvedBalance != nil {
				t.Fatal("unexpected balance update")
			}
		})
	}
}

func TestCandidateResolutionOrderAndOpaqueSubscription(t *testing.T) {
	type entitlement struct {
		ID               int
		UpstreamNewField string
	}
	original := entitlement{ID: 8, UpstreamNewField: "do not project away"}
	var seen []int64
	key := Key{Primary: ptr(int64(2)), GroupIDs: []int64{1, 2, -1, 0, 1}, Platform: "openai"}
	selected, err := Select(context.Background(), &key, Host[int64, entitlement]{
		ResolveGroup: func(_ context.Context, id int64) (*Group[int64], error) {
			seen = append(seen, id)
			return &Group[int64]{Value: id, Platform: "openai", Active: true, Subscription: true}, nil
		},
		ListSubscriptions: func(_ context.Context, _ int64, _ int64) ([]entitlement, error) { return []entitlement{original}, nil },
	})
	if err != nil || selected.Group.Value != 2 || !reflect.DeepEqual(seen, []int64{2, 1}) || *selected.Subscription != original {
		t.Fatalf("selection=%+v seen=%v err=%v", selected, seen, err)
	}
	// Do not leave output fields from a preceding selection on a reused view.
	key.GroupIDs = nil
	key.Primary = nil
	key.InferredPlatform = "stale"
	key.ResolvedBalance = ptr(99.0)
	_, err = Select(context.Background(), &key, Host[int64, entitlement]{})
	if err != nil || key.InferredPlatform != "" || key.ResolvedBalance != nil {
		t.Fatal("stale selection updates retained")
	}
}
