package botnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// QQ platform adapter for the official QQ bot API (bot.q.qq.com):
// REST v2 for sending, WebSocket gateway for receiving group @messages and
// C2C messages. Credentials ride in the shared token field as a JSON blob
// {"app_id":"...","client_secret":"..."}; the whole blob is encrypted at rest.
//
// Push/reply targets use prefixed openids: "group:<group_openid>" for group
// chats and "user:<user_openid>" for C2C. Note QQ restricts proactive
// messages (monthly quotas, group pushes may be rejected); API errors are
// surfaced to the bot status instead of being retried blindly.
type QQ struct {
	Client   *http.Client
	APIBase  string
	TokenURL string

	mu       sync.Mutex
	tokens   map[string]qqCachedToken
	sessions map[string]*qqSession
}

type qqCreds struct {
	AppID        string `json:"app_id"`
	ClientSecret string `json:"client_secret"`
}

func ParseQQCredentials(token string) (string, error) {
	creds, err := parseQQCreds(token)
	if err != nil {
		return "", err
	}
	return creds.AppID, nil
}

func parseQQCreds(token string) (qqCreds, error) {
	var creds qqCreds
	if err := json.Unmarshal([]byte(token), &creds); err != nil {
		return creds, errors.New("QQ 凭证格式错误，应为 AppID + AppSecret")
	}
	if creds.AppID == "" || creds.ClientSecret == "" {
		return creds, errors.New("QQ 凭证缺少 AppID 或 AppSecret")
	}
	return creds, nil
}

type qqCachedToken struct {
	value     string
	expiresAt time.Time
}

func NewQQ() *QQ {
	return &QQ{
		Client:   &http.Client{Timeout: 40 * time.Second},
		APIBase:  "https://api.sgroup.qq.com",
		TokenURL: "https://bots.qq.com/app/getAppAccessToken",
		tokens:   map[string]qqCachedToken{},
		sessions: map[string]*qqSession{},
	}
}

func hashStringID(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64() & math.MaxInt64)
}

