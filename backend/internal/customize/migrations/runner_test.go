package migrations

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Isolated transactional SQL fixtures, never the user's PostgreSQL database.
// PostgreSQL advisory-lock composition is covered in repository tests.
func fixture(t *testing.T) *sql.Conn {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); _ = db.Close() })
	execute(t, conn, "CREATE TABLE schema_migrations (filename TEXT PRIMARY KEY, checksum TEXT NOT NULL)")
	execute(t, conn, "CREATE TABLE api_keys (id INTEGER PRIMARY KEY)")
	return conn
}
func execute(t *testing.T, conn *sql.Conn, statement string, args ...any) {
	t.Helper()
	_, err := conn.ExecContext(context.Background(), statement, args...)
	require.NoError(t, err)
}
func createSettings(t *testing.T, conn *sql.Conn) {
	t.Helper()
	execute(t, conn, "CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)")
}
func installation(t *testing.T, conn *sql.Conn) string {
	t.Helper()
	var kind string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT installation_kind FROM custom_extension_installation WHERE id=1").Scan(&kind))
	return kind
}
func flags(t *testing.T, conn *sql.Conn) map[string]string {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), "SELECT key, value FROM settings")
	require.NoError(t, err)
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var key, value string
		require.NoError(t, rows.Scan(&key, &value))
		result[key] = value
	}
	require.NoError(t, rows.Err())
	return result
}
func recorded(t *testing.T, conn *sql.Conn) int {
	t.Helper()
	var count int
	require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM custom_extension_schema_migrations").Scan(&count))
	return count
}

func TestInstallationProvenanceAndDefaultFlags(t *testing.T) {
	for _, tc := range []struct{ name, previous, kind, enabled string }{
		{"fresh", "", "native", "false"},
		{"existing_upstream", "001_init.sql", "native", "false"},
		{"legacy_early_fork", "138_marketing_lottery_coupon.sql", "legacy", "true"},
		{"legacy_campaigns", "240_recharge_campaigns.sql", "legacy", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, conn := context.Background(), fixture(t)
			if tc.previous != "" {
				execute(t, conn, "INSERT INTO schema_migrations VALUES ($1, 'unchanged')", tc.previous)
			}
			require.NoError(t, (Runner{}).Prepare(ctx, conn))
			require.Equal(t, tc.kind, installation(t, conn))
			// Host migrations can add fork tables before a failed startup retries.
			execute(t, conn, "INSERT INTO schema_migrations VALUES ('239_custom_api_key_platform_sync.sql', 'unchanged') ON CONFLICT DO NOTHING")
			require.NoError(t, (Runner{}).Prepare(ctx, conn))
			require.Equal(t, tc.kind, installation(t, conn), "retry must not reclassify a fresh installation")
			createSettings(t, conn)
			require.NoError(t, (Runner{}).Apply(ctx, conn))
			got := flags(t, conn)
			require.Len(t, got, 9, "only separately adopted modules get flags")
			for _, id := range []string{"premium-home", "playground", "infinite-canvas", "operations-analytics", "recharge-campaigns", "customer-support"} {
				require.Equal(t, tc.enabled, got[customize.Key(id)], id)
			}
			require.Equal(t, tc.enabled, got[customize.Key("marketing-tools")])
			require.Equal(t, tc.enabled, got[customize.Key("multi-group-billing")])
			require.Equal(t, tc.enabled, got[customize.Key("admin-efficiency")])
			require.Equal(t, 5, recorded(t, conn))
			require.NoError(t, (Runner{}).Apply(ctx, conn))
			require.Equal(t, got, flags(t, conn))
			require.Equal(t, 5, recorded(t, conn))
			var original string
			require.NoError(t, conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename='239_custom_api_key_platform_sync.sql'").Scan(&original))
			require.Equal(t, "unchanged", original, "do not rewrite host history")
		})
	}
}

