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
	// A second matrix is only needed for sparse strips. Keep each row's
	// original position and its 90-minute rate when replacing the timeline.
	type sparseGroup struct{ index, known int }
	sparse := make(map[groupKey]sparseGroup)
	channels := make([]botnotify.Channel, 0, len(recent.Items))
	for _, row := range recent.Items {
		if row.GroupID == nil || row.GroupName == "" {
			continue
		}
		points, known := passiveTimeline(row.Buckets, filter.End, filter.Bucket)
		if known < botnotify.StatusBarSlots/2 {
			sparse[groupKey{row.Platform, *row.GroupID}] = sparseGroup{index: len(channels), known: known}
			filter.GroupIDs = append(filter.GroupIDs, *row.GroupID)
		}
		channels = append(channels, botnotify.Channel{
			GroupName: row.GroupName, Status: passiveStatus(row.Health.Overall), Timeline: points,
			Passive: &botnotify.PassiveMetrics{Platform: platformLabel(row.Platform), Success90m: passiveSuccessRate(row)},
		})
	}
	if len(sparse) == 0 {
		return channels, nil
	}
	// Use the same reporting instant for both reads. The repository bins
	// minute facts on UTC half hours, including the current partial bucket.
	filter.Bucket = 30 * time.Minute
	filter.Range = "5h"
	filter.End = filter.End.Add(-time.Nanosecond).Truncate(filter.Bucket).Add(filter.Bucket)
	filter.Start = filter.End.Add(-botnotify.StatusBarSlots * filter.Bucket)
	coarse, err := d.passive.Matrix(ctx, filter, service.ChannelMonitorV2GroupByPlatformGroup, false)
	if err != nil {
		return nil, err
	}
	for _, row := range coarse.Items {
		if row.GroupID == nil {
			continue
		}
		group, ok := sparse[groupKey{row.Platform, *row.GroupID}]
		if !ok {
			continue
		}
		points, known := passiveTimeline(row.Buckets, filter.End, filter.Bucket)
		if known > group.known {
			channels[group.index].Timeline = points
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

// Keep gaps at their time positions. A completely unknown strip is omitted.
func passiveTimeline(buckets []service.ChannelMonitorV2TrendPoint, end time.Time, width time.Duration) ([]botnotify.Point, int) {
	points := make([]botnotify.Point, botnotify.StatusBarSlots)
	start := end.Add(-time.Duration(len(points)) * width)
	for i := range points {
		points[i] = botnotify.Point{CheckedAt: start.Add(time.Duration(i) * width), Status: "unknown"}
	}
	for _, point := range buckets {
		if !point.BucketStart.Before(start) && point.BucketStart.Before(end) {
			points[int(point.BucketStart.Sub(start)/width)].Status = passiveStatus(point.Health.Overall)
		}
	}
	known := 0
	for _, point := range points {
		if point.Status != "unknown" {
			known++
		}
	}
	if known == 0 {
		return nil, 0
	}
	return points, known
}
