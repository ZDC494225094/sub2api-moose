package wiring

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/stretchr/testify/require"
)

// Real catalog + real persisted settings through the production provider.
func TestKeyRoutingAdmissionReadsPersistentSwitchEveryTime(t *testing.T) {
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
	gate := ProvideKeyRoutingAdmission(settings)
	ctx := context.Background()

	manifest, ok := customize.Lookup(multigroupbilling.ModuleID)
	require.True(t, ok)
	require.True(t, manifest.Managed)

	enabled, err := gate.MultiGroupEnabled(ctx)
	require.NoError(t, err)
	require.False(t, enabled, "missing flag means new keys are upstream-routed")
	require.NoError(t, manager.SetEnabled(ctx, multigroupbilling.ModuleID, true))
	enabled, err = gate.MultiGroupEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled, "switch changes apply without restart")
	require.NoError(t, settings.Set(ctx, customize.Key(multigroupbilling.ModuleID), "malformed"))
	_, err = gate.MultiGroupEnabled(ctx)
	require.Error(t, err, "invalid state must not be read as enabled or disabled")
	_, err = db.Exec("DROP TABLE settings")
	require.NoError(t, err)
	_, err = gate.MultiGroupEnabled(ctx)
	require.Error(t, err)
}

func TestKeyRoutingAdmissionRejectsBrokenRegistration(t *testing.T) {
	probe := &marketingStateProbe{enabled: true}
	for _, broken := range []*keyRoutingAdmission{
		nil,
		{},
		{states: probe},
		{states: probe, lookup: func(string) (customize.Manifest, bool) { return customize.Manifest{}, false }},
		{states: probe, lookup: func(id string) (customize.Manifest, bool) { return customize.Manifest{ID: id}, true }},
	} {
		_, err := broken.MultiGroupEnabled(context.Background())
		require.ErrorIs(t, err, errKeyRoutingAdmissionUnavailable)
	}
	require.Zero(t, probe.calls)
}
