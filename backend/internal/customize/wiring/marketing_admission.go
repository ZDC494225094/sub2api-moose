package wiring

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const marketingModuleID = "marketing-tools"

type marketingStateReader interface {
	Enabled(context.Context, string) (bool, error)
}

type marketingAdmission struct {
	states marketingStateReader
	lookup func(string) (customize.Manifest, bool)
}

// Every new operation reads the managed setting. Missing registration, wiring
// or state fails closed; there is no pending/legacy bypass after adoption.
func (a *marketingAdmission) RequireNewBusiness(ctx context.Context) error {
	if a == nil || a.lookup == nil {
		return marketing.ErrMarketingAdmissionUnavailable
	}
	manifest, exists := a.lookup(marketingModuleID)
	if !exists || manifest.ID != marketingModuleID {
		return marketing.ErrMarketingAdmissionUnavailable
	}
	if !manifest.Managed {
		return marketing.ErrMarketingAdmissionUnavailable
	}
	if a.states == nil {
		return marketing.ErrMarketingAdmissionUnavailable
	}
	enabled, err := a.states.Enabled(ctx, marketingModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return customize.Disabled(marketingModuleID)
	}
	return nil
}

func ProvideMarketingAdmission(settings service.SettingRepository) marketing.NewBusinessAdmission {
	return &marketingAdmission{states: customize.NewManager(marketingSettingReader{settings}), lookup: customize.Lookup}
}

func ProvideCouponService(templates service.CouponTemplateRepository, coupons service.UserCouponRepository, discounts service.PaymentOrderDiscountRepository, admission marketing.NewBusinessAdmission) *service.CouponService {
	return marketing.NewCouponServiceWithAdmission(templates, coupons, discounts, admission)
}

func ProvideLotteryService(
	client *ent.Client,
	activities service.LotteryActivityRepository,
	prizes service.LotteryPrizeRepository,
	states service.LotteryUserStateRepository,
	chances service.LotteryChanceLogRepository,
	records service.LotteryDrawRecordRepository,
	progress service.LotteryConsumeProgressRepository,
	users service.UserRepository,
	redeem *service.RedeemService,
	coupons *service.CouponService,
	admission marketing.NewBusinessAdmission,
) *service.LotteryService {
	return service.NewLotteryServiceWithAdmission(client, activities, prizes, states, chances, records, progress, users, redeem, coupons, admission)
}

// The upstream setting repository reads from the root client. Admission inside
// a draw/payment transaction must reuse that transaction's connection rather
// than acquire another one (which can deadlock a saturated pool). Keep this
// adaptation here, without changing the upstream repository's semantics.
type marketingSettingReader struct{ service.SettingRepository }

func (r marketingSettingReader) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if tx := ent.TxFromContext(ctx); tx != nil {
		rows, err := tx.Client().Setting.Query().Where(setting.KeyIn(keys...)).All(ctx)
		if err != nil {
			return nil, err
		}
		values := make(map[string]string, len(rows))
		for _, row := range rows {
			values[row.Key] = row.Value
		}
		return values, nil
	}
	if r.SettingRepository == nil {
		return nil, marketing.ErrMarketingAdmissionUnavailable
	}
	return r.SettingRepository.GetMultiple(ctx, keys)
}
