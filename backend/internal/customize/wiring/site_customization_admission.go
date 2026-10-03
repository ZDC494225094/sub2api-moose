package wiring

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/sitecustomization"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const siteCustomizationModuleID = "site-customization"

type siteCustomizationStateReader interface {
	Enabled(context.Context, string) (bool, error)
}

// siteCustomizationAdmission implements footer-link settings filtering.
type siteCustomizationAdmission struct {
	states siteCustomizationStateReader
	lookup func(string) (customize.Manifest, bool)
}

// ShouldExposeCustomUI checks if custom UI elements should be exposed in public settings.
func (a *siteCustomizationAdmission) ShouldExposeCustomUI(ctx context.Context) bool {
	if a == nil || a.lookup == nil || a.states == nil {
		return false
	}
	manifest, exists := a.lookup(siteCustomizationModuleID)
	if !exists || manifest.ID != siteCustomizationModuleID || !manifest.Managed {
		return false
	}
	enabled, _ := a.states.Enabled(ctx, siteCustomizationModuleID)
	return enabled
}

// ProvideSiteCustomizationAdmission returns a combined admission implementation
// for site-customization extension checks.
func ProvideSiteCustomizationAdmission(settings service.SettingRepository) interface{} {
	impl := &siteCustomizationAdmission{
		states: customize.NewManager(siteCustomizationSettingReader{settings}),
		lookup: customize.Lookup,
	}
	return impl
}

// siteCustomizationSettingReader wraps the setting repository for extension state checks.
type siteCustomizationSettingReader struct{ service.SettingRepository }

func (r siteCustomizationSettingReader) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if r.SettingRepository == nil {
		return nil, sitecustomization.ErrExtensionDisabled
	}
	return r.SettingRepository.GetMultiple(ctx, keys)
}
