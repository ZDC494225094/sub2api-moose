package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/adminefficiency"

const AccountUpstreamGroupMaxLength = adminefficiency.AccountUpstreamGroupMaxLength

// NormalizeAccountUpstreamGroup is the compatibility seam for all account write paths.
func NormalizeAccountUpstreamGroup(value string) (string, error) {
	return adminefficiency.NormalizeAccountUpstreamGroup(value)
}
