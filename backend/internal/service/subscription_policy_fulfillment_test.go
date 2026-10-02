//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSubscriptionOrderCreationFreezesPolicy(t *testing.T) {
	for _, policy := range []string{subscriptionextensions.Native, subscriptionextensions.Independent, "unavailable"} {
		t.Run(policy, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			user, err := client.User.Create().SetEmail("policy@test.invalid").SetPasswordHash("hash").Save(ctx)
			require.NoError(t, err)
			sub, _ := nativeIssuanceFixture()
			calls := 0
			sub.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) {
				calls++
				if policy == "unavailable" {
					return "", errors.New("offline")
				}
				return policy, nil
			})
			svc := &PaymentService{entClient: client, subscriptionSvc: sub}
			order, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: user.ID, PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeSubscription}, &User{ID: user.ID, Email: user.Email}, &dbent.SubscriptionPlan{ID: 1, GroupID: 1, ValidityDays: 30, ValidityUnit: "day"}, &PaymentConfig{MaxPendingOrders: 10, OrderTimeoutMin: 30}, 10, 10, 0, 10, nil)
			require.Equal(t, 1, calls)
			if policy == "unavailable" {
				require.Error(t, err)
				n, countErr := client.PaymentOrder.Query().Count(ctx)
				require.NoError(t, countErr)
				require.Zero(t, n)
				return
			}
			require.NoError(t, err)
			require.Equal(t, policy, order.ProviderSnapshot[subscriptionextensions.SnapshotKey])
		})
	}
}
func TestSubscriptionPaymentFulfillmentUsesFrozenPolicyAndIsIdempotent(t *testing.T) {
	for _, policy := range []string{"", subscriptionextensions.Native, subscriptionextensions.Independent, "invalid"} {
		t.Run("policy="+policy, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			user, err := client.User.Create().SetEmail("fulfill@test.invalid").SetPasswordHash("hash").Save(ctx)
			require.NoError(t, err)
			sub, repo := nativeIssuanceFixture()
			sub.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) {
				t.Fatal("historical payment consulted live flag")
				return "", errors.New("outage")
			})
			oldExpiry := time.Now().AddDate(0, 0, 20)
			repo.seed(&UserSubscription{ID: 100, UserID: user.ID, GroupID: 1, CustomSubscriptionPolicy: subscriptionextensions.Native, Status: SubscriptionStatusActive, ExpiresAt: oldExpiry})
			repo.seed(&UserSubscription{ID: 200, UserID: user.ID, GroupID: 1, CustomSubscriptionPolicy: subscriptionextensions.Independent, Status: SubscriptionStatusActive, ExpiresAt: oldExpiry, DailyUsageUSD: 9})
			b := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName("user").SetAmount(10).SetPayAmount(10).SetFeeRate(0).SetRechargeCode("PAY-POLICY").SetOutTradeNo("policy-1").SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-policy").SetOrderType(payment.OrderTypeSubscription).SetPlanID(1).SetSubscriptionGroupID(1).SetSubscriptionDays(30).SetStatus(OrderStatusPaid).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("test.invalid")
			if policy != "" {
				b.SetProviderSnapshot(map[string]any{subscriptionextensions.SnapshotKey: policy})
			}
			order, err := b.Save(ctx)
			require.NoError(t, err)
			svc := &PaymentService{entClient: client, groupRepo: &subscriptionGroupRepoStub{group: &Group{ID: 1, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}}, subscriptionSvc: sub}
			err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
			if policy == "invalid" {
				require.Error(t, err)
				require.Zero(t, repo.createCalls)
				return
			}
			require.NoError(t, err)
			current, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, current.Status)
			require.NotNil(t, current.SubscriptionID)
			if policy == subscriptionextensions.Native {
				require.Equal(t, int64(100), *current.SubscriptionID)
				require.Zero(t, repo.createCalls)
				require.Equal(t, oldExpiry.AddDate(0, 0, 30), repo.byID[100].ExpiresAt)
			} else {
				require.Equal(t, 1, repo.createCalls)
				require.Equal(t, subscriptionextensions.Independent, repo.byID[*current.SubscriptionID].CustomSubscriptionPolicy)
			}
			require.Equal(t, float64(9), repo.byID[200].DailyUsageUSD)
			expiry := repo.byID[*current.SubscriptionID].ExpiresAt
			creates := repo.createCalls
			require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))
			require.Equal(t, creates, repo.createCalls)
			require.Equal(t, expiry, repo.byID[*current.SubscriptionID].ExpiresAt)
		})
	}
}

