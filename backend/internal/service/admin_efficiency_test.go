package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type adminEfficiencySettingsStub struct {
	SettingRepository
	value string
	err   error
}

func (s adminEfficiencySettingsStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.value == "" {
		return map[string]string{}, s.err
	}
	return map[string]string{customize.Key("admin-efficiency"): s.value}, s.err
}
func enabledAdminEfficiencySettings() *SettingService {
	return &SettingService{settingRepo: adminEfficiencySettingsStub{value: "true"}}
}

type enabledAdminEfficiencyReader struct{}

func (enabledAdminEfficiencyReader) Enabled(context.Context, string) (bool, error) { return true, nil }

func TestAdminEfficiencyRejectsEveryMutationBeforeRepositoryAccess(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		err         error
	}{
		{"off", "false", nil}, {"missing", "", nil}, {"malformed", "TRUE", nil}, {"failed", "true", errors.New("offline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &adminServiceImpl{settingService: &SettingService{settingRepo: adminEfficiencySettingsStub{value: tc.value, err: tc.err}}}
			ctx := context.Background()
			require.Error(t, s.UpdateAccountSortOrders(ctx, nil))
			_, err := s.RenameAccountUpstreamGroup(ctx, 1, "new")
			require.Error(t, err)
			require.Error(t, s.UpdateAccountUpstreamGroupSortOrders(ctx, nil))
			_, err = s.UpdateGroupAccounts(ctx, 1, []int64{2})
			require.Error(t, err)
			_, err = s.CreateAccount(ctx, &CreateAccountInput{UpstreamGroup: "new"})
			require.Error(t, err)
			value := ""
			_, err = s.BulkUpdateAccounts(ctx, &BulkUpdateAccountsInput{UpstreamGroup: &value})
			require.Error(t, err)
		})
	}
}
func TestAdminEfficiencyDisabledKeepsHistoryAndCoreUpdates(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("historical")
	s := &adminServiceImpl{accountRepo: repo, settingService: &SettingService{settingRepo: adminEfficiencySettingsStub{value: "false"}}}
	value := "historical"
	_, err := s.UpdateAccount(context.Background(), 41, &UpdateAccountInput{UpstreamGroup: &value})
	require.NoError(t, err)
	value = "changed"
	_, err = s.UpdateAccount(context.Background(), 41, &UpdateAccountInput{Name: "must-not-mutate", UpstreamGroup: &value})
	require.Equal(t, "CUSTOM_EXTENSION_DISABLED", infraerrors.Reason(err))
	require.Equal(t, "historical", repo.account.UpstreamGroup)
	require.Equal(t, "existing", repo.account.Name)
	require.Equal(t, 1, repo.updateCalls)
	general := NewAccountService(repo, nil)
	_, err = general.Update(context.Background(), 41, UpdateAccountRequest{UpstreamGroup: &value})
	require.Error(t, err)
	_, err = general.Create(context.Background(), CreateAccountRequest{UpstreamGroup: "new"})
	require.Error(t, err)
	_, err = general.Create(context.Background(), CreateAccountRequest{Name: "core", Platform: PlatformOpenAI, Type: AccountTypeAPIKey})
	require.NoError(t, err)
}

// Narrow repository fakes exercise the host safety boundary without touching a database.
type efficiencyGroupRepo struct {
	GroupRepository
	group *Group
}

func (r efficiencyGroupRepo) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

type efficiencyAccountsRepo struct {
	AccountRepository
	accounts []*Account
	replaced []int64
	calls    int
}

func (r *efficiencyAccountsRepo) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return r.accounts, nil
}
func (r *efficiencyAccountsRepo) ReplaceGroupAccounts(_ context.Context, _ int64, ids []int64) error {
	r.calls++
	r.replaced = append([]int64{}, ids...)
	return nil
}
func (r *efficiencyAccountsRepo) ListGroupAccounts(context.Context, int64) ([]Account, error) {
	result := []Account{}
	for _, a := range r.accounts {
		result = append(result, *a)
	}
	return result, nil
}
func TestAdminEfficiencyMembershipPreservesHostSafetyAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name, platform, kind, reason string
		missing                      bool
	}{
		{name: "allowed", platform: PlatformOpenAI, kind: AccountTypeOAuth},
		{name: "wrong platform", platform: PlatformAnthropic, kind: AccountTypeOAuth, reason: "ACCOUNT_PLATFORM_MISMATCH"},
		{name: "apikey in OAuth group", platform: PlatformOpenAI, kind: AccountTypeAPIKey, reason: "GROUP_OAUTH_ONLY"},
		{name: "missing", missing: true, reason: "ACCOUNT_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &efficiencyAccountsRepo{}
			if !tc.missing {
				repo.accounts = []*Account{{ID: 2, Platform: tc.platform, Type: tc.kind}}
			}
			s := &adminServiceImpl{accountRepo: repo, groupRepo: efficiencyGroupRepo{group: &Group{ID: 7, Platform: PlatformOpenAI, RequireOAuthOnly: true}}, settingService: enabledAdminEfficiencySettings()}
			_, err := s.UpdateGroupAccounts(context.Background(), 7, []int64{2, 2, 0, -1})
			if tc.reason != "" {
				require.Equal(t, tc.reason, infraerrors.Reason(err))
				require.Zero(t, repo.calls)
			} else {
				require.NoError(t, err)
				require.Equal(t, []int64{2}, repo.replaced)
				require.Equal(t, 1, repo.calls)
			}
			s.settingService = nil
			history, err := s.GetGroupAccounts(context.Background(), 7)
			require.NoError(t, err)
			require.Len(t, history, len(repo.accounts))
		})
	}
}
