package rechargecampaigns

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"time"
)

// Repository is restricted to this module's configuration table. It does not
// own payment orders, balances, coupon reservations, rewards or refunds.
type Repository interface {
	List(context.Context) ([]Campaign, error)
	Save(context.Context, Campaign) (*Campaign, error)
}

type Service struct {
	repository       Repository
	affiliateEnabled func(context.Context) bool
	now              func() time.Time
}

func NewService(repository Repository, affiliateEnabled func(context.Context) bool) *Service {
	return &Service{repository: repository, affiliateEnabled: affiliateEnabled, now: time.Now}
}

func (s *Service) List(ctx context.Context, public bool) ([]Campaign, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Campaign, 0, len(items))
	now := s.now()
	for _, a := range items {
		a.Revision = Revision(a)
		// Upcoming activities are intentionally visible to the public ticker.
		if !public || (a.Enabled && a.EndsAt.After(now)) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *Service) Save(ctx context.Context, a Campaign) (*Campaign, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if a.Enabled && a.RewardPercent > 0 && (s.affiliateEnabled == nil || !s.affiliateEnabled(ctx)) {
		return nil, infraerrors.BadRequest("AFFILIATE_DISABLED", "请先在系统设置开启邀请返利，再配置活动邀请奖励")
	}
	a.Revision = ""
	saved, err := s.repository.Save(ctx, a)
	if err != nil {
		return nil, err
	}
	saved.Revision = Revision(*saved)
	return saved, nil
}
