package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/mediagateway"
)

func (s *SettingService) CheckMediaGatewayAdmission(ctx context.Context, operation mediagateway.Operation) error {
	return mediagateway.CheckAdmission(ctx, s.CustomExtensions(), operation)
}

func (s *OpenAIGatewayService) CheckMediaGatewayAdmission(ctx context.Context, operation mediagateway.Operation) error {
	var settings *SettingService
	if s != nil {
		settings = s.settingService
	}
	return settings.CheckMediaGatewayAdmission(ctx, operation)
}
