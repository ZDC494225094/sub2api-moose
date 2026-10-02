package adminefficiency

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrAccountUpstreamGroupNotFound = infraerrors.NotFound("ACCOUNT_UPSTREAM_GROUP_NOT_FOUND", "account upstream group not found")
	ErrAccountUpstreamGroupExists   = infraerrors.Conflict("ACCOUNT_UPSTREAM_GROUP_EXISTS", "account upstream group name already exists")
)

// Directory owns admin-only upstream group operations, not account credentials or routing.
// Capabilities are independent so read-only adapters can still list historical groups.
type Directory struct {
	reader  AccountUpstreamGroupRepository
	renamer AccountUpstreamGroupRenameRepository
	sorter  AccountUpstreamGroupSortOrderRepository
}

func NewDirectory(reader AccountUpstreamGroupRepository, renamer AccountUpstreamGroupRenameRepository, sorter AccountUpstreamGroupSortOrderRepository) *Directory {
	return &Directory{reader: reader, renamer: renamer, sorter: sorter}
}

// ListAccountUpstreamGroups returns the persistent, credential-independent
// upstream group directory. Empty groups remain visible for future assignment.
func (s *Directory) ListAccountUpstreamGroups(ctx context.Context) ([]AccountUpstreamGroup, error) {
	repo := s.reader
	if repo == nil {
		return nil, infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.ListUpstreamGroups(ctx)
}

func (s *Directory) RenameAccountUpstreamGroup(ctx context.Context, id int64, name string) (*AccountUpstreamGroup, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_INVALID", "upstream group id must be positive")
	}
	name, err := NormalizeAccountUpstreamGroup(name)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_NAME_REQUIRED", "upstream group name is required")
	}
	repo := s.renamer
	if repo == nil {
		return nil, infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.RenameUpstreamGroup(ctx, id, name)
}

func (s *Directory) UpdateAccountUpstreamGroupSortOrders(ctx context.Context, updates []AccountUpstreamGroupSortOrderUpdate) error {
	if len(updates) == 0 {
		return infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_SORT_EMPTY", "at least one upstream group sort update is required")
	}
	seen := make(map[int64]struct{}, len(updates))
	for _, update := range updates {
		if update.ID <= 0 || update.SortOrder < 0 {
			return infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_SORT_INVALID", "upstream group sort update is invalid")
		}
		if _, exists := seen[update.ID]; exists {
			return infraerrors.BadRequest("ACCOUNT_UPSTREAM_GROUP_SORT_DUPLICATE", "upstream group sort updates must use unique ids")
		}
		seen[update.ID] = struct{}{}
	}
	repo := s.sorter
	if repo == nil {
		return infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.UpdateUpstreamGroupSortOrders(ctx, updates)
}
