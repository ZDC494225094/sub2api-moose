// Package userguide is a business extension that serves the public user
// manual (docs center) from admin-editable configuration. It owns one
// single-row config table; content falls back to the embedded default
// manual whenever nothing has been saved yet.
package userguide

import (
	_ "embed"
	"time"
)

const ID = "user-guide"

const DefaultTitle = "使用手册：从注册到用上 Claude Code / Codex"

//go:embed default.md
var defaultContent string

func DefaultContent() string { return defaultContent }

type Config struct {
	Title     string     `json:"title"`
	ContentMD string     `json:"content_md"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	IsDefault bool       `json:"is_default"`
}

// resolve applies the default-manual fallback: an untouched (or reset)
// config serves the embedded default title and content.
func resolve(title, content string, updatedAt *time.Time) Config {
	if title == "" && content == "" {
		return Config{Title: DefaultTitle, ContentMD: defaultContent, IsDefault: true}
	}
	cfg := Config{Title: title, ContentMD: content, UpdatedAt: updatedAt}
	if cfg.Title == "" {
		cfg.Title = DefaultTitle
	}
	if cfg.ContentMD == "" {
		cfg.ContentMD = defaultContent
		cfg.IsDefault = true
	}
	return cfg
}
