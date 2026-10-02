package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/adminefficiency"
)

func (s *adminServiceImpl) requireAdminEfficiency(ctx context.Context) error {
	return adminefficiency.CheckWrite(ctx, s.settingService.CustomExtensions())
}
func (s *adminServiceImpl) checkUpstreamGroupChange(ctx context.Context, before, after string) error {
	return adminefficiency.CheckUpstreamGroupChange(ctx, s.settingService.CustomExtensions(), before, after)
}
