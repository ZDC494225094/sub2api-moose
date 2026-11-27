package wiring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type botMatrixRepo struct {
	service.ChannelMonitorV2Repository
	t       *testing.T
	windows []string
}

func (r *botMatrixRepo) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{Enabled: true}, nil
}

func (r *botMatrixRepo) GetMatrix(_ context.Context, f service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, by service.ChannelMonitorV2GroupBy, admin bool) (*service.ChannelMonitorV2Matrix, error) {
	r.windows = append(r.windows, f.Range)
	require.False(r.t, admin)
	require.True(r.t, f.RestrictGroups)
	require.Equal(r.t, []int64{3, 4}, []int64(f.AllowedGroupIDs))
	require.Equal(r.t, service.ChannelMonitorV2GroupByPlatformGroup, by)
	id, quietID := int64(3), int64(4)
	warn := service.ChannelMonitorV2Health{Overall: "warning", Score: ptr(55), ErrorRateScore: ptr(0)}
	row := service.ChannelMonitorV2MatrixRow{
		Platform: "openai", GroupID: &id, GroupName: "公开组", Health: warn,
		Buckets: []service.ChannelMonitorV2TrendPoint{{BucketStart: f.End.Add(-10 * time.Minute), Health: warn}},
	}
	if f.Range == "7d" {
		row.Metrics.SuccessRate = 0.975
	}
	return &service.ChannelMonitorV2Matrix{
		Coverage: service.ChannelMonitorV2Coverage{RequestedEnd: f.End, ComputedAt: f.End, DataThrough: f.End.Add(-time.Minute)},
		Items:    []service.ChannelMonitorV2MatrixRow{row, {Platform: "openai", GroupID: &quietID, GroupName: "冷门组"}},
	}, nil
}

func TestBotPassiveStatusRendersPublicGroupMatrix(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &botMatrixRepo{t: t}
	data := botData{db: db, passive: service.NewChannelMonitorV2Service(repo), runtime: func(context.Context) (service.ChannelMonitorRuntime, error) {
		return service.ChannelMonitorRuntime{Enabled: true, Mode: service.ChannelMonitorModeV2}, nil
	}}
	mock.ExpectQuery("SELECT ARRAY.*is_exclusive = false").WillReturnRows(sqlmock.NewRows([]string{"ids"}).AddRow("{3,4}"))
	output, err := botnotify.Render(context.Background(), data, "{channel_status}\n---\n{channel_status_abnormal}", time.Now())
	require.NoError(t, err)
	require.Equal(t, []string{"90m", "7d"}, repo.windows)
	require.Contains(t, output, "🟨⬜\n状态 降级｜首字延迟 P50 暂无样本｜90 分钟成功率 0.00%｜近 7 天成功率 97.50%（部分覆盖）")
	require.Contains(t, output, "冷门组 · OpenAI/GPT")
	require.Contains(t, output, "状态 样本不足")
	require.Contains(t, output, "1 个分组样本不足")
	require.NotContains(t, output, "冷门组 · OpenAI/GPT ⚠️")
	require.NotContains(t, output, "近10次检测")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBotChannelModeReadFailureAndDisabledMonitor(t *testing.T) {
	failure := errors.New("read settings failed")
	data := botData{runtime: func(context.Context) (service.ChannelMonitorRuntime, error) {
		return service.ChannelMonitorRuntime{}, failure
	}}
	_, err := data.Channels(context.Background())
	require.ErrorIs(t, err, failure)
	data.runtime = func(context.Context) (service.ChannelMonitorRuntime, error) {
		return service.ChannelMonitorRuntime{}, nil
	}
	output, err := botnotify.Render(context.Background(), data, "日报\n{channel_status}", time.Now())
	require.NoError(t, err)
	require.Equal(t, "日报\n渠道监控已关闭", output)
}

func TestPassiveSuccessRateWithLatencyOnlyHealthWeights(t *testing.T) {
	thresholds := service.DefaultChannelMonitorV2HealthThresholds()
	thresholds.ErrorWeight, thresholds.CacheWeight, thresholds.TTFTWeight = 0, 0, 1
	metrics := service.ChannelMonitorV2Metric{RequestCount: thresholds.MinimumSample, SuccessRate: 0.8}
	row := service.ChannelMonitorV2MatrixRow{Metrics: metrics, Health: service.ChannelMonitorV2HealthForWithThresholds(metrics, thresholds)}
	require.Nil(t, row.Health.Score)
	require.NotNil(t, passiveSuccessRate(row))
	require.Equal(t, 80.0, *passiveSuccessRate(row))
}
