package repository

import (
	"context"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// Marketing repositories must use the host transaction, not merely receive its
// context while continuing to execute through the root connection pool. Reads
// with FOR UPDATE and every coupon/chance/stock/reward write share this executor.
// Outside a transaction, existing standalone catalog/read behavior is unchanged.
func marketingSQL(ctx context.Context, fallback sqlExecutor) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}
