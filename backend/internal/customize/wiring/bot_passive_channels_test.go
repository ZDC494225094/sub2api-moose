package wiring

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type botMatrixRepo struct {
	service.ChannelMonitorV2Repository
	filters []service.ChannelMonitorV2Filter
	rows    func(service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow
	t       *testing.T
}

func (r *botMatrixRepo) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{Enabled: true}, nil
}

func (r *botMatrixRepo) GetMatrix(_ context.Context, f service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, by service.ChannelMonitorV2GroupBy, admin bool) (*service.ChannelMonitorV2Matrix, error) {
	r.filters = append(r.filters, f)
	require.False(r.t, admin)
	require.True(r.t, f.RestrictGroups)
	require.Equal(r.t, service.ChannelMonitorV2GroupByPlatformGroup, by)
	return &service.ChannelMonitorV2Matrix{Items: r.rows(f)}, nil
}

func matrixGroup(f service.ChannelMonitorV2Filter, id int64, name, overall string, rate float64, bands ...string) service.ChannelMonitorV2MatrixRow {
	row := service.ChannelMonitorV2MatrixRow{
		Platform: "openai", GroupID: &id, GroupName: name, Health: service.ChannelMonitorV2Health{Overall: overall},
		Metrics: service.ChannelMonitorV2Metric{SuccessRate: rate},
	}
	for i, band := range bands {
		row.Buckets = append(row.Buckets, service.ChannelMonitorV2TrendPoint{
			BucketStart: f.End.Add(-time.Duration(len(bands)-i) * f.Bucket),
			Health:      service.ChannelMonitorV2Health{Overall: band},
		})
	}
	return row
}

func passiveBotFixture(t *testing.T, ids string, rows func(service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow) (botData, *botMatrixRepo) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	mock.ExpectQuery("SELECT ARRAY.*is_exclusive = false").WillReturnRows(sqlmock.NewRows([]string{"ids"}).AddRow(ids))
	repo := &botMatrixRepo{t: t, rows: rows}
	data := botData{db: db, passive: service.NewChannelMonitorV2Service(repo), runtime: func(context.Context) (service.ChannelMonitorRuntime, error) {
		return service.ChannelMonitorRuntime{Enabled: true, Mode: service.ChannelMonitorModeV2}, nil
	}}
	return data, repo
}

func TestBotPassiveStatusRendersPublicGroupMatrix(t *testing.T) {
	data, repo := passiveBotFixture(t, "{3,4,5,6,7}", func(f service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow {
		if f.Bucket == 5*time.Minute {
			return []service.ChannelMonitorV2MatrixRow{
				matrixGroup(f, 3, "活跃组", "healthy", 0.999, "healthy", "healthy", "healthy", "healthy", "healthy"),
				matrixGroup(f, 4, "低流量组", "warning", 0.875, "warning", "unknown"),
				matrixGroup(f, 5, "无流量组", "unknown", 0),
				matrixGroup(f, 6, "全部失败组", "unknown", 0, "unknown"),
				matrixGroup(f, 7, "其他时段有流量", "unknown", 0),
			}
		}
		// Different row order and very different five-hour rates. One group
		// still has no scorable bucket, another has no recent requests.
		return []service.ChannelMonitorV2MatrixRow{
			matrixGroup(f, 7, "其他时段有流量", "critical", 0.5, "critical", "unknown", "unknown", "unknown", "unknown"),
			matrixGroup(f, 6, "全部失败组", "unknown", 0.91),
			matrixGroup(f, 5, "无流量组", "unknown", 0),
			matrixGroup(f, 4, "低流量组", "healthy", 1, "healthy", "warning", "critical", "healthy"),
		}
	})
	now := time.Date(2026, 3, 7, 9, 5, 0, 0, time.FixedZone("CST", 8*3600))
	output, err := botnotify.Render(context.Background(), data, "{channel_status}\n---\n{channel_status_abnormal}", now)
	require.NoError(t, err)
	require.Len(t, repo.filters, 2)
	recent, coarse := repo.filters[0], repo.filters[1]
	require.Equal(t, 90*time.Minute, recent.End.Sub(recent.Start))
	require.Empty(t, recent.GroupIDs)
	require.Equal(t, 30*time.Minute, coarse.Bucket)
	require.Equal(t, 5*time.Hour, coarse.End.Sub(coarse.Start))
	require.Equal(t, coarse.End.Truncate(30*time.Minute), coarse.End)
	require.False(t, coarse.End.Before(recent.End))
	require.Less(t, coarse.End.Sub(recent.End), 30*time.Minute)
	require.Equal(t, []int64{4, 5, 6, 7}, coarse.GroupIDs)
	require.Equal(t, []int64{3, 4, 5, 6, 7}, coarse.AllowedGroupIDs)
	title := "【03月07日 09:05】分组状态"
	sparse := "低流量组 · OpenAI/GPT\n······🟩🟨🟥🟩\n近90分钟成功率 87.50%"
	require.Equal(t, strings.Join([]string{
		title,
		"活跃组 · OpenAI/GPT\n·····🟩🟩🟩🟩🟩\n近90分钟成功率 99.90%",
		sparse,
		"无流量组 · OpenAI/GPT\n近90分钟成功率 —",
		"全部失败组 · OpenAI/GPT\n近90分钟成功率 0.00%",
		"其他时段有流量 · OpenAI/GPT\n·····🟥····\n近90分钟成功率 —",
	}, "\n\n")+"\n---\n"+title+"\n\n"+sparse, output)
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

func TestPassiveStatusRetainsFineTimeline(t *testing.T) {
	for _, tc := range []struct {
		name      string
		bands     []string
		wantReads int
	}{
		{"dense skips second query", []string{"healthy", "healthy", "healthy", "healthy", "healthy"}, 1},
		{"equally sparse prefers fine buckets", []string{"warning", "healthy"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, repo := passiveBotFixture(t, "{3}", func(f service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow {
				if f.Bucket == 30*time.Minute {
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 3, "A", "critical", 0.3, "critical", "critical")}
				}
				return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 3, "A", "healthy", 0.8, tc.bands...)}
			})
			channels, err := data.Channels(context.Background())
			require.NoError(t, err)
			require.Len(t, repo.filters, tc.wantReads)
			require.Len(t, channels, 1)
			points := channels[0].Timeline
			require.Len(t, points, botnotify.StatusBarSlots)
			require.Equal(t, 5*time.Minute, points[1].CheckedAt.Sub(points[0].CheckedAt))
			require.Equal(t, "operational", points[len(points)-1].Status)
		})
	}
}
