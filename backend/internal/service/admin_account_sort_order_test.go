package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountSortOrderRepositoryStub struct {
	AccountRepository
	updates []AccountSortOrderUpdate
	err     error
}

func (s *accountSortOrderRepositoryStub) UpdateSortOrders(_ context.Context, updates []AccountSortOrderUpdate) error {
	s.updates = append([]AccountSortOrderUpdate(nil), updates...)
	return s.err
}

func TestAdminServiceUpdateAccountSortOrdersDelegatesToOptionalRepository(t *testing.T) {
	repo := &accountSortOrderRepositoryStub{}
	svc := &adminServiceImpl{accountRepo: repo}
	updates := []AccountSortOrderUpdate{{ID: 7, SortOrder: 10}, {ID: 8, SortOrder: 20}}

	err := svc.UpdateAccountSortOrders(context.Background(), updates)

	require.NoError(t, err)
	require.Equal(t, updates, repo.updates)
}

func TestAdminServiceUpdateAccountSortOrdersRequiresCapableRepository(t *testing.T) {
	svc := &adminServiceImpl{accountRepo: accountSortOrderRepositoryStub{}.AccountRepository}

	err := svc.UpdateAccountSortOrders(context.Background(), []AccountSortOrderUpdate{{ID: 7, SortOrder: 10}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "sort order repository is not configured")
}
