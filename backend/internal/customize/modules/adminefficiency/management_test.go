package adminefficiency

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stateStub struct {
	enabled bool
	err     error
}

func (s stateStub) Enabled(context.Context, string) (bool, error) { return s.enabled, s.err }
func TestAdmissionAndHistoricalNoop(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("unavailable")
	require.ErrorIs(t, CheckWrite(ctx, nil), ErrExtensionDisabled)
	require.ErrorIs(t, CheckWrite(ctx, stateStub{}), ErrExtensionDisabled)
	require.ErrorIs(t, CheckWrite(ctx, stateStub{err: failure}), failure)
	require.NoError(t, CheckWrite(ctx, stateStub{enabled: true}))
	require.NoError(t, CheckUpstreamGroupChange(ctx, nil, "old", "old"))
	require.Error(t, CheckUpstreamGroupChange(ctx, nil, "old", ""))
	require.Error(t, CheckUpstreamGroupChange(ctx, nil, "", "new"))
}
func TestBatchUserPolicy(t *testing.T) {
	for _, ids := range [][]int64{nil, {0}, {-1}, make([]int64, 501)} {
		_, err := NormalizeBatchUserIDs(ids)
		require.Error(t, err)
	}
	ids, err := NormalizeBatchUserIDs([]int64{3, 1, 3, 2})
	require.NoError(t, err)
	require.Equal(t, []int64{3, 1, 2}, ids)
	called := []int64{}
	result := RunBatchUserAction(ids, func(id int64) error {
		called = append(called, id)
		if id == 1 {
			return errors.New("protected")
		}
		return nil
	})
	require.Equal(t, ids, called)
	require.Equal(t, 2, result.Affected)
	require.Equal(t, []BatchUserActionSkipped{{UserID: 1, Reason: "protected"}}, result.Skipped)
	require.NotNil(t, RunBatchUserAction(nil, nil).Skipped)
}
func TestGroupAccountReplacementAdmissionAndOrder(t *testing.T) {
	for _, stage := range []string{"disabled", "validate", "replace", "list", "success"} {
		t.Run(stage, func(t *testing.T) {
			calls := []string{}
			failure := errors.New("failed")
			ports := GroupAccountPorts[int64]{
				Validate: func(_ context.Context, g int64, ids []int64) error {
					calls = append(calls, "validate")
					require.Equal(t, int64(7), g)
					require.Equal(t, []int64{3, 1}, ids)
					if stage == "validate" {
						return failure
					}
					return nil
				},
				Replace: func(_ context.Context, g int64, ids []int64) error {
					calls = append(calls, "replace")
					require.Equal(t, []int64{3, 1}, ids)
					if stage == "replace" {
						return failure
					}
					return nil
				},
				List: func(context.Context, int64) ([]int64, error) {
					calls = append(calls, "list")
					if stage == "list" {
						return nil, failure
					}
					return []int64{3, 1}, nil
				},
			}
			result, err := ReplaceGroupAccounts(context.Background(), stateStub{enabled: stage != "disabled"}, 7, []int64{0, 3, 3, -1, 1}, ports)
			switch stage {
			case "disabled":
				require.ErrorIs(t, err, ErrExtensionDisabled)
				require.Empty(t, calls)
			case "validate":
				require.ErrorIs(t, err, failure)
				require.Equal(t, []string{"validate"}, calls)
			case "replace":
				require.ErrorIs(t, err, failure)
				require.Equal(t, []string{"validate", "replace"}, calls)
			case "list":
				require.ErrorIs(t, err, failure)
				require.Len(t, calls, 3)
			default:
				require.NoError(t, err)
				require.Equal(t, []int64{3, 1}, result)
				require.Len(t, calls, 3)
			}
		})
	}
	require.Empty(t, NormalizeGroupAccountIDs(nil))
}

type sortStub struct {
	got []AccountSortOrderUpdate
	err error
}

func (s *sortStub) UpdateSortOrders(_ context.Context, updates []AccountSortOrderUpdate) error {
	s.got = updates
	return s.err
}
func TestAccountSortPort(t *testing.T) {
	require.Error(t, SortAccounts(context.Background(), nil, nil))
	failure := errors.New("transaction failed")
	repo := &sortStub{err: failure}
	updates := []AccountSortOrderUpdate{{ID: 2, SortOrder: 20}}
	require.ErrorIs(t, SortAccounts(context.Background(), repo, updates), failure)
	require.Equal(t, updates, repo.got)
}
