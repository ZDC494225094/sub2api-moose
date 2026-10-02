package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/adminefficiency"
)

// Keep capability discovery at the host boundary; the module never imports AccountRepository.
func (s *adminServiceImpl) upstreamGroupDirectory() *adminefficiency.Directory {
	reader, _ := s.accountRepo.(AccountUpstreamGroupRepository)
	renamer, _ := s.accountRepo.(AccountUpstreamGroupRenameRepository)
	sorter, _ := s.accountRepo.(AccountUpstreamGroupSortOrderRepository)
	return adminefficiency.NewDirectory(reader, renamer, sorter)
}

func (s *adminServiceImpl) ListAccountUpstreamGroups(ctx context.Context) ([]AccountUpstreamGroup, error) {
	return s.upstreamGroupDirectory().ListAccountUpstreamGroups(ctx)
}

func (s *adminServiceImpl) RenameAccountUpstreamGroup(ctx context.Context, id int64, name string) (*AccountUpstreamGroup, error) {
	if err := s.requireAdminEfficiency(ctx); err != nil {
		return nil, err
	}
	return s.upstreamGroupDirectory().RenameAccountUpstreamGroup(ctx, id, name)
}

func (s *adminServiceImpl) UpdateAccountUpstreamGroupSortOrders(ctx context.Context, updates []AccountUpstreamGroupSortOrderUpdate) error {
	if err := s.requireAdminEfficiency(ctx); err != nil {
		return err
	}
	return s.upstreamGroupDirectory().UpdateAccountUpstreamGroupSortOrders(ctx, updates)
}
