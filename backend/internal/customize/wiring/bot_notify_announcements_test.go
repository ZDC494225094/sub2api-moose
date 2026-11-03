package wiring

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type botAnnouncementRepoStub struct {
	service.AnnouncementRepository
	items []service.Announcement
}

func (s *botAnnouncementRepoStub) List(_ context.Context, params pagination.PaginationParams, _ service.AnnouncementListFilters) ([]service.Announcement, *pagination.PaginationResult, error) {
	if params.Page > 1 {
		return nil, &pagination.PaginationResult{Total: int64(len(s.items))}, nil
	}
	return s.items, &pagination.PaginationResult{Total: int64(len(s.items))}, nil
}

func TestBotAnnouncementsSkipTargetedAnnouncements(t *testing.T) {
	repo := &botAnnouncementRepoStub{items: []service.Announcement{
		{ID: 1, Title: "everyone", Status: service.AnnouncementStatusActive},
		{ID: 2, Title: "vip only", Status: service.AnnouncementStatusActive, Targeting: service.AnnouncementTargeting{
			AnyOf: []service.AnnouncementConditionGroup{{}},
		}},
	}}
	data := botData{announcements: service.NewAnnouncementService(repo, nil, nil, nil)}

	got, err := data.Announcements(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "everyone", got[0].Title)
}
