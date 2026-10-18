package botnotify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AIConfig struct {
	BotID            int64  `json:"bot_id"`
	Enabled          bool   `json:"enabled"`
	APIKeyID         *int64 `json:"api_key_id"`
	Model            string `json:"model"`
	MaxInputChars    int    `json:"max_input_chars"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	CooldownSeconds  int    `json:"cooldown_seconds"`
	DailyLimit       int    `json:"daily_limit"`
	AllowRecharge    bool   `json:"allow_recharge"`
	AllowTechnical   bool   `json:"allow_technical"`
	KnowledgeVersion string `json:"knowledge_version"`
}

type AIKeyOption struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Platform string   `json:"platform"`
	Status   string   `json:"status"`
	Models   []string `json:"models"`
}

type AIRecord struct {
	ID            int64     `json:"id"`
	BotID         int64     `json:"bot_id"`
	CreatedAt     time.Time `json:"created_at"`
	ChatID        string    `json:"chat_id"`
	SenderName    string    `json:"sender_name"`
	Question      string    `json:"question"`
	Answer        string    `json:"answer"`
	Blocked       bool      `json:"blocked"`
	BlockedReason string    `json:"blocked_reason,omitempty"`
	Model         string    `json:"model,omitempty"`
	LatencyMS     int       `json:"latency_ms,omitempty"`
	ErrorCode     string    `json:"error_code,omitempty"`
}

func DefaultAIConfig(botID int64) AIConfig {
	return AIConfig{BotID: botID, MaxInputChars: 4000, MaxOutputTokens: 800, CooldownSeconds: 10, DailyLimit: 100, AllowRecharge: true, AllowTechnical: true, KnowledgeVersion: "public-v1"}
}

func ValidateAIConfig(c AIConfig) error {
	if c.MaxInputChars < 1 || c.MaxInputChars > AIInputLimit {
		return errors.New("最大输入字符须为 1 至 8000")
	}
	if c.MaxOutputTokens < 1 || c.MaxOutputTokens > 4000 {
		return errors.New("最大输出 token 须为 1 至 4000")
	}
	if c.CooldownSeconds < 0 || c.CooldownSeconds > 3600 {
		return errors.New("冷却时间须为 0 至 3600 秒")
	}
	if c.DailyLimit < 0 || c.DailyLimit > 100000 {
		return errors.New("每日上限须为 0 至 100000")
	}
	if c.Enabled && (c.APIKeyID == nil || *c.APIKeyID < 1 || strings.TrimSpace(c.Model) == "") {
		return errors.New("启用 AI 客服时必须选择 API Key 和模型")
	}
	return nil
}

func (s *Store) AIConfig(ctx context.Context, botID int64) (AIConfig, error) {
	c := DefaultAIConfig(botID)
	row := s.DB.QueryRowContext(ctx, `SELECT enabled,api_key_id,model,max_input_chars,max_output_tokens,cooldown_seconds,daily_limit,allow_recharge,allow_technical,knowledge_version FROM bot_ai_configs WHERE bot_id=$1`, botID)
	err := row.Scan(&c.Enabled, &c.APIKeyID, &c.Model, &c.MaxInputChars, &c.MaxOutputTokens, &c.CooldownSeconds, &c.DailyLimit, &c.AllowRecharge, &c.AllowTechnical, &c.KnowledgeVersion)
	if err == sql.ErrNoRows {
		return c, nil
	}
	return c, err
}

func (s *Store) SaveAIConfig(ctx context.Context, c AIConfig) error {
	if err := ValidateAIConfig(c); err != nil {
		return err
	}
	if c.APIKeyID != nil {
		if s.KeyValidator == nil {
			return errors.New("AI 客服尚未配置安全的 API Key 校验器")
		}
		if err := s.KeyValidator(ctx, *c.APIKeyID, c.Model); err != nil {
			return err
		}
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO bot_ai_configs(bot_id,enabled,api_key_id,model,max_input_chars,max_output_tokens,cooldown_seconds,daily_limit,allow_recharge,allow_technical,knowledge_version,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,CURRENT_TIMESTAMP) ON CONFLICT(bot_id) DO UPDATE SET enabled=EXCLUDED.enabled,api_key_id=EXCLUDED.api_key_id,model=EXCLUDED.model,max_input_chars=EXCLUDED.max_input_chars,max_output_tokens=EXCLUDED.max_output_tokens,cooldown_seconds=EXCLUDED.cooldown_seconds,daily_limit=EXCLUDED.daily_limit,allow_recharge=EXCLUDED.allow_recharge,allow_technical=EXCLUDED.allow_technical,knowledge_version=EXCLUDED.knowledge_version,updated_at=CURRENT_TIMESTAMP`, c.BotID, c.Enabled, c.APIKeyID, c.Model, c.MaxInputChars, c.MaxOutputTokens, c.CooldownSeconds, c.DailyLimit, c.AllowRecharge, c.AllowTechnical, c.KnowledgeVersion)
	return err
}

func (s *Store) AIKeys(ctx context.Context) ([]AIKeyOption, error) {
	if s.KeyLister != nil {
		return s.KeyLister(ctx)
	}
	return nil, errors.New("AI 客服尚未配置安全的 API Key 列表提供器")
}

func (s *Store) AIRecords(ctx context.Context, botID int64, keyword, blocked string, page, size int) ([]AIRecord, int64, error) {
	where := ` FROM bot_ai_messages m JOIN bot_ai_conversations c ON c.id=m.conversation_id WHERE c.bot_id=$1 AND m.direction='assistant' AND ($2='' OR strpos(m.content_redacted,$2)>0 OR strpos(m.blocked_reason,$2)>0)`
	args := []any{botID, keyword}
	if blocked == "true" {
		where += " AND m.blocked=true"
	} else if blocked == "false" {
		where += " AND m.blocked=false"
	}
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*)"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT m.id,m.created_at,c.chat_id,m.sender_name,m.question_redacted,m.content_redacted,m.blocked,m.blocked_reason,m.model,m.latency_ms,m.error_code"+where+" ORDER BY m.id DESC LIMIT $3 OFFSET $4", append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AIRecord{}
	for rows.Next() {
		var r AIRecord
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.ChatID, &r.SenderName, &r.Question, &r.Answer, &r.Blocked, &r.BlockedReason, &r.Model, &r.LatencyMS, &r.ErrorCode); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// PublicDTO is intentionally narrow; it is the only context shape an AI
// provider may receive. It contains no users, credentials, SQL or admin data.
type PublicDTO struct {
	SiteName      string
	ChannelStatus string
	Announcements string
}
type AIRequest struct {
	Config  AIConfig
	Message Message
	Context PublicDTO
	History []string
}
type AIProvider interface {
	Complete(context.Context, AIRequest) (string, error)
}

func aiError(code string) error { return fmt.Errorf("ai_%s", code) }
