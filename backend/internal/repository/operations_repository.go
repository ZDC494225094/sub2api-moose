package repository

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// operationsRepository owns extension queries; the native usage repository is
// not extended with operations methods or optional runtime type assertions.
type operationsRepository struct {
	sql sqlExecutor
}

var _ service.OperationsRepository = (*operationsRepository)(nil)

func NewOperationsRepository(db *sql.DB) service.OperationsRepository {
	return &operationsRepository{sql: db}
}
