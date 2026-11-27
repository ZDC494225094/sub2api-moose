package wiring

import (
	"context"
	"errors"
	"math"
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

func matrixGroup(f service.ChannelMonitorV2Filter, id int64, name string, requests int64, rate float64, scores ...float64) service.ChannelMonitorV2MatrixRow {
	total := service.ChannelMonitorV2Metric{RequestCount: requests, SuccessRate: rate, ErrorRate: 1 - rate}
	row := service.ChannelMonitorV2MatrixRow{
		Platform: "openai", GroupID: &id, GroupName: name,
		Metrics: total, Health: service.ChannelMonitorV2HealthFor(total),
	}
	for i, score := range scores {
		// With error rate as the only sampled signal this produces the
		// requested score, including the real coarse "critical" for a 40.
		metrics := service.ChannelMonitorV2Metric{RequestCount: 1}
		if !math.IsNaN(score) {
			metrics.RequestCount = 100
			metrics.ErrorRate = (1 - score/100) * service.DefaultChannelMonitorV2HealthThresholds().CriticalErrorRate
		}
		row.Buckets = append(row.Buckets, service.ChannelMonitorV2TrendPoint{
			BucketStart: f.End.Add(-time.Duration(len(scores)-i) * f.Bucket),
			Metrics:     metrics,
			Health:      service.ChannelMonitorV2HealthFor(metrics),
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
	minimum := service.DefaultChannelMonitorV2HealthThresholds().MinimumSample
	data, repo := passiveBotFixture(t, "{3,4,5,6,7,8}", func(f service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow {
		switch f.Bucket {
		case 5 * time.Minute:
			return []service.ChannelMonitorV2MatrixRow{
				matrixGroup(f, 3, "活跃组", 10000, 0.9952, 100, 70, 60, 50, 60, 60, 50, 40, 40, 40),
				matrixGroup(f, 4, "低流量组", 1600, 0.875, 70, 70, 70, 70, 70, 70, 70, 70, 70),
				matrixGroup(f, 5, "无流量组", 0, 0),
				matrixGroup(f, 6, "全部失败组", minimum, 0, math.NaN()),
				matrixGroup(f, 7, "周样本组", 1, 1, math.NaN()),
				matrixGroup(f, 8, "ClaudeCode-Max", 0, 0),
			}
		case 30 * time.Minute:
			// Return groups in a different order. The wider window has its own rates.
			return []service.ChannelMonitorV2MatrixRow{
				matrixGroup(f, 8, "ClaudeCode-Max", 1000, 1, 70, 70, 70, 70, 70, 70, 70, 70, 70, 70),
				matrixGroup(f, 4, "低流量组", 2000, 1, 70, 70, 70, 70, 70, 70, 50, 50, 40, 0),
			}
		case time.Hour:
			return []service.ChannelMonitorV2MatrixRow{
				// The half-hour query already filled group 8's strip. Its rate still needs a full day.
				matrixGroup(f, 8, "ClaudeCode-Max", 2500, 0.9624, 40),
				matrixGroup(f, 7, "周样本组", minimum-1, 1, math.NaN()),
				matrixGroup(f, 6, "全部失败组", 100, 0.5, math.NaN()),
			}
		case 12 * time.Hour:
			return []service.ChannelMonitorV2MatrixRow{
				matrixGroup(f, 7, "周样本组", 4000, 0.9625, 70, 70, 70, 70, 70, 70, 70, 70, math.NaN(), math.NaN()),
			}
		default:
			return nil
		}
	})
	now := time.Date(2026, 3, 7, 9, 5, 0, 0, time.FixedZone("CST", 8*3600))
	output, err := botnotify.Render(context.Background(), data, "{channel_status}\n---\n{channel_status_abnormal}", now)
	require.NoError(t, err)
	require.Len(t, repo.filters, 5)
	recent := repo.filters[0]
	require.Equal(t, 90*time.Minute, recent.End.Sub(recent.Start))
	require.Empty(t, recent.GroupIDs)
	for i, want := range []struct {
		width, window time.Duration
		ids           []int64
	}{
		{30 * time.Minute, 5 * time.Hour, []int64{4, 5, 6, 7, 8}},
		{time.Hour, 24 * time.Hour, []int64{5, 6, 7, 8}},
		{12 * time.Hour, 7 * 24 * time.Hour, []int64{5, 6, 7}},
		{24 * time.Hour, 10 * 24 * time.Hour, []int64{5, 6, 7}},
	} {
		coarse := repo.filters[i+1]
		require.Equal(t, want.width, coarse.Bucket)
		require.Equal(t, want.window, coarse.End.Sub(coarse.Start))
		require.Equal(t, coarse.End.Truncate(want.width), coarse.End)
		require.GreaterOrEqual(t, coarse.End.Sub(recent.End), time.Duration(0))
		require.Less(t, coarse.End.Sub(recent.End), want.width)
		require.Equal(t, want.ids, coarse.GroupIDs)
		require.Equal(t, []int64{3, 4, 5, 6, 7, 8}, coarse.AllowedGroupIDs)
	}
	title := "【03月07日 09:05】分组状态"
	sparse := "低流量组\n🟩🟩🟩🟩🟩🟩🟨🟨🟧🟥\n近90分钟成功率 87.50%"
	empty := strings.Repeat("⬜", botnotify.StatusBarSlots)
	allFailed := "全部失败组\n" + empty + "\n近90分钟成功率 0.00%"
	require.Equal(t, strings.Join([]string{
		title,
		"活跃组\n🟩🟩🟨🟨🟨🟨🟨🟧🟧🟧\n近90分钟成功率 99.52%",
		sparse,
		"无流量组\n" + empty + "\n近7天成功率 样本不足",
		allFailed,
		"周样本组\n" + strings.Repeat("🟩", 8) + "⬜⬜\n近7天成功率 96.25%",
		"ClaudeCode-Max\n" + strings.Repeat("🟩", botnotify.StatusBarSlots) + "\n近1天成功率 96.24%",
	}, "\n\n")+"\n---\n"+strings.Join([]string{title, sparse, allFailed}, "\n\n"), output)
}

func TestBotGroupStatusToolUsesMatchingPlatformAndRateWindow(t *testing.T) {
	minimum := service.DefaultChannelMonitorV2HealthThresholds().MinimumSample
	for _, tc := range []struct {
		weekRequests int64
		want         string
	}{
		{minimum, "0.00%"},
		{minimum - 1, "样本不足"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			data, _ := passiveBotFixture(t, "{8}", func(f service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow {
				switch f.Bucket {
				case 5 * time.Minute:
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 8, "同名组", 1, 1, math.NaN())}
				case time.Hour:
					other := matrixGroup(f, 8, "同名组", 1000, 1, 100)
					other.Platform = "anthropic"
					return []service.ChannelMonitorV2MatrixRow{other}
				case 12 * time.Hour:
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 8, "同名组", tc.weekRequests, 0, math.NaN())}
				case 24 * time.Hour:
					// The ten-day strip includes older traffic outside all three rate windows.
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 8, "同名组", 2000, 0.9)}
				default:
					return nil
				}
			})
			tools := &botAITools{data: data}
			require.Equal(t, "同名组｜近7天成功率 "+tc.want, tools.groupStatus(t.Context(), "同名"))
		})
	}
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

