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
	// ReasoningEffort is passed to reasoning models ("" leaves the model
	// default). Low keeps answers fast and the output budget for the reply.
	ReasoningEffort string `json:"reasoning_effort"`
	AllowImages     bool   `json:"allow_images"`
	// AllowUsageStats lets the bot quote aggregate per-group cache hit rates
	// of public groups; per-user data is never reachable.
	AllowUsageStats bool `json:"allow_usage_stats"`
	// UseUserGuide feeds the public docs-center manual to the model.
	UseUserGuide bool `json:"use_user_guide"`
	// KnowledgeBase is the admin-maintained fact sheet; empty means
	// DefaultKnowledgeBase.
	KnowledgeBase string `json:"knowledge_base"`
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
	InputTokens   int       `json:"input_tokens,omitempty"`
	OutputTokens  int       `json:"output_tokens,omitempty"`
	ImageCount    int       `json:"image_count,omitempty"`
}

var aiReasoningEfforts = map[string]bool{"": true, "low": true, "medium": true, "high": true}

func DefaultAIConfig(botID int64) AIConfig {
	return AIConfig{BotID: botID, MaxInputChars: 4000, MaxOutputTokens: 1200, CooldownSeconds: 10, DailyLimit: 100, AllowRecharge: true, AllowTechnical: true, KnowledgeVersion: "public-v1", ReasoningEffort: "low", AllowImages: true, AllowUsageStats: true, UseUserGuide: true}
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
	if !aiReasoningEfforts[c.ReasoningEffort] {
		return errors.New("推理强度只能是 low、medium、high 或留空")
	}
	if len([]rune(c.KnowledgeBase)) > MaxKnowledgeBaseRunes {
		return fmt.Errorf("知识库不能超过 %d 字", MaxKnowledgeBaseRunes)
	}
	if c.Enabled && (c.APIKeyID == nil || *c.APIKeyID < 1 || strings.TrimSpace(c.Model) == "") {
		return errors.New("启用 AI 客服时必须选择 API Key 和模型")
	}
	return nil
}

func (s *Store) AIConfig(ctx context.Context, botID int64) (AIConfig, error) {
	c := DefaultAIConfig(botID)
	row := s.DB.QueryRowContext(ctx, `SELECT enabled,api_key_id,model,max_input_chars,max_output_tokens,cooldown_seconds,daily_limit,allow_recharge,allow_technical,knowledge_version,reasoning_effort,allow_images,allow_usage_stats,use_user_guide,knowledge_base FROM bot_ai_configs WHERE bot_id=$1`, botID)
	err := row.Scan(&c.Enabled, &c.APIKeyID, &c.Model, &c.MaxInputChars, &c.MaxOutputTokens, &c.CooldownSeconds, &c.DailyLimit, &c.AllowRecharge, &c.AllowTechnical, &c.KnowledgeVersion, &c.ReasoningEffort, &c.AllowImages, &c.AllowUsageStats, &c.UseUserGuide, &c.KnowledgeBase)
	if err == sql.ErrNoRows {
		return c, nil
	}
	return c, err
}

func (s *Store) SaveAIConfig(ctx context.Context, c AIConfig) error {
	c.KnowledgeBase = normalizeKnowledgeBase(c.KnowledgeBase)
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
	_, err := s.DB.ExecContext(ctx, `INSERT INTO bot_ai_configs(bot_id,enabled,api_key_id,model,max_input_chars,max_output_tokens,cooldown_seconds,daily_limit,allow_recharge,allow_technical,knowledge_version,reasoning_effort,allow_images,allow_usage_stats,use_user_guide,knowledge_base,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,CURRENT_TIMESTAMP) ON CONFLICT(bot_id) DO UPDATE SET enabled=EXCLUDED.enabled,api_key_id=EXCLUDED.api_key_id,model=EXCLUDED.model,max_input_chars=EXCLUDED.max_input_chars,max_output_tokens=EXCLUDED.max_output_tokens,cooldown_seconds=EXCLUDED.cooldown_seconds,daily_limit=EXCLUDED.daily_limit,allow_recharge=EXCLUDED.allow_recharge,allow_technical=EXCLUDED.allow_technical,knowledge_version=EXCLUDED.knowledge_version,reasoning_effort=EXCLUDED.reasoning_effort,allow_images=EXCLUDED.allow_images,allow_usage_stats=EXCLUDED.allow_usage_stats,use_user_guide=EXCLUDED.use_user_guide,knowledge_base=EXCLUDED.knowledge_base,updated_at=CURRENT_TIMESTAMP`, c.BotID, c.Enabled, c.APIKeyID, c.Model, c.MaxInputChars, c.MaxOutputTokens, c.CooldownSeconds, c.DailyLimit, c.AllowRecharge, c.AllowTechnical, c.KnowledgeVersion, c.ReasoningEffort, c.AllowImages, c.AllowUsageStats, c.UseUserGuide, c.KnowledgeBase)
	return err
}

