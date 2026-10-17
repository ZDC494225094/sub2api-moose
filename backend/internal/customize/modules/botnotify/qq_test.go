package botnotify

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQQCredentialsParsing(t *testing.T) {
	appID, err := ParseQQCredentials(`{"app_id":"102000001","client_secret":"s3cret"}`)
	require.NoError(t, err)
	require.Equal(t, "102000001", appID)
	_, err = ParseQQCredentials(`{"app_id":"102000001"}`)
	require.Error(t, err)
	_, err = ParseQQCredentials("plain-token")
	require.Error(t, err)
}

func TestQQEventConversion(t *testing.T) {
	s := &qqSession{queue: make(chan Message, 4), dead: make(chan struct{})}
	s.username.Store("MooseBot")

	raw, _ := json.Marshal(map[string]any{
		"id": "ev-1", "group_openid": "G123", "content": " 渠道状态 ",
		"author": map[string]any{"member_openid": "M9", "username": "阿鹿"},
	})
	m, ok := convertQQEvent("GROUP_AT_MESSAGE_CREATE", raw, s)
	require.True(t, ok)
	require.Equal(t, "group:G123", m.ChatID)
	require.Equal(t, "group", m.ChatType)
	require.Equal(t, "渠道状态", m.Text)
	require.Equal(t, "阿鹿", m.SenderName)
	require.Equal(t, []string{"@MooseBot"}, m.Mentions)
	// The bot only receives group events when mentioned, so this must pass
	// the shared addressing gate with the QQ bot's identity.
	require.True(t, Addressed(m, Bot{ID: 1, Username: "MooseBot"}))

	raw, _ = json.Marshal(map[string]any{
		"id": "ev-2", "content": "公告",
		"author": map[string]any{"user_openid": "U7"},
	})
	m, ok = convertQQEvent("C2C_MESSAGE_CREATE", raw, s)
	require.True(t, ok)
	require.Equal(t, "user:U7", m.ChatID)
	require.Equal(t, "private", m.ChatType)
	require.True(t, Addressed(m, Bot{ID: 1, Username: "MooseBot"}))

	_, ok = convertQQEvent("GUILD_MESSAGE_CREATE", raw, s)
	require.False(t, ok)
}

func TestQQSendTargetValidation(t *testing.T) {
	q := NewQQ()
	err := q.SendMessage(t.Context(), `{"app_id":"1","client_secret":"2"}`, "12345", "hi")
	require.ErrorContains(t, err, "group:")
}
