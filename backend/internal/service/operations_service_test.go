package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type operationsPortStub struct {
	OperationsRepository
	calls int
	err   error
}

func (r *operationsPortStub) GetOperationsFunnel(context.Context, time.Time, time.Time) (*OperationsFunnelStats, error) {
	r.calls++
	return &OperationsFunnelStats{}, r.err
}

func TestOperationsServiceUsesIndependentPort(t *testing.T) {
	repo := &operationsPortStub{}
	svc := NewOperationsService(repo)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	report, err := svc.GetOperationsFunnel(context.Background(), start, start.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Equal(t, "2026-09-01", report.StartDate)
	require.Equal(t, 1, repo.calls)
	// No DashboardService, UsageLogRepository or native dashboard cache is involved.
	repo.err = errors.New("repository unavailable")
	_, err = svc.GetOperationsFunnel(context.Background(), start, start.AddDate(0, 0, 1))
	require.ErrorIs(t, err, repo.err)
}

func TestOperationsServiceMissingPortFailsExplicitly(t *testing.T) {
	svc := NewOperationsService(nil)
	now := time.Now()
	_, err := svc.GetOperationsFunnel(context.Background(), now, now.AddDate(0, 0, 1))
	require.ErrorIs(t, err, ErrOperationsFunnelUnsupported)
	_, _, err = svc.ListOperationsUserDetails(context.Background(), OperationsUserDetailFilter{})
	require.ErrorIs(t, err, ErrOperationsFunnelUnsupported)
	_, err = svc.GetOperationsFinance(context.Background(), now, now.AddDate(0, 0, 1))
	require.ErrorIs(t, err, ErrOperationsFunnelUnsupported)
	_, err = svc.GetOperationsCustomers(context.Background(), OperationsCustomerFilter{
		Start: now, End: now.AddDate(0, 0, 1), Segment: "all", ChurnDays: 30, Page: 1, PageSize: 20,
	})
	require.ErrorIs(t, err, ErrOperationsFunnelUnsupported)
}
