package botnotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTelegramPollUTF16AndReply(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/botsecret/getUpdates", r.URL.Path)
		w.Write([]byte(`{"ok":true,"result":[{"update_id":42,"message":{"message_id":9,"chat":{"id":-123,"type":"group"},"from":{"id":7,"first_name":"张"},"text":"😀 @MyBot /status","entities":[{"type":"mention","offset":3,"length":6}],"reply_to_message":{"from":{"id":99}}}}]}`))
	}))
	defer s.Close()
	p := &Telegram{Client: s.Client(), BaseURL: s.URL}
	items, err := p.Poll(context.Background(), "secret", 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []string{"@MyBot"}, items[0].Mentions)
	require.Equal(t, "-123", items[0].ChatID)
	require.EqualValues(t, 99, items[0].ReplyToBotID)
	require.EqualValues(t, 42, items[0].UpdateID)
}

func TestTelegramErrorsNeverExposeToken(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"ok":false,"description":"secret-token"}`))
	}))
	defer s.Close()
	p := &Telegram{Client: s.Client(), BaseURL: s.URL}
	_, err := p.ValidateToken(context.Background(), "secret-token")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-token")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	p.BaseURL = "http://127.0.0.1:1"
	err = p.SendMessage(ctx, "secret-token", "123", strings.Repeat("a", 5000))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-token")
}
