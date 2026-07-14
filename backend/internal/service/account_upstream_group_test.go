package service

import (
	"context"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type accountUpstreamGroupPersistenceRepoStub struct {
	AccountRepository
	account     *Account
	created     *Account
	updated     *Account
	createCalls int
	updateCalls int
}

func newAccountUpstreamGroupPersistenceRepoStub(group string) *accountUpstreamGroupPersistenceRepoStub {
	return &accountUpstreamGroupPersistenceRepoStub{
		account: &Account{
			ID:            41,
			Name:          "existing",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeAPIKey,
			Status:        StatusActive,
			UpstreamGroup: group,
		},
	}
}

func (s *accountUpstreamGroupPersistenceRepoStub) Create(_ context.Context, account *Account) error {
	s.createCalls++
	if account.ID == 0 {
		account.ID = 42
	}
	copy := *account
	s.created = &copy
	s.account = account
	return nil
}

func (s *accountUpstreamGroupPersistenceRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return s.account, nil
}

func (s *accountUpstreamGroupPersistenceRepoStub) Update(_ context.Context, account *Account) error {
	s.updateCalls++
	copy := *account
	s.updated = &copy
	s.account = account
	return nil
}

func TestAccountServiceCreateNormalizesUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("")
	svc := NewAccountService(repo, nil)

	account, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:          "general-create",
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		UpstreamGroup: "  Edge China  ",
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, "Edge China", repo.created.UpstreamGroup)
	require.Equal(t, "Edge China", account.UpstreamGroup)
}

func TestAccountServiceCreateRejectsOverlongUnicodeUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("")
	svc := NewAccountService(repo, nil)

	account, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:          "general-create",
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		UpstreamGroup: strings.Repeat("界", AccountUpstreamGroupMaxLength+1),
	})

	require.Nil(t, account)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_TOO_LONG", infraerrors.Reason(err))
	require.Zero(t, repo.createCalls)
}

func TestAccountServiceUpdateNormalizesAndClearsUpstreamGroup(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "trim", value: "  Premium Edge  ", want: "Premium Edge"},
		{name: "clear", value: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newAccountUpstreamGroupPersistenceRepoStub("Old Group")
			svc := NewAccountService(repo, nil)
			value := tt.value

			account, err := svc.Update(context.Background(), repo.account.ID, UpdateAccountRequest{
				UpstreamGroup: &value,
			})

			require.NoError(t, err)
			require.Equal(t, 1, repo.updateCalls)
			require.Equal(t, tt.want, repo.updated.UpstreamGroup)
			require.Equal(t, tt.want, account.UpstreamGroup)
		})
	}
}

func TestAccountServiceUpdateRejectsOverlongUnicodeUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("Old Group")
	svc := NewAccountService(repo, nil)
	value := strings.Repeat("界", AccountUpstreamGroupMaxLength+1)

	account, err := svc.Update(context.Background(), repo.account.ID, UpdateAccountRequest{
		UpstreamGroup: &value,
	})

	require.Nil(t, account)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_TOO_LONG", infraerrors.Reason(err))
	require.Zero(t, repo.updateCalls)
	require.Equal(t, "Old Group", repo.account.UpstreamGroup)
}

func TestAdminServiceCreateAccountNormalizesUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("")
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "admin-create",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		UpstreamGroup:        "  Edge China  ",
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, "Edge China", repo.created.UpstreamGroup)
	require.Equal(t, "Edge China", account.UpstreamGroup)
}

func TestAdminServiceCreateAccountRejectsOverlongUnicodeUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("")
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "admin-create",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		UpstreamGroup:        strings.Repeat("界", AccountUpstreamGroupMaxLength+1),
		SkipDefaultGroupBind: true,
	})

	require.Nil(t, account)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_TOO_LONG", infraerrors.Reason(err))
	require.Zero(t, repo.createCalls)
}

func TestAdminServiceUpdateAccountNormalizesAndClearsUpstreamGroup(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "trim", value: "  Premium Edge  ", want: "Premium Edge"},
		{name: "clear", value: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newAccountUpstreamGroupPersistenceRepoStub("Old Group")
			svc := &adminServiceImpl{accountRepo: repo}
			value := tt.value

			account, err := svc.UpdateAccount(context.Background(), repo.account.ID, &UpdateAccountInput{
				UpstreamGroup: &value,
			})

			require.NoError(t, err)
			require.Equal(t, 1, repo.updateCalls)
			require.Equal(t, tt.want, repo.updated.UpstreamGroup)
			require.Equal(t, tt.want, account.UpstreamGroup)
		})
	}
}

func TestAdminServiceUpdateAccountRejectsOverlongUnicodeUpstreamGroup(t *testing.T) {
	repo := newAccountUpstreamGroupPersistenceRepoStub("Old Group")
	svc := &adminServiceImpl{accountRepo: repo}
	value := strings.Repeat("界", AccountUpstreamGroupMaxLength+1)

	account, err := svc.UpdateAccount(context.Background(), repo.account.ID, &UpdateAccountInput{
		UpstreamGroup: &value,
	})

	require.Nil(t, account)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_TOO_LONG", infraerrors.Reason(err))
	require.Zero(t, repo.updateCalls)
	require.Equal(t, "Old Group", repo.account.UpstreamGroup)
}
