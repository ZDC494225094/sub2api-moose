package accesspolicy

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEvaluateRegistrationEmail(t *testing.T) {
	cases := []struct {
		name, email string
		whitelist   []string
		quota       bool
		want        EmailAdmission
	}{
		{"empty policy preserves host validation", "not-an-email", nil, false, EmailAdmission{Allowed: true}},
		{"allowlisted", "user@example.com.", []string{"@example.com"}, false, EmailAdmission{Allowed: true}},
		{"wildcard", "user@team.example.com", []string{"*.example.com"}, true, EmailAdmission{Allowed: true}},
		{"strict denies other domain", "user@other.com", []string{"@example.com"}, false, EmailAdmission{}},
		{"quota canonicalizes subdomain", "user@team.other.co.uk", []string{"@example.com"}, true, EmailAdmission{true, "other.co.uk"}},
		{"quota malformed denies", "not-an-email", []string{"@example.com"}, true, EmailAdmission{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, EvaluateRegistrationEmail(tc.email, tc.whitelist, tc.quota))
		})
	}
}
func TestMainlandChinaDecisionPreservesPageOnlyPolicy(t *testing.T) {
	require.True(t, MainlandChinaAccessRestricted(true, "223.5.5.5"))
	require.True(t, MainlandChinaAccessRestricted(true, "::ffff:223.5.5.5"))
	for _, ip := range []string{"8.8.8.8", "127.0.0.1", "invalid", ""} {
		require.False(t, MainlandChinaAccessRestricted(true, ip))
	}
	require.False(t, MainlandChinaAccessRestricted(false, "223.5.5.5"))
}
