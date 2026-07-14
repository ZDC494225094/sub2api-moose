package service

import (
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const AccountUpstreamGroupMaxLength = 100

// NormalizeAccountUpstreamGroup trims an explicit upstream group label and
// enforces the same character limit as the database column.
func NormalizeAccountUpstreamGroup(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if utf8.RuneCountInString(normalized) > AccountUpstreamGroupMaxLength {
		return "", infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_TOO_LONG", "upstream_group must be at most 100 characters")
	}
	return normalized, nil
}
