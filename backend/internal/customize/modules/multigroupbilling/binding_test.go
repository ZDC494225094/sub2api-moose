package multigroupbilling

import (
	"reflect"
	"testing"
)

func TestBindingPlanFieldPresence(t *testing.T) {
	old, next, zero := int64(1), int64(3), int64(0)
	protocol := "new"
	current := Binding{Primary: &old, GroupIDs: []int64{1, 2}, Platform: "old"}
	tests := []struct {
		name           string
		patch          BindingPatch
		changed, write bool
		primary        *int64
		ids            []int64
		platform       string
	}{
		{name: "absent"},
		{name: "list without presence ignored", patch: BindingPatch{GroupIDs: []int64{3}}},
		{"platform only", BindingPatch{Platform: &protocol}, true, false, &old, []int64{1, 2}, "new"},
		{"replace primary", BindingPatch{Primary: &next}, true, true, &next, []int64{3}, ""},
		{"explicit clear", BindingPatch{GroupIDsSet: true}, true, true, nil, []int64{}, ""},
		{"replace list", BindingPatch{GroupIDsSet: true, GroupIDs: []int64{3, 3, -1, 2}}, true, true, &next, []int64{3, 2}, ""},
		{"primary wins", BindingPatch{Primary: &next, GroupIDsSet: true, GroupIDs: []int64{2, 3}}, true, true, &next, []int64{3, 2}, ""},
		{"explicit platform", BindingPatch{Primary: &next, Platform: &protocol}, true, true, &next, []int64{3}, "new"},
		{"legacy zero primary", BindingPatch{Primary: &zero, GroupIDs: []int64{3}}, true, true, &zero, []int64{3}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanUpdate(current, tt.patch)
			if got.Changed != tt.changed || got.WriteGroups != tt.write || !reflect.DeepEqual(got.Primary, tt.primary) || !reflect.DeepEqual(got.GroupIDs, tt.ids) || got.Platform != tt.platform {
				t.Fatalf("unexpected plan: %+v", got)
			}
			if got.Primary != nil {
				*got.Primary = 90
			}
			if len(got.GroupIDs) > 0 {
				got.GroupIDs[0] = 90
			}
			if old != 1 || next != 3 || zero != 0 || current.GroupIDs[0] != 1 {
				t.Fatal("plan aliases input")
			}
		})
	}
}

func TestBindingCreatePreservesLegacyPrimary(t *testing.T) {
	zero := int64(0)
	cases := []struct {
		primary *int64
		ids     []int64
		want    *int64
	}{
		{nil, nil, nil}, {nil, []int64{2, 2}, func() *int64 { x := int64(2); return &x }()}, {&zero, []int64{2}, &zero},
	}
	for _, tt := range cases {
		got := PlanCreate(tt.primary, tt.ids, " raw ")
		if !reflect.DeepEqual(got.Primary, tt.want) || got.Platform != " raw " {
			t.Fatalf("unexpected binding %+v", got)
		}
		if got.Primary != nil {
			*got.Primary = 99
		}
		if zero != 0 {
			t.Fatal("aliased primary")
		}
	}
}
