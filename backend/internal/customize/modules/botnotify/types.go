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
	// Images reference pictures attached to the message. Bytes are fetched
	// only when the AI answers and are never stored.
	Images []ImageRef `json:"-"`
	// Attachment names a non-image attachment the bot cannot read ("voice",
	// "video", "file", "sticker"); empty when there is none.
	Attachment string `json:"-"`
	// ReplyRef is the platform's native id of the inbound message, used to
	// thread replies (QQ needs it to send a passive reply at all).
	ReplyRef string `json:"-"`
}

// ImageRef points at one image attached to an inbound message.
type ImageRef struct {
	ID   string // Telegram file_id or QQ attachment URL
	MIME string // declared by the platform; re-checked after download
	Size int64  // declared bytes, 0 when unknown
}

// Image is downloaded image content whose type was verified by magic bytes.
type Image struct {
	MIME string
	Data []byte
}

// A transport implementation owns its protocol and maps inbound events to Message.
type Platform interface {
	ValidateToken(context.Context, string) (Bot, error)
	SendMessage(context.Context, string, string, string) error
	Poll(context.Context, string, int64) ([]Message, error)
}

// ImageFetcher is implemented by transports that can download attachments.
type ImageFetcher interface {
	FetchImage(ctx context.Context, token string, ref ImageRef) (Image, error)
}

// Replier is implemented by transports that can thread a reply to the
// inbound message: Telegram quotes it in groups, QQ sends a passive reply.
type Replier interface {
	Reply(ctx context.Context, token string, to Message, text string) error
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
	// AppID is the QQ bot's app id in clear text (the secret rides encrypted
	// in BotToken as a JSON credential blob). Empty for Telegram bots.
	AppID string `json:"app_id,omitempty"`
	// AnnounceSync pushes newly published or edited announcements to the
	// bot's push targets automatically.
	AnnounceSync bool `json:"announce_sync_enabled"`
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

// Announcement is one currently-active site announcement with its raw
// Markdown body; formatting (Markdown flattening) happens in this package.
type Announcement struct {
	ID      int64
	Title   string
	Content string
}

type Data interface {
	Channels(context.Context) ([]Channel, error)
	Announcements(context.Context) ([]Announcement, error)
	SiteName(context.Context) (string, error)
}
