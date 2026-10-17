package botnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

type Telegram struct {
	Client  *http.Client
	BaseURL string
}

func NewTelegram() *Telegram {
	return &Telegram{Client: &http.Client{Timeout: 40 * time.Second}, BaseURL: "https://api.telegram.org"}
}

func (t *Telegram) call(ctx context.Context, token, method string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, t.BaseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
		if err != nil {
			cancel()
			return fmt.Errorf("telegram %s: invalid request", method)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, callErr := t.Client.Do(req)
		code := 0
		if callErr == nil {
			code = resp.StatusCode
			var envelope struct {
				OK          bool            `json:"ok"`
				Result      json.RawMessage `json:"result"`
				Description string          `json:"description"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&envelope)
			resp.Body.Close()
			cancel()
			if code == 200 && decodeErr == nil && envelope.OK {
				return json.Unmarshal(envelope.Result, output)
			}
			if code >= 400 && code < 500 && code != 429 {
				if envelope.Description != "" {
					// Surface Telegram's reason (e.g. "chat not found"), but
					// never let the credential itself leak through an echoed
					// response body.
					safe := strings.ReplaceAll(envelope.Description, token, "***")
					return fmt.Errorf("telegram %s rejected (HTTP %d): %s", method, code, safe)
				}
				return fmt.Errorf("telegram %s rejected (HTTP %d)", method, code)
			}
		} else {
			cancel()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == 2 {
			return fmt.Errorf("telegram %s failed after 3 attempts (HTTP %d)", method, code)
		}
		timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (t *Telegram) ValidateToken(ctx context.Context, token string) (Bot, error) {
	var bot Bot
	err := t.call(ctx, token, "getMe", struct{}{}, &bot)
	return bot, err
}

func (t *Telegram) SendMessage(ctx context.Context, token, chat, text string) error {
	// Telegram limits messages to 4096 characters. Smaller rune chunks also
	// remain below the UTF-16 limit for emoji-heavy channel summaries.
	runes := []rune(text)
	for len(runes) > 0 {
		n := len(runes)
		if n > 2000 {
			n = 2000
		}
		var result json.RawMessage
		if err := t.call(ctx, token, "sendMessage", map[string]any{"chat_id": chat, "text": string(runes[:n])}, &result); err != nil {
			return err
		}
		runes = runes[n:]
	}
	return nil
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
type telegramMessage struct {
	ID   int64        `json:"message_id"`
	From telegramUser `json:"from"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
	Text            string           `json:"text"`
	Caption         string           `json:"caption"`
	Entities        []telegramEntity `json:"entities"`
	CaptionEntities []telegramEntity `json:"caption_entities"`
	Reply           *telegramMessage `json:"reply_to_message"`
}
type telegramEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

func (t *Telegram) Poll(ctx context.Context, token string, offset int64) ([]Message, error) {
	var updates []struct {
		ID      int64            `json:"update_id"`
		Message *telegramMessage `json:"message"`
		Edited  *telegramMessage `json:"edited_message"`
		Channel *telegramMessage `json:"channel_post"`
	}
	err := t.call(ctx, token, "getUpdates", map[string]any{"offset": offset, "timeout": 30, "allowed_updates": []string{"message", "edited_message", "channel_post"}}, &updates)
	if err != nil {
		return nil, err
	}
	result := make([]Message, 0, len(updates))
	for _, u := range updates {
		raw := u.Message
		if raw == nil {
			raw = u.Edited
		}
		if raw == nil {
			raw = u.Channel
		}
		m := Message{Platform: "telegram", UpdateID: u.ID}
		if raw != nil {
			m.MessageID, m.ChatID, m.ChatType = raw.ID, strconv.FormatInt(raw.Chat.ID, 10), raw.Chat.Type
			m.SenderID = strconv.FormatInt(raw.From.ID, 10)
			m.SenderName = strings.TrimSpace(raw.From.FirstName + " " + raw.From.LastName)
			if m.SenderName == "" {
				m.SenderName = raw.From.Username
			}
			m.Text = raw.Text
			entities := raw.Entities
			if m.Text == "" {
				m.Text, entities = raw.Caption, raw.CaptionEntities
			}
			units := utf16.Encode([]rune(m.Text))
			for _, e := range entities {
				if e.Offset < 0 || e.Length < 0 || e.Offset > len(units) || e.Length > len(units)-e.Offset {
					continue
				}
				value := string(utf16.Decode(units[e.Offset : e.Offset+e.Length]))
				if e.Type == "mention" {
					m.Mentions = append(m.Mentions, value)
				}
				if e.Type == "bot_command" {
					if parts := strings.SplitN(value, "@", 2); len(parts) == 2 {
						m.Mentions = append(m.Mentions, "@"+parts[1])
					}
				}
			}
			if raw.Reply != nil {
				m.ReplyToBotID = raw.Reply.From.ID
			}
		}
		result = append(result, m)
	}
	return result, nil
}