func (q *QQ) accessToken(ctx context.Context, creds qqCreds, force bool) (string, error) {
	q.mu.Lock()
	if cached, ok := q.tokens[creds.AppID]; ok && !force && time.Until(cached.expiresAt) > time.Minute {
		q.mu.Unlock()
		return cached.value, nil
	}
	q.mu.Unlock()

	body, err := json.Marshal(map[string]string{"appId": creds.AppID, "clientSecret": creds.ClientSecret})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.TokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return "", errors.New("qq token request failed")
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Code        int    `json:"code"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("qq token rejected (HTTP %d, code %d)", resp.StatusCode, out.Code)
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	q.mu.Lock()
	q.tokens[creds.AppID] = qqCachedToken{value: out.AccessToken, expiresAt: time.Now().Add(ttl)}
	q.mu.Unlock()
	return out.AccessToken, nil
}

// apiCall performs one authenticated REST call; QQ errors (non-2xx or a
// nonzero business code) become sanitized errors without credential material.
func (q *QQ) apiCall(ctx context.Context, creds qqCreds, method, path string, input, output any) error {
	token, err := q.accessToken(ctx, creds, false)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var reader io.Reader
		if input != nil {
			raw, err := json.Marshal(input)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(ctx, method, q.APIBase+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "QQBot "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := q.Client.Do(req)
		if err != nil {
			return errors.New("qq api request failed")
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		var envelope struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &envelope)
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			token, err = q.accessToken(ctx, creds, true)
			if err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
			message := envelope.Message
			if len(message) > 200 {
				message = message[:200]
			}
			return fmt.Errorf("qq %s rejected (HTTP %d, code %d): %s", path, resp.StatusCode, envelope.Code, message)
		}
		if output != nil {
			return json.Unmarshal(raw, output)
		}
		return nil
	}
	return errors.New("qq api request failed")
}

func (q *QQ) ValidateToken(ctx context.Context, token string) (Bot, error) {
	var bot Bot
	creds, err := parseQQCreds(token)
	if err != nil {
		return bot, err
	}
	if _, err := q.accessToken(ctx, creds, true); err != nil {
		return bot, err
	}
	var me struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := q.apiCall(ctx, creds, http.MethodGet, "/users/@me", nil, &me); err != nil {
		return bot, err
	}
	return Bot{ID: hashStringID("qq-bot:" + me.ID), Username: me.Username}, nil
}

func (q *QQ) SendMessage(ctx context.Context, token, target, text string) error {
	creds, err := parseQQCreds(token)
	if err != nil {
		return err
	}
	var path string
	switch {
	case strings.HasPrefix(target, "group:"):
		path = "/v2/groups/" + strings.TrimPrefix(target, "group:") + "/messages"
	case strings.HasPrefix(target, "user:"):
		path = "/v2/users/" + strings.TrimPrefix(target, "user:") + "/messages"
	default:
		return errors.New("QQ 目标格式错误，应为 group:群openid 或 user:用户openid")
	}
	runes := []rune(text)
	for len(runes) > 0 {
		n := len(runes)
		if n > 2000 {
			n = 2000
		}
		if err := q.apiCall(ctx, creds, http.MethodPost, path, map[string]any{"content": string(runes[:n]), "msg_type": 0}, nil); err != nil {
			return err
		}
		runes = runes[n:]
	}
	return nil
}

// --- Receiving: one WebSocket gateway session per bot credential set. ---

type qqSession struct {
	queue    chan Message
	dead     chan struct{}
	lastPoll atomic.Int64
	username atomic.Value // string
}

func (q *QQ) sessionFor(creds qqCreds) *qqSession {
	key := creds.AppID
	q.mu.Lock()
	defer q.mu.Unlock()
	if s, ok := q.sessions[key]; ok {
		select {
		case <-s.dead:
			delete(q.sessions, key)
		default:
			return s
		}
	}
	s := &qqSession{queue: make(chan Message, 128), dead: make(chan struct{})}
	s.lastPoll.Store(time.Now().UnixNano())
	q.sessions[key] = s
	go q.runSession(creds, s)
	return s
}

func (q *QQ) Poll(ctx context.Context, token string, _ int64) ([]Message, error) {
	creds, err := parseQQCreds(token)
	if err != nil {
		return nil, err
	}
	s := q.sessionFor(creds)
	s.lastPoll.Store(time.Now().UnixNano())
	timer := time.NewTimer(25 * time.Second)
	defer timer.Stop()
	var first Message
	select {
	case first = <-s.queue:
	case <-s.dead:
		return nil, errors.New("qq gateway disconnected; reconnecting")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return []Message{}, nil
	}
	out := []Message{first}
	for {
		select {
		case m := <-s.queue:
			out = append(out, m)
		default:
			return out, nil
		}
	}
}

type qqGatewayFrame struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	S  int64           `json:"s"`
	D  json.RawMessage `json:"d"`
}

func (q *QQ) runSession(creds qqCreds, s *qqSession) {
	defer close(s.dead)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	access, err := q.accessToken(ctx, creds, false)
	if err != nil {
		return
	}
	var gateway struct {
		URL string `json:"url"`
	}
	if err := q.apiCall(ctx, creds, http.MethodGet, "/gateway", nil, &gateway); err != nil || gateway.URL == "" {
		return
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gateway.URL, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}
	var seq atomic.Int64
	identified := false
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var frame qqGatewayFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		switch frame.Op {
		case 10: // Hello: identify, then heartbeat at the server interval.
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			_ = json.Unmarshal(frame.D, &hello)
			identify := map[string]any{"op": 2, "d": map[string]any{
				"token":   "QQBot " + access,
				"intents": 1 << 25, // GROUP_AND_C2C_EVENT
				"shard":   []int{0, 1},
				"properties": map[string]any{
					"$os": "linux", "$browser": "sub2api-bot-notify", "$device": "sub2api-bot-notify",
				},
			}}
			if err := writeJSON(identify); err != nil {
				return
			}
			identified = true
			interval := time.Duration(hello.HeartbeatInterval) * time.Millisecond
			if interval <= 0 {
				interval = 40 * time.Second
			}
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for range ticker.C {
					if time.Since(time.Unix(0, s.lastPoll.Load())) > 3*time.Minute {
						conn.Close() // idle: nobody is polling this bot anymore
						return
					}
					if err := writeJSON(map[string]any{"op": 1, "d": seq.Load()}); err != nil {
						return
					}
				}
			}()
		case 0: // Dispatch.
			seq.Store(frame.S)
			if !identified {
				continue
			}
			if m, ok := convertQQEvent(frame.T, frame.D, s); ok {
				select {
				case s.queue <- m:
				default: // queue full: drop rather than block the gateway loop
				}
			}
		case 7, 9: // Reconnect / invalid session: drop and let Poll rebuild.
			return
		}
	}
}

func convertQQEvent(t string, raw json.RawMessage, s *qqSession) (Message, bool) {
	switch t {
	case "READY":
		var ready struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		}
		if err := json.Unmarshal(raw, &ready); err == nil && ready.User.Username != "" {
			s.username.Store(ready.User.Username)
		}
	case "GROUP_AT_MESSAGE_CREATE":
		var ev struct {
			ID          string `json:"id"`
			GroupOpenID string `json:"group_openid"`
			Content     string `json:"content"`
			Author      struct {
				MemberOpenID string `json:"member_openid"`
				Username     string `json:"username"`
			} `json:"author"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil || ev.GroupOpenID == "" {
			return Message{}, false
		}
		m := Message{
			Platform:  "qq",
			UpdateID:  hashStringID("qq-ev:" + ev.ID),
			ChatID:    "group:" + ev.GroupOpenID,
			ChatType:  "group",
			SenderID:  ev.Author.MemberOpenID,
			Text:      strings.TrimSpace(ev.Content),
		}
		m.MessageID = m.UpdateID
		m.SenderName = ev.Author.Username
		if m.SenderName == "" {
			m.SenderName = ev.Author.MemberOpenID
		}
		// Group messages only arrive when the bot was @-mentioned, so the
		// addressing check passes via the session's own username.
		if name, ok := s.username.Load().(string); ok && name != "" {
			m.Mentions = []string{"@" + name}
		}
		return m, true
	case "C2C_MESSAGE_CREATE":
		var ev struct {
			ID      string `json:"id"`
			Content string `json:"content"`
			Author  struct {
				UserOpenID string `json:"user_openid"`
				Username   string `json:"username"`
			} `json:"author"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil || ev.Author.UserOpenID == "" {
			return Message{}, false
		}
		m := Message{
			Platform:  "qq",
			UpdateID:  hashStringID("qq-ev:" + ev.ID),
			ChatID:    "user:" + ev.Author.UserOpenID,
			ChatType:  "private",
			SenderID:  ev.Author.UserOpenID,
			Text:      strings.TrimSpace(ev.Content),
		}
		m.MessageID = m.UpdateID
		m.SenderName = ev.Author.Username
		if m.SenderName == "" {
			m.SenderName = ev.Author.UserOpenID
		}
		return m, true
	}
	return Message{}, false
}
