package adminefficiency

import (
	"context"
	"errors"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type accountUpstreamGroupDirectoryRepoStub struct {
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

func TestDirectoryListAccountUpstreamGroupsUsesPersistentDirectory(t *testing.T) {
	want := []AccountUpstreamGroup{
		{ID: 3, Key: "hi-code", Name: "hi-code", AccountCount: 4, SortOrder: 10},
		{ID: 9, Key: "official", Name: "Official", AccountCount: 0, SortOrder: 20},
	}
	svc := NewDirectory(&accountUpstreamGroupDirectoryRepoStub{groups: want}, nil, nil)

	got, err := svc.ListAccountUpstreamGroups(context.Background())

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestDirectoryListAccountUpstreamGroupsReturnsRepositoryError(t *testing.T) {
	repoErr := errors.New("list upstream groups failed")
	svc := NewDirectory(&accountUpstreamGroupDirectoryRepoStub{listErr: repoErr}, nil, nil)

	groups, err := svc.ListAccountUpstreamGroups(context.Background())

	require.ErrorIs(t, err, repoErr)
	require.Nil(t, groups)
}

func TestDirectoryListAccountUpstreamGroupsRequiresCapableRepository(t *testing.T) {
	svc := NewDirectory(nil, nil, nil)

	groups, err := svc.ListAccountUpstreamGroups(context.Background())

	require.Nil(t, groups)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", infraerrors.Reason(err))
}

func TestDirectoryRenameAccountUpstreamGroupNormalizesName(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{
		renameResult: &AccountUpstreamGroup{ID: 3, Key: "hi-code", Name: "hi-code"},
	}
	svc := NewDirectory(repo, repo, repo)

	group, err := svc.RenameAccountUpstreamGroup(context.Background(), 3, "  hi-code  ")

	require.NoError(t, err)
	require.Equal(t, int64(3), repo.renameID)
	require.Equal(t, "hi-code", repo.renameName)
	require.Equal(t, repo.renameResult, group)
}

func TestDirectoryRenameAccountUpstreamGroupRejectsBlankName(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := NewDirectory(repo, repo, repo)

	_, err := svc.RenameAccountUpstreamGroup(context.Background(), 3, "  ")

	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_NAME_REQUIRED", infraerrors.Reason(err))
	require.Zero(t, repo.renameID)
}

func TestDirectoryUpdateAccountUpstreamGroupSortOrdersValidatesAndDelegates(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := NewDirectory(repo, repo, repo)
	updates := []AccountUpstreamGroupSortOrderUpdate{
		{ID: 3, SortOrder: 10},
		{ID: 9, SortOrder: 20},
	}

	err := svc.UpdateAccountUpstreamGroupSortOrders(context.Background(), updates)

	require.NoError(t, err)
	require.Equal(t, updates, repo.sortUpdates)
}

func TestDirectoryUpdateAccountUpstreamGroupSortOrdersRejectsDuplicateIDs(t *testing.T) {
	repo := &accountUpstreamGroupDirectoryRepoStub{}
	svc := NewDirectory(repo, repo, repo)

	err := svc.UpdateAccountUpstreamGroupSortOrders(context.Background(), []AccountUpstreamGroupSortOrderUpdate{
		{ID: 3, SortOrder: 10},
		{ID: 3, SortOrder: 20},
	})

	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_SORT_DUPLICATE", infraerrors.Reason(err))
	require.Empty(t, repo.sortUpdates)
}

func TestDirectoryRenameValidationBeforeWrite(t *testing.T) {
	for _, tt := range []struct {
		name          string
		id            int64
		value, reason string
	}{
		{"zero id", 0, "name", "ACCOUNT_UPSTREAM_GROUP_INVALID"},
		{"negative id", -1, "name", "ACCOUNT_UPSTREAM_GROUP_INVALID"},
		{"blank", 1, " ", "ACCOUNT_UPSTREAM_GROUP_NAME_REQUIRED"},
		{"unicode limit", 1, strings.Repeat("界", 101), "ACCOUNT_UPSTREAM_GROUP_TOO_LONG"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountUpstreamGroupDirectoryRepoStub{}
			_, err := NewDirectory(repo, repo, repo).RenameAccountUpstreamGroup(context.Background(), tt.id, tt.value)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
			require.Zero(t, repo.renameID)
			// Validation precedence remains unchanged even with no repository configured.
			_, err = NewDirectory(nil, nil, nil).RenameAccountUpstreamGroup(context.Background(), tt.id, tt.value)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
		})
	}
}

func TestDirectorySortValidationBeforeWrite(t *testing.T) {
	for _, tt := range []struct {
		name    string
		updates []AccountUpstreamGroupSortOrderUpdate
		reason  string
	}{
		{"empty", nil, "ACCOUNT_UPSTREAM_GROUP_SORT_EMPTY"},
		{"zero id", []AccountUpstreamGroupSortOrderUpdate{{ID: 0}}, "ACCOUNT_UPSTREAM_GROUP_SORT_INVALID"},
		{"negative id", []AccountUpstreamGroupSortOrderUpdate{{ID: -1}}, "ACCOUNT_UPSTREAM_GROUP_SORT_INVALID"},
		{"negative order", []AccountUpstreamGroupSortOrderUpdate{{ID: 1, SortOrder: -1}}, "ACCOUNT_UPSTREAM_GROUP_SORT_INVALID"},
		{"late duplicate", []AccountUpstreamGroupSortOrderUpdate{{ID: 1}, {ID: 2}, {ID: 1}}, "ACCOUNT_UPSTREAM_GROUP_SORT_DUPLICATE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountUpstreamGroupDirectoryRepoStub{}
			err := NewDirectory(repo, repo, repo).UpdateAccountUpstreamGroupSortOrders(context.Background(), tt.updates)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
			require.Nil(t, repo.sortUpdates)
		})
	}
}

func TestDirectoryOptionalCapabilitiesAndRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	svc := NewDirectory(nil, nil, nil)
	_, err := svc.RenameAccountUpstreamGroup(ctx, 1, "name")
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", infraerrors.Reason(err))
	err = svc.UpdateAccountUpstreamGroupSortOrders(ctx, []AccountUpstreamGroupSortOrderUpdate{{ID: 1}})
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", infraerrors.Reason(err))
	for _, repoErr := range []error{ErrAccountUpstreamGroupNotFound, ErrAccountUpstreamGroupExists, context.Canceled, errors.New("transaction failed")} {
		repo := &accountUpstreamGroupDirectoryRepoStub{renameErr: repoErr, sortErr: repoErr}
		svc = NewDirectory(nil, repo, repo)
		_, err = svc.RenameAccountUpstreamGroup(ctx, 1, "name")
		require.ErrorIs(t, err, repoErr)
		err = svc.UpdateAccountUpstreamGroupSortOrders(ctx, []AccountUpstreamGroupSortOrderUpdate{{ID: 1}})
		require.ErrorIs(t, err, repoErr)
	}
}

func TestNormalizeAccountUpstreamGroupUnicodeAndClear(t *testing.T) {
	for _, value := range []string{"", " \t\n", "  Edge China  ", strings.Repeat("界", 100)} {
		got, err := NormalizeAccountUpstreamGroup(value)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(value), got)
	}
	got, err := NormalizeAccountUpstreamGroup(strings.Repeat("界", 101))
	require.Empty(t, got)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUP_TOO_LONG", infraerrors.Reason(err))
}
