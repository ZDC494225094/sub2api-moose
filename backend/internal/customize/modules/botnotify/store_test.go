package botnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type testCipher struct{}

func (testCipher) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (testCipher) Decrypt(s string) (string, error) {
	if !strings.HasPrefix(s, "encrypted:") {
		return "", errors.New("invalid cipher")
	}
	return strings.TrimPrefix(s, "encrypted:"), nil
}

type testPlatform struct {
	sent      int
	validates int
	err       error
}

func (p *testPlatform) ValidateToken(context.Context, string) (Bot, error) {
	p.validates++
	return Bot{ID: 12, Username: "TestBot"}, p.err
}
func (p *testPlatform) SendMessage(context.Context, string, string, string) error {
	p.sent++
	return p.err
}
func (p *testPlatform) Poll(context.Context, string, int64) ([]Message, error) { return nil, p.err }

type enabledState struct{ enabled bool }

func (s enabledState) Enabled(context.Context, string) (bool, error) { return s.enabled, nil }

func TestConfigEncryptionAndEmptyTokenPreservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	s := &Store{DB: db, Cipher: testCipher{}}
	p := &testPlatform{}
	old := Config{BotToken: "encrypted:secret1234", BotID: 12, BotUsername: "TestBot", IntervalMinutes: 60}
	raw, _ := json.Marshal(old)
	mock.ExpectQuery("SELECT config").WillReturnRows(sqlmock.NewRows([]string{"config"}).AddRow(raw))
	masked, err := s.Config(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, "****1234", masked.BotToken)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT config.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"config"}).AddRow(raw))
	old.ReceiveEnabled = true
	expected, _ := json.Marshal(old)
	mock.ExpectExec("UPDATE bot_notify_config").WithArgs(expected, false).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	input := old
	input.BotToken = ""
	require.NoError(t, s.SaveConfig(context.Background(), input, p))
	require.Zero(t, p.validates)
	// A new bot is validated, encrypted, and starts its own offset history.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT config.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"config"}).AddRow([]byte(`{"bot_id":99}`)))
	input.BotToken = "new-secret1234"
	input.BotID = 12
	input.BotUsername = "TestBot"
	expectedConfig := input
	expectedConfig.BotToken = "encrypted:new-secret1234"
	expected, _ = json.Marshal(expectedConfig)
	mock.ExpectExec("UPDATE bot_notify_config").WithArgs(expected, true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, s.SaveConfig(context.Background(), input, p))
	require.Equal(t, 1, p.validates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessPersistsUnaddressedGroupAndOffset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := &testPlatform{}
	r := NewRuntime(&Store{DB: db}, p, &testData{}, enabledState{true})
	m := Message{Platform: "telegram", UpdateID: 42, MessageID: 4, ChatID: "-123", ChatType: "group", Text: "渠道状态"}
	mock.ExpectQuery("INSERT INTO bot_messages").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec("UPDATE bot_notify_config SET update_offset").WithArgs(int64(43), int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.process(context.Background(), Config{BotID: 12}, m))
	require.Zero(t, p.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessDuplicateDoesNotReplyAgain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := &testPlatform{}
	r := NewRuntime(&Store{DB: db}, p, &testData{}, enabledState{true})
	mock.ExpectQuery("INSERT INTO bot_messages").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("UPDATE bot_notify_config SET update_offset").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.process(context.Background(), Config{BotID: 12}, Message{ChatType: "private", Text: "/status"}))
	require.Zero(t, p.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessRepliesOnlyFirstMatchingRule(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := &testPlatform{}
	r := NewRuntime(&Store{DB: db}, p, &testData{}, enabledState{true})
	mock.ExpectQuery("INSERT INTO bot_messages").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	now := time.Now()
	mock.ExpectQuery("SELECT id,keyword").WillReturnRows(sqlmock.NewRows([]string{"id", "keyword", "match_type", "reply_template", "enabled", "sort", "created_at", "updated_at"}).
		AddRow(1, "/status", "command", "first", true, 0, now, now).
		AddRow(2, "/status", "contains", "second", true, 1, now, now))
	mock.ExpectExec("UPDATE bot_messages SET rule_id").WithArgs(int64(1), "first", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE bot_messages SET replied=true").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE bot_notify_config SET update_offset").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.process(context.Background(), Config{BotID: 12}, Message{ChatType: "private", Text: "/status"}))
	require.Equal(t, 1, p.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPruneUsesBothLimits(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM bot_messages WHERE created_at < $1 OR id IN (SELECT id FROM bot_messages ORDER BY id DESC OFFSET 2000)")).WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 5))
	require.NoError(t, (&Store{DB: db}).Prune(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}
