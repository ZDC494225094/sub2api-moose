package wiring

import (
	"context"
	"database/sql"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	_ "modernc.org/sqlite"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type marketingStateProbe struct {
	enabled bool
	err     error
	calls   int
	ctx     context.Context
}

func (p *marketingStateProbe) Enabled(ctx context.Context, id string) (bool, error) {
	p.calls++
	p.ctx = ctx
	if id != marketingModuleID {
		panic("wrong module")
	}
	return p.enabled, p.err
}
func marketingManifest(managed bool) func(string) (customize.Manifest, bool) {
	return func(id string) (customize.Manifest, bool) { return customize.Manifest{ID: id, Managed: managed}, true }
}

func TestMarketingHostAdmissionReadsEveryManagedOperation(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "native transaction context")
	reader := &marketingStateProbe{enabled: true}
	gate := &marketingAdmission{states: reader, lookup: marketingManifest(true)}
	require.NoError(t, gate.RequireNewBusiness(ctx))
	reader.enabled = false
	require.EqualError(t, gate.RequireNewBusiness(ctx), customize.Disabled(marketingModuleID).Error())
	reader.err = errors.New("shared settings unavailable")
	require.ErrorIs(t, gate.RequireNewBusiness(ctx), reader.err)
	reader.err = nil
	reader.enabled = true
	require.NoError(t, gate.RequireNewBusiness(ctx))
	require.Equal(t, 4, reader.calls)
	require.Equal(t, ctx, reader.ctx)
}

func TestMarketingHostAdmissionRejectsUnmanagedAndBrokenRegistration(t *testing.T) {
	reader := &marketingStateProbe{err: errors.New("must not read pending flag")}
	gate := &marketingAdmission{states: reader, lookup: marketingManifest(false)}
	require.ErrorIs(t, gate.RequireNewBusiness(context.Background()), marketing.ErrMarketingAdmissionUnavailable)
	require.Zero(t, reader.calls)
	for _, broken := range []*marketingAdmission{
		nil,
		{},
		{lookup: func(string) (customize.Manifest, bool) { return customize.Manifest{}, false }},
		{lookup: func(string) (customize.Manifest, bool) { return customize.Manifest{ID: "wrong"}, true }},
		{lookup: marketingManifest(true)},
	} {
		require.ErrorIs(t, broken.RequireNewBusiness(context.Background()), marketing.ErrMarketingAdmissionUnavailable)
	}
	// The actual factory/catalog must be managed and reject unavailable state.
	manifest, exists := customize.Lookup(marketingModuleID)
	require.True(t, exists)
	require.True(t, manifest.Managed)
	require.Error(t, ProvideMarketingAdmission(nil).RequireNewBusiness(context.Background()))
}

func TestMarketingProductionProvidersPassAdmissionToBothServices(t *testing.T) {
	reader := &marketingStateProbe{enabled: false}
	gate := &marketingAdmission{states: reader, lookup: marketingManifest(true)}
	coupons := ProvideCouponService(nil, nil, nil, gate)
	lottery := ProvideLotteryService(nil, nil, nil, nil, nil, nil, nil, nil, nil, coupons, gate)
	ctx := context.Background()
	_, err := coupons.IssueCouponFromTemplate(ctx, 1, 2, "lottery", nil)
	require.EqualError(t, err, customize.Disabled(marketingModuleID).Error())
	_, err = coupons.PreviewCouponForOrder(ctx, marketing.ApplyPaymentCouponInput{})
	require.EqualError(t, err, customize.Disabled(marketingModuleID).Error())
	_, err = coupons.ReserveCouponForOrder(ctx, 3, marketing.ApplyPaymentCouponInput{})
	require.EqualError(t, err, customize.Disabled(marketingModuleID).Error())
	_, err = lottery.Draw(ctx, marketing.LotteryDrawInput{})
	require.EqualError(t, err, customize.Disabled(marketingModuleID).Error())
	require.Equal(t, 4, reader.calls, "nil repositories and Ent must never be reached")
}

type historicalCouponRows struct {
	service.UserCouponRepository
	used, released int
}

func (r *historicalCouponRows) MarkUsedByOrderID(context.Context, int64, time.Time) error {
	r.used++
	return nil
}
func (r *historicalCouponRows) ReleaseReservationByOrderID(context.Context, int64, time.Time) error {
	r.released++
	return nil
}

type historicalDiscountRows struct {
	service.PaymentOrderDiscountRepository
	used, released int
}

func (r *historicalDiscountRows) MarkUsedByOrderID(context.Context, int64, time.Time) error {
	r.used++
	return nil
}
func (r *historicalDiscountRows) MarkReleasedByOrderID(context.Context, int64, time.Time) error {
	r.released++
	return nil
}

