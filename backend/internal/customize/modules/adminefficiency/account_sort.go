package adminefficiency

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AccountSortOrderUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int64 `json:"sort_order"`
}
type AccountSortOrderRepository interface {
	UpdateSortOrders(context.Context, []AccountSortOrderUpdate) error
}

// SortAccounts changes display order only; scheduling priority belongs to the host.
func SortAccounts(ctx context.Context, repo AccountSortOrderRepository, updates []AccountSortOrderUpdate) error {
	if repo == nil {
		return infraerrors.InternalServer("ACCOUNT_SORT_ORDER_UNAVAILABLE", "account sort order repository is not configured")
	}
	return repo.UpdateSortOrders(ctx, updates)
}
