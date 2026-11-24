package wiring

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/iqmonitor"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ProvideIQMonitor wires the 降智监测 runtime. Probes go through this
// server's own /v1 relay on loopback with the probe account's keys, so
// routing, reasoning-effort policy, billing and usage logs are exactly
// those of a normal user request. Wire's cleanup stops the runtime.
func ProvideIQMonitor(db *sql.DB, settingService *service.SettingService, bot *botnotify.Handler) *iqmonitor.Handler {
	extensions := settingService.CustomExtensions()
	base := func() string { return "http://" + gatewayLoopbackAddr() }
	host := iqMonitorHost(db, base)
	if bot != nil && bot.Runtime != nil {
		host.Bots = iqMonitorBots(bot.Runtime)
		host.Notify = func(ctx context.Context, botIDs []int64, text string) error {
			// Bot output follows the bot-notify switch like every other push.
			if on, err := extensions.Enabled(ctx, botnotify.ID); err != nil || !on {
				return errors.New("消息机器人扩展未启用")
			}
			return iqMonitorNotify(ctx, bot.Runtime, botIDs, text)
		}
	}
	caller := &iqmonitor.GatewayCaller{BaseURL: base, Client: &http.Client{}, Headers: setLoopbackHeaders}
	r := iqmonitor.NewRuntime(&iqmonitor.Store{DB: db}, extensions, host, caller)
	r.Start()
	return &iqmonitor.Handler{Runtime: r}
}

