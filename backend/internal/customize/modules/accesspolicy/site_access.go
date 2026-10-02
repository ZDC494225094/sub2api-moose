package accesspolicy

import "github.com/Wei-Shaw/sub2api/internal/pkg/geo"

// MainlandChinaAccessRestricted is a site-navigation decision, NOT an API
// authorization rule. Keep gateway access and host authentication unchanged.
func MainlandChinaAccessRestricted(enabled bool, remoteIP string) bool {
	return enabled && geo.IsMainlandChina(remoteIP)
}
