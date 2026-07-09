//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type lotteryActivityRepoStubForDelete struct {
	deleteErr  error
	deletedIDs []int64
}

func (s *lotteryActivityRepoStubForDelete) Create(context.Context, *LotteryActivity) error {
	panic("unexpected Create call")
}

func (s *lotteryActivityRepoStubForDelete) Update(context.Context, *LotteryActivity) error {
	panic("unexpected Update call")
}

func (s *lotteryActivityRepoStubForDelete) Delete(_ context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteErr
}

func (s *lotteryActivityRepoStubForDelete) GetByID(context.Context, int64) (*LotteryActivity, error) {
	panic("unexpected GetByID call")
}

func (s *lotteryActivityRepoStubForDelete) GetByIDForUpdate(context.Context, int64) (*LotteryActivity, error) {
	panic("unexpected GetByIDForUpdate call")
}

func (s *lotteryActivityRepoStubForDelete) GetActiveActivity(context.Context) (*LotteryActivity, error) {
	panic("unexpected GetActiveActivity call")
}

func (s *lotteryActivityRepoStubForDelete) List(context.Context, pagination.PaginationParams, LotteryActivityListFilter) ([]LotteryActivity, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

type lotteryDrawRecordRepoStubForDelete struct {
	exists    bool
	existsErr error
	checkedID int64
}

func (s *lotteryDrawRecordRepoStubForDelete) Create(context.Context, *LotteryDrawRecord) error {
	panic("unexpected Create call")
}

func (s *lotteryDrawRecordRepoStubForDelete) ListByUser(context.Context, int64, int64, pagination.PaginationParams) ([]LotteryDrawRecord, *pagination.PaginationResult, error) {
	panic("unexpected ListByUser call")
}

func (s *lotteryDrawRecordRepoStubForDelete) ListRecentByActivity(context.Context, int64, int) ([]LotteryDrawRecord, error) {
	panic("unexpected ListRecentByActivity call")
}

func (s *lotteryDrawRecordRepoStubForDelete) ExistsByActivity(_ context.Context, activityID int64) (bool, error) {
	s.checkedID = activityID
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.exists, nil
}

type lotteryPrizeRepoNoop struct{}

func (lotteryPrizeRepoNoop) Create(context.Context, *LotteryPrize) error { panic("unexpected") }
func (lotteryPrizeRepoNoop) Update(context.Context, *LotteryPrize) error { panic("unexpected") }
func (lotteryPrizeRepoNoop) GetByID(context.Context, int64) (*LotteryPrize, error) {
	panic("unexpected")
}
func (lotteryPrizeRepoNoop) ListByActivity(context.Context, int64) ([]LotteryPrize, error) {
	panic("unexpected")
}
func (lotteryPrizeRepoNoop) ListActiveByActivityForUpdate(context.Context, int64) ([]LotteryPrize, error) {
	panic("unexpected")
}
func (lotteryPrizeRepoNoop) DecrementStock(context.Context, int64) error { panic("unexpected") }

type lotteryUserStateRepoNoop struct{}

func (lotteryUserStateRepoNoop) GetOrCreate(context.Context, int64, int64) (*LotteryUserState, error) {
	panic("unexpected")
}
func (lotteryUserStateRepoNoop) GetOrCreateForUpdate(context.Context, int64, int64) (*LotteryUserState, error) {
	panic("unexpected")
}
func (lotteryUserStateRepoNoop) Update(context.Context, *LotteryUserState) error { panic("unexpected") }

type lotteryChanceLogRepoNoop struct{}

func (lotteryChanceLogRepoNoop) Create(context.Context, *LotteryChanceLog) error { panic("unexpected") }

type lotteryConsumeProgressRepoNoop struct{}

func (lotteryConsumeProgressRepoNoop) GetQualifiedAmount(context.Context, int64, int64, float64) (float64, error) {
	panic("unexpected")
}

type lotteryUserRepoNoop struct{}

func (lotteryUserRepoNoop) Create(context.Context, *User) error           { panic("unexpected") }
func (lotteryUserRepoNoop) GetByID(context.Context, int64) (*User, error) { panic("unexpected") }
func (lotteryUserRepoNoop) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) GetByEmail(context.Context, string) (*User, error) { panic("unexpected") }
func (lotteryUserRepoNoop) GetFirstAdmin(context.Context) (*User, error)      { panic("unexpected") }
func (lotteryUserRepoNoop) Update(context.Context, *User) error               { panic("unexpected") }
func (lotteryUserRepoNoop) Delete(context.Context, int64) error               { panic("unexpected") }
func (lotteryUserRepoNoop) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) DeleteUserAvatar(context.Context, int64) error { panic("unexpected") }
func (lotteryUserRepoNoop) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) UpdateBalance(context.Context, int64, float64) error { panic("unexpected") }
func (lotteryUserRepoNoop) DeductBalance(context.Context, int64, float64) error { panic("unexpected") }
func (lotteryUserRepoNoop) UpdateConcurrency(context.Context, int64, int) error { panic("unexpected") }
func (lotteryUserRepoNoop) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) ExistsByEmail(context.Context, string) (bool, error) { panic("unexpected") }
func (lotteryUserRepoNoop) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected")
}
func (lotteryUserRepoNoop) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected")
}
func (lotteryUserRepoNoop) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected")
}
func (lotteryUserRepoNoop) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected")
}
func (lotteryUserRepoNoop) EnableTotp(context.Context, int64) error  { panic("unexpected") }
func (lotteryUserRepoNoop) DisableTotp(context.Context, int64) error { panic("unexpected") }
func (lotteryUserRepoNoop) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("unexpected")
}
func (lotteryUserRepoNoop) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("unexpected")
}

