package botnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	return t.send(ctx, token, chat, text, 0)
}

// Reply quotes the inbound message in group chats so busy groups can see
// whom the answer is for; private chats get a plain message.
func (t *Telegram) Reply(ctx context.Context, token string, to Message, text string) error {
	if to.ChatType == "private" {
		return t.send(ctx, token, to.ChatID, text, 0)
	}
	return t.send(ctx, token, to.ChatID, text, to.MessageID)
}

func (t *Telegram) send(ctx context.Context, token, chat, text string, replyTo int64) error {
	// Telegram limits messages to 4096 characters. Smaller rune chunks also
	// remain below the UTF-16 limit for emoji-heavy channel summaries.
	runes := []rune(text)
	for len(runes) > 0 {
		n := len(runes)
		if n > 2000 {
			n = 2000
		}
		payload := map[string]any{"chat_id": chat, "text": string(runes[:n])}
		if replyTo > 0 {
			// Only the first chunk quotes; a deleted original never blocks it.
			payload["reply_parameters"] = map[string]any{"message_id": replyTo, "allow_sending_without_reply": true}
			replyTo = 0
		}
		var result json.RawMessage
		if err := t.call(ctx, token, "sendMessage", payload, &result); err != nil {
			return err
		}
		runes = runes[n:]
	}
	return nil
}

// FetchImage resolves a file_id with getFile and downloads it. The download
// URL embeds the bot token, so its errors are reduced to fixed strings.
func (t *Telegram) FetchImage(ctx context.Context, token string, ref ImageRef) (Image, error) {
	if ref.Size > MaxImageBytes {
		return Image{}, ErrImageTooLarge
	}
	var file struct {
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
	}
	if err := t.call(ctx, token, "getFile", map[string]any{"file_id": ref.ID}, &file); err != nil {
		return Image{}, err
	}
	if file.FileSize > MaxImageBytes {
		return Image{}, ErrImageTooLarge
	}
	if file.FilePath == "" || strings.Contains(file.FilePath, "..") {
		return Image{}, errors.New("telegram getFile returned no usable path")
	}
	segments := strings.Split(file.FilePath, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return downloadImage(ctx, t.Client, t.BaseURL+"/file/bot"+token+"/"+strings.Join(segments, "/"))
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
	Photo           []struct {
		FileID   string `json:"file_id"`
		FileSize int64  `json:"file_size"`
	} `json:"photo"`
	Document *struct {
		FileID   string `json:"file_id"`
		MimeType string `json:"mime_type"`
		FileSize int64  `json:"file_size"`
	} `json:"document"`
	Voice     json.RawMessage `json:"voice"`
	Audio     json.RawMessage `json:"audio"`
	Video     json.RawMessage `json:"video"`
	VideoNote json.RawMessage `json:"video_note"`
	Animation json.RawMessage `json:"animation"`
	Sticker   json.RawMessage `json:"sticker"`
}

// media maps a message's attachment to an image reference (the largest
// photo size within the download limit, or an image document) or names the
// unreadable attachment kind.
func (raw *telegramMessage) media() ([]ImageRef, string) {
	if len(raw.Photo) > 0 {
		best := raw.Photo[0]
		for _, p := range raw.Photo[1:] {
			if p.FileSize <= MaxImageBytes && p.FileSize >= best.FileSize {
				best = p
			}
		}
		return []ImageRef{{ID: best.FileID, MIME: "image/jpeg", Size: best.FileSize}}, ""
	}
	switch {
	case len(raw.Animation) > 0:
		return nil, "video"
	case raw.Document != nil && strings.HasPrefix(raw.Document.MimeType, "image/"):
		return []ImageRef{{ID: raw.Document.FileID, MIME: raw.Document.MimeType, Size: raw.Document.FileSize}}, ""
	case len(raw.Voice) > 0 || len(raw.Audio) > 0:
		return nil, "voice"
	case len(raw.Video) > 0 || len(raw.VideoNote) > 0:
		return nil, "video"
	case len(raw.Sticker) > 0:
		return nil, "sticker"
	case raw.Document != nil:
		return nil, "file"
	}
	return nil, ""
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
			m.Images, m.Attachment = raw.media()
			m.ReplyRef = strconv.FormatInt(raw.ID, 10)
		}
		result = append(result, m)
	}
	return result, nil
}
