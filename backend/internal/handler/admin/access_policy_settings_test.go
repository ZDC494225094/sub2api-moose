package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccessPolicySettingsHTTPAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, state := range []string{"true", "false", "missing", "malformed"} {
		for _, field := range []string{"registration_proof_enabled", "registration_proof_difficulty", "registration_email_domain_quota_enabled", "mainland_china_access_restriction_enabled"} {
			t.Run(state+"/"+field, func(t *testing.T) {
				repo := &settingHandlerRepoStub{values: map[string]string{service.SettingKeyRegistrationProofEnabled: "true"}}
				if state != "missing" {
					repo.values[customize.Key("access-policy")] = state
				}
				h := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), nil, nil, nil, nil, nil, nil)
				put := func(payload map[string]any) *httptest.ResponseRecorder {
					body, err := json.Marshal(payload)
					require.NoError(t, err)
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(body))
					c.Request.Header.Set("Content-Type", "application/json")
					h.UpdateSettings(c)
					return rec
				}
				value := any(true)
				if field == "registration_proof_enabled" {
					value = false
				}
				if field == "registration_proof_difficulty" {
					value = 20
				}
				rec := put(map[string]any{field: value})
				want := http.StatusForbidden
				if state == "true" {
					want = http.StatusOK
				}
				if state == "malformed" {
					want = http.StatusServiceUnavailable
				}
				require.Equal(t, want, rec.Code, rec.Body.String())
				if state != "true" {
					require.Empty(t, repo.lastUpdates)
					require.Equal(t, "true", repo.values[service.SettingKeyRegistrationProofEnabled])
				}
				repo.values[service.SettingKeyRegistrationProofEnabled] = "true"
				rec = put(map[string]any{"site_name": "native save", "registration_email_suffix_whitelist": []string{"@example.com"}})
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Equal(t, "true", repo.values[service.SettingKeyRegistrationProofEnabled])
				require.Equal(t, "native save", repo.values[service.SettingKeySiteName])
			})
		}
	}
}
