package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ListAccountUpstreamGroups returns the persistent, credential-independent
// upstream group directory. Empty groups remain visible for future assignment.
func (s *adminServiceImpl) ListAccountUpstreamGroups(ctx context.Context) ([]AccountUpstreamGroup, error) {
	repo, ok := s.accountRepo.(AccountUpstreamGroupRepository)
	if !ok {
		return nil, infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.ListUpstreamGroups(ctx)
}

func (s *adminServiceImpl) RenameAccountUpstreamGroup(ctx context.Context, id int64, name string) (*AccountUpstreamGroup, error) {
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
	repo, ok := s.accountRepo.(AccountUpstreamGroupRenameRepository)
	if !ok {
		return nil, infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.RenameUpstreamGroup(ctx, id, name)
}

func (s *adminServiceImpl) UpdateAccountUpstreamGroupSortOrders(ctx context.Context, updates []AccountUpstreamGroupSortOrderUpdate) error {
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
	repo, ok := s.accountRepo.(AccountUpstreamGroupSortOrderRepository)
	if !ok {
		return infraerrors.InternalServer("ACCOUNT_UPSTREAM_GROUPS_UNAVAILABLE", "account upstream group repository is not configured")
	}
	return repo.UpdateUpstreamGroupSortOrders(ctx, updates)
}
