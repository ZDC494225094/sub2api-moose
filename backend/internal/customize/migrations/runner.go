// Package migrations owns only new business-extension migrations. Historical
// SQL files remain in the upstream migration directory with their original names.
package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed sql/*/*.sql
var migrationFiles embed.FS

// Runner is called on the host's pinned connection while its migration advisory
// lock is held. It never acquires another connection or changes the host ledger.
type Runner struct{}

const installationDDL = `CREATE TABLE IF NOT EXISTS custom_extension_installation (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    installation_kind TEXT NOT NULL CHECK (installation_kind IN ('native', 'legacy')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// This frozen list identifies already-installed versions of this fork, not
// generic upstream installations. Evaluate BEFORE applying this release's host
// migrations, otherwise a fresh install would appear to be a legacy deployment.
const rememberInstallation = `INSERT INTO custom_extension_installation (id, installation_kind)
SELECT 1, CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE filename IN (
        '138_marketing_lottery_coupon.sql',
        '145_subscription_multi_instance_api_key_groups.sql',
        '146_api_key_platform.sql',
        '147_payment_order_subscription_id.sql',
        '159_operations_marketing_email_records.sql',
        '174_add_account_sort_order.sql',
        '175_add_account_upstream_group.sql',
        '176_clear_inferred_account_upstream_groups.sql',
        '177_create_account_upstream_group_directory.sql',
        '185_group_billing_rate_sync.sql',
        '191_playground_video_asset_urls.sql',
        '194_marketing_redeem_codes.sql',
        '239_custom_api_key_platform_sync.sql',
        '240_recharge_campaigns.sql'
    )
) THEN 'legacy' ELSE 'native' END
ON CONFLICT (id) DO NOTHING`

// Prepare durably remembers provenance even if a later host migration fails.
// Retrying never reclassifies a native installation after it acquired fork SQL.
func (Runner) Prepare(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return errors.New("custom migrations: nil connection")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin custom installation detection: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{installationDDL, rememberInstallation} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("remember custom installation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit custom installation detection: %w", err)
	}
	return nil
}

const ledgerDDL = `CREATE TABLE IF NOT EXISTS custom_extension_schema_migrations (
    module_id TEXT NOT NULL,
    version TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (module_id, version)
)`

var moduleName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var migrationName = regexp.MustCompile(`^[0-9]{12}_[a-z][a-z0-9_]*\.sql$`)

type migration struct{ module, version, content, checksum string }

func registeredMigrations(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "sql/*/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list custom migrations: %w", err)
	}
	sort.Strings(names)
	result := make([]migration, 0, len(names))
	for _, name := range names {
		module, version := path.Base(path.Dir(name)), path.Base(name)
		if !moduleName.MatchString(module) || !migrationName.MatchString(version) || strings.HasSuffix(version, "_notx.sql") {
			return nil, fmt.Errorf("invalid custom migration name: %s (transactional, module-scoped migrations only)", name)
		}
		raw, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read custom migration %s: %w", name, err)
		}
		// Identical checkouts must retain checksums across Windows and Linux.
		content := strings.TrimSpace(strings.ReplaceAll(string(raw), "\r\n", "\n"))
		if content == "" {
			return nil, fmt.Errorf("empty custom migration: %s", name)
		}
		sum := sha256.Sum256([]byte(content))
		result = append(result, migration{module, strings.TrimSuffix(version, ".sql"), content, hex.EncodeToString(sum[:])})
	}
	return result, nil
}

// Apply runs even for disabled modules: switching a feature off never drops its
// data/schema or prevents migration of historical orders and entitlements.
func (Runner) Apply(ctx context.Context, conn *sql.Conn) error {
	return applyFS(ctx, conn, migrationFiles)
}

func applyFS(ctx context.Context, conn *sql.Conn, files fs.FS) error {
	if conn == nil {
		return errors.New("custom migrations: nil connection")
	}
	migrations, err := registeredMigrations(files)
	if err != nil {
		return err
	}
	var kind string
	if err := conn.QueryRowContext(ctx, "SELECT installation_kind FROM custom_extension_installation WHERE id = 1").Scan(&kind); err != nil {
		return fmt.Errorf("read custom installation provenance: %w", err)
	}
	if kind != "native" && kind != "legacy" {
		return fmt.Errorf("invalid custom installation provenance: %s", kind)
	}
	if _, err := conn.ExecContext(ctx, ledgerDDL); err != nil {
		return fmt.Errorf("create custom migration ledger: %w", err)
	}
	for _, item := range migrations {
		var applied string
		err := conn.QueryRowContext(ctx, "SELECT checksum FROM custom_extension_schema_migrations WHERE module_id = $1 AND version = $2", item.module, item.version).Scan(&applied)
		if err == nil {
			if applied != item.checksum {
				return fmt.Errorf("custom migration %s/%s checksum mismatch; restore the original file and add a new migration", item.module, item.version)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read custom migration %s/%s: %w", item.module, item.version, err)
		}
		if err := applyOne(ctx, conn, item); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, conn *sql.Conn, item migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin custom migration %s/%s: %w", item.module, item.version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, item.content); err != nil {
		return fmt.Errorf("apply custom migration %s/%s: %w", item.module, item.version, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO custom_extension_schema_migrations (module_id, version, checksum) VALUES ($1, $2, $3)", item.module, item.version, item.checksum); err != nil {
		return fmt.Errorf("record custom migration %s/%s: %w", item.module, item.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit custom migration %s/%s: %w", item.module, item.version, err)
	}
	return nil
}
