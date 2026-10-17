// Package botnotify owns bot configuration, transport, history and workers.
package botnotify

import (
	"context"
	"time"
)

const ID = "bot-notify"
const RetentionDays = 30
const RetentionCount = 2000

type Bot struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Message struct {
	Platform     string   `json:"platform"`
	UpdateID     int64    `json:"update_id"`
	MessageID    int64    `json:"message_id"`
	ChatID       string   `json:"chat_id"`
	ChatType     string   `json:"chat_type"`
	SenderID     string   `json:"sender_id"`
	SenderName   string   `json:"sender_name"`
	Text         string   `json:"text"`
	Mentions     []string `json:"-"`
	ReplyToBotID int64    `json:"-"`
}

// A transport implementation owns its protocol and maps inbound events to Message.
type Platform interface {
	ValidateToken(context.Context, string) (Bot, error)
	SendMessage(context.Context, string, string, string) error
	Poll(context.Context, string, int64) ([]Message, error)
}

type Encryptor interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type Config struct {
	BotToken        string   `json:"bot_token"`
	ReceiveEnabled  bool     `json:"receive_enabled"`
	PushEnabled     bool     `json:"push_enabled"`
	ChatIDs         []string `json:"chat_ids"`
	IntervalMinutes int      `json:"interval_minutes"`
	PushTemplate    string   `json:"push_template"`
	BotUsername     string   `json:"bot_username"`
	BotID           int64    `json:"bot_id"`
}

// BotEntry is one configured bot row: an admin label, a platform and the
// per-bot Config (flattened into the same JSON object).
type BotEntry struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Config
}

type Rule struct {
	ID            int64     `json:"id"`
	BotID         int64     `json:"bot_id"`
	Keyword       string    `json:"keyword"`
	MatchType     string    `json:"match_type"`
	ReplyTemplate string    `json:"reply_template"`
	Enabled       bool      `json:"enabled"`
	Sort          int       `json:"sort"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Point struct {
	Status    string
	CheckedAt time.Time
}
type Channel struct {
	GroupName, Model, Status string
	LatencyMS                int
	Availability             float64
	Timeline                 []Point
}

type Data interface {
	Channels(context.Context) ([]Channel, error)
	Announcements(context.Context) (string, error)
	SiteName(context.Context) (string, error)
}
