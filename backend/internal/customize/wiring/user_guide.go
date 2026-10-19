package wiring

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/userguide"
)

func ProvideUserGuide(db *sql.DB) *userguide.Handler {
	return &userguide.Handler{Store: &userguide.Store{DB: db}}
}
