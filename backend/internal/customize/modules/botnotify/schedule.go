package botnotify

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Push modes. An empty mode in a stored config is the legacy interval mode,
// so bots saved before daily times existed keep their schedule unchanged.
const (
	PushModeInterval = "interval"
	PushModeDaily    = "daily"
)

// MaxPushTimes bounds the daily time points one bot may configure.
const MaxPushTimes = 48

// pushCatchUp is how long after a slot a freshly started worker still sends
// it, so a restart or lease handover right at a push time does not skip it.
const pushCatchUp = 10 * time.Minute

// ParsePushTimes validates "HH:MM" (or "H:MM") time points and returns them
// canonical ("09:00"), de-duplicated and sorted.
func ParsePushTimes(values []string) ([]string, error) {
	seen := map[int]bool{}
	minutes := make([]int, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		m, ok := parseClock(value)
		if !ok {
			return nil, fmt.Errorf("推送时间点 %q 格式错误，应为 HH:MM（00:00–23:59）", value)
		}
		if !seen[m] {
			seen[m] = true
			minutes = append(minutes, m)
		}
	}
	if len(minutes) > MaxPushTimes {
		return nil, fmt.Errorf("推送时间点最多 %d 个", MaxPushTimes)
	}
	sort.Ints(minutes)
	out := make([]string, len(minutes))
	for i, m := range minutes {
		out[i] = fmt.Sprintf("%02d:%02d", m/60, m%60)
	}
	return out, nil
}

// parseClock returns minutes after midnight for "H:MM" / "HH:MM".
func parseClock(value string) (int, bool) {
	hh, mm, ok := strings.Cut(value, ":")
	if !ok || len(hh) < 1 || len(hh) > 2 || len(mm) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 || strings.ContainsAny(value, "+- ") {
		return 0, false
	}
	return h*60 + m, true
}

// pushSchedule is a bot's resolved broadcast schedule: either daily time
// points (minutes after local midnight) or a wall-clock aligned interval.
type pushSchedule struct {
	daily    []int
	interval time.Duration
}

func scheduleOf(c Config) (pushSchedule, error) {
	if c.PushMode == PushModeDaily {
		times, err := ParsePushTimes(c.PushTimes)
		if err != nil {
			return pushSchedule{}, err
		}
		if len(times) == 0 {
			return pushSchedule{}, errors.New("已启用每日定时推送，但还没有配置推送时间点")
		}
		s := pushSchedule{daily: make([]int, len(times))}
		for i, v := range times {
			s.daily[i], _ = parseClock(v)
		}
		return s, nil
	}
	interval := time.Duration(c.IntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Hour
	}
	return pushSchedule{interval: interval}, nil
}

func midnightOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// slotOn builds the wall-clock slot of a day; time.Date resolves DST gaps.
func slotOn(day time.Time, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, day.Location())
}

// next returns the first slot strictly after now, in now's location.
func (s pushSchedule) next(now time.Time) time.Time {
	if len(s.daily) == 0 {
		midnight := midnightOf(now)
		elapsed := now.Sub(midnight)
		return midnight.Add((elapsed/s.interval + 1) * s.interval)
	}
	day := midnightOf(now)
	for i := 0; i < 3; i++ {
		for _, m := range s.daily {
			if slot := slotOn(day, m); slot.After(now) {
				return slot
			}
		}
		day = midnightOf(day.AddDate(0, 0, 1))
	}
	return now.Add(24 * time.Hour) // unreachable for a non-empty schedule
}

// previous returns the latest slot at or before now.
func (s pushSchedule) previous(now time.Time) time.Time {
	if len(s.daily) == 0 {
		midnight := midnightOf(now)
		elapsed := now.Sub(midnight)
		return midnight.Add(elapsed / s.interval * s.interval)
	}
	day := midnightOf(now)
	for i := 0; i < 3; i++ {
		for j := len(s.daily) - 1; j >= 0; j-- {
			if slot := slotOn(day, s.daily[j]); !slot.After(now) {
				return slot
			}
		}
		day = midnightOf(day.AddDate(0, 0, -1))
	}
	return now.Add(-24 * time.Hour)
}

// catchUpSlot decides whether a worker that starts at now should still send
// the most recent slot. It must be recent, a push must have been recorded
// before (a bot upgraded from the untracked scheduler, or one never pushed,
// starts clean), that push must be older than the slot, and the bot config
// must predate the slot: enabling or editing a schedule never fires a time
// point that has already passed.
func catchUpSlot(now time.Time, s pushSchedule, lastPush *time.Time, updatedAt time.Time) (time.Time, bool) {
	slot := s.previous(now)
	if lastPush == nil || now.Sub(slot) > pushCatchUp || !lastPush.Before(slot) || updatedAt.After(slot) {
		return time.Time{}, false
	}
	return slot, true
}

// nextPushDelay returns the wait until the next wall-clock boundary that is
// a whole multiple of interval after local midnight, so a 60-minute interval
// fires exactly on the hour (13:00, 14:00, ...) instead of drifting with the
// worker's start time.
func nextPushDelay(now time.Time, interval time.Duration) time.Duration {
	if interval < time.Minute {
		interval = time.Hour
	}
	return pushSchedule{interval: interval}.next(now).Sub(now)
}
