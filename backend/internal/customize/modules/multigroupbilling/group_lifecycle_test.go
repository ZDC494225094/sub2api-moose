package multigroupbilling

import (
	"reflect"
	"testing"
)

func TestGroupLifecycle(t *testing.T) {
	primary := int64(1)
	ids := []int64{2, 1, 3, 2, 0, -1}
	if !ContainsGroupID(&primary, ids, 1) || !ContainsGroupID(&primary, ids, 3) || ContainsGroupID(&primary, ids, 0) || ContainsGroupID(nil, nil, 1) {
		t.Fatal("membership")
	}
	cases := []struct {
		name      string
		got, want []int64
	}{
		{"remove primary", RemoveGroupID(&primary, ids, 1), []int64{2, 3}},
		{"remove secondary", RemoveGroupID(&primary, ids, 2), []int64{1, 3}},
		{"absent", RemoveGroupID(&primary, ids, 9), []int64{1, 2, 3}},
		{"empty", RemoveGroupID(&primary, nil, 1), nil},
		{"replace primary", ReplaceGroupID(&primary, ids, 1, 2), []int64{2, 3}},
		{"replace secondary", ReplaceGroupID(&primary, ids, 2, 4), []int64{1, 4, 3}},
		{"same", ReplaceGroupID(&primary, ids, 2, 2), []int64{1, 2, 3}},
		{"nonpositive", ReplaceGroupID(&primary, ids, 1, 0), []int64{2, 3}},
		{"empty replace", ReplaceGroupID(nil, nil, 1, 2), []int64{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("got %v want %v", tt.got, tt.want)
			}
			if len(tt.got) > 0 {
				tt.got[0] = 99
			}
			if primary != 1 || !reflect.DeepEqual(ids, []int64{2, 1, 3, 2, 0, -1}) {
				t.Fatal("mutated input")
			}
		})
	}
}
