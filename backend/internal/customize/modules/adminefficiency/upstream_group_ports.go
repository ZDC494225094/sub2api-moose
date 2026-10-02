package adminefficiency

import "context"

// AccountUpstreamGroupRepository exposes the persistent admin-only upstream
// group directory without broadening the main AccountRepository contract.
type AccountUpstreamGroupRepository interface {
	ListUpstreamGroups(ctx context.Context) ([]AccountUpstreamGroup, error)
}

type AccountUpstreamGroup struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	AccountCount int    `json:"account_count"`
	SortOrder    int64  `json:"sort_order"`
}

// Rename must atomically update the directory name and all linked account labels.
// The host adapter owns locking, uniqueness checks, commit and rollback.
type AccountUpstreamGroupRenameRepository interface {
	RenameUpstreamGroup(ctx context.Context, id int64, name string) (*AccountUpstreamGroup, error)
}

type AccountUpstreamGroupSortOrderUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int64 `json:"sort_order"`
}

// Updates must be all-or-nothing, including when a requested group does not exist.
type AccountUpstreamGroupSortOrderRepository interface {
	UpdateUpstreamGroupSortOrders(ctx context.Context, updates []AccountUpstreamGroupSortOrderUpdate) error
}
