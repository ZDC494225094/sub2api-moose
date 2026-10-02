package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
	"github.com/stretchr/testify/require"
)

type subscriptionPolicyAdmissionFunc func(context.Context) (string, error)

func (f subscriptionPolicyAdmissionFunc) NewSubscriptionPolicy(ctx context.Context) (string, error) {
	return f(ctx)
}

type nativeSubscriptionRepo struct {
	*subscriptionUserSubRepoStub
	locks     int
	lookupErr error
	lockErr   error
}

func (r *nativeSubscriptionRepo) LockSubscriptionOwner(context.Context, int64) error {
	r.locks++
	return r.lockErr
}
func (r *nativeSubscriptionRepo) FindNativeForAssignment(_ context.Context, uid, gid int64) (*UserSubscription, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	for _, sub := range r.byID {
		if sub.UserID == uid && sub.GroupID == gid && sub.CustomSubscriptionPolicy == subscriptionextensions.Native {
			cp := *sub
			return &cp, nil
		}
	}
	return nil, ErrSubscriptionNotFound
}
func (r *nativeSubscriptionRepo) ExtendExpiry(_ context.Context, id int64, expiry time.Time) error {
	r.byID[id].ExpiresAt = expiry
	return nil
}
func (r *nativeSubscriptionRepo) UpdateStatus(_ context.Context, id int64, status string) error {
	r.byID[id].Status = status
	return nil
}
func (r *nativeSubscriptionRepo) UpdateNotes(_ context.Context, id int64, notes string) error {
	r.byID[id].Notes = notes
	return nil
}
func nativeIssuanceFixture() (*SubscriptionService, *nativeSubscriptionRepo) {
	repo := &nativeSubscriptionRepo{subscriptionUserSubRepoStub: newSubscriptionUserSubRepoStub()}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, nil, nil)
	return svc, repo
}
func TestSubscriptionIssuanceNativeAssignmentAndRenewal(t *testing.T) {
	ctx := context.Background()
	svc, repo := nativeIssuanceFixture()
	input := &AssignSubscriptionInput{UserID: 7, GroupID: 1, ValidityDays: 30, Notes: "admin"}
	// Disabled/unconfigured host creates native lineage, never selects a historical independent grant.
	repo.seed(&UserSubscription{ID: 100, UserID: 7, GroupID: 1, CustomSubscriptionPolicy: subscriptionextensions.Independent, DailyUsageUSD: 9})
	first, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.Equal(t, subscriptionextensions.Native, first.CustomSubscriptionPolicy)
	again, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.Equal(t, first.ExpiresAt, again.ExpiresAt)
	require.Equal(t, 1, repo.createCalls)
	conflict := *input
	conflict.Notes = "different"
	_, err = svc.AssignSubscription(ctx, &conflict)
	require.ErrorIs(t, err, ErrSubscriptionAssignConflict)
	renewed, reused, err := svc.AssignOrExtendSubscription(ctx, &conflict)
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, first.ExpiresAt.AddDate(0, 0, 30), renewed.ExpiresAt)
	require.Equal(t, "admin\ndifferent", renewed.Notes)
	require.Equal(t, float64(9), repo.byID[100].DailyUsageUSD)
	require.Equal(t, 4, repo.locks)
}
func TestSubscriptionFrozenPolicyNeverReadsLiveAdmission(t *testing.T) {
	for _, policy := range []string{"", subscriptionextensions.Independent, subscriptionextensions.Native} {
		t.Run("persisted_"+policy, func(t *testing.T) {
			svc, repo := nativeIssuanceFixture()
			svc.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) {
				t.Fatal("fulfillment must not read live switch")
				return "", nil
			})
			frozen, err := subscriptionextensions.Resolve(policy)
			require.NoError(t, err)
			for i := 0; i < 2; i++ {
				_, _, err := svc.assignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{UserID: 7, GroupID: 1, ValidityDays: 30, issuancePolicy: frozen}, true)
				require.NoError(t, err)
			}
			if frozen == subscriptionextensions.Native {
				require.Equal(t, 1, repo.createCalls)
				require.Equal(t, 2, repo.locks)
			} else {
				require.Equal(t, 2, repo.createCalls)
				require.Zero(t, repo.locks)
			}
		})
	}
}
func TestSubscriptionIssuanceFailsBeforeMutation(t *testing.T) {
	for _, kind := range []string{"state", "unknown", "empty", "lock", "lookup"} {
		t.Run(kind, func(t *testing.T) {
			svc, repo := nativeIssuanceFixture()
			switch kind {
			case "state":
				svc.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) { return "", errors.New("store down") })
			case "unknown", "empty":
				svc.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) {
					if kind == "empty" {
						return "", nil
					}
					return "unknown", nil
				})
			case "lock":
				repo.lockErr = errors.New("lock failed")
			case "lookup":
				repo.lookupErr = errors.New("lookup failed")
			}
			_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{UserID: 7, GroupID: 1})
			require.Error(t, err)
			require.Zero(t, repo.createCalls)
		})
	}
}
func TestSubscriptionNativeExpiredAssignmentResetsOnlyNativeWindow(t *testing.T) {
	svc, repo := nativeIssuanceFixture()
	now := time.Now()
	svc.now = func() time.Time { return now }
	repo.seed(&UserSubscription{ID: 10, UserID: 7, GroupID: 1, CustomSubscriptionPolicy: subscriptionextensions.Native, ExpiresAt: now.Add(-time.Hour), Status: SubscriptionStatusExpired, DailyUsageUSD: 8, WeeklyUsageUSD: 10, MonthlyUsageUSD: 20})
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{UserID: 7, GroupID: 1, ValidityDays: 12})
	require.NoError(t, err)
	require.Equal(t, int64(10), sub.ID)
	require.Equal(t, now.AddDate(0, 0, 12), sub.ExpiresAt)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.Zero(t, repo.createCalls)
}
func TestSubscriptionBulkFreezesOneAdmissionDecision(t *testing.T) {
	svc, repo := nativeIssuanceFixture()
	reads := 0
	svc.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) { reads++; return subscriptionextensions.Independent, nil })
	result, err := svc.BulkAssignSubscription(context.Background(), &BulkAssignSubscriptionInput{UserIDs: []int64{1, 2}, GroupID: 1})
	require.NoError(t, err)
	require.Equal(t, 2, result.CreatedCount)
	require.Equal(t, 1, reads)
	require.Equal(t, 2, repo.createCalls)
}
