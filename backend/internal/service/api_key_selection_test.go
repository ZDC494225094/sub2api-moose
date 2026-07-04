package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeySelectionGroupRepo struct {
	groupRepoNoop
	groups       map[int64]*Group
	getByIDCalls int
	getLiteCalls int
}

func (r *apiKeySelectionGroupRepo) ListActive(_ context.Context) ([]Group, error) {
	out := make([]Group, 0, len(r.groups))
	for _, group := range r.groups {
		if group == nil || !group.IsActive() {
			continue
		}
		cp := *group
		out = append(out, cp)
	}
	return out, nil
}

func (r *apiKeySelectionGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	r.getByIDCalls++
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}

func (r *apiKeySelectionGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	r.getLiteCalls++
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}

type apiKeySelectionBalanceResolver struct {
	balance float64
	err     error
	calls   int
}

func (r *apiKeySelectionBalanceResolver) GetUserBalance(_ context.Context, _ int64) (float64, error) {
	r.calls++
	return r.balance, r.err
}

type apiKeySelectionUserRepo struct {
	user *User
}

func (r *apiKeySelectionUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, ErrUserNotFound
	}
	cp := *r.user
	return &cp, nil
}

func (r *apiKeySelectionUserRepo) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return r.GetByID(ctx, id)
}

func (r *apiKeySelectionUserRepo) Create(context.Context, *User) error {
	panic("unexpected Create call")
}
func (r *apiKeySelectionUserRepo) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected GetByEmail call")
}
func (r *apiKeySelectionUserRepo) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}
func (r *apiKeySelectionUserRepo) Update(context.Context, *User) error {
	panic("unexpected Update call")
}
func (r *apiKeySelectionUserRepo) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (r *apiKeySelectionUserRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("unexpected GetUserAvatar call")
}
func (r *apiKeySelectionUserRepo) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertUserAvatar call")
}
func (r *apiKeySelectionUserRepo) DeleteUserAvatar(context.Context, int64) error {
	panic("unexpected DeleteUserAvatar call")
}
func (r *apiKeySelectionUserRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (r *apiKeySelectionUserRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (r *apiKeySelectionUserRepo) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserIDs call")
}
func (r *apiKeySelectionUserRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserID call")
}
func (r *apiKeySelectionUserRepo) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("unexpected UpdateUserLastActiveAt call")
}
func (r *apiKeySelectionUserRepo) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}
func (r *apiKeySelectionUserRepo) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}
func (r *apiKeySelectionUserRepo) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (r *apiKeySelectionUserRepo) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchSetConcurrency call")
}
func (r *apiKeySelectionUserRepo) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchAddConcurrency call")
}
func (r *apiKeySelectionUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}
func (r *apiKeySelectionUserRepo) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}
func (r *apiKeySelectionUserRepo) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}
func (r *apiKeySelectionUserRepo) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}
func (r *apiKeySelectionUserRepo) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected ListUserAuthIdentities call")
}
func (r *apiKeySelectionUserRepo) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected UnbindUserAuthProvider call")
}
func (r *apiKeySelectionUserRepo) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}
func (r *apiKeySelectionUserRepo) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}
func (r *apiKeySelectionUserRepo) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func TestSelectUsableGroupForAPIKeyBalanceFirstFallsBackToSubscription(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	standard := &Group{ID: 10, Name: "balance", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 20, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		standard.ID:     standard,
		subscription.ID: subscription,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 2001,
		UserID:             701,
		GroupID:            subscription.ID,
		Status:             SubscriptionStatusActive,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		DailyWindowStart:   &now,
		WeeklyWindowStart:  &now,
		MonthlyWindowStart: &now,
	})
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              1,
		UserID:          701,
		User:            &User{ID: 701, Status: StatusActive, Balance: 5},
		GroupIDs:        []int64{standard.ID, subscription.ID},
		BillingPriority: BillingPriorityBalanceFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, standard.ID, selection.Group.ID)
	require.Nil(t, selection.Subscription)

	apiKey.User.Balance = 0
	selection, err = apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)
	require.Equal(t, int64(2001), selection.Subscription.ID)
}

func TestSelectUsableGroupForAPIKeySubscriptionFirstFallsBackToBalance(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	standard := &Group{ID: 11, Name: "balance", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 21, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		standard.ID:     standard,
		subscription.ID: subscription,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 2101,
		UserID:             702,
		GroupID:            subscription.ID,
		Status:             SubscriptionStatusActive,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		DailyWindowStart:   &now,
		WeeklyWindowStart:  &now,
		MonthlyWindowStart: &now,
	})
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              2,
		UserID:          702,
		User:            &User{ID: 702, Status: StatusActive, Balance: 5},
		GroupIDs:        []int64{subscription.ID, standard.ID},
		BillingPriority: BillingPrioritySubscriptionFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)

	subRepo.byID[2101].DailyUsageUSD = dailyLimit
	selection, err = apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, standard.ID, selection.Group.ID)
	require.Nil(t, selection.Subscription)
}

