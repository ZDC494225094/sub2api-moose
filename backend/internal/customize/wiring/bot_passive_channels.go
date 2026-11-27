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
	type sparseGroup struct{ index, known int }
	sparse := make(map[groupKey]sparseGroup)
	channels := make([]botnotify.Channel, 0, len(recent.Items))
	for _, row := range recent.Items {
		if row.GroupID == nil || row.GroupName == "" {
			continue
		}
		scores, known := passiveTimeline(row.Buckets, filter.End, filter.Bucket)
		if known < botnotify.StatusBarSlots {
			sparse[groupKey{row.Platform, *row.GroupID}] = sparseGroup{index: len(channels), known: known}
		}
		channels = append(channels, botnotify.Channel{
			GroupName: row.GroupName,
			Status:    passiveStatus(row.Health.Overall),
			Passive: &botnotify.PassiveMetrics{
				Platform:   platformLabel(row.Platform),
				Success90m: passiveSuccessRate(row),
				Scores:     scores,
			},
		})
	}
	// Try half-hour facts first, then the V2 monitor's existing rollup tiers.
	// Only replace a whole strip when more of its ten buckets have scores.
	for _, width := range [...]time.Duration{30 * time.Minute, time.Hour, 12 * time.Hour, 24 * time.Hour} {
		if len(sparse) == 0 {
			break
		}
		coarse := filter
		coarse.GroupIDs = make([]int64, 0, len(sparse))
		for key := range sparse {
			coarse.GroupIDs = append(coarse.GroupIDs, key.id)
		}
		slices.Sort(coarse.GroupIDs)
		coarse.GroupIDs = slices.Compact(coarse.GroupIDs)
		coarse.Bucket = width
		// Anchor every tier to the same exclusive end, including the current
		// partial bucket without adding a future slot at an exact boundary.
		coarse.End = filter.End.Add(-time.Nanosecond).Truncate(width).Add(width)
		coarse.Start = coarse.End.Add(-botnotify.StatusBarSlots * width)
		coarse.Range = coarse.End.Sub(coarse.Start).String()
		matrix, err := d.passive.Matrix(ctx, coarse, service.ChannelMonitorV2GroupByPlatformGroup, false)
		if err != nil {
			return nil, err
		}
		for _, row := range matrix.Items {
			if row.GroupID == nil {
				continue
			}
			key := groupKey{row.Platform, *row.GroupID}
			group, ok := sparse[key]
			if !ok {
				continue
			}
			scores, known := passiveTimeline(row.Buckets, coarse.End, width)
			if known <= group.known {
				continue
			}
			channels[group.index].Passive.Scores = scores
			if known == botnotify.StatusBarSlots {
				delete(sparse, key)
			} else {
				group.known = known
				sparse[key] = group
			}
		}
	}
	return channels, nil
}

// A public matrix includes a trend bucket only where it found request
// facts. An empty (seeded) group and an observed 0% must stay distinct.
// The aggregate rate already weights each request across the whole 90m.
func passiveSuccessRate(row service.ChannelMonitorV2MatrixRow) *float64 {
	if len(row.Buckets) == 0 {
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
