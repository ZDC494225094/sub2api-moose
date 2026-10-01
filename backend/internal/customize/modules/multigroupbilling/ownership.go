package multigroupbilling

import "errors"

// Routing ownership belongs to the key, not the current global toggle.
const (
	LegacyRouting = "multigroup-v1"
	NativeRouting = "upstream-v1"
)

var ErrUnknownRoutingPolicy = errors.New("unknown API key routing policy")

// Empty is the pre-ownership representation used by old host adapters/tests.
// Never silently reinterpret an unknown future policy as either native or legacy.
func RoutingPolicy(value string) (string, error) {
	switch value {
	case "", LegacyRouting:
		return LegacyRouting, nil
	case NativeRouting:
		return NativeRouting, nil
	default:
		return "", ErrUnknownRoutingPolicy
	}
}
