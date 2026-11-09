package wiring

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/invoices"
)

// ProvideInvoices wires the invoice application module on the shared database.
func ProvideInvoices(db *sql.DB) *invoices.Handler {
	return &invoices.Handler{Store: &invoices.Store{DB: db}}
}
