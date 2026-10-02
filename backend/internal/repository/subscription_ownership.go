package repository

import (
	"context"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func subscriptionPolicyOrLegacy(policy string) string {
	if policy == "" {
		return subscriptionextensions.Independent
	}
	return policy
}
func (r *userSubscriptionRepository) LockSubscriptionOwner(ctx context.Context, userID int64) error {
	if dbent.TxFromContext(ctx) == nil {
		return fmt.Errorf("subscription owner lock requires a transaction")
	}
	_, err := clientFromContext(ctx, r.client).User.Query().Where(user.IDEQ(userID)).ForUpdate().Only(ctx)
	return err
}
func (r *userSubscriptionRepository) FindNativeForAssignment(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	if dbent.TxFromContext(ctx) == nil {
		return nil, fmt.Errorf("native subscription lookup requires a transaction")
	}
	entity, err := clientFromContext(ctx, r.client).UserSubscription.Query().Where(usersubscription.UserIDEQ(userID), usersubscription.GroupIDEQ(groupID), usersubscription.CustomSubscriptionPolicyEQ(subscriptionextensions.Native)).Order(dbent.Desc(usersubscription.FieldID)).First(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSubscriptionNotFound, nil)
	}
	return userSubscriptionEntityToService(entity), nil
}