func iqMonitorHost(db *sql.DB, base func() string) iqmonitor.Host {
	return iqmonitor.Host{
		APIKey: func(ctx context.Context, keyID, userID int64) (string, error) {
			var key string
			err := db.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id = $1 AND user_id = $2 AND status = 'active' AND deleted_at IS NULL`, keyID, userID).Scan(&key)
			if errors.Is(err, sql.ErrNoRows) {
				return "", errors.New("令牌不存在、未启用或不属于探测账号")
			}
			return key, err
		},
		Groups: func(ctx context.Context) ([]iqmonitor.GroupOption, error) {
			rows, err := db.QueryContext(ctx, `SELECT id, name, platform, rate_multiplier, status, (claude_code_only AND fallback_group_id IS NULL) FROM groups WHERE deleted_at IS NULL ORDER BY sort_order, id`)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []iqmonitor.GroupOption{}
			for rows.Next() {
				var g iqmonitor.GroupOption
				if err := rows.Scan(&g.ID, &g.Name, &g.Platform, &g.RateMultiplier, &g.Status, &g.ClaudeCodeOnly); err != nil {
					return nil, err
				}
				out = append(out, g)
			}
			return out, rows.Err()
		},
		Keys: func(ctx context.Context, userID int64) ([]iqmonitor.KeyOption, error) {
			rows, err := db.QueryContext(ctx, `SELECT id, name, group_id, COALESCE(group_ids::text, '[]'), status FROM api_keys
WHERE user_id = $1 AND deleted_at IS NULL AND name NOT LIKE '__sub2api_canvas_group__:%' ORDER BY (status = 'active') DESC, id DESC LIMIT 200`, userID)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []iqmonitor.KeyOption{}
			for rows.Next() {
				var k iqmonitor.KeyOption
				var primary sql.NullInt64
				var raw string
				if err := rows.Scan(&k.ID, &k.Name, &primary, &raw, &k.Status); err != nil {
					return nil, err
				}
				k.GroupIDs = keyGroups(primary, raw)
				out = append(out, k)
			}
			return out, rows.Err()
		},
		Users: func(ctx context.Context, q string, ids []int64) ([]iqmonitor.UserOption, error) {
			var rows *sql.Rows
			var err error
			if len(ids) > 0 {
				rows, err = db.QueryContext(ctx, `SELECT id, email, COALESCE(username, '') FROM users WHERE id = $1 AND deleted_at IS NULL`, ids[0])
			} else {
				like := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q) + "%"
				rows, err = db.QueryContext(ctx, `SELECT id, email, COALESCE(username, '') FROM users WHERE deleted_at IS NULL AND (email ILIKE $1 OR username ILIKE $1 OR CAST(id AS TEXT) = $2) ORDER BY id LIMIT 20`, like, q)
			}
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []iqmonitor.UserOption{}
			for rows.Next() {
				var u iqmonitor.UserOption
				if err := rows.Scan(&u.ID, &u.Email, &u.Username); err != nil {
					return nil, err
				}
				out = append(out, u)
			}
			return out, rows.Err()
		},
		Effort: func(ctx context.Context, groupID int64, model, effort string) iqmonitor.EffortPreview {
			g, err := loadEffortGroup(ctx, db, groupID)
			if err != nil {
				return iqmonitor.EffortPreview{Sent: effort}
			}
			p := service.PreviewChatReasoningEffort(g, model, effort)
			return iqmonitor.EffortPreview{Sent: p.Sent, Denied: p.Denied, Note: p.Note}
		},
		Models: func(ctx context.Context, keyID, userID int64) ([]string, error) {
			var key string
			if err := db.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id = $1 AND user_id = $2 AND status = 'active' AND deleted_at IS NULL`, keyID, userID).Scan(&key); err != nil {
				return nil, errors.New("令牌不可用")
			}
			return gatewayModels(ctx, base(), key)
		},
		Cost: func(ctx context.Context, keyID int64, model string, since time.Time) (iqmonitor.UnitCost, error) {
			var in, out, n int64
			var outCost, total, actual float64
			err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(input_tokens + cache_creation_tokens + cache_read_tokens), 0), COALESCE(SUM(output_tokens), 0),
COALESCE(SUM(output_cost), 0), COALESCE(SUM(total_cost), 0), COALESCE(SUM(actual_cost), 0), COUNT(*)
FROM usage_logs WHERE api_key_id = $1 AND model = $2 AND created_at >= $3`, keyID, model, since).Scan(&in, &out, &outCost, &total, &actual, &n)
			if err != nil || n == 0 {
				return iqmonitor.UnitCost{}, err
			}
			// Split the charged cost by the list-price output share; cache
			// costs are input-side and land on input tokens.
			share := 0.0
			if total > 0 {
				share = outCost / total
			}
			u := iqmonitor.UnitCost{Samples: n}
			if in > 0 {
				u.InputPerToken = actual * (1 - share) / float64(in)
			}
			if out > 0 {
				u.OutputPerToken = actual * share / float64(out)
			}
			return u, nil
		},
	}
}

func keyGroups(primary sql.NullInt64, raw string) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if primary.Valid {
		add(primary.Int64)
	}
	var ids []int64
	if json.Unmarshal([]byte(raw), &ids) == nil {
		for _, id := range ids {
			add(id)
		}
	}
	return out
}

func loadEffortGroup(ctx context.Context, db *sql.DB, groupID int64) (*service.Group, error) {
	g := &service.Group{ID: groupID}
	var mappings string
	err := db.QueryRowContext(ctx, `SELECT platform, COALESCE(max_reasoning_effort, ''), COALESCE(max_reasoning_effort_over_limit, ''), COALESCE(reasoning_effort_mappings::text, '[]')
FROM groups WHERE id = $1 AND deleted_at IS NULL`, groupID).Scan(&g.Platform, &g.MaxReasoningEffort, &g.MaxReasoningEffortOverLimit, &mappings)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(mappings), &g.ReasoningEffortMappings)
	return g, nil
}

func gatewayModels(ctx context.Context, base, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "sub2api-iq-monitor")
	setLoopbackHeaders(req.Header)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("读取模型列表失败")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取模型列表失败（HTTP %d）", resp.StatusCode)
	}
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, errors.New("模型列表格式无法解析")
	}
	out := make([]string, 0, len(decoded.Data))
	for _, m := range decoded.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func iqMonitorBots(rt *botnotify.Runtime) func(context.Context) ([]iqmonitor.BotOption, error) {
	return func(ctx context.Context) ([]iqmonitor.BotOption, error) {
		bots, err := rt.Store.Bots(ctx, false)
		if err != nil {
			return nil, err
		}
		out := make([]iqmonitor.BotOption, 0, len(bots))
		for _, b := range bots {
			platform := b.Platform
			if platform == "" {
				platform = "telegram"
			}
			out = append(out, iqmonitor.BotOption{ID: b.ID, Name: b.Name, Platform: platform, Targets: len(b.ChatIDs)})
		}
		return out, nil
	}
}

// iqMonitorNotify sends text to every push target of the selected bots and
// reports all failures together.
func iqMonitorNotify(ctx context.Context, rt *botnotify.Runtime, botIDs []int64, text string) error {
	var failures []string
	for _, id := range botIDs {
		bot, err := rt.Store.BotByID(ctx, id, true)
		if err != nil {
			failures = append(failures, fmt.Sprintf("机器人 %d 不存在", id))
			continue
		}
		platform, ok := rt.PlatformFor(bot.Platform)
		if !ok || bot.BotToken == "" || len(bot.ChatIDs) == 0 {
			failures = append(failures, fmt.Sprintf("%s 未配置凭证或推送目标", bot.Name))
			continue
		}
		for _, chat := range bot.ChatIDs {
			if err := platform.SendMessage(ctx, bot.BotToken, chat, text); err != nil {
				failures = append(failures, fmt.Sprintf("%s → %s：%v", bot.Name, chat, err))
			}
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "；"))
	}
	return nil
}
