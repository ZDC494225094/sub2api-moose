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

func normalizeConfig(c Config) Config {
	if c.IntervalMinutes == 0 {
		c.IntervalMinutes = 60
	}
	if c.PushTemplate == "" {
		c.PushTemplate = "{channel_status}"
	}
	if c.ChatIDs == nil {
		c.ChatIDs = []string{}
	}
	return c
}

// decodeConfig turns a stored config blob into a Config whose token is either
// decrypted (decrypt=true) or masked for display.
func (s *Store) decodeConfig(raw []byte, decrypt bool) (Config, error) {
	var c Config
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

func (s *Store) scanBot(row interface{ Scan(...any) error }, decrypt bool) (BotEntry, error) {
	var e BotEntry
	var raw []byte
	if err := row.Scan(&e.ID, &e.Name, &e.Platform, &raw); err != nil {
		return e, err
	}
	cfg, err := s.decodeConfig(raw, decrypt)
	if err != nil {
		return e, err
	}
	e.Config = cfg
	return e, nil
}

func (s *Store) Bots(ctx context.Context, decrypt bool) ([]BotEntry, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,name,platform,config FROM bot_notify_bots ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []BotEntry{}
	for rows.Next() {
		e, err := s.scanBot(rows, decrypt)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (s *Store) BotByID(ctx context.Context, id int64, decrypt bool) (BotEntry, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT id,name,platform,config FROM bot_notify_bots WHERE id=$1", id)
	return s.scanBot(row, decrypt)
}

// validateAndEncrypt resolves a plaintext token to (username, platform bot id,
// encrypted token) via the platform's identity endpoint.
func (s *Store) validateAndEncrypt(ctx context.Context, token string, platform Platform) (string, int64, string, error) {
	if strings.HasPrefix(token, "****") {
		return "", 0, "", errors.New("请输入完整的新 token，或留空保留原 token")
	}
	bot, err := platform.ValidateToken(ctx, token)
	if err != nil {
		return "", 0, "", err
	}
	encrypted, err := s.Cipher.Encrypt(token)
	if err != nil {
		return "", 0, "", errors.New("bot token encryption failed")
	}
	return bot.Username, bot.ID, encrypted, nil
}

func (s *Store) CreateBot(ctx context.Context, e BotEntry, platform Platform) (int64, error) {
	if e.Platform == "" {
		e.Platform = "telegram"
	}
	if e.Platform != "telegram" {
		return 0, errors.New("暂只支持 Telegram 机器人")
	}
	e.Config = normalizeConfig(e.Config)
	if err := ValidateConfig(e.Config); err != nil {
		return 0, err
	}
	if e.BotToken == "" {
		return 0, errors.New("请填写 bot token")
	}
	username, tgID, encrypted, err := s.validateAndEncrypt(ctx, e.BotToken, platform)
	if err != nil {
		return 0, err
	}
	e.BotToken, e.BotUsername, e.BotID = encrypted, username, tgID
	if strings.TrimSpace(e.Name) == "" {
		e.Name = username
	}
	raw, err := json.Marshal(e.Config)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.DB.QueryRowContext(ctx, "INSERT INTO bot_notify_bots(name,platform,config) VALUES($1,$2,$3) RETURNING id", e.Name, e.Platform, raw).Scan(&id)
	if err != nil {
		return 0, err
	}
	// Seed the default keyword rules every new bot starts with.
	for _, r := range []Rule{
		{BotID: id, Keyword: "渠道状态", MatchType: "contains", ReplyTemplate: "{channel_status}", Enabled: true, Sort: 0},
		{BotID: id, Keyword: "/status", MatchType: "command", ReplyTemplate: "{channel_status}", Enabled: true, Sort: 1},
	} {
		if _, err := s.SaveRule(ctx, r); err != nil {
			return id, err
		}
	}
	return id, nil
}

func (s *Store) SaveBot(ctx context.Context, e BotEntry, platform Platform) error {
	e.Config = normalizeConfig(e.Config)
	if err := ValidateConfig(e.Config); err != nil {
		return err
	}
	// The row lock serializes simultaneous saves, including empty-token edits.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT config FROM bot_notify_bots WHERE id=$1 FOR UPDATE", e.ID).Scan(&raw); err != nil {
		return err
	}
	var old Config
	if err = json.Unmarshal(raw, &old); err != nil {
		return err
	}
	newBot := false
	if e.BotToken == "" {
		e.BotToken, e.BotUsername, e.BotID = old.BotToken, old.BotUsername, old.BotID
	} else {
		username, tgID, encrypted, encErr := s.validateAndEncrypt(ctx, e.BotToken, platform)
		if encErr != nil {
			return encErr
		}
		e.BotToken, e.BotUsername, e.BotID = encrypted, username, tgID
		newBot = old.BotID != tgID
	}
	raw, err = json.Marshal(e.Config)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE bot_notify_bots SET name=$1, config=$2, update_offset=CASE WHEN $3 THEN 0 ELSE update_offset END, updated_at=CURRENT_TIMESTAMP WHERE id=$4", e.Name, raw, newBot, e.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteBot(ctx context.Context, id int64) (bool, error) {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM bot_notify_bots WHERE id=$1", id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
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

func (s *Store) Rules(ctx context.Context, botID int64) ([]Rule, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,bot_id,keyword,match_type,reply_template,enabled,sort,created_at,updated_at FROM bot_reply_rules WHERE bot_id=$1 ORDER BY sort,id", botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Rule{}
	for rows.Next() {
		var r Rule
		if err = rows.Scan(&r.ID, &r.BotID, &r.Keyword, &r.MatchType, &r.ReplyTemplate, &r.Enabled, &r.Sort, &r.CreatedAt, &r.UpdatedAt); err != nil {
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
		err := s.DB.QueryRowContext(ctx, "INSERT INTO bot_reply_rules(bot_id,keyword,match_type,reply_template,enabled,sort) VALUES($1,$2,$3,$4,$5,$6) RETURNING id", r.BotID, r.Keyword, r.MatchType, r.ReplyTemplate, r.Enabled, r.Sort).Scan(&r.ID)
		return r.ID, err
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE bot_reply_rules SET keyword=$1,match_type=$2,reply_template=$3,enabled=$4,sort=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$6 AND bot_id=$7", r.Keyword, r.MatchType, r.ReplyTemplate, r.Enabled, r.Sort, r.ID, r.BotID)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		err = sql.ErrNoRows
	}
	return r.ID, err
}

func (s *Store) DeleteRule(ctx context.Context, botID, ruleID int64) (bool, error) {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM bot_reply_rules WHERE id=$1 AND bot_id=$2", ruleID, botID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

type StoredMessage struct {
	Message
	ID           int64     `json:"id"`
	BotRowID     int64     `json:"bot_row_id"`
	RuleID       *int64    `json:"rule_id"`
	Replied      bool      `json:"replied"`
	ReplyContent string    `json:"reply_content"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) Messages(ctx context.Context, botRowID int64, chat, keyword string, page, size int) ([]StoredMessage, int64, error) {
	where := " FROM bot_messages WHERE bot_row_id=$1 AND ($2='' OR chat_id=$2) AND ($3='' OR strpos(text,$3)>0)"
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*)"+where, botRowID, chat, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id,bot_row_id,platform,chat_id,chat_type,sender_id,sender_name,text,message_id,update_id,rule_id,replied,reply_content,created_at"+where+" ORDER BY id DESC LIMIT $4 OFFSET $5", botRowID, chat, keyword, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []StoredMessage{}
	for rows.Next() {
		var m StoredMessage
		if err = rows.Scan(&m.ID, &m.BotRowID, &m.Platform, &m.ChatID, &m.ChatType, &m.SenderID, &m.SenderName, &m.Text, &m.MessageID, &m.UpdateID, &m.RuleID, &m.Replied, &m.ReplyContent, &m.CreatedAt); err != nil {
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