func TestAdoptionPreservesExplicitChoicesAndNeverReenablesDeletedFlags(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	execute(t, conn, "INSERT INTO schema_migrations VALUES ('240_recharge_campaigns.sql', 'unchanged')")
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	createSettings(t, conn)
	for key, value := range map[string]string{
		customize.Key("premium-home"): "false", customize.Key("playground"): "true",
		customize.Key("recharge-campaigns"): "malformed", "native_setting": "keep",
	} {
		execute(t, conn, "INSERT INTO settings (key,value) VALUES ($1,$2)", key, value)
	}
	require.NoError(t, (Runner{}).Apply(ctx, conn))
	got := flags(t, conn)
	require.Equal(t, "false", got[customize.Key("premium-home")])
	require.Equal(t, "true", got[customize.Key("playground")])
	require.Equal(t, "malformed", got[customize.Key("recharge-campaigns")], "do not silently repair an invalid flag by enabling it")
	require.Equal(t, "keep", got["native_setting"])
	execute(t, conn, "DELETE FROM settings WHERE key=$1", customize.Key("customer-support"))
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	require.NoError(t, (Runner{}).Apply(ctx, conn))
	require.NotContains(t, flags(t, conn), customize.Key("customer-support"), "completed adoption never seeds flags again")
}

func TestFailedHostAdoptionRollsBackItsFlagsAndCanRetry(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	createSettings(t, conn)
	execute(t, conn, `CREATE TRIGGER reject_campaign BEFORE INSERT ON settings
      WHEN NEW.key = 'custom_extensions.recharge-campaigns.enabled'
      BEGIN SELECT RAISE(FAIL, 'fixture write failed'); END`)
	require.ErrorContains(t, (Runner{}).Apply(ctx, conn), "fixture write failed")
	// Each module commits independently; the alphabetically earlier admin migration survives.
	require.Equal(t, map[string]string{customize.Key("admin-efficiency"): "false"}, flags(t, conn))
	require.Equal(t, 1, recorded(t, conn))
	execute(t, conn, "DROP TRIGGER reject_campaign")
	require.NoError(t, (Runner{}).Apply(ctx, conn))
	require.Len(t, flags(t, conn), 9)
	require.Equal(t, 5, recorded(t, conn))
}

func TestCustomMigrationChecksumAndLineEndingStability(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	createSettings(t, conn)
	require.NoError(t, (Runner{}).Apply(ctx, conn))
	names, err := fs.Glob(migrationFiles, "sql/*/*.sql")
	require.NoError(t, err)
	name := names[0]
	raw, err := fs.ReadFile(migrationFiles, name)
	require.NoError(t, err)
	lf := strings.ReplaceAll(string(raw), "\r\n", "\n")
	crlf := fstest.MapFS{name: &fstest.MapFile{Data: []byte(strings.ReplaceAll(lf, "\n", "\r\n"))}}
	require.NoError(t, applyFS(ctx, conn, crlf))
	changed := fstest.MapFS{name: &fstest.MapFile{Data: []byte(lf + "\n-- changed historical migration")}}
	require.ErrorContains(t, applyFS(ctx, conn, changed), "checksum mismatch")
	require.Equal(t, 5, recorded(t, conn))
	require.Len(t, flags(t, conn), 9)
}

func TestMigrationRegistrationAndMissingProvenanceFailClosed(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	require.Error(t, (Runner{}).Prepare(ctx, nil))
	require.Error(t, (Runner{}).Apply(ctx, nil))
	require.ErrorContains(t, (Runner{}).Apply(ctx, conn), "provenance")
	for _, name := range []string{"sql/Bad/202609260001_test.sql", "sql/good/001_test.sql", "sql/good/202609260001_test_notx.sql"} {
		_, err := registeredMigrations(fstest.MapFS{name: &fstest.MapFile{Data: []byte("SELECT 1;")}})
		require.ErrorContains(t, err, "invalid custom migration name")
	}
	_, err := registeredMigrations(fstest.MapFS{"sql/good/202609260001_empty.sql": &fstest.MapFile{Data: []byte(" ")}})
	require.ErrorContains(t, err, "empty custom migration")
}