type policyCodeRepo struct {
	paymentOrderLifecycleRedeemRepo
	created []RedeemCode
}

func (r *policyCodeRepo) Create(_ context.Context, c *RedeemCode) error {
	r.created = append(r.created, *c)
	return nil
}
func (r *policyCodeRepo) CreateBatch(_ context.Context, c []RedeemCode) error {
	r.created = append(r.created, c...)
	return nil
}
func TestSubscriptionCodesFreezePolicyAtCreation(t *testing.T) {
	for _, policy := range []string{subscriptionextensions.Native, subscriptionextensions.Independent} {
		t.Run(policy, func(t *testing.T) {
			sub, _ := nativeIssuanceFixture()
			calls := 0
			sub.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) { calls++; return policy, nil })
			repo := &policyCodeRepo{}
			svc := NewRedeemService(repo, nil, sub, nil, nil, nil, nil, nil)
			codes, err := svc.GenerateCodes(context.Background(), GenerateCodesRequest{Type: RedeemTypeSubscription, Value: 1, Count: 3})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			for _, code := range codes {
				require.Equal(t, policy, code.CustomSubscriptionPolicy)
			}
			code := &RedeemCode{Code: "explicit", Type: RedeemTypeSubscription, Value: 1, CustomSubscriptionPolicy: "client-override"}
			require.NoError(t, svc.CreateCode(context.Background(), code))
			require.Equal(t, policy, code.CustomSubscriptionPolicy)
			sub.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) { return "", errors.New("offline") })
			before := len(repo.created)
			_, err = svc.GenerateCodes(context.Background(), GenerateCodesRequest{Type: RedeemTypeSubscription, Value: 1, Count: 2})
			require.Error(t, err)
			require.Equal(t, before, len(repo.created))
		})
	}
}
func TestSubscriptionRedeemUsesPersistedPolicy(t *testing.T) {
	for _, policy := range []string{"", subscriptionextensions.Native, subscriptionextensions.Independent} {
		t.Run("policy="+policy, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			sub, repo := nativeIssuanceFixture()
			sub.issuanceAdmission = subscriptionPolicyAdmissionFunc(func(context.Context) (string, error) {
				t.Fatal("redemption read live flag")
				return "", errors.New("offline")
			})
			gid := int64(1)
			code := &RedeemCode{ID: 1, Code: "POLICY", Type: RedeemTypeSubscription, Status: StatusUnused, Value: 1, GroupID: &gid, ValidityDays: 30, CustomSubscriptionPolicy: policy}
			codes := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{code.Code: code}}
			svc := NewRedeemService(codes, &mockUserRepo{getByIDUser: &User{ID: 7}}, sub, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil)
			got, err := svc.Redeem(ctx, 7, code.Code)
			require.NoError(t, err)
			require.Equal(t, StatusUsed, got.Status)
			require.Equal(t, 1, repo.createCalls)
			want, _ := subscriptionextensions.Resolve(policy)
			for _, grant := range repo.byID {
				require.Equal(t, want, grant.CustomSubscriptionPolicy, fmt.Sprint(grant.ID))
			}
			_, err = svc.Redeem(ctx, 7, code.Code)
			require.ErrorIs(t, err, ErrRedeemCodeUsed)
			require.Equal(t, 1, repo.createCalls)
		})
	}
}
