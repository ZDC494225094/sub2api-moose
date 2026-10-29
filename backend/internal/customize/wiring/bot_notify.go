package wiring

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/userguide"
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

func (d botData) Announcements(ctx context.Context) ([]botnotify.Announcement, error) {
	// Read through the service, applying current schedules; formatting and
	// truncation happen in the botnotify package.
	var active []service.Announcement
	for page := 1; ; page++ {
		items, paging, err := d.announcements.List(ctx, pagination.PaginationParams{Page: page, PageSize: 100, SortBy: "created_at", SortOrder: "desc"}, service.AnnouncementListFilters{Status: service.AnnouncementStatusActive})
		if err != nil {
			return nil, err
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
	out := make([]botnotify.Announcement, 0, len(active))
	for _, a := range active {
		out = append(out, botnotify.Announcement{ID: a.ID, Title: a.Title, Content: a.Content})
	}
	return out, nil
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

// botKnowledge reads the public context the AI may see: site settings that
// the contact widget already shows and, while the docs center is enabled,
// the public user manual.
type botKnowledge struct {
	settings service.SettingRepository
	guide    *userguide.Store
	enabled  func(context.Context, string) bool
}

func (k botKnowledge) AIKnowledge(ctx context.Context) (botnotify.PublicDTO, error) {
	values, err := k.settings.GetMultiple(ctx, []string{
		service.SettingKeySiteName, service.SettingKeyFrontendURL, service.SettingKeyContactInfo,
		service.SettingKeyAfterSalesGroup, service.SettingKeyCustomerServiceLink,
	})
	if err != nil {
		return botnotify.PublicDTO{}, err
	}
	dto := botnotify.PublicDTO{
		SiteName:            values[service.SettingKeySiteName],
		SiteURL:             values[service.SettingKeyFrontendURL],
		ContactInfo:         values[service.SettingKeyContactInfo],
		AfterSalesGroup:     values[service.SettingKeyAfterSalesGroup],
		CustomerServiceLink: values[service.SettingKeyCustomerServiceLink],
	}
	if dto.SiteName == "" {
		dto.SiteName = "Sub2API"
	}
	if k.guide != nil && k.enabled(ctx, userguide.ID) {
		// The manual is optional context: a read failure must not stop answers.
		if guide, err := k.guide.Config(ctx); err == nil {
			dto.GuideTitle, dto.Guide = guide.Title, guide.ContentMD
		}
	}
	return dto, nil
}

// Wire's cleanup runs before database infrastructure is closed.
func ProvideBotNotify(db *sql.DB, cipher service.SecretEncryptor, settings service.SettingRepository, settingService *service.SettingService, channels *service.ChannelMonitorService, announcements *service.AnnouncementService, plaza *service.ModelPlazaService, payments *service.PaymentConfigService) *botnotify.Handler {
	store := &botnotify.Store{DB: db, Cipher: cipher, KeyLister: botAIKeyLister(db), KeyValidator: botAIKeyValidator(db)}
	data := botData{channels: channels, announcements: announcements, settings: settings}
	extensions := settingService.CustomExtensions()
	// Disabled modules and unreadable switches both hide their data.
	enabled := func(ctx context.Context, id string) bool {
		on, err := extensions.Enabled(ctx, id)
		return err == nil && on
	}
	tools := &botAITools{
		db:   db,
		data: data,
		plazaPublic: func(ctx context.Context) bool {
			rt := settingService.GetModelPlazaRuntime(ctx)
			return rt.Enabled && !rt.RequireAuth
		},
		// The catalog only lists campaigns here; the affiliate check matters
		// for checkout, which never runs through the bot.
		campaigns:    rechargecampaigns.NewService(rechargecampaigns.NewSQLRepository(db), nil),
		extensionsOn: enabled,
		now:          time.Now,
	}
	if plaza != nil {
		tools.plaza = plaza
	}
	if payments != nil {
		tools.payments = payments
	}
	r := botnotify.NewRuntime(store, map[string]botnotify.Platform{"telegram": botnotify.NewTelegram(), "qq": botnotify.NewQQ()}, data, extensions)
	r.SetAIProvider(newBotAIProvider(db, tools))
	r.SetAIKnowledge(botKnowledge{settings: settings, guide: &userguide.Store{DB: db}, enabled: enabled})
	r.Start()
	return &botnotify.Handler{Runtime: r}
}
