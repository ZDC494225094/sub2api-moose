package rechargecampaigns

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func validQuoteCampaign() Campaign {
	return Campaign{Name: "秋日充值礼", Enabled: true, Kind: "bonus", Percent: 10, StartsAt: time.Now().Add(-time.Hour), EndsAt: time.Now().Add(time.Hour), RewardCap: 10, FreezeHours: 72, NewInviteesOnly: true}
}

func TestRechargeCampaignAmounts(t *testing.T) {
	for _, tc := range []struct {
		kind                                             string
		percent, amount, multiplier, principal, credited float64
	}{
		{"bonus", 10, 50, 1, 50, 55}, {"bonus", 10, 100, 1, 100, 110}, {"discount", 90, 100, 1, 90, 100},
		{"bonus", 10, 50, 2, 50, 110}, {"discount", 90, 100, 2, 90, 200}, {"discount", 90, 0.05, 1, 0.05, 0.05},
	} {
		a := validQuoteCampaign()
		a.Kind = tc.kind
		a.Percent = tc.percent
		p, c := campaignAmounts(a, QuoteInput{Amount: tc.amount, Multiplier: tc.multiplier, NativeCredited: decimal.NewFromFloat(tc.amount).Mul(decimal.NewFromFloat(tc.multiplier)).Round(2).InexactFloat64()})
		require.Equal(t, tc.principal, p)
		require.Equal(t, tc.credited, c)
	}
}
func TestRechargeCampaignValidationAndTimeBoundaries(t *testing.T) {
	a := validQuoteCampaign()
	require.NoError(t, a.Validate())
	require.True(t, a.Active(a.StartsAt))
	require.False(t, a.Active(a.EndsAt))
	require.False(t, a.Active(a.StartsAt.Add(-time.Nanosecond)))
	for _, change := range []func(*Campaign){
		func(a *Campaign) { a.Percent = math.NaN() }, func(a *Campaign) { a.Percent = math.Inf(1) },
		func(a *Campaign) { a.Kind = "invalid" }, func(a *Campaign) { a.Kind = "discount"; a.Percent = 100 },
		func(a *Campaign) { a.RewardPercent = 5; a.RewardCap = 0 }, func(a *Campaign) { a.EndsAt = a.StartsAt },
		func(a *Campaign) { a.FreezeHours = -1 }, func(a *Campaign) { a.MinAmount = -1 },
	} {
		b := a
		change(&b)
		require.Error(t, b.Validate())
	}
}
func TestRechargeCampaignAutomaticSelection(t *testing.T) {
	now := time.Now()
	bonus := validQuoteCampaign()
	bonus.ID = 1
	discount := bonus
	discount.ID, discount.Kind, discount.Percent, discount.MinAmount = 2, "discount", 90, 100
	require.Equal(t, int64(1), automaticCampaign([]Campaign{discount, bonus}, 50, now).ID)
	require.Equal(t, int64(2), automaticCampaign([]Campaign{bonus, discount}, 100, now).ID)
	newest := bonus
	newest.ID = 3
	require.Equal(t, int64(3), automaticCampaign([]Campaign{newest, bonus}, 50, now).ID)
	require.Nil(t, automaticCampaign([]Campaign{bonus}, 50, bonus.EndsAt))
	require.Nil(t, automaticCampaign([]Campaign{discount}, 50, now))
	bonus.Enabled = false
	require.Nil(t, automaticCampaign([]Campaign{bonus}, 50, now))
}

func TestCampaignQuoteKeepsNativeBaseAndRejectsStaleOrCombinedOffers(t *testing.T) {
	ctx := context.Background()
	a := testCampaign()
	a.Kind, a.Percent = "discount", 90
	repo := &fakeRepository{items: []Campaign{a}}
	svc := NewService(repo, nil)
	svc.now = func() time.Time { return a.StartsAt.Add(time.Hour) }
	input := QuoteInput{Amount: 100, Multiplier: 2, NativeCredited: 199.99}
	snap, err := svc.Quote(ctx, input, nil)
	require.NoError(t, err)
	require.Equal(t, 90.0, snap.Principal)
	require.Equal(t, 199.99, snap.Credited, "a discount must use the native host's credited amount")
	input.CampaignID, input.CampaignRevision = a.ID, Revision(a)
	_, err = svc.Quote(ctx, input, nil)
	require.NoError(t, err)
	input.UserCouponID = 10
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorContains(t, err, "优惠券")
	input.UserCouponID = 0
	repo.items[0].Percent = 80
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorContains(t, err, "活动规则已更新")
	require.Equal(t, 90.0, snap.Campaign.Percent, "catalog edits cannot rewrite issued snapshots")
	input.CampaignID = 999
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorContains(t, err, "活动规则已更新")
	repo.items = nil
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorContains(t, err, "活动已结束")
	input.CampaignID, input.UserCouponID = 0, 10
	snap, err = svc.Quote(ctx, input, nil)
	require.NoError(t, err)
	require.Nil(t, snap, "an absent campaign must not intercept the native coupon flow")
	repo.err = errors.New("catalog unavailable")
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorIs(t, err, repo.err, "never silently replace a failed activity quote with a different price")
}

func TestCampaignBonusRetainsSingleRoundingAndMinimumDiscountAmount(t *testing.T) {
	input := QuoteInput{Amount: 0.045, Multiplier: 1, NativeCredited: 0.05}
	a := testCampaign()
	principal, credited := campaignAmounts(a, input)
	require.Equal(t, input.Amount, principal)
	require.Equal(t, 0.05, credited, "bonus math rounds once after applying both multiplier and benefit")
	a.Kind, a.Percent = "discount", 1
	svc := NewService(&fakeRepository{items: []Campaign{a}}, nil)
	svc.now = func() time.Time { return a.StartsAt }
	_, err := svc.Quote(context.Background(), input, nil)
	require.ErrorContains(t, err, "折扣后金额过小")
}

