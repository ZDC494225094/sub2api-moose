package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type customersRepositoryStub struct {
	UsageLogRepository
	filter OperationsCustomerFilter
}

func (r *customersRepositoryStub) GetOperationsCustomers(_ context.Context, f OperationsCustomerFilter) (*OperationsCustomersResponse, error) {
	r.filter = f
	return &OperationsCustomersResponse{Summary: OperationsCustomerSummary{Paying: 10, Repeat: 4, PreviousActive: 6, Churned: 3},
		Items: []OperationsCustomer{{Consumption: 0, Cost: 3}}}, nil
}
func TestOperationsCustomersRatesAndAsOf(t *testing.T) {
	repo := &customersRepositoryStub{}
	s := &DashboardService{usageRepo: repo}
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := OperationsCustomerFilter{Start: end.AddDate(0, 0, -30), End: end, ChurnDays: 30, Page: 1, PageSize: 20, Segment: "repeat"}
	result, err := s.GetOperationsCustomers(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, end, repo.filter.AsOf)
	require.Equal(t, 40.0, *result.Summary.RepeatRate)
	require.Equal(t, 50.0, *result.Summary.ChurnRate)
	require.Equal(t, -3.0, result.Items[0].Profit)
	require.Nil(t, result.Items[0].Margin)
	require.Nil(t, operationsCustomerRate(0, 0))
	f.End = time.Now().AddDate(0, 0, 5)
	before := time.Now()
	_, err = s.GetOperationsCustomers(context.Background(), f)
	require.NoError(t, err)
	require.False(t, repo.filter.AsOf.Before(before))
	require.False(t, repo.filter.AsOf.After(time.Now()))
}
func TestOperationsCustomersRejectInvalidFilters(t *testing.T) {
	s := &DashboardService{usageRepo: &customersRepositoryStub{}}
	for _, f := range []OperationsCustomerFilter{
		{Segment: "unknown", ChurnDays: 30, Page: 1, PageSize: 20},
		{Segment: "all", ChurnDays: 1, Page: 1, PageSize: 20},
		{Segment: "all", ChurnDays: 30, Page: 0, PageSize: 20},
	} {
		_, err := s.GetOperationsCustomers(context.Background(), f)
		require.Error(t, err)
	}
}
