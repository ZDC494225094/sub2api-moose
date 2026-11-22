package botnotify

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNextPushDelayAlignsToWallClock(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	at := func(hour, min, sec int) time.Time {
		return time.Date(2026, 10, 17, hour, min, sec, 0, zone)
	}
	// Hourly pushes land exactly on the hour.
	require.Equal(t, 39*time.Minute+30*time.Second, nextPushDelay(at(12, 20, 30), time.Hour))
	// Exactly on a boundary waits for the following one, never zero.
	require.Equal(t, time.Hour, nextPushDelay(at(12, 0, 0), time.Hour))
	// A 30-minute interval aligns to :00 and :30.
	require.Equal(t, 10*time.Minute, nextPushDelay(at(12, 20, 0), 30*time.Minute))
	// The last run of the day rolls to midnight.
	require.Equal(t, time.Minute, nextPushDelay(at(23, 59, 0), time.Hour))
	// Sub-minute/invalid intervals fall back to hourly alignment.
	require.Equal(t, 39*time.Minute+30*time.Second, nextPushDelay(at(12, 20, 30), 0))
}

func TestParsePushTimesCanonicalizesDedupesAndSorts(t *testing.T) {
	got, err := ParsePushTimes([]string{"21:30", " 9:00", "13:00", "09:00", "", "00:00"})
	require.NoError(t, err)
	require.Equal(t, []string{"00:00", "09:00", "13:00", "21:30"}, got)
	empty, err := ParsePushTimes(nil)
	require.NoError(t, err)
	require.Empty(t, empty)
	for _, bad := range []string{"24:00", "12:60", "9", "9:5", "12:00:00", "ab:cd", "-1:00", "+1:00", "123:00", "1 :00"} {
		_, err := ParsePushTimes([]string{bad})
		require.Error(t, err, bad)
	}
	many := make([]string, 0, MaxPushTimes+1)
	for i := 0; i <= MaxPushTimes; i++ {
		many = append(many, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Minute).Format("15:04"))
	}
	_, err = ParsePushTimes(many[:MaxPushTimes])
	require.NoError(t, err)
	_, err = ParsePushTimes(many)
	require.Error(t, err)
}

func TestDailyScheduleNextAndPrevious(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	at := func(day, hour, min, sec int) time.Time {
		return time.Date(2026, 11, day, hour, min, sec, 0, zone)
	}
	s, err := scheduleOf(Config{PushMode: PushModeDaily, PushTimes: []string{"21:30", "09:00", "13:00"}})
	require.NoError(t, err)
	require.Equal(t, at(22, 9, 0, 0), s.next(at(22, 8, 59, 59)))
	// Exactly at a time point the next one is the following point, never itself.
	require.Equal(t, at(22, 13, 0, 0), s.next(at(22, 9, 0, 0)))
	require.Equal(t, at(22, 21, 30, 0), s.next(at(22, 13, 6, 0)))
	// After the last point of the day it rolls to the first one tomorrow.
	require.Equal(t, at(23, 9, 0, 0), s.next(at(22, 21, 30, 0)))
	require.Equal(t, at(22, 9, 0, 0), s.previous(at(22, 9, 0, 0)))
	require.Equal(t, at(22, 13, 0, 0), s.previous(at(22, 13, 6, 0)))
	require.Equal(t, at(21, 21, 30, 0), s.previous(at(22, 8, 0, 0)))
	// Slots follow the location of now (the server timezone), not UTC.
	utcNow := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC) // 12:00 CST
	require.Equal(t, at(22, 13, 0, 0).Unix(), s.next(utcNow.In(zone)).Unix())

	_, err = scheduleOf(Config{PushMode: PushModeDaily})
	require.Error(t, err)
	legacy, err := scheduleOf(Config{IntervalMinutes: 30})
	require.NoError(t, err)
	require.Equal(t, at(22, 13, 30, 0), legacy.next(at(22, 13, 6, 0)))
	require.Equal(t, at(22, 13, 0, 0), legacy.previous(at(22, 13, 6, 0)))
}

