package wiring

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

type footerSettingsRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r footerSettingsRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := map[string]string{}
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}
func TestFooterAdmissionFiltersOnlyForkSettings(t *testing.T) {
	for _, flag := range []string{"missing", "false", "malformed", "true"} {
		t.Run(flag, func(t *testing.T) {
			repo := footerSettingsRepo{values: map[string]string{"footer_friend_links": `[{"label":"friend","url":"https://example.org"}]`, "custom_menu_items": "[]", "custom_endpoints": "[]"}}
			if flag != "missing" {
				repo.values["custom_extensions.site-customization.enabled"] = flag
			}
			svc := service.NewSettingService(repo, nil)
			svc.SetSiteCustomizationSettingsAdmission(ProvideSiteCustomizationAdmission(repo))
			settings, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, "[]", string(settings.CustomMenuItems))
			require.Equal(t, "[]", string(settings.CustomEndpoints))
			if flag == "true" {
				require.NotEmpty(t, settings.FooterFriendLinks)
			} else {
				require.Empty(t, settings.FooterFriendLinks)
			}
		})
	}
}
