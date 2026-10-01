package multigroupbilling

import (
	"errors"
	"testing"
)

func TestRoutingOwnership(t *testing.T) {
	for _, value := range []string{"", LegacyRouting, NativeRouting, "future-v2", " upstream-v1 "} {
		got, err := RoutingPolicy(value)
		switch value {
		case "", LegacyRouting:
			if err != nil || got != LegacyRouting {
				t.Fatal(got, err)
			}
		case NativeRouting:
			if err != nil || got != NativeRouting {
				t.Fatal(got, err)
			}
		default:
			if !errors.Is(err, ErrUnknownRoutingPolicy) || got != "" {
				t.Fatal(got, err)
			}
		}
	}
}
