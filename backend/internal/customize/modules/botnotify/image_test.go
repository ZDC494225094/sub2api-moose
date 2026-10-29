package botnotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// pngBytes is a 1x1 PNG.
var pngBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=")

var farFuture = time.Now().Add(24 * time.Hour)

func TestSniffImageTrustsMagicBytesOnly(t *testing.T) {
	mime, ok := SniffImage(pngBytes)
	require.True(t, ok)
	require.Equal(t, "image/png", mime)
	_, ok = SniffImage([]byte("<html><script>alert(1)</script></html>"))
	require.False(t, ok)
}

func TestParseImageDataURIChecksTypeAndSize(t *testing.T) {
	img, err := ParseImageDataURI("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(pngBytes))
	require.NoError(t, err)
	require.Equal(t, "image/png", img.MIME, "declared type is ignored")
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(pngBytes), img.DataURI())
	_, err = ParseImageDataURI("data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("hello")))
	require.ErrorIs(t, err, ErrImageType)
	_, err = ParseImageDataURI("data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, MaxImageBytes+10)))
	require.ErrorIs(t, err, ErrImageTooLarge)
}

func TestQQMediaURLAllowsOnlyQQHosts(t *testing.T) {
	for raw, want := range map[string]string{
		"https://multimedia.nt.qq.com.cn/download?appid=1407&fileid=x": "https://multimedia.nt.qq.com.cn/download?appid=1407&fileid=x",
		"gchat.qpic.cn/qmeetpic/0/0-0-ABC/0":                           "https://gchat.qpic.cn/qmeetpic/0/0-0-ABC/0",
	} {
		got, ok := qqMediaURL(raw)
		require.True(t, ok, raw)
		require.Equal(t, want, got)
	}
	for _, raw := range []string{"https://evil.com/x.png", "http://127.0.0.1:8080/x", "https://qq.com.evil.cn/x", "file:///etc/passwd", "https://user:pw@gchat.qpic.cn/x"} {
		_, ok := qqMediaURL(raw)
		require.False(t, ok, raw)
	}
	require.Error(t, refusePrivateAddress("tcp", "127.0.0.1:443", nil))
	require.Error(t, refusePrivateAddress("tcp", "10.1.2.3:443", nil))
	require.NoError(t, refusePrivateAddress("tcp", "203.0.113.7:443", nil))
}

func TestTelegramMediaFetchAndReply(t *testing.T) {
	var sent []map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/botsecret/getUpdates":
			w.Write([]byte(`{"ok":true,"result":[
				{"update_id":1,"message":{"message_id":5,"chat":{"id":-9,"type":"group"},"from":{"id":7},"caption":"@MyBot 这是什么报错","caption_entities":[{"type":"mention","offset":0,"length":6}],"photo":[{"file_id":"small","file_size":900},{"file_id":"large","file_size":90000}]}},
				{"update_id":2,"message":{"message_id":6,"chat":{"id":7,"type":"private"},"from":{"id":7},"document":{"file_id":"doc","mime_type":"image/png","file_size":2000}}},
				{"update_id":3,"message":{"message_id":7,"chat":{"id":7,"type":"private"},"from":{"id":7},"voice":{"file_id":"v"}}}]}`))
		case "/botsecret/getFile":
			w.Write([]byte(`{"ok":true,"result":{"file_path":"photos/file_1.png","file_size":68}}`))
		case "/file/botsecret/photos/file_1.png":
			w.Write(pngBytes)
		case "/botsecret/sendMessage":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			sent = append(sent, payload)
			w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()
	p := &Telegram{Client: s.Client(), BaseURL: s.URL}
	items, err := p.Poll(context.Background(), "secret", 0)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "@MyBot 这是什么报错", items[0].Text)
	require.Equal(t, []ImageRef{{ID: "large", MIME: "image/jpeg", Size: 90000}}, items[0].Images)
	require.Equal(t, "5", items[0].ReplyRef)
	require.Equal(t, []ImageRef{{ID: "doc", MIME: "image/png", Size: 2000}}, items[1].Images)
	require.Empty(t, items[2].Images)
	require.Equal(t, "voice", items[2].Attachment)

	img, err := p.FetchImage(context.Background(), "secret", items[0].Images[0])
	require.NoError(t, err)
	require.Equal(t, "image/png", img.MIME)
	_, err = p.FetchImage(context.Background(), "secret", ImageRef{ID: "huge", Size: MaxImageBytes + 1})
	require.ErrorIs(t, err, ErrImageTooLarge)

	require.NoError(t, p.Reply(context.Background(), "secret", items[0], "答复"))
	require.NoError(t, p.Reply(context.Background(), "secret", items[1], "私聊答复"))
	require.Len(t, sent, 2)
	require.Equal(t, map[string]any{"message_id": float64(5), "allow_sending_without_reply": true}, sent[0]["reply_parameters"])
	require.NotContains(t, sent[1], "reply_parameters", "private chats are not quoted")
}

func TestTelegramImageErrorsNeverExposeToken(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/botsecret-token/getFile" {
			w.Write([]byte(`{"ok":true,"result":{"file_path":"photos/x.jpg"}}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer s.Close()
	p := &Telegram{Client: s.Client(), BaseURL: s.URL}
	_, err := p.FetchImage(context.Background(), "secret-token", ImageRef{ID: "x"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-token")
}

func TestQQAttachmentsAndPassiveReplies(t *testing.T) {
	s := &qqSession{queue: make(chan Message, 4), dead: make(chan struct{})}
	raw, _ := json.Marshal(map[string]any{
		"id": "ev-9", "group_openid": "G1", "content": " ",
		"author":      map[string]any{"member_openid": "M1"},
		"attachments": []map[string]any{{"content_type": "image/png", "url": "https://multimedia.nt.qq.com.cn/download?fileid=1", "size": 1234}},
	})
	m, ok := convertQQEvent("GROUP_AT_MESSAGE_CREATE", raw, s)
	require.True(t, ok)
	require.Equal(t, "ev-9", m.ReplyRef)
	require.Equal(t, []ImageRef{{ID: "https://multimedia.nt.qq.com.cn/download?fileid=1", MIME: "image/png", Size: 1234}}, m.Images)

	raw, _ = json.Marshal(map[string]any{
		"id": "ev-10", "content": "", "author": map[string]any{"user_openid": "U1"},
		"attachments": []map[string]any{{"content_type": "voice", "url": "https://multimedia.nt.qq.com.cn/v"}},
	})
	m, ok = convertQQEvent("C2C_MESSAGE_CREATE", raw, s)
	require.True(t, ok)
	require.Equal(t, "voice", m.Attachment)

	var bodies []map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Write([]byte(`{"access_token":"t","expires_in":"7200"}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		require.True(t, strings.HasPrefix(r.URL.Path, "/v2/groups/G1/messages"))
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	q := NewQQ()
	q.Client, q.APIBase, q.TokenURL = api.Client(), api.URL, api.URL+"/token"
	q.tokens["1"] = qqCachedToken{value: "t", expiresAt: farFuture}
	creds := `{"app_id":"1","client_secret":"2"}`
	to := Message{ChatID: "group:G1", ReplyRef: "ev-9"}
	require.NoError(t, q.Reply(context.Background(), creds, to, "第一条"))
	require.NoError(t, q.Reply(context.Background(), creds, to, "第二条"))
	require.Len(t, bodies, 2)
	require.Equal(t, "ev-9", bodies[0]["msg_id"])
	require.Equal(t, float64(1), bodies[0]["msg_seq"])
	require.Equal(t, float64(2), bodies[1]["msg_seq"], "each passive reply needs a fresh msg_seq")
}
