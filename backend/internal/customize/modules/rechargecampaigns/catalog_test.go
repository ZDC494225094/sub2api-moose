package rechargecampaigns

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testCampaign() Campaign {
	return Campaign{ID: 7, Name: "bonus", Enabled: true, Kind: "bonus", Percent: 10,
		StartsAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

type fakeRepository struct {
	items []Campaign
	saved *Campaign
	err   error
}

func (r *fakeRepository) List(context.Context) ([]Campaign, error) { return r.items, r.err }
func (r *fakeRepository) Save(_ context.Context, a Campaign) (*Campaign, error) {
	r.saved = &a
	return &a, r.err
}

func TestCatalogPublicVisibilityAndHistoricalRevision(t *testing.T) {
	a := testCampaign()
	disabled, expired, future := a, a, a
	disabled.ID, disabled.Enabled = 6, false
	expired.ID, expired.EndsAt = 5, a.StartsAt
	future.ID, future.StartsAt = 4, a.EndsAt.Add(-time.Hour)
	repo := &fakeRepository{items: []Campaign{a, disabled, expired, future}}
	service := NewService(repo, nil)
	service.now = func() time.Time { return a.StartsAt.Add(time.Hour) }
	public, err := service.List(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, public, 2)
	require.Equal(t, int64(4), public[1].ID, "upcoming activities must remain visible")
	all, err := service.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, all, 4)
	require.Empty(t, repo.items[0].Revision, "reads must not mutate repository data")
	// Revision and JSON field order must match the pre-extraction model because
	// existing signed checkout requests carry this digest.
	legacyJSON := `{"revision":"","id":0,"name":"bonus","description":"","enabled":true,"starts_at":"2026-09-01T00:00:00Z","ends_at":"2026-10-01T00:00:00Z","kind":"bonus","percent":10,"min_amount":0,"reward_percent":0,"reward_cap":0,"freeze_hours":0,"new_invitees_only":false}`
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(legacyJSON))), public[0].Revision)
	a.Revision = "client-supplied"
	require.Equal(t, public[0].Revision, Revision(a))
	a.Percent = 20
	require.NotEqual(t, public[0].Revision, Revision(a))
}

func TestCatalogSaveValidationAndAffiliatePort(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepository{}
	enabled := false
	service := NewService(repo, func(context.Context) bool { return enabled })
	a := testCampaign()
	a.Name = ""
	_, err := service.Save(ctx, a)
	require.Error(t, err)
	require.Nil(t, repo.saved)
	a = testCampaign()
	a.RewardPercent, a.RewardCap = 5, 10
	_, err = service.Save(ctx, a)
	require.ErrorContains(t, err, "邀请返利")
	require.Nil(t, repo.saved)
	enabled = true
	a.Revision = "untrusted"
	saved, err := service.Save(ctx, a)
	require.NoError(t, err)
	require.NotEqual(t, "untrusted", saved.Revision)
	a.Enabled = false
	enabled = false
	_, err = service.Save(ctx, a)
	require.NoError(t, err, "admin can disable an old activity even if affiliate rewards are off")
	repo.err = errors.New("storage unavailable")
	_, err = service.List(ctx, true)
	require.ErrorIs(t, err, repo.err)
	_, err = service.Save(ctx, a)
	require.ErrorIs(t, err, repo.err)
}

func TestSQLCatalogKeepsTableIdentityAndDetectsReadErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewSQLRepository(db)
	raw, err := json.Marshal(testCampaign())
	require.NoError(t, err)
	query := regexp.QuoteMeta("SELECT id, config FROM recharge_campaigns ORDER BY id DESC")
	mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id", "config"}).AddRow(99, raw)).RowsWillBeClosed()
	items, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(99), items[0].ID, "SQL primary key wins over stale JSON id")
	for _, rows := range []*sqlmock.Rows{
		sqlmock.NewRows([]string{"id", "config"}).AddRow(1, "invalid json"),
		sqlmock.NewRows([]string{"id", "config"}).AddRow(1, raw).RowError(0, errors.New("stream failed")),
		sqlmock.NewRows([]string{"id", "config"}).AddRow("bad-id", raw),
	} {
		mock.ExpectQuery(query).WillReturnRows(rows).RowsWillBeClosed()
		_, err = repo.List(context.Background())
		require.Error(t, err)
	}
	mock.ExpectQuery(query).WillReturnError(errors.New("connection failed"))
	_, err = repo.List(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLCatalogCreateUpdateAndNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewSQLRepository(db)
	a := testCampaign()
	a.ID = 0
	insert := regexp.QuoteMeta("INSERT INTO recharge_campaigns(config) VALUES($1) RETURNING id")
	update := regexp.QuoteMeta("UPDATE recharge_campaigns SET config=$1,updated_at=NOW() WHERE id=$2 RETURNING id")
	raw, _ := json.Marshal(a)
	mock.ExpectQuery(insert).WithArgs(string(raw)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11)).RowsWillBeClosed()
	saved, err := repo.Save(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, int64(11), saved.ID)
	a.ID = 11
	raw, _ = json.Marshal(a)
	mock.ExpectQuery(update).WithArgs(string(raw), int64(11)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11)).RowsWillBeClosed()
	_, err = repo.Save(context.Background(), a)
	require.NoError(t, err)
	mock.ExpectQuery(update).WithArgs(string(raw), int64(11)).WillReturnRows(sqlmock.NewRows([]string{"id"})).RowsWillBeClosed()
	_, err = repo.Save(context.Background(), a)
	require.ErrorContains(t, err, "不存在")
	mock.ExpectQuery(update).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11).RowError(0, errors.New("write stream failed"))).RowsWillBeClosed()
	_, err = repo.Save(context.Background(), a)
	require.ErrorContains(t, err, "write stream failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

type fakeCatalog struct {
	public []bool
	saved  []Campaign
	err    error
}

func (s *fakeCatalog) List(_ context.Context, public bool) ([]Campaign, error) {
	s.public = append(s.public, public)
	return []Campaign{}, s.err
}
func (s *fakeCatalog) Save(_ context.Context, a Campaign) (*Campaign, error) {
	s.saved = append(s.saved, a)
	return &a, s.err
}

func TestHandlerPreservesURLsEnvelopeAndAuthoritativePathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &fakeCatalog{}
	h := &Handler{catalog: catalog}
	r := gin.New()
	r.GET("/public", h.ListPublic)
	r.GET("/admin", h.ListAdmin)
	r.POST("/admin", h.Save)
	r.PUT("/admin/:id", h.Save)
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	require.Contains(t, request("GET", "/public", "", 200).Body.String(), `"data":[]`)
	request("GET", "/admin", "", 200)
	require.Equal(t, []bool{true, false}, catalog.public)
	request("POST", "/admin", `{"id":999}`, 200)
	request("PUT", "/admin/7", `{"id":999}`, 200)
	require.Equal(t, int64(0), catalog.saved[0].ID)
	require.Equal(t, int64(7), catalog.saved[1].ID)
	for _, path := range []string{"/admin/bad", "/admin/0", "/admin/-1"} {
		request("PUT", path, `{}`, 400)
	}
	request("POST", "/admin", `{`, 400)
	require.Len(t, catalog.saved, 2, "invalid requests must never reach persistence")
	catalog.err = errors.New("storage unavailable")
	request("GET", "/admin", "", http.StatusInternalServerError)
	request("POST", "/admin", `{}`, http.StatusInternalServerError)
}