type inviterLookupFunc func(context.Context, int64, Campaign) (int64, error)

func (f inviterLookupFunc) EligibleInviter(ctx context.Context, id int64, a Campaign) (int64, error) {
	return f(ctx, id, a)
}

func TestCampaignQuoteFreezesCappedRewardAndUsesReadOnlyInviterPort(t *testing.T) {
	a := testCampaign()
	a.Kind, a.Percent, a.RewardPercent, a.RewardCap, a.FreezeHours, a.NewInviteesOnly = "discount", 90, 5, 3, 72, true
	enabled := true
	svc := NewService(&fakeRepository{items: []Campaign{a}}, func(context.Context) bool { return enabled })
	svc.now = func() time.Time { return a.StartsAt }
	input := QuoteInput{UserID: 42, Amount: 100, Multiplier: 2, NativeCredited: 200}
	type testContextKey struct{}
	ctx := context.WithValue(context.Background(), testContextKey{}, "host-quote-context")
	calls := 0
	lookup := inviterLookupFunc(func(got context.Context, id int64, campaign Campaign) (int64, error) {
		calls++
		require.Equal(t, ctx, got)
		require.Equal(t, input.UserID, id)
		require.Equal(t, a.StartsAt, campaign.StartsAt)
		require.True(t, campaign.NewInviteesOnly)
		return 99, nil
	})
	snap, err := svc.Quote(ctx, input, lookup)
	require.NoError(t, err)
	require.Equal(t, 3.0, snap.Reward)
	require.Equal(t, 90.0, snap.Principal)
	require.Equal(t, int64(99), snap.InviterID)
	require.Equal(t, 72, snap.Campaign.FreezeHours)
	require.Equal(t, 1, calls)
	enabled = false
	snap, err = svc.Quote(ctx, input, lookup)
	require.NoError(t, err)
	require.Zero(t, snap.InviterID)
	require.Zero(t, snap.Reward)
	require.Equal(t, 1, calls, "disabled affiliate rewards must not touch the inviter port")
	enabled = true
	_, err = svc.Quote(ctx, input, nil)
	require.ErrorContains(t, err, "邀请关系查询不可用")
	wantErr := errors.New("inviter lookup failed")
	_, err = svc.Quote(ctx, input, inviterLookupFunc(func(context.Context, int64, Campaign) (int64, error) { return 0, wantErr }))
	require.ErrorIs(t, err, wantErr)
	snap, err = svc.Quote(ctx, input, inviterLookupFunc(func(context.Context, int64, Campaign) (int64, error) { return 0, nil }))
	require.NoError(t, err)
	require.Zero(t, snap.Reward)
}

func TestSnapshotRoundTripPreservesLegacyWireShapeWithoutCatalogOrFlags(t *testing.T) {
	legacy := `{"campaign":{"revision":"frozen","id":7,"name":"old","description":"","enabled":true,"starts_at":"2020-01-01T00:00:00Z","ends_at":"2020-02-01T00:00:00Z","kind":"bonus","percent":10,"min_amount":0,"reward_percent":5,"reward_cap":3,"freeze_hours":72,"new_invitees_only":true},"principal":50,"credited":55,"inviter_id":99,"reward":2.5}`
	var value any
	require.NoError(t, json.Unmarshal([]byte(legacy), &value))
	snap, err := ReadSnapshot(map[string]any{SnapshotKey: value, "provider_secret": "not-part-of-public-snapshot"})
	require.NoError(t, err)
	require.Equal(t, 55.0, snap.Credited)
	require.Equal(t, 2.5, snap.Reward)
	require.False(t, snap.Campaign.Active(time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)))
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	require.Equal(t, legacy, string(raw), "persisted field identity and order remain unchanged")
	for _, provider := range []map[string]any{nil, {}, {"provider": "native"}} {
		snap, err := ReadSnapshot(provider)
		require.NoError(t, err)
		require.Nil(t, snap)
	}
	_, err = ReadSnapshot(map[string]any{SnapshotKey: "malformed"})
	require.ErrorContains(t, err, "invalid campaign snapshot")
	_, err = ReadSnapshot(map[string]any{SnapshotKey: make(chan int)})
	require.Error(t, err)
}

func TestSnapshotAttachmentUsesOrderCreationBoundaryAndKeepsProviderData(t *testing.T) {
	a := testCampaign()
	snap := &Snapshot{Campaign: a, Principal: 50, Credited: 55}
	provider := map[string]any{"provider_key": "native", "credential": "opaque"}
	attached, err := AttachSnapshot(provider, nil, a.EndsAt)
	require.NoError(t, err)
	require.Equal(t, provider, attached)
	attached, err = AttachSnapshot(provider, snap, a.StartsAt)
	require.NoError(t, err)
	require.Same(t, snap, attached[SnapshotKey])
	require.Equal(t, "native", attached["provider_key"])
	require.Equal(t, "opaque", attached["credential"])
	attached, err = AttachSnapshot(nil, snap, a.EndsAt.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.NotNil(t, attached)
	for _, now := range []time.Time{a.StartsAt.Add(-time.Nanosecond), a.EndsAt} {
		_, err = AttachSnapshot(map[string]any{}, snap, now)
		require.ErrorContains(t, err, "活动已结束")
	}
	_, err = ReadSnapshot(attached)
	require.NoError(t, err, "saved snapshots are still readable after eligibility expires")
}