func TestPassiveTimelineAlignsLatestTenWebsiteBuckets(t *testing.T) {
	const width = 5 * time.Minute
	end := time.Date(2026, 11, 27, 14, 20, 0, 0, time.FixedZone("CST", 8*3600))
	bucket := func(at time.Time, score float64) service.ChannelMonitorV2TrendPoint {
		return service.ChannelMonitorV2TrendPoint{BucketStart: at.UTC(), Health: service.ChannelMonitorV2Health{Score: &score}}
	}
	scores, known := passiveTimeline([]service.ChannelMonitorV2TrendPoint{
		bucket(end.Add(-width), 40),
		bucket(end.Add(-11*width), 25),
		bucket(end.Add(-10*width), 0),
		bucket(end.Add(-time.Minute), 90),
		bucket(end, 100),
	}, end, width)
	require.Equal(t, 2, known)
	require.Equal(t, 0.0, *scores[0])
	require.Equal(t, 40.0, *scores[len(scores)-1])
}

func TestPassiveStatusChoosesMostCompleteThenFinestTimeline(t *testing.T) {
	full := []float64{70, 70, 70, 70, 70, 70, 70, 70, 70, 70}
	for _, tc := range []struct {
		name       string
		fine       []float64
		coarse     []float64
		wantReads  int
		wantBlocks string
	}{
		{"ten recent scores stop widening", full, nil, 1, strings.Repeat("🟩", 10)},
		{"nine scores still widen and stop at ten", full[1:], full, 2, strings.Repeat("🟩", 10)},
		{"equal coverage keeps finer colors and gap positions", []float64{40, math.NaN(), 100}, []float64{0, 0}, 5, strings.Repeat("⬜", 7) + "🟧⬜🟩"},
		{"partial improvement survives sparser later tiers", []float64{40}, []float64{0, 40}, 5, strings.Repeat("⬜", 8) + "🟥🟧"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, repo := passiveBotFixture(t, "{3}", func(f service.ChannelMonitorV2Filter) []service.ChannelMonitorV2MatrixRow {
				switch f.Bucket {
				case 5 * time.Minute:
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 3, "A", 1000, 0.8, tc.fine...)}
				case 30 * time.Minute:
					return []service.ChannelMonitorV2MatrixRow{matrixGroup(f, 3, "A", 1500, 0.3, tc.coarse...)}
				default:
					return nil
				}
			})
			channels, err := data.Channels(context.Background())
			require.NoError(t, err)
			require.Len(t, repo.filters, tc.wantReads)
			require.Len(t, channels, 1)
			require.Contains(t, botnotify.ChannelStatus(channels, false, time.Now()), "\n"+tc.wantBlocks+"\n近90分钟成功率 80.00%")
		})
	}
}