func TestCatchUpSlotOnlyForRecentlyMissedSlots(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	at := func(hour, min int) time.Time { return time.Date(2026, 11, 22, hour, min, 0, 0, zone) }
	s, err := scheduleOf(Config{PushMode: PushModeDaily, PushTimes: []string{"09:00", "13:00"}})
	require.NoError(t, err)
	yesterday := at(9, 0).AddDate(0, 0, -1)
	lastNine := at(9, 0)
	// Restarted shortly after 13:00 without having pushed it: send it now.
	slot, ok := catchUpSlot(at(13, 3), s, &lastNine, yesterday)
	require.True(t, ok)
	require.Equal(t, at(13, 0), slot)
	// Already pushed 13:00 before the restart: nothing to catch up.
	lastThirteen := at(13, 0)
	_, ok = catchUpSlot(at(13, 3), s, &lastThirteen, yesterday)
	require.False(t, ok)
	// Too long after the slot.
	_, ok = catchUpSlot(at(13, 11), s, &lastNine, yesterday)
	require.False(t, ok)
	// Never pushed before (new bot, or upgraded from the untracked scheduler).
	_, ok = catchUpSlot(at(13, 3), s, nil, yesterday)
	require.False(t, ok)
	// Config saved after the slot: enabling a past time point never fires it.
	_, ok = catchUpSlot(at(13, 3), s, &lastNine, at(13, 2))
	require.False(t, ok)
}

func TestPushSlotSendsOnlyWhenClaimWins(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := &testPlatform{}
	r := NewRuntime(&Store{DB: db}, map[string]Platform{"telegram": p}, &testData{}, enabledState{enabled: true})
	e := BotEntry{ID: 7, Config: Config{BotToken: "t", ChatIDs: []string{"1", "2"}, PushTemplate: "hello"}}
	slot := time.Date(2026, 11, 22, 13, 0, 0, 0, time.FixedZone("CST", 8*3600))
	claim := regexp.QuoteMeta("UPDATE bot_notify_bots SET last_push_at=$1 WHERE id=$2 AND (last_push_at IS NULL OR last_push_at < $1)")
	mock.ExpectExec(claim).WithArgs(slot, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.True(t, r.pushSlot(context.Background(), e, p, slot))
	require.Equal(t, 2, p.sent)
	// Another instance (or this one before a restart) already claimed it.
	mock.ExpectExec(claim).WithArgs(slot, int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.False(t, r.pushSlot(context.Background(), e, p, slot))
	require.Equal(t, 2, p.sent)
	// A failed claim never sends.
	mock.ExpectExec(claim).WithArgs(slot, int64(7)).WillReturnError(errors.New("db down"))
	require.False(t, r.pushSlot(context.Background(), e, p, slot))
	require.Equal(t, 2, p.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPushStateReadsWatermark(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	s := &Store{DB: db}
	updated := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	last := time.Date(2026, 11, 22, 5, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT last_push_at, updated_at FROM bot_notify_bots").WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"last_push_at", "updated_at"}).AddRow(nil, updated))
	got, gotUpdated, err := s.PushState(context.Background(), 3)
	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, updated, gotUpdated)
	mock.ExpectQuery("SELECT last_push_at, updated_at FROM bot_notify_bots").WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"last_push_at", "updated_at"}).AddRow(last, updated))
	got, _, err = s.PushState(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, last, *got)
}

func TestConfigPushModeValidationAndLegacyDefaults(t *testing.T) {
	// Configs saved before daily times decode as interval mode.
	s := &Store{Cipher: testCipher{}}
	c, err := s.decodeConfig([]byte(`{"push_enabled":true,"interval_minutes":60,"chat_ids":["1"]}`), true)
	require.NoError(t, err)
	require.Equal(t, PushModeInterval, c.PushMode)
	require.Equal(t, []string{}, c.PushTimes)
	sched, err := scheduleOf(c)
	require.NoError(t, err)
	require.Equal(t, time.Hour, sched.interval)

	n := normalizeConfig(Config{PushMode: PushModeDaily, PushTimes: []string{"21:30", "9:00", "09:00"}})
	require.Equal(t, []string{"09:00", "21:30"}, n.PushTimes)
	require.NoError(t, ValidateConfig("telegram", n))
	require.Error(t, ValidateConfig("telegram", normalizeConfig(Config{PushMode: "weekly"})))
	require.Error(t, ValidateConfig("telegram", normalizeConfig(Config{PushMode: PushModeDaily, PushTimes: []string{"25:00"}})))
	require.Error(t, ValidateConfig("telegram", normalizeConfig(Config{PushEnabled: true, PushMode: PushModeDaily})))
	// Daily mode with push disabled may be saved without times.
	require.NoError(t, ValidateConfig("telegram", normalizeConfig(Config{PushMode: PushModeDaily})))
}