func TestLotteryServiceDeleteActivity_BlocksWhenDrawRecordsExist(t *testing.T) {
	activityRepo := &lotteryActivityRepoStubForDelete{}
	drawRecordRepo := &lotteryDrawRecordRepoStubForDelete{exists: true}
	svc := &LotteryService{
		activityRepo:        activityRepo,
		prizeRepo:           lotteryPrizeRepoNoop{},
		userStateRepo:       lotteryUserStateRepoNoop{},
		chanceLogRepo:       lotteryChanceLogRepoNoop{},
		drawRecordRepo:      drawRecordRepo,
		consumeProgressRepo: lotteryConsumeProgressRepoNoop{},
		userRepo:            lotteryUserRepoNoop{},
		clock:               realLotteryClock{},
	}

	err := svc.DeleteActivity(context.Background(), 88)
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot be deleted")
	require.Equal(t, int64(88), drawRecordRepo.checkedID)
	require.Empty(t, activityRepo.deletedIDs)
}

func TestLotteryServiceDeleteActivity_DeletesWhenNoDrawRecords(t *testing.T) {
	activityRepo := &lotteryActivityRepoStubForDelete{}
	drawRecordRepo := &lotteryDrawRecordRepoStubForDelete{exists: false}
	svc := &LotteryService{
		activityRepo:        activityRepo,
		prizeRepo:           lotteryPrizeRepoNoop{},
		userStateRepo:       lotteryUserStateRepoNoop{},
		chanceLogRepo:       lotteryChanceLogRepoNoop{},
		drawRecordRepo:      drawRecordRepo,
		consumeProgressRepo: lotteryConsumeProgressRepoNoop{},
		userRepo:            lotteryUserRepoNoop{},
		clock:               realLotteryClock{},
	}

	err := svc.DeleteActivity(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, []int64{99}, activityRepo.deletedIDs)
}

func TestLotteryServiceDeleteActivity_PropagatesExistsCheckError(t *testing.T) {
	activityRepo := &lotteryActivityRepoStubForDelete{}
	drawRecordRepo := &lotteryDrawRecordRepoStubForDelete{existsErr: errors.New("query failed")}
	svc := &LotteryService{
		activityRepo:        activityRepo,
		prizeRepo:           lotteryPrizeRepoNoop{},
		userStateRepo:       lotteryUserStateRepoNoop{},
		chanceLogRepo:       lotteryChanceLogRepoNoop{},
		drawRecordRepo:      drawRecordRepo,
		consumeProgressRepo: lotteryConsumeProgressRepoNoop{},
		userRepo:            lotteryUserRepoNoop{},
		clock:               realLotteryClock{},
	}

	err := svc.DeleteActivity(context.Background(), 66)
	require.Error(t, err)
	require.ErrorContains(t, err, "query failed")
	require.Empty(t, activityRepo.deletedIDs)
}