func TestSelectUsableGroupForAPIKeySkipsExhaustedSubscriptionInstance(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	subscription := &Group{ID: 30, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{subscription.ID: subscription}}
	subRepo := newSubscriptionUserSubRepoStub()
	for _, item := range []struct {
		id    int64
		usage float64
	}{
		{id: 3001, usage: dailyLimit},
		{id: 3002, usage: 0},
	} {
		subRepo.seed(&UserSubscription{
			ID:                 item.id,
			UserID:             703,
			GroupID:            subscription.ID,
			Status:             SubscriptionStatusActive,
			StartsAt:           now.Add(-time.Hour),
			ExpiresAt:          now.Add(24 * time.Hour),
			DailyWindowStart:   &now,
			WeeklyWindowStart:  &now,
			MonthlyWindowStart: &now,
			DailyUsageUSD:      item.usage,
		})
	}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              3,
		UserID:          703,
		User:            &User{ID: 703, Status: StatusActive, Balance: 0},
		GroupIDs:        []int64{subscription.ID},
		BillingPriority: BillingPrioritySubscriptionFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)
	require.Equal(t, int64(3002), selection.Subscription.ID)
}

func TestSelectUsableGroupForAPIKeySkipsGroupsFromOtherPlatforms(t *testing.T) {
	openai := &Group{ID: 40, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	gemini := &Group{ID: 41, Name: "gemini", Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		openai.ID: openai,
		gemini.ID: gemini,
	}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	apiKey := &APIKey{
		ID:       4,
		UserID:   704,
		Platform: PlatformOpenAI,
		User:     &User{ID: 704, Status: StatusActive, Balance: 5},
		GroupIDs: []int64{gemini.ID, openai.ID},
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, nil)
	require.NoError(t, err)
	require.Equal(t, openai.ID, selection.Group.ID)
	require.Equal(t, PlatformOpenAI, apiKey.Group.Platform)
}

func TestSelectUsableGroupForAPIKeyUsesLiveBalanceForFallback(t *testing.T) {
	now := time.Now()
	standard := &Group{ID: 42, Name: "balance", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 43, Name: "sub", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		standard.ID:     standard,
		subscription.ID: subscription,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 4301,
		UserID:             708,
		GroupID:            subscription.ID,
		Status:             SubscriptionStatusActive,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		DailyWindowStart:   &now,
		WeeklyWindowStart:  &now,
		MonthlyWindowStart: &now,
	})
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	balanceResolver := &apiKeySelectionBalanceResolver{balance: 0}
	apiKeySvc.SetBillingBalanceResolver(balanceResolver)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              5,
		UserID:          708,
		Platform:        PlatformOpenAI,
		User:            &User{ID: 708, Status: StatusActive, Balance: 10},
		GroupIDs:        []int64{standard.ID, subscription.ID},
		BillingPriority: BillingPriorityBalanceFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)
	require.Equal(t, int64(4301), selection.Subscription.ID)
	require.Equal(t, 1, balanceResolver.calls)
	require.Zero(t, apiKey.User.Balance)
}

func TestSelectUsableGroupForAPIKeyUsesLiteGroupLookup(t *testing.T) {
	standard := &Group{ID: 44, Name: "balance", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{standard.ID: standard}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	balanceResolver := &apiKeySelectionBalanceResolver{balance: 5}
	apiKeySvc.SetBillingBalanceResolver(balanceResolver)
	apiKey := &APIKey{
		ID:       6,
		UserID:   709,
		Platform: PlatformOpenAI,
		User:     &User{ID: 709, Status: StatusActive, Balance: 5},
		GroupIDs: []int64{standard.ID},
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, nil)
	require.NoError(t, err)
	require.Equal(t, standard.ID, selection.Group.ID)
	require.Equal(t, 1, groupRepo.getLiteCalls)
	require.Zero(t, groupRepo.getByIDCalls)
	require.Zero(t, balanceResolver.calls)
}

func TestValidateBindableGroupIDsRejectsMixedPlatforms(t *testing.T) {
	openai := &Group{ID: 50, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	gemini := &Group{ID: 51, Name: "gemini", Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		openai.ID: openai,
		gemini.ID: gemini,
	}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)

	platform, err := apiKeySvc.validateBindableGroupIDs(context.Background(), &User{ID: 705}, "", []int64{openai.ID, gemini.ID})
	require.ErrorIs(t, err, ErrAPIKeyGroupPlatformMismatch)
	require.Empty(t, platform)
}

func TestValidateBindableGroupIDsRejectsPlatformMismatch(t *testing.T) {
	openai := &Group{ID: 60, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{openai.ID: openai}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)

	platform, err := apiKeySvc.validateBindableGroupIDs(context.Background(), &User{ID: 706}, PlatformGemini, []int64{openai.ID})
	require.ErrorIs(t, err, ErrAPIKeyGroupPlatformMismatch)
	require.Empty(t, platform)
}

func TestGetAvailableGroupsMergesMultipleSubscriptionsForSameGroup(t *testing.T) {
	now := time.Now()
	subscription := &Group{ID: 70, Name: "sub", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}
	standard := &Group{ID: 71, Name: "standard", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		subscription.ID: subscription,
		standard.ID:     standard,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	for _, id := range []int64{7001, 7002} {
		subRepo.seed(&UserSubscription{
			ID:                 id,
			UserID:             707,
			GroupID:            subscription.ID,
			Status:             SubscriptionStatusActive,
			StartsAt:           now.Add(-time.Hour),
			ExpiresAt:          now.Add(24 * time.Hour),
			DailyWindowStart:   &now,
			WeeklyWindowStart:  &now,
			MonthlyWindowStart: &now,
		})
	}
	apiKeySvc := NewAPIKeyService(nil, &apiKeySelectionUserRepo{user: &User{ID: 707, Status: StatusActive}}, groupRepo, subRepo, nil, nil, nil)

	groups, err := apiKeySvc.GetAvailableGroups(context.Background(), 707)

	require.NoError(t, err)
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	require.ElementsMatch(t, []int64{subscription.ID, standard.ID}, groupIDs)
}
