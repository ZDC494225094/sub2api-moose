package botnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	DB     *sql.DB
	Cipher Encryptor
}

func (s *Store) Config(ctx context.Context, decrypt bool) (Config, error) {
	var raw []byte
	var c Config
	if err := s.DB.QueryRowContext(ctx, "SELECT config FROM bot_notify_config WHERE id=1").Scan(&raw); err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.BotToken != "" {
		plain, err := s.Cipher.Decrypt(c.BotToken)
		if err != nil {
			return Config{}, errors.New("bot token decryption failed; reconfigure token")
		}
		if decrypt {
			c.BotToken = plain
		} else {
			c.BotToken = MaskToken(plain)
		}
	}
	if c.ChatIDs == nil {
		c.ChatIDs = []string{}
	}
	return c, nil
}

func MaskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func ValidateConfig(c Config) error {
	if c.IntervalMinutes < 1 || c.IntervalMinutes > 43200 {
		return errors.New("推送间隔须为 1 至 43200 分钟")
	}
	if len(c.PushTemplate) > 32000 || len(c.ChatIDs) > 100 {
		return errors.New("模板或推送目标过多")
	}
	for _, id := range c.ChatIDs {
		if !ValidChatID(id) {
			return errors.New("chat_id 必须是数字，可带负号")
		}
	}
	return nil
}

func ValidChatID(id string) bool {
	if len(id) == 0 || len(id) > 24 {
		return false
	}
	if id[0] == '-' {
		id = id[1:]
	}
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Store) SaveConfig(ctx context.Context, c Config, platform Platform) error {
	if err := ValidateConfig(c); err != nil {
		return err
	}
	// The row lock serializes simultaneous saves, including empty-token edits.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT config FROM bot_notify_config WHERE id=1 FOR UPDATE").Scan(&raw); err != nil {
		return err
	}
	var old Config
	if err = json.Unmarshal(raw, &old); err != nil {
		return err
	}
	newBot := false
	if c.BotToken == "" {
		c.BotToken, c.BotUsername, c.BotID = old.BotToken, old.BotUsername, old.BotID
	} else {
		if strings.HasPrefix(c.BotToken, "****") {
			return errors.New("请输入完整的新 token，或留空保留原 token")
		}
		bot, e := platform.ValidateToken(ctx, c.BotToken)
		if e != nil {
			return e
		}
		c.BotUsername, c.BotID = bot.Username, bot.ID
		newBot = old.BotID != bot.ID
		c.BotToken, err = s.Cipher.Encrypt(c.BotToken)
		if err != nil {
			return errors.New("bot token encryption failed")
		}
	}
	raw, err = json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE bot_notify_config SET config=$1, update_offset=CASE WHEN $2 THEN 0 ELSE update_offset END, updated_at=CURRENT_TIMESTAMP WHERE id=1", raw, newBot)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func ValidateRule(r Rule) error {
	if strings.TrimSpace(r.Keyword) == "" || len(r.Keyword) > 500 || len(r.ReplyTemplate) > 32000 {
		return errors.New("关键词不能为空且不超过500字节，模板不超过32000字节")
	}
	if r.MatchType != "contains" && r.MatchType != "exact" && r.MatchType != "command" {
		return errors.New("无效匹配方式")
	}
	if r.MatchType == "command" && (strings.ContainsAny(r.Keyword, " \t\r\n@") || strings.TrimPrefix(r.Keyword, "/") == "") {
		return errors.New("命令不能包含空白或 @")
	}
	return nil
}

func (s *Store) Rules(ctx context.Context) ([]Rule, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,keyword,match_type,reply_template,enabled,sort,created_at,updated_at FROM bot_reply_rules ORDER BY sort,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Rule{}
	for rows.Next() {
		var r Rule
		if err = rows.Scan(&r.ID, &r.Keyword, &r.MatchType, &r.ReplyTemplate, &r.Enabled, &r.Sort, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

func (s *Store) SaveRule(ctx context.Context, r Rule) (int64, error) {
	if err := ValidateRule(r); err != nil {
		return 0, err
	}
	if r.ID == 0 {
		err := s.DB.QueryRowContext(ctx, "INSERT INTO bot_reply_rules(keyword,match_type,reply_template,enabled,sort) VALUES($1,$2,$3,$4,$5) RETURNING id", r.Keyword, r.MatchType, r.ReplyTemplate, r.Enabled, r.Sort).Scan(&r.ID)
		return r.ID, err
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE bot_reply_rules SET keyword=$1,match_type=$2,reply_template=$3,enabled=$4,sort=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$6", r.Keyword, r.MatchType, r.ReplyTemplate, r.Enabled, r.Sort, r.ID)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		err = sql.ErrNoRows
	}
	return r.ID, err
}

type StoredMessage struct {
	Message
	ID           int64     `json:"id"`
	RuleID       *int64    `json:"rule_id"`
	Replied      bool      `json:"replied"`
	ReplyContent string    `json:"reply_content"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) Messages(ctx context.Context, chat, keyword string, page, size int) ([]StoredMessage, int64, error) {
	where := " FROM bot_messages WHERE ($1='' OR chat_id=$1) AND ($2='' OR strpos(text,$2)>0)"
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*)"+where, chat, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id,platform,chat_id,chat_type,sender_id,sender_name,text,message_id,update_id,rule_id,replied,reply_content,created_at"+where+" ORDER BY id DESC LIMIT $3 OFFSET $4", chat, keyword, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []StoredMessage{}
	for rows.Next() {
		var m StoredMessage
		if err = rows.Scan(&m.ID, &m.Platform, &m.ChatID, &m.ChatType, &m.SenderID, &m.SenderName, &m.Text, &m.MessageID, &m.UpdateID, &m.RuleID, &m.Replied, &m.ReplyContent, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

func (s *Store) Prune(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf(`DELETE FROM bot_messages WHERE created_at < $1 OR id IN (SELECT id FROM bot_messages ORDER BY id DESC OFFSET %d)`, RetentionCount), time.Now().AddDate(0, 0, -RetentionDays))
	return err
}
