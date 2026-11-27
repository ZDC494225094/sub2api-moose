package wiring

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (d botData) passiveChannels(ctx context.Context) ([]botnotify.Channel, error) {
	// Chat recipients have no authenticated group scope. Only public, active
	// groups may be reported, further narrowed by the V2 monitor configuration.
	var publicIDs pq.Int64Array
	err := d.db.QueryRowContext(ctx, `SELECT ARRAY(SELECT id FROM groups
		WHERE deleted_at IS NULL AND status = 'active' AND is_exclusive = false)`).Scan(&publicIDs)
	if err != nil || len(publicIDs) == 0 {
		return nil, err
	}
	matrix := func(window string) (*service.ChannelMonitorV2Matrix, error) {
		filter, err := d.passive.ParseFilter(window, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		filter.RestrictGroups, filter.AllowedGroupIDs = true, publicIDs
		return d.passive.Matrix(ctx, filter, service.ChannelMonitorV2GroupByPlatformGroup, false)
	}
	recent, err := matrix("90m")
	if err != nil || len(recent.Items) == 0 {
		return nil, err
	}
	week, err := matrix("7d")
	if err != nil {
		return nil, err
	}
	type groupKey struct {
		platform string
		id       int64
	}
	weeklyRates := make(map[groupKey]*float64, len(week.Items))
	for _, row := range week.Items {
		if row.GroupID != nil {
			weeklyRates[groupKey{row.Platform, *row.GroupID}] = passiveSuccessRate(row)
		}
	}
	channels := make([]botnotify.Channel, 0, len(recent.Items))
	var through time.Time
	if !recent.Coverage.ComputedAt.IsZero() {
		through = recent.Coverage.DataThrough
	}
	for _, row := range recent.Items {
		if row.GroupID == nil || row.GroupName == "" {
			continue
		}
		channels = append(channels, botnotify.Channel{
			GroupName: row.GroupName, Status: passiveStatus(row.Health.Overall),
			Timeline: passiveTimeline(row.Buckets, recent.Coverage.RequestedEnd),
			Passive: &botnotify.PassiveMetrics{
				Platform: platformLabel(row.Platform), TTFTP50MS: row.Metrics.TTFT.P50Ms,
				Success90m: passiveSuccessRate(row), Success7d: weeklyRates[groupKey{row.Platform, *row.GroupID}],
				PartialWeek: !week.Coverage.CoverageComplete, DataThrough: through,
			},
		})
	}
	return channels, nil
}

// Public metrics omit request counts. The error-rate score is present once
// the request sample is sufficient, regardless of the configured health weights.
func passiveSuccessRate(row service.ChannelMonitorV2MatrixRow) *float64 {
	if row.Health.ErrorRateScore == nil {
		return nil
	}
	percent := row.Metrics.SuccessRate * 100
	return &percent
}

func passiveStatus(band string) string {
	switch band {
	case "healthy":
		return "operational"
	case "warning":
		return "degraded"
	case "critical":
		return "failed"
	default:
		return "unknown"
	}
}

// Sparse matrix buckets must stay at their actual positions in the strip.
func passiveTimeline(buckets []service.ChannelMonitorV2TrendPoint, end time.Time) []botnotify.Point {
	const width = 5 * time.Minute
	points := make([]botnotify.Point, 10)
	start := end.Add(-time.Duration(len(points)) * width)
	for i := range points {
		points[i] = botnotify.Point{CheckedAt: start.Add(time.Duration(i) * width), Status: "unknown"}
	}
	for _, point := range buckets {
		if !point.BucketStart.Before(start) && point.BucketStart.Before(end) {
			points[int(point.BucketStart.Sub(start)/width)].Status = passiveStatus(point.Health.Overall)
		}
	}
	return points
}
