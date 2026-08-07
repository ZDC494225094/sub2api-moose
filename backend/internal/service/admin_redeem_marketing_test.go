package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type marketingGenerationRedeemRepo struct {
	created []*RedeemCode
}

func (r *marketingGenerationRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	r.created = append(r.created, code)
	return nil
}

func (r *marketingGenerationRedeemRepo) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *marketingGenerationRedeemRepo) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (r *marketingGenerationRedeemRepo) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}

func (r *marketingGenerationRedeemRepo) Update(context.Context, *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *marketingGenerationRedeemRepo) BatchUpdate(context.Context, []int64, RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (r *marketingGenerationRedeemRepo) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (r *marketingGenerationRedeemRepo) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}

func (r *marketingGenerationRedeemRepo) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *marketingGenerationRedeemRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *marketingGenerationRedeemRepo) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *marketingGenerationRedeemRepo) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *marketingGenerationRedeemRepo) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestAdminService_GenerateMarketingRedeemCodesSharesBatch(t *testing.T) {
	repo := &marketingGenerationRedeemRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 3,
		Type:  RedeemTypeMarketing,
		Value: 8.5,
	})

	require.NoError(t, err)
	require.Len(t, codes, 3)
	require.Len(t, repo.created, 3)
	require.NotNil(t, codes[0].BatchID)
	require.NotEmpty(t, *codes[0].BatchID)
	for _, code := range codes {
		require.Equal(t, RedeemTypeMarketing, code.Type)
		require.Equal(t, 8.5, code.Value)
		require.Equal(t, codes[0].BatchID, code.BatchID)
		require.Equal(t, StatusUnused, code.Status)
	}
}

func TestAdminService_GenerateMarketingRedeemCodesRejectsNonPositiveValue(t *testing.T) {
	for _, value := range []float64{0, -1} {
		t.Run("value", func(t *testing.T) {
			svc := &adminServiceImpl{redeemCodeRepo: &marketingGenerationRedeemRepo{}}

			_, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
				Count: 1,
				Type:  RedeemTypeMarketing,
				Value: value,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "marketing redeem code value must be greater than 0")
		})
	}
}
