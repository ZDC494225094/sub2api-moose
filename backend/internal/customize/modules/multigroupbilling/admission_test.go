package multigroupbilling

import (
	"context"
	"errors"
	"testing"
)

type switchProbe struct {
	on    bool
	err   error
	calls int
}

func (p *switchProbe) read(context.Context) (bool, error) { p.calls++; return p.on, p.err }

func TestExtensionConfigIsSemanticNotFieldPresence(t *testing.T) {
	cases := []struct {
		ids      []int64
		priority string
		want     bool
	}{
		{nil, "", false},
		{[]int64{3}, BalanceFirst, false},
		{[]int64{3, 3, 0, -1}, "", false}, // duplicates are not a second group
		{[]int64{3, 4}, "", true},
		{nil, SubscriptionFirst, true},
		{[]int64{3}, "future-value", false}, // unknown preference normalizes to balance-first
	}
	for _, tc := range cases {
		if got := ExtensionConfig(tc.ids, tc.priority); got != tc.want {
			t.Fatalf("ExtensionConfig(%v,%q)=%v", tc.ids, tc.priority, got)
		}
	}
}

func TestDecideCreateOwnership(t *testing.T) {
	failure := errors.New("settings unavailable")
	cases := []struct {
		name     string
		probe    *switchProbe
		ids      []int64
		priority string
		want     string
		wantErr  error
	}{
		{"enabled single", &switchProbe{on: true}, []int64{1}, "", LegacyRouting, nil},
		{"enabled multi", &switchProbe{on: true}, []int64{1, 2}, SubscriptionFirst, LegacyRouting, nil},
		{"disabled upstream shape", &switchProbe{}, []int64{1}, BalanceFirst, NativeRouting, nil},
		{"disabled no group", &switchProbe{}, nil, "", NativeRouting, nil},
		{"disabled multi rejected", &switchProbe{}, []int64{1, 2}, "", "", ErrExtensionDisabled},
		{"disabled subscription first rejected", &switchProbe{}, []int64{1}, SubscriptionFirst, "", ErrExtensionDisabled},
		{"unreadable never guesses durable ownership", &switchProbe{on: true, err: failure}, []int64{1}, "", "", failure},
		{"no host means upstream", nil, []int64{1}, "", NativeRouting, nil},
		{"no host rejects multi", nil, []int64{1, 2}, "", "", ErrExtensionDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var read EnabledFunc
			if tc.probe != nil {
				read = tc.probe.read
			}
			got, err := DecideCreate(context.Background(), read, tc.ids, tc.priority)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("got %q err %v", got, err)
			}
			if tc.probe != nil && tc.probe.calls != 1 {
				t.Fatalf("switch must be read exactly once, got %d", tc.probe.calls)
			}
		})
	}
}

func TestCheckUpdateByOwnership(t *testing.T) {
	one, two, three := int64(1), int64(2), int64(3)
	legacyMulti := Binding{Primary: &one, GroupIDs: []int64{1, 2}}
	single := Binding{Primary: &one, GroupIDs: []int64{1}}
	sub, bal := SubscriptionFirst, BalanceFirst
	planFor := func(current Binding, patch BindingPatch) BindingPlan { return PlanUpdate(current, patch) }
	failure := errors.New("settings unavailable")
	cases := []struct {
		name      string
		policy    string
		current   Binding
		priority  string
		patch     BindingPatch
		next      *string
		probe     switchProbe
		wantErr   error
		wantReads int
	}{
		{name: "native single group change", policy: NativeRouting, current: single, patch: BindingPatch{Primary: &two}},
		{name: "native clear", policy: NativeRouting, current: single, patch: BindingPatch{GroupIDsSet: true}},
		{name: "native balance-first echo", policy: NativeRouting, current: single, next: &bal},
		{name: "native multi rejected even when enabled", policy: NativeRouting, current: single, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 2}}, probe: switchProbe{on: true}, wantErr: ErrNativeKeyExtension},
		{name: "native subscription-first rejected", policy: NativeRouting, current: single, next: &sub, probe: switchProbe{on: true}, wantErr: ErrNativeKeyExtension},
		{name: "legacy unchanged multi echoed while disabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 2}}},
		{name: "legacy empty policy is legacy", policy: "", current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 2}}},
		{name: "legacy reduce to single while disabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{2}}},
		{name: "legacy keep existing subscription-first", policy: LegacyRouting, current: legacyMulti, priority: sub, next: &sub},
		{name: "legacy return to balance-first while disabled", policy: LegacyRouting, current: legacyMulti, priority: sub, next: &bal},
		{name: "legacy add group while disabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 2, 3}}, wantErr: ErrExtensionDisabled, wantReads: 1},
		{name: "legacy reorder of existing groups while disabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{2, 1}}},
		{name: "legacy stored list without primary first", policy: LegacyRouting, current: Binding{Primary: &two, GroupIDs: []int64{1, 2}}, patch: BindingPatch{Primary: &one, GroupIDsSet: true, GroupIDs: []int64{1, 2}}},
		{name: "legacy swap one group for a new one while disabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 3}}, wantErr: ErrExtensionDisabled, wantReads: 1},
		{name: "legacy single to multi while disabled", policy: LegacyRouting, current: single, patch: BindingPatch{GroupIDsSet: true, GroupIDs: []int64{1, 3}}, wantErr: ErrExtensionDisabled, wantReads: 1},
		{name: "legacy new subscription-first while disabled", policy: LegacyRouting, current: single, next: &sub, wantErr: ErrExtensionDisabled, wantReads: 1},
		{name: "legacy add group while enabled", policy: LegacyRouting, current: legacyMulti, patch: BindingPatch{Primary: &three, GroupIDsSet: true, GroupIDs: []int64{1, 2}}, probe: switchProbe{on: true}, wantReads: 1},
		{name: "legacy unreadable state", policy: LegacyRouting, current: single, next: &sub, probe: switchProbe{on: true, err: failure}, wantErr: failure, wantReads: 1},
		{name: "unknown policy", policy: "future-v2", current: single, wantErr: ErrUnknownRoutingPolicy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := tc.probe
			err := CheckUpdate(context.Background(), probe.read, UpdateRequest{
				Policy: tc.policy, Current: tc.current, CurrentPriority: tc.priority,
				Plan: planFor(tc.current, tc.patch), NextPriority: tc.next,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want %v", err, tc.wantErr)
			}
			if probe.calls != tc.wantReads {
				t.Fatalf("switch reads=%d want %d (upstream-shaped edits must not depend on switch storage)", probe.calls, tc.wantReads)
			}
		})
	}
	if err := CheckUpdate(context.Background(), nil, UpdateRequest{Policy: LegacyRouting, Current: single, NextPriority: &sub}); !errors.Is(err, ErrExtensionDisabled) {
		t.Fatalf("missing host must not admit new extension configuration: %v", err)
	}
}
