package service

import (
	"context"
	"time"
)

// OperationsRepository is the read-only port owned by the operations-analytics
// extension. Keeping it separate avoids growing the upstream usage/dashboard ports.
type OperationsRepository interface {
	GetOperationsFunnel(context.Context, time.Time, time.Time) (*OperationsFunnelStats, error)
	ListOperationsUserDetails(context.Context, OperationsUserDetailFilter) ([]OperationsUserDetail, int64, error)
	GetOperationsFinance(context.Context, time.Time, time.Time) (*OperationsFinanceResponse, error)
	GetOperationsCustomers(context.Context, OperationsCustomerFilter) (*OperationsCustomersResponse, error)
}

// OperationsService does not decorate or replace the upstream DashboardService.
// Disabling its routes leaves the upstream dashboard implementation untouched.
type OperationsService struct {
	repository OperationsRepository
}

func NewOperationsService(repository OperationsRepository) *OperationsService {
	return &OperationsService{repository: repository}
}
