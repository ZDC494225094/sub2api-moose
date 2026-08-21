// Package geo classifies public network addresses using bundled allocation data.
package geo

import (
	"embed"
	"net/netip"
	"strings"
	"sync"
)

// mainlandChinaCIDRs is sourced from gaoyifan/china-operator-ip's
// all_cn list. It is bundled with the binary so page visits never disclose an
// address to an external geolocation provider.
//
//go:embed china_ipv4.txt china_ipv6.txt
var mainlandChinaCIDRs embed.FS

var (
	mainlandChinaPrefixesOnce sync.Once
	mainlandChinaPrefixes     []netip.Prefix
)

func loadMainlandChinaPrefixes() {
	for _, filename := range []string{"china_ipv4.txt", "china_ipv6.txt"} {
		contents, err := mainlandChinaCIDRs.ReadFile(filename)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(contents), "\n") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(line))
			if err == nil {
				mainlandChinaPrefixes = append(mainlandChinaPrefixes, prefix)
			}
		}
	}
}

// IsMainlandChina reports whether an address belongs to the bundled
// mainland-China allocation list. Private and invalid addresses do not match;
// IPv4-mapped IPv6 addresses are normalized before lookup.
func IsMainlandChina(value string) bool {
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	address = address.Unmap()

	mainlandChinaPrefixesOnce.Do(loadMainlandChinaPrefixes)
	for _, prefix := range mainlandChinaPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
