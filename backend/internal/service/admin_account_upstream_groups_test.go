package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type accountUpstreamGroupDirectoryRepoStub struct {
	AccountRepository
	groups       []AccountUpstreamGroup
	listErr      error
	renameID     int64
	renameName   string
	renameResult *AccountUpstreamGroup
	renameErr    error
	sortUpdates  []AccountUpstreamGroupSortOrderUpdate
	sortErr      error
}

func (s *accountUpstreamGroupDirectoryRepoStub) ListUpstreamGroups(context.Context) ([]AccountUpstreamGroup, error) {
	return s.groups, s.listErr
}

func (s *accountUpstreamGroupDirectoryRepoStub) RenameUpstreamGroup(_ context.Context, id int64, name string) (*AccountUpstreamGroup, error) {
	s.renameID = id
	s.renameName = name
	return s.renameResult, s.renameErr
}

func (s *accountUpstreamGroupDirectoryRepoStub) UpdateUpstreamGroupSortOrders(_ context.Context, updates []AccountUpstreamGroupSortOrderUpdate) error {
	s.sortUpdates = append([]AccountUpstreamGroupSortOrderUpdate(nil), updates...)
	return s.sortErr
}

type accountUpstreamGroupUnsupportedRepoStub struct {
	AccountRepository
}

func TestAdminServiceListAccountUpstreamGroupsUsesPersistentDirectory(t *testing.T) {
	want := []AccountUpstreamGroup{
		{ID: 3, Key: "hi-code", Name: "hi-code", AccountCount: 4, SortOrder: 10},
		{ID: 9, Key: "official", Name: "Official", AccountCount: 0, SortOrder: 20},
	}
	svc := &adminServiceImpl{accountRepo: &accountUpstreamGroupDirectoryRepoStub{groups: want}}

	got, err := svc.ListAccountUpstreamGroups(context.Background())

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestAdminServiceListAccountUpstreamGroupsReturnsRepositoryError(t *testing.T) {
	repoErr := errors.New("list upstream groups failed")
	svc := &adminServiceImpl{accountRepo: &accountUpstreamGroupDirectoryRepoStub{listErr: repoErr}}

	groups, err := svc.ListAccountUpstreamGroups(context.Background())

	require.ErrorIs(t, err, repoErr)
	require.Nil(t, groups)
}

func TestAdminServiceListAccountUpstreamGroupsRequiresCapableRepository(t *testing.T) {
	svc := &adminServiceImpl{accountRepo: &accountUpstreamGroupUnsupportedRepoStub{}}

	groups, err := svc.ListAccountUpstreamGroups(context.Background())

	require.Nil(t, groups)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", infraerrors.Reason(err))
}

func TestAdminServiceRenameAccountUpstreamGroupNormalizesName(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{
		renameResult: &AccountUpstreamGroup{ID: 3, Key: "hi-code", Name: "hi-code"},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	group, err := svc.RenameAccountUpstreamGroup(context.Background(), 3, "  hi-code  ")

	require.NoError(t, err)
	require.Equal(t, int64(3), repo.renameID)
	require.Equal(t, "hi-code", repo.renameName)
	require.Equal(t, repo.renameResult, group)
}

func TestAdminServiceRenameAccountUpstreamGroupRejectsBlankName(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.RenameAccountUpstreamGroup(context.Background(), 3, "  ")

	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_NAME_REQUIRED", infraerrors.Reason(err))
	require.Zero(t, repo.renameID)
}

func TestAdminServiceUpdateAccountUpstreamGroupSortOrdersValidatesAndDelegates(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}
	updates := []AccountUpstreamGroupSortOrderUpdate{
		{ID: 3, SortOrder: 10},
		{ID: 9, SortOrder: 20},
	}

	err := svc.UpdateAccountUpstreamGroupSortOrders(context.Background(), updates)

	require.NoError(t, err)
	require.Equal(t, updates, repo.sortUpdates)
}

func TestAdminServiceUpdateAccountUpstreamGroupSortOrdersRejectsDuplicateIDs(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.UpdateAccountUpstreamGroupSortOrders(context.Background(), []AccountUpstreamGroupSortOrderUpdate{
		{ID: 3, SortOrder: 10},
		{ID: 3, SortOrder: 20},
	})

	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_SORT_DUPLICATE", infraerrors.Reason(err))
	require.Empty(t, repo.sortUpdates)
}
