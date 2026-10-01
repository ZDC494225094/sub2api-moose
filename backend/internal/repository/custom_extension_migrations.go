package repository

import (
	"context"
	"database/sql"

	extensionmigrations "github.com/Wei-Shaw/sub2api/internal/customize/migrations"
)

// The host supplies a single locked connection. Keep extension provenance,
// registration, checksums and SQL outside the upstream migration implementation.
type extensionMigrationLifecycle interface {
	Prepare(context.Context, *sql.Conn) error
	Apply(context.Context, *sql.Conn) error
}

func builtInExtensionMigrations() extensionMigrationLifecycle { return extensionmigrations.Runner{} }
