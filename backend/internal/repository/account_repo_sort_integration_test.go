//go:build integration

package repository

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByDisplayOrderAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "later", SortOrder: 200})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "earlier", SortOrder: 100})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "sort_order",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("earlier", accounts[0].Name)
	s.Require().Equal("later", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByExplicitUpstreamGroupThenDisplayOrderAndID() {
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "alpha", UpstreamGroup: "alpha", SortOrder: 300,
	})
	zetaFirst := mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "zeta-first", UpstreamGroup: "ZETA", SortOrder: 100,
	})
	zetaSecond := mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "zeta-second", UpstreamGroup: " zeta ", SortOrder: 100,
	})
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "zeta-later", UpstreamGroup: "zeta", SortOrder: 200,
	})
	ungroupedEmpty := mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "ungrouped-empty", SortOrder: 50,
	})
	ungroupedLabel := mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "ungrouped-label", UpstreamGroup: "未分组", SortOrder: 50,
	})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "upstream",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 6)

	positions := make(map[string]int, len(accounts))
	for i, account := range accounts {
		positions[account.Name] = i
	}

	// Group key precedes display order: alpha sorts before zeta despite its larger sort_order.
	s.Require().Less(positions["alpha"], positions["zeta-first"])

	// Group keys are case-insensitive and trimmed; equal sort_order values use ID.
	s.Require().Less(positions["zeta-first"], positions["zeta-second"])
	s.Require().Equal(zetaFirst.ID, accounts[positions["zeta-first"]].ID)
	s.Require().Equal(zetaSecond.ID, accounts[positions["zeta-second"]].ID)
	s.Require().Equal(positions["zeta-first"]+1, positions["zeta-second"])
	s.Require().Equal(positions["zeta-second"]+1, positions["zeta-later"])

	// Empty groups share the explicit "未分组" key and therefore remain adjacent.
	s.Require().Equal(ungroupedEmpty.ID, accounts[positions["ungrouped-empty"]].ID)
	s.Require().Equal(ungroupedLabel.ID, accounts[positions["ungrouped-label"]].ID)
	s.Require().Equal(positions["ungrouped-empty"]+1, positions["ungrouped-label"])
}

func (s *AccountRepoSuite) TestUpdateSortOrdersPersistsBatch() {
	first := mustCreateAccount(s.T(), s.client, &service.Account{Name: "first", SortOrder: 10})
	second := mustCreateAccount(s.T(), s.client, &service.Account{Name: "second", SortOrder: 20})

	err := s.repo.UpdateSortOrders(s.ctx, []service.AccountSortOrderUpdate{
		{ID: first.ID, SortOrder: 200},
		{ID: second.ID, SortOrder: 100},
	})
	s.Require().NoError(err)

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page: 1, PageSize: 10, SortBy: "sort_order", SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal([]int64{second.ID, first.ID}, []int64{accounts[0].ID, accounts[1].ID})
}

func (s *AccountRepoSuite) TestUpdateSortOrdersMissingAccountDoesNotPartiallyUpdate() {
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "existing", SortOrder: 10})

	err := s.repo.UpdateSortOrders(s.ctx, []service.AccountSortOrderUpdate{
		{ID: account.ID, SortOrder: 200},
		{ID: 999999999, SortOrder: 100},
	})
	s.Require().ErrorIs(err, service.ErrAccountNotFound)

	got, err := s.repo.GetByID(s.ctx, account.ID)
	s.Require().NoError(err)
	s.Require().Equal(int64(10), got.SortOrder)
}