func (s *Store) AIKeys(ctx context.Context, userID int64) ([]AIKeyOption, error) {
	if s.KeyLister != nil {
		return s.KeyLister(ctx, userID)
	}
	return nil, errors.New("AI 客服尚未配置安全的 API Key 列表提供器")
}

func (s *Store) AIRecords(ctx context.Context, botID int64, keyword, blocked string, page, size int) ([]AIRecord, int64, error) {
	where := ` FROM bot_ai_messages m JOIN bot_ai_conversations c ON c.id=m.conversation_id WHERE c.bot_id=$1 AND m.direction='assistant' AND ($2='' OR strpos(m.question_redacted,$2)>0 OR strpos(m.content_redacted,$2)>0 OR strpos(m.blocked_reason,$2)>0 OR strpos(c.chat_id,$2)>0)`
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
	rows, err := s.DB.QueryContext(ctx, "SELECT m.id,m.created_at,c.chat_id,m.sender_name,m.question_redacted,m.content_redacted,m.blocked,m.blocked_reason,m.model,m.latency_ms,m.error_code,m.input_tokens,m.output_tokens,m.image_count"+where+" ORDER BY m.id DESC LIMIT $3 OFFSET $4", append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AIRecord{}
	for rows.Next() {
		var r AIRecord
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.ChatID, &r.SenderName, &r.Question, &r.Answer, &r.Blocked, &r.BlockedReason, &r.Model, &r.LatencyMS, &r.ErrorCode, &r.InputTokens, &r.OutputTokens, &r.ImageCount); err != nil {
			return nil, 0, err
		}
		r.BotID = botID
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// PublicDTO is intentionally narrow; it is the only context shape an AI
// provider may receive. It contains no users, credentials, SQL or admin data:
// only site settings and the docs manual that visitors can already see.
type PublicDTO struct {
	SiteName      string
	ChannelStatus string
	Announcements string
	// SiteURL is the public frontend address from site settings.
	SiteURL string
	// Support channels shown in the site's contact widget.
	ContactInfo         string
	AfterSalesGroup     string
	CustomerServiceLink string
	// Guide is the public docs-center manual (Markdown), empty when that
	// module is off.
	GuideTitle string
	Guide      string
}

// KnowledgeSource supplies the PublicDTO from host settings and modules.
type KnowledgeSource interface {
	AIKnowledge(context.Context) (PublicDTO, error)
}

// AIHistoryTurn is one past exchange in the same conversation, taken from
// the redacted audit store. Blocked or failed exchanges are never loaded.
type AIHistoryTurn struct {
	User      string
	Assistant string
}

type AIRequest struct {
	Config  AIConfig
	Message Message
	Context PublicDTO
	History []AIHistoryTurn
	// Images are the verified pictures of this turn, at most MaxImagesPerAnswer.
	Images []Image
	// ImageNote tells the model about images it will not see (too large,
	// download failed, recognition disabled).
	ImageNote string
}

// AIResult is a completed answer plus the accounting the audit log keeps.
type AIResult struct {
	Text         string
	InputTokens  int
	OutputTokens int
	Tools        []string
	// ToolOutputs are the tool results the answer was based on; links in
	// them are trusted by the output link check.
	ToolOutputs []string
	// ImagesDropped reports that the model rejected image input and the
	// answer was produced from the text alone.
	ImagesDropped bool
}

type AIProvider interface {
	Complete(context.Context, AIRequest) (AIResult, error)
}

func aiError(code string) error { return fmt.Errorf("ai_%s", code) }

// AIErrorCode reduces a provider error to a short code without secrets for
// the audit log: "provider_http_429", "key_unavailable", "timeout", ...
func AIErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	msg := err.Error()
	if code, ok := strings.CutPrefix(msg, "ai_"); ok {
		if i := strings.IndexAny(code, ": "); i >= 0 {
			code = code[:i]
		}
		return code
	}
	return "provider_error"
}

// aiErrorNeedsAdmin reports provider errors that persist until an admin acts
// (bad key, no balance, model not in the key's group) rather than transient
// upstream trouble.
func aiErrorNeedsAdmin(code string) bool {
	switch code {
	case "not_configured", "key_unavailable", "provider_http_401", "provider_http_402", "provider_http_403", "provider_http_404":
		return true
	}
	return false
}
