//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAdminEfficiencyShadowInheritanceCannotBypassAdmission(t *testing.T) {
	ctx := context.Background()
	repo := newSparkShadowRepoStub()
	parent := &Account{Name: "parent", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, UpstreamGroup: "historical", GroupIDs: []int64{7}, Credentials: map[string]any{"chatgpt_account_id": "org-test"}}
	require.NoError(t, repo.Create(ctx, parent))
	svc := &adminServiceImpl{accountRepo: repo, settingService: &SettingService{settingRepo: adminEfficiencySettingsStub{value: "false"}}}
	_, err := svc.CreateShadow(ctx, parent.ID, ShadowOptions{Name: "shadow"})
	require.Equal(t, "CUSTOM_EXTENSION_DISABLED", infraerrors.Reason(err))
	shadows, err := repo.ListShadowsByParent(ctx, parent.ID)
	require.NoError(t, err)
	require.Empty(t, shadows)
	svc.settingService = enabledAdminEfficiencySettings()
	shadow, err := svc.CreateShadow(ctx, parent.ID, ShadowOptions{Name: "shadow"})
	require.NoError(t, err)
	require.Equal(t, "historical", shadow.UpstreamGroup)
}

func TestAdminEfficiencyDisabledPreservesNativeDuplicateWithoutNewLabel(t *testing.T) {
	ctx := context.Background()
	repo := newDuplicateAccountRepoStub()
	source := &Account{Name: "source", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, UpstreamGroup: "historical", Credentials: map[string]any{"api_key": "fixture"}}
	require.NoError(t, repo.Create(ctx, source))
	svc := &adminServiceImpl{accountRepo: repo, accountDuplicateRepo: repo}
	duplicate, err := svc.DuplicateAccount(ctx, source.ID, "admin:1", "")
	require.NoError(t, err)
	require.Empty(t, duplicate.UpstreamGroup, "native duplicate does not copy the custom label")
	require.Equal(t, "historical", source.UpstreamGroup)
}
