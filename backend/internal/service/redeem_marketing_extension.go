package service

import (
	"context"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/redeemmarketing"
)

// GenerateMarketingBatch keeps code issuance and the eligibility snapshot in
// one transaction, reusing the host's generator and repository context adapter.
func (s *RedeemService) GenerateMarketingBatch(ctx context.Context, options redeemmarketing.Options, generate func(context.Context) ([]RedeemCode, error)) ([]RedeemCode, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := redeemmarketing.RequireEnabled(txCtx, tx.Client()); err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC()
	codes, err := generate(txCtx)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 || codes[0].BatchID == nil {
		return nil, errors.New("marketing batch was not generated")
	}
	if options.NewUsersOnly {
		if err := redeemmarketing.SavePolicy(txCtx, tx.Client(), *codes[0].BatchID, cutoff); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *RedeemService) MarketingBatches(ctx context.Context, batch string, page, size int) ([]redeemmarketing.Batch, error) {
	return redeemmarketing.ListBatches(ctx, s.entClient, batch, page, size)
}