func (s *AccountRepoSuite) TestUpdatePreservesSortOrder() {
	created := mustCreateAccount(s.T(), s.client, &service.Account{Name: "before", SortOrder: 77})
	account, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	account.Name = "after"

	s.Require().NoError(s.repo.Update(s.ctx, account))
	updated, err := s.repo.GetByID(s.ctx, account.ID)
	s.Require().NoError(err)
	s.Require().Equal(int64(77), updated.SortOrder)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByUpstreamBillingRateWithMissingLast() {
	makeAccount := func(name, status string, rate any) {
		extra := map[string]any{}
		if rate != nil {
			extra[service.UpstreamBillingProbeExtraKey] = map[string]any{
				"status": status,
				"data":   map[string]any{"effective_rate_multiplier": rate},
			}
		}
		mustCreateAccount(s.T(), s.client, &service.Account{Name: name, Extra: extra})
	}
	makeAccount("high-rate", service.UpstreamBillingProbeStatusOK, 0.8)
	makeAccount("low-rate", service.UpstreamBillingProbeStatusOK, 0.03)
	makeAccount("missing-rate", "", nil)
	makeAccount("unsupported-with-retained-rate", service.UpstreamBillingProbeStatusUnsupported, 0.01)

	for _, tc := range []struct {
		order string
		want  []string
	}{
		{order: "asc", want: []string{"low-rate", "high-rate", "missing-rate", "unsupported-with-retained-rate"}},
		{order: "desc", want: []string{"high-rate", "low-rate", "unsupported-with-retained-rate", "missing-rate"}},
	} {
		accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
			Page: 1, PageSize: 10, SortBy: "upstream_billing_rate", SortOrder: tc.order,
		}, "", "", "", "", 0, "")
		s.Require().NoError(err)
		s.Require().Len(accounts, 4)
		for i, name := range tc.want {
			s.Require().Equal(name, accounts[i].Name)
		}
	}
}

func (s *AccountRepoSuite) TestListWithFilters_SortByCurrentUpstreamBillingRateDuringPeak() {
	now := time.Now()
	locations := []string{"UTC", "Asia/Shanghai", "America/New_York", "Europe/London"}
	var timezone string
	var minute int
	for _, name := range locations {
		location, err := time.LoadLocation(name)
		s.Require().NoError(err)
		local := now.In(location)
		candidate := local.Hour()*60 + local.Minute()
		if candidate >= 2 && candidate <= 1436 {
			timezone = name
			minute = candidate
			break
		}
	}
	s.Require().NotEmpty(timezone)

	peakStart := fmt.Sprintf("%02d:%02d", (minute-2)/60, (minute-2)%60)
	peakEnd := fmt.Sprintf("%02d:%02d", (minute+3)/60, (minute+3)%60)
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "current-peak-rate",
		Extra: map[string]any{
			service.UpstreamBillingProbeExtraKey: map[string]any{
				"status": service.UpstreamBillingProbeStatusOK,
				"data": map[string]any{
					"billing_scope":             "token",
					"resolved_rate_multiplier":  1.0,
					"effective_rate_multiplier": 1.0,
					"peak_rate_enabled":         true,
					"peak_start":                peakStart,
					"peak_end":                  peakEnd,
					"peak_rate_multiplier":      10.0,
					"timezone":                  timezone,
				},
			},
		},
	})
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "current-off-peak-rate",
		Extra: map[string]any{
			service.UpstreamBillingProbeExtraKey: map[string]any{
				"status": service.UpstreamBillingProbeStatusOK,
				"data":   map[string]any{"effective_rate_multiplier": 5.0},
			},
		},
	})

	for _, tc := range []struct {
		order string
		want  []string
	}{
		{order: "asc", want: []string{"current-off-peak-rate", "current-peak-rate"}},
		{order: "desc", want: []string{"current-peak-rate", "current-off-peak-rate"}},
	} {
		accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
			Page: 1, PageSize: 10, SortBy: "upstream_billing_rate", SortOrder: tc.order,
		}, "", "", "", "", 0, "")
		s.Require().NoError(err)
		s.Require().Len(accounts, 2)
		for i, name := range tc.want {
			s.Require().Equal(name, accounts[i].Name)
		}
	}
}
