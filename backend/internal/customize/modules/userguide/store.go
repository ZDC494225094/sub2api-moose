package userguide

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Store struct {
	DB *sql.DB
}

func (s *Store) Config(ctx context.Context) (Config, error) {
	if s == nil || s.DB == nil {
		return Config{}, errors.New("userguide: store unavailable")
	}
	var title, content string
	var updatedAt sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT title, content_md, updated_at FROM user_guide_config WHERE id = 1`).
		Scan(&title, &content, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return resolve("", "", nil), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("userguide: load config: %w", err)
	}
	var updated *time.Time
	if updatedAt.Valid {
		updated = &updatedAt.Time
	}
	return resolve(title, content, updated), nil
}

func (s *Store) Save(ctx context.Context, title, content string) (Config, error) {
	if s == nil || s.DB == nil {
		return Config{}, errors.New("userguide: store unavailable")
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO user_guide_config (id, title, content_md, updated_at)
		VALUES (1, $1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title, content_md = EXCLUDED.content_md, updated_at = CURRENT_TIMESTAMP`,
		title, content)
	if err != nil {
		return Config{}, fmt.Errorf("userguide: save config: %w", err)
	}
	return s.Config(ctx)
}

// Reset clears the saved config so the embedded default manual is served.
func (s *Store) Reset(ctx context.Context) (Config, error) {
	if s == nil || s.DB == nil {
		return Config{}, errors.New("userguide: store unavailable")
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE user_guide_config SET title = '', content_md = '', updated_at = CURRENT_TIMESTAMP WHERE id = 1`); err != nil {
		return Config{}, fmt.Errorf("userguide: reset config: %w", err)
	}
	return s.Config(ctx)
}