func TestMarketingProductionProviderPreservesHistoricalSettlementWhenDisabled(t *testing.T) {
	reader := &marketingStateProbe{err: errors.New("switch unavailable")}
	gate := &marketingAdmission{states: reader, lookup: marketingManifest(true)}
	coupons, discounts := &historicalCouponRows{}, &historicalDiscountRows{}
	svc := ProvideCouponService(nil, coupons, discounts, gate)
	require.NoError(t, svc.ConsumeReservedCouponByOrderID(context.Background(), 31))
	require.NoError(t, svc.ReleaseCouponReservationByOrderID(context.Background(), 32))
	require.Equal(t, 1, coupons.used)
	require.Equal(t, 1, discounts.used)
	require.Equal(t, 1, coupons.released)
	require.Equal(t, 1, discounts.released)
	require.Zero(t, reader.calls)
}

func TestMarketingSettingsReadReusesTransactionConnection(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY, key TEXT UNIQUE NOT NULL, value TEXT NOT NULL, updated_at DATETIME NOT NULL)")
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	txCtx := ent.NewTxContext(ctx, tx)
	key := customize.Key(marketingModuleID)
	_, err = tx.Client().Setting.Create().SetKey(key).SetValue("true").SetUpdatedAt(time.Now()).Save(txCtx)
	require.NoError(t, err)
	reader := marketingSettingReader{repository.NewSettingRepository(client)}
	// Use the real marketing catalog and production transaction-aware reader.
	// A single-connection pool detects accidental root-client reads.
	manager := customize.NewManager(reader)
	enabled, err := manager.Enabled(txCtx, marketingModuleID)
	require.NoError(t, err)
	require.True(t, enabled, "must see this transaction's uncommitted setting without a second connection")
	require.NoError(t, ProvideMarketingAdmission(repository.NewSettingRepository(client)).RequireNewBusiness(txCtx))
	require.NoError(t, tx.Rollback())
	values, err := reader.GetMultiple(ctx, []string{key})
	require.NoError(t, err)
	require.Empty(t, values, "admission must not commit or persist a transaction")
	_, err = (marketingSettingReader{}).GetMultiple(ctx, []string{key})
	require.ErrorIs(t, err, marketing.ErrMarketingAdmissionUnavailable)
}

type marketingPreviewRows struct {
	historicalCouponRows
	reads int
}

func (r *marketingPreviewRows) GetByID(context.Context, int64) (*service.UserCoupon, error) {
	r.reads++
	return &service.UserCoupon{ID: 7, UserID: 2, Scope: "universal", Status: "unused", DiscountAmount: 2, ThresholdAmount: 10}, nil
}

// Real catalog + real persisted settings + production providers, rather than a
// pretend managed lookup. Business rows are probes; SQL settings are real SQLite.
func TestMarketingManagedSwitchUsesPersistentSettingsWithoutInterruptingHistory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY, key TEXT UNIQUE NOT NULL, value TEXT NOT NULL, updated_at DATETIME NOT NULL)")
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	settings := repository.NewSettingRepository(client)
	manager := customize.NewManager(settings)
	gate := ProvideMarketingAdmission(settings)
	rows, discounts := &marketingPreviewRows{}, &historicalDiscountRows{}
	svc := ProvideCouponService(nil, rows, discounts, gate)
	ctx := context.Background()
	calls := 0
	for _, state := range []string{"missing", "true", "false", "malformed", "true", "offline"} {
		t.Run(state, func(t *testing.T) {
			switch state {
			case "true", "false":
				require.NoError(t, manager.SetEnabled(ctx, marketingModuleID, state == "true"))
			case "malformed":
				require.NoError(t, settings.Set(ctx, customize.Key(marketingModuleID), state))
			case "offline":
				_, err := db.Exec("DROP TABLE settings")
				require.NoError(t, err)
			}
			quote, err := svc.PreviewCouponForOrder(ctx, marketing.ApplyPaymentCouponInput{UserID: 2, UserCouponID: 7, OrderType: "balance", OrderAmount: 20})
			if state == "true" {
				require.NoError(t, err)
				require.Equal(t, 18.0, quote.DiscountedAmount)
				calls++
			} else {
				require.Error(t, err)
				require.Nil(t, quote)
			}
			require.Equal(t, calls, rows.reads, "denied new business must not reach coupon storage")
			require.NoError(t, svc.ConsumeReservedCouponByOrderID(ctx, 31))
			require.NoError(t, svc.ReleaseCouponReservationByOrderID(ctx, 32))
		})
	}
	require.Equal(t, 6, rows.used)
	require.Equal(t, 6, rows.released)
	require.Equal(t, 6, discounts.used)
	require.Equal(t, 6, discounts.released)
}
