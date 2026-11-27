package wiring

import (
	"context"
	"math"
	"slices"
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
	filter, err := d.passive.ParseFilter("90m", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	filter.RestrictGroups, filter.AllowedGroupIDs = true, publicIDs
	recent, err := d.passive.Matrix(ctx, filter, service.ChannelMonitorV2GroupByPlatformGroup, false)
	if err != nil {
		return nil, err
	}
	type groupKey struct {
		platform string
		id       int64
	}
	type pendingGroup struct {
		metrics *botnotify.PassiveMetrics
		known   int
	}
	pending := make(map[groupKey]*pendingGroup)
	channels := make([]botnotify.Channel, 0, len(recent.Items))
	for _, row := range recent.Items {
		if row.GroupID == nil || row.GroupName == "" {
			continue
		}
		scores, known := passiveTimeline(row.Buckets, filter.End, filter.Bucket)
		metrics := &botnotify.PassiveMetrics{
			Scores:        scores,
			SuccessRate:   passiveSuccessRate(row),
			SuccessWindow: filter.End.Sub(filter.Start),
		}
		if known < botnotify.StatusBarSlots || metrics.SuccessRate == nil {
			pending[groupKey{row.Platform, *row.GroupID}] = &pendingGroup{metrics: metrics, known: known}
		}
		channels = append(channels, botnotify.Channel{
			GroupName: row.GroupName,
			Status:    passiveStatus(row.Health.Overall),
			Passive:   metrics,
		})
	}
	// The hourly and half-day rollups can supply both the last ten squares
	// and the aggregate for the website's full day or week in one read.
	for _, tier := range [...]struct{ bucket, rateWindow time.Duration }{
		{30 * time.Minute, 0},
		{time.Hour, 24 * time.Hour},
		{12 * time.Hour, 7 * 24 * time.Hour},
		{24 * time.Hour, 0},
	} {
		if len(pending) == 0 {
			break
		}
		coarse := filter
		coarse.Bucket = tier.bucket
		window := botnotify.StatusBarSlots * tier.bucket
		coarse.GroupIDs = make([]int64, 0, len(pending))
		for key, group := range pending {
			if tier.rateWindow > 0 && group.metrics.SuccessRate == nil {
				window = tier.rateWindow
				group.metrics.SuccessWindow = tier.rateWindow
			} else if group.known == botnotify.StatusBarSlots {
				continue
			}
			coarse.GroupIDs = append(coarse.GroupIDs, key.id)
		}
		if len(coarse.GroupIDs) == 0 {
			continue
		}
		slices.Sort(coarse.GroupIDs)
		coarse.GroupIDs = slices.Compact(coarse.GroupIDs)
		// Align each tier to the bucket containing the end of the recent data.
		// An exclusive end on a bucket boundary belongs to the preceding bucket.
		coarse.End = filter.End.Add(-time.Nanosecond).Truncate(tier.bucket).Add(tier.bucket)
		coarse.Start = coarse.End.Add(-window)
		coarse.Range = window.String()
		matrix, err := d.passive.Matrix(ctx, coarse, service.ChannelMonitorV2GroupByPlatformGroup, false)
		if err != nil {
			return nil, err
		}
		for _, row := range matrix.Items {
			if row.GroupID == nil {
				continue
			}
			key := groupKey{row.Platform, *row.GroupID}
			group, ok := pending[key]
			if !ok {
				continue
			}
			if tier.rateWindow > 0 && group.metrics.SuccessRate == nil {
				group.metrics.SuccessRate = passiveSuccessRate(row)
			}
			if scores, known := passiveTimeline(row.Buckets, coarse.End, coarse.Bucket); known > group.known {
				group.metrics.Scores = scores
				group.known = known
			}
			if group.known == botnotify.StatusBarSlots && group.metrics.SuccessRate != nil {
				delete(pending, key)
			}
		}
	}
	return channels, nil
}

// The public matrix hides request counts. A scored aggregate error rate
// means the group's request count met the configured minimum in this window.
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

// Align the last ten bucket starts exactly as the website's heatmap does.
func passiveTimeline(buckets []service.ChannelMonitorV2TrendPoint, end time.Time, width time.Duration) (scores [botnotify.StatusBarSlots]*float64, known int) {
	start := end.Add(-botnotify.StatusBarSlots * width)
	for _, bucket := range buckets {
		offset := bucket.BucketStart.Sub(start)
		if offset >= 0 && bucket.BucketStart.Before(end) && offset%width == 0 {
			scores[offset/width] = bucket.Health.Score
		}
	}
	for _, score := range scores {
		if score != nil && !math.IsNaN(*score) {
			known++
		}
	}
	return scores, known
}
