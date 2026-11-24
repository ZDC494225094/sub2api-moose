package iqmonitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Store persists module data. All SQL is portable between PostgreSQL and the
// SQLite test fixture; times are always passed in from Go (UTC).
type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

var errStoreUnavailable = errors.New("iq-monitor: store unavailable")

func (s *Store) ready() error {
	if s == nil || s.DB == nil {
		return errStoreUnavailable
	}
	return nil
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func intArg(v *int) any {
	if v == nil {
		return nil
	}
	return int64(*v)
}

func decodeAnswers(raw string) []string {
	var out []string
	if json.Unmarshal([]byte(raw), &out) != nil || out == nil {
		return []string{}
	}
	return out
}

func encodeAnswers(v []string) string {
	if v == nil {
		v = []string{}
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// Config returns the stored configuration merged over defaults.
func (s *Store) Config(ctx context.Context) (Config, *time.Time, error) {
	if err := s.ready(); err != nil {
		return Config{}, nil, err
	}
	var raw string
	var updated sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT config, updated_at FROM iq_monitor_settings WHERE id = 1`).Scan(&raw, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultConfig(), nil, nil
	}
	if err != nil {
		return Config{}, nil, fmt.Errorf("iq-monitor: load config: %w", err)
	}
	cfg := DefaultConfig()
	if raw != "" && raw != "{}" {
		// Unmarshal over defaults keeps fields that older documents lack.
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return Config{}, nil, fmt.Errorf("iq-monitor: decode config: %w", err)
		}
	}
	return cfg.normalize(), nullTimePtr(updated), nil
}

func (s *Store) SaveConfig(ctx context.Context, cfg Config) error {
	if err := s.ready(); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_settings SET config = $1, updated_at = $2 WHERE id = 1`, string(raw), s.now())
	if err != nil {
		return fmt.Errorf("iq-monitor: save config: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_, err = s.DB.ExecContext(ctx, `INSERT INTO iq_monitor_settings (id, config, updated_at) VALUES (1, $1, $2)`, string(raw), s.now())
	}
	return err
}
