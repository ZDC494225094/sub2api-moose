package wiring

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type botData struct {
	channels      *service.ChannelMonitorService
	announcements *service.AnnouncementService
	settings      service.SettingRepository
}

func (d botData) Channels(ctx context.Context) ([]botnotify.Channel, error) {
	views, err := d.channels.ListUserView(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]botnotify.Channel, 0, len(views))
	for _, v := range views {
		// Monitors are usually labelled by name; group_name is an optional
		// extra field that is often empty. Fall back to the monitor name so
		// the rendered heading never loses its label.
		label := v.GroupName
		if label == "" {
			label = v.Name
		}
		c := botnotify.Channel{GroupName: label, Model: v.PrimaryModel, Status: v.PrimaryStatus, Availability: v.Availability7d}
		if v.PrimaryLatencyMs != nil {
			c.LatencyMS = *v.PrimaryLatencyMs
		}
		for _, p := range v.Timeline {
			c.Timeline = append(c.Timeline, botnotify.Point{Status: p.Status, CheckedAt: p.CheckedAt})
		}
		items = append(items, c)
	}
	return items, nil
}

func (d botData) Announcements(ctx context.Context) (string, error) {
	// Read through the service, applying current schedules before taking five.
	var active []service.Announcement
	for page := 1; ; page++ {
		items, paging, err := d.announcements.List(ctx, pagination.PaginationParams{Page: page, PageSize: 100, SortBy: "created_at", SortOrder: "desc"}, service.AnnouncementListFilters{Status: service.AnnouncementStatusActive})
		if err != nil {
			return "", err
		}
		for _, a := range items {
			if a.IsActiveAt(time.Now()) {
				active = append(active, a)
			}
		}
		if len(items) == 0 || paging == nil || int64(page*100) >= paging.Total {
			break
		}
	}
	published := func(a service.Announcement) time.Time {
		if a.StartsAt != nil {
			return *a.StartsAt
		}
		return a.CreatedAt
	}
	sort.SliceStable(active, func(i, j int) bool { return published(active[i]).After(published(active[j])) })
	if len(active) > 5 {
		active = active[:5]
	}
	parts := make([]string, 0, len(active))
	for _, a := range active {
		parts = append(parts, a.Title+"\n"+a.Content)
	}
	return strings.Join(parts, "\n\n"), nil
}

func (d botData) SiteName(ctx context.Context) (string, error) {
	values, err := d.settings.GetMultiple(ctx, []string{"site_name"})
	if err != nil {
		return "", err
	}
	name := values["site_name"]
	if name == "" {
		name = "Sub2API"
	}
	return name, nil
}

// Wire's cleanup runs before database infrastructure is closed.
func ProvideBotNotify(db *sql.DB, cipher service.SecretEncryptor, settings service.SettingRepository, settingService *service.SettingService, channels *service.ChannelMonitorService, announcements *service.AnnouncementService) *botnotify.Handler {
	r := botnotify.NewRuntime(&botnotify.Store{DB: db, Cipher: cipher}, botnotify.NewTelegram(), botData{channels: channels, announcements: announcements, settings: settings}, settingService.CustomExtensions())
	r.Start()
	return &botnotify.Handler{Runtime: r}
}
