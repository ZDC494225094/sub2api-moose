package wiring

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
)

// botAIKeyLister lists the API keys an admin may attach to a bot's AI
// customer-service configuration: only the requesting admin's own keys —
// the same set the API keys page shows that admin (canvas-managed keys
// excluded). It returns redacted metadata only (id, display name, group
// label, status); key material never leaves the server through this port.
func botAIKeyLister(db *sql.DB) func(context.Context, int64) ([]botnotify.AIKeyOption, error) {
	return func(ctx context.Context, userID int64) ([]botnotify.AIKeyOption, error) {
		rows, err := db.QueryContext(ctx, `
SELECT k.id, k.name, COALESCE(g.name, ''), k.status
FROM api_keys k
LEFT JOIN groups g ON g.id = k.group_id
WHERE k.deleted_at IS NULL AND k.user_id = $1
AND k.name NOT LIKE '__sub2api_canvas_group__:%'
ORDER BY (k.status = 'active') DESC, k.id DESC
LIMIT 200`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []botnotify.AIKeyOption{}
		for rows.Next() {
			var o botnotify.AIKeyOption
			if err := rows.Scan(&o.ID, &o.Name, &o.Platform, &o.Status); err != nil {
				return nil, err
			}
			out = append(out, o)
		}
		return out, rows.Err()
	}
}

// botAIKeyValidator re-checks a selected key at AI-config save time: the
// key must exist, not be deleted, and be active, and a model must be set.
func botAIKeyValidator(db *sql.DB) func(context.Context, int64, string) error {
	return func(ctx context.Context, keyID int64, model string) error {
		if strings.TrimSpace(model) == "" {
			return errors.New("请选择或填写模型")
		}
		var status string
		err := db.QueryRowContext(ctx, `SELECT status FROM api_keys WHERE id=$1 AND deleted_at IS NULL`, keyID).Scan(&status)
		if err == sql.ErrNoRows {
			return errors.New("所选 API Key 不存在或已删除")
		}
		if err != nil {
			return err
		}
		if status != "active" {
			return errors.New("所选 API Key 未启用")
		}
		return nil
	}
}

// gatewayLoopbackAddr resolves this process's own HTTP listen port so the
// AI provider can call the gateway on loopback regardless of the public
// host name the server is reached through.
func gatewayLoopbackAddr() string {
	_, port, err := net.SplitHostPort(config.GetServerAddress())
	if err != nil || port == "" {
		port = "8080"
	}
	return "127.0.0.1:" + port
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
