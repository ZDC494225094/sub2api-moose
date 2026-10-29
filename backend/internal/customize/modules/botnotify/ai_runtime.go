package botnotify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	// aiHistoryWindow drops stale context: a question asked hours later is
	// usually a new topic.
	aiHistoryWindow = 6 * time.Hour
	// aiAnswerTimeout bounds one answer including tool rounds and retries.
	aiAnswerTimeout = 3 * time.Minute
	aiImageTimeout  = 30 * time.Second
)

func (r *Runtime) newBotDispatcher(ctx context.Context, e BotEntry, platform Platform) *aiDispatcher {
	return newAIDispatcher(ctx,
		func(ctx context.Context, items []aiItem) { r.answerAI(ctx, e, platform, items) },
		func(ctx context.Context) time.Duration {
			config, err := r.Store.AIConfig(ctx, e.ID)
			if err != nil {
				return 0
			}
			return time.Duration(config.CooldownSeconds) * time.Second
		})
}

// routeAI runs on the poll loop and stays cheap: messages that need no model
// call get their fixed reply here, everything else is queued for answerAI.
func (r *Runtime) routeAI(ctx context.Context, e BotEntry, platform Platform, m Message, rowID int64, dispatcher *aiDispatcher) {
	config, err := r.Store.AIConfig(ctx, e.ID)
	if err != nil || !config.Enabled {
		return
	}
	items := []aiItem{{msg: m, rowID: rowID, at: time.Now()}}
	hasText := strings.TrimSpace(m.Text) != ""
	if !hasText && len(m.Images) == 0 {
		// Stickers are usually a "thanks"; answering them would be noise.
		if m.Attachment != "" && m.Attachment != "sticker" {
			r.replyFixed(ctx, e, platform, items, config, AttachmentReply(m.Attachment, config.AllowImages), "unsupported_attachment")
		}
		return
	}
	if !hasText && !config.AllowImages {
		r.replyFixed(ctx, e, platform, items, config, AttachmentReply("image", false), "images_disabled")
		return
	}
	if hasText {
		if guard := InspectInput(m.Text); !guard.Allowed {
			r.replyFixed(ctx, e, platform, items, config, SafeReply(), guard.Reason)
			return
		}
	}
	if dispatcher == nil || !dispatcher.enqueue(conversationKey(m), items[0]) {
		slog.Warn("bot-notify ai queue unavailable; message not answered", "bot", e.ID)
	}
}

// answerAI produces one answer for a batch of merged messages from the same
// conversation. It runs on a dispatcher worker, never on the poll loop.
func (r *Runtime) answerAI(ctx context.Context, e BotEntry, platform Platform, items []aiItem) {
	config, err := r.Store.AIConfig(ctx, e.ID)
	if err != nil || !config.Enabled || !r.allowed(ctx) {
		return
	}
	merged := mergeAIItems(items)
	// Each message passed the input guard alone. Check the batch glued
	// together too, so an instruction split across messages ("忽略之前的" +
	// "指令") is caught; the newest text is kept if the batch is long.
	if len(items) > 1 {
		joined := []rune(strings.ReplaceAll(merged.Text, "\n", ""))
		if len(joined) > AIInputLimit {
			joined = joined[len(joined)-AIInputLimit:]
		}
		if guard := InspectInput(string(joined)); !guard.Allowed && guard.Reason != GuardReasonEmpty {
			r.replyFixed(ctx, e, platform, items, config, SafeReply(), guard.Reason)
			return
		}
	}
	knowledge := r.aiContext(ctx)
	if r.aiOverDailyLimit(ctx, e, config) {
		r.replyFixed(ctx, e, platform, items, config, DailyLimitReply(knowledge), "daily_limit")
		return
	}
	req := AIRequest{Config: config, Message: merged, Context: knowledge, History: r.aiHistory(ctx, e, merged)}
	req.Images, req.ImageNote = r.aiImages(ctx, e, platform, merged.Images, config)
	if strings.TrimSpace(merged.Text) == "" && len(req.Images) == 0 {
		r.replyFixed(ctx, e, platform, items, config, AttachmentReply("image", config.AllowImages), "image_unreadable")
		return
	}
	out := r.runAI(ctx, e, req, true)
	_ = r.persistAI(ctx, e, config, aiAudit{
		msg: merged, question: auditQuestion(merged), answer: out.answer, blocked: out.blocked, reason: out.reason,
		errCode: out.errCode, images: len(merged.Images), latency: out.latency,
		inTokens: out.result.InputTokens, outTokens: out.result.OutputTokens,
	})
	r.deliver(ctx, e, platform, items, out.answer)
}

// aiOutcome is the checked result of one model answer.
type aiOutcome struct {
	answer  string
	blocked bool
	reason  string
	errCode string
	latency time.Duration
	result  AIResult
}

// runAI calls the provider and applies every output check. Failures and
// rejected answers become fixed replies; nothing model-generated that failed
// a check is ever returned.
func (r *Runtime) runAI(ctx context.Context, e BotEntry, req AIRequest, report bool) aiOutcome {
	start := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, aiAnswerTimeout)
	result, err := r.AI.Complete(callCtx, req)
	cancel()
	out := aiOutcome{latency: time.Since(start), result: result}
	if err != nil {
		out.blocked, out.reason, out.errCode = true, "provider_error", AIErrorCode(err)
		if aiErrorNeedsAdmin(out.errCode) {
			out.answer = UnavailableReply(req.Context)
			if report {
				r.report(e.ID, ctx, fmt.Errorf("AI 客服调用失败（%s），请检查所选 API Key 的状态、余额和模型", out.errCode))
			}
		} else {
			out.answer = BusyReply(req.Context)
			if report {
				r.report(e.ID, ctx, fmt.Errorf("AI 客服调用失败（%s）", out.errCode))
			}
		}
		return out
	}
	answer := ChatText(result.Text)
	guard := InspectOutput(answer)
	if guard.Allowed {
		guard = CheckLinks(answer, trustedHosts(req, result))
	}
	if !guard.Allowed {
		out.blocked, out.reason = true, guard.Reason
		out.answer = OutputBlockedReply(req.Context)
		return out
	}
	out.answer = answer
	return out
}

// trustedHosts collects the domains an answer may link to: everything the
// admin wrote (knowledge base, manual, contacts), the tool results of this
// answer and, in a private chat, links the user sent themselves.
func trustedHosts(req AIRequest, result AIResult) []string {
	texts := []string{EffectiveKnowledgeBase(req.Config), req.Context.SiteURL, req.Context.ContactInfo, req.Context.AfterSalesGroup, req.Context.CustomerServiceLink}
	if req.Config.UseUserGuide {
		texts = append(texts, req.Context.Guide)
	}
	texts = append(texts, result.ToolOutputs...)
	if req.Message.ChatType == "private" {
		texts = append(texts, req.Message.Text)
	}
	var hosts []string
	for _, t := range texts {
		hosts = append(hosts, LinkHosts(t)...)
	}
	return hosts
}

// mergeAIItems combines a conversation batch into one question: texts in
// arrival order and the most recent images.
func mergeAIItems(items []aiItem) Message {
	merged := items[len(items)-1].msg
	var texts []string
	var images []ImageRef
	for _, it := range items {
		if t := strings.TrimSpace(it.msg.Text); t != "" {
			texts = append(texts, t)
		}
		images = append(images, it.msg.Images...)
	}
	if len(images) > MaxImagesPerAnswer {
		images = images[len(images)-MaxImagesPerAnswer:]
	}
	merged.Text = strings.Join(texts, "\n")
	merged.Images = images
	return merged
}

// auditQuestion is the stored question; images are only counted.
func auditQuestion(m Message) string {
	q := strings.TrimSpace(m.Text)
	if n := len(m.Images); n > 0 {
		if q != "" {
			q += "\n"
		}
		q += fmt.Sprintf("[图片×%d]", n)
	}
	return q
}

// aiImages downloads the batch's images for the model. Failures never abort
// the answer; the model is told what it cannot see instead.
func (r *Runtime) aiImages(ctx context.Context, e BotEntry, platform Platform, refs []ImageRef, c AIConfig) ([]Image, string) {
	if len(refs) == 0 {
		return nil, ""
	}
	if !c.AllowImages {
		return nil, fmt.Sprintf("用户还发送了 %d 张图片，但图片识别未开启，你看不到图片内容；需要时请用户用文字描述", len(refs))
	}
	fetcher, ok := platform.(ImageFetcher)
	if !ok {
		return nil, fmt.Sprintf("用户还发送了 %d 张图片，但当前平台无法读取图片；需要时请用户用文字描述", len(refs))
	}
	var images []Image
	failed := 0
	for _, ref := range refs {
		fetchCtx, cancel := context.WithTimeout(ctx, aiImageTimeout)
		img, err := fetcher.FetchImage(fetchCtx, e.BotToken, ref)
		cancel()
		if err != nil {
			failed++
			if ctx.Err() == nil {
				slog.Info("bot-notify image unavailable", "bot", e.ID, "error", err)
			}
			continue
		}
		images = append(images, img)
	}
	if failed == 0 {
		return images, ""
	}
	return images, fmt.Sprintf("用户还发送了 %d 张图片，但读取失败（可能过大或格式不支持），你看不到这些图片；需要时请用户重新截图或用文字描述", failed)
}

// replyFixed answers with a fixed text and audits it as blocked, so it never
// enters conversation history.
func (r *Runtime) replyFixed(ctx context.Context, e BotEntry, platform Platform, items []aiItem, c AIConfig, text, reason string) {
	merged := mergeAIItems(items)
	_ = r.persistAI(ctx, e, c, aiAudit{msg: merged, question: auditQuestion(merged), answer: text, blocked: true, reason: reason, images: len(merged.Images)})
	r.deliver(ctx, e, platform, items, text)
}

// deliver sends the answer to the newest message of the batch and marks
// every merged message replied once the send succeeded.
func (r *Runtime) deliver(ctx context.Context, e BotEntry, platform Platform, items []aiItem, text string) {
	if text == "" || !r.allowed(ctx) {
		return
	}
	if err := r.sendReply(ctx, e, platform, items[len(items)-1].msg, text); err != nil {
		r.report(e.ID, ctx, fmt.Errorf("AI 回复发送失败：%w", err))
		return
	}
	for _, it := range items {
		_, _ = r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET replied=true,reply_content=$1 WHERE id=$2", text, it.rowID)
	}
}

// aiContext assembles the public-only context an AI provider may see. It
// holds no live data: the prompt stays byte-stable so upstream prompt caches
// stay effective, and live site information (status, prices, plans,
// announcements) reaches the model through read-only query tools.
func (r *Runtime) aiContext(ctx context.Context) PublicDTO {
	if r.Knowledge != nil {
		dto, err := r.Knowledge.AIKnowledge(ctx)
		if err == nil {
			return dto
		}
		slog.Warn("bot-notify ai knowledge unavailable", "error", err)
	}
	dto := PublicDTO{}
	if name, err := r.Data.SiteName(ctx); err == nil {
		dto.SiteName = name
	}
	return dto
}

// aiHistory loads the most recent successful exchanges of this conversation
// so follow-up questions keep their context and the shared prefix can hit
// upstream prompt caches. In a group each member only sees their own turns.
// Blocked and failed exchanges are excluded, and every side is truncated so
// history stays a bounded, small part of the request.
func (r *Runtime) aiHistory(ctx context.Context, e BotEntry, m Message) []AIHistoryTurn {
	sender := ""
	if m.ChatType != "private" {
		sender = m.SenderID
	}
	rows, err := r.Store.DB.QueryContext(ctx, `
		SELECT msg.question_redacted, msg.content_redacted
		FROM bot_ai_messages msg
		JOIN bot_ai_conversations conv ON conv.id = msg.conversation_id
		WHERE conv.bot_id = $1 AND conv.platform = $2 AND conv.chat_id = $3
		  AND msg.direction = 'assistant' AND msg.blocked = false AND msg.error_code = ''
		  AND ($4 = '' OR msg.sender_id = $4) AND msg.created_at > $5
		ORDER BY msg.id DESC
		LIMIT 4`, e.ID, m.Platform, m.ChatID, sender, time.Now().Add(-aiHistoryWindow))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var turns []AIHistoryTurn
	for rows.Next() {
		var turn AIHistoryTurn
		if err := rows.Scan(&turn.User, &turn.Assistant); err != nil {
			return nil
		}
		turn.User = truncateAudit(turn.User, 600)
		turn.Assistant = truncateAudit(turn.Assistant, 600)
		if turn.User != "" && turn.Assistant != "" {
			turns = append(turns, turn)
		}
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	return turns
}

func truncateAudit(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

// aiOverDailyLimit enforces the per-bot daily answer cap. The limit notices
// themselves do not count, so the cap reflects real answers.
func (r *Runtime) aiOverDailyLimit(ctx context.Context, e BotEntry, c AIConfig) bool {
	if c.DailyLimit <= 0 {
		return false
	}
	var answered int
	err := r.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bot_ai_messages msg JOIN bot_ai_conversations conv ON conv.id=msg.conversation_id WHERE conv.bot_id=$1 AND msg.direction='assistant' AND msg.created_at >= CURRENT_DATE AND msg.blocked_reason <> 'daily_limit'`, e.ID).Scan(&answered)
	return err == nil && answered >= c.DailyLimit
}

// aiAudit is one stored AI exchange (question, final reply, accounting).
type aiAudit struct {
	msg       Message
	question  string
	answer    string
	blocked   bool
	reason    string
	errCode   string
	images    int
	latency   time.Duration
	inTokens  int
	outTokens int
}

func (r *Runtime) persistAI(ctx context.Context, e BotEntry, c AIConfig, a aiAudit) error {
	var conversationID int64
	if err := r.Store.DB.QueryRowContext(ctx, `INSERT INTO bot_ai_conversations(bot_id,platform,chat_id,external_user_id,last_message_at) VALUES($1,$2,$3,$4,CURRENT_TIMESTAMP) ON CONFLICT(bot_id,platform,chat_id) DO UPDATE SET last_message_at=CURRENT_TIMESTAMP RETURNING id`, e.ID, a.msg.Platform, a.msg.ChatID, a.msg.SenderID).Scan(&conversationID); err != nil {
		return err
	}
	latency := int(a.latency / time.Millisecond)
	if a.latency > 0 && latency == 0 {
		latency = 1
	}
	_, err := r.Store.DB.ExecContext(ctx, `INSERT INTO bot_ai_messages(conversation_id,direction,external_message_id,question_redacted,sender_name,sender_id,content_redacted,blocked,blocked_reason,model,api_key_id,error_code,latency_ms,input_tokens,output_tokens,image_count) VALUES($1,'assistant',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, conversationID, fmt.Sprint(a.msg.MessageID), auditText(a.question), auditText(a.msg.SenderName), a.msg.SenderID, auditText(a.answer), a.blocked, a.reason, c.Model, c.APIKeyID, a.errCode, latency, a.inTokens, a.outTokens, a.images)
	return err
}

func auditText(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 2000 {
		s = string([]rune(s)[:2000])
	}
	return s
}

// AITestResult is the admin console's view of one trial answer.
type AITestResult struct {
	Answer        string   `json:"answer"`
	Blocked       bool     `json:"blocked"`
	Reason        string   `json:"reason,omitempty"`
	ErrorCode     string   `json:"error_code,omitempty"`
	LatencyMS     int      `json:"latency_ms"`
	InputTokens   int      `json:"input_tokens"`
	OutputTokens  int      `json:"output_tokens"`
	Tools         []string `json:"tools"`
	ImagesDropped bool     `json:"images_dropped,omitempty"`
}

// TestAI runs the full answer pipeline (input guard, knowledge, tools, output
// checks) for an admin trial with the saved configuration. Nothing is sent
// to any chat or stored; the model call is billed to the configured key.
func (r *Runtime) TestAI(ctx context.Context, e BotEntry, text string, images []Image) (AITestResult, error) {
	if r.AI == nil {
		return AITestResult{}, errors.New("AI 客服未接入")
	}
	config, err := r.Store.AIConfig(ctx, e.ID)
	if err != nil {
		return AITestResult{}, err
	}
	if config.APIKeyID == nil || strings.TrimSpace(config.Model) == "" {
		return AITestResult{}, errors.New("请先选择 API Key 和模型并保存配置")
	}
	text = strings.TrimSpace(text)
	if text == "" && len(images) == 0 {
		return AITestResult{}, errors.New("请输入问题或上传图片")
	}
	fixed := func(answer, reason string) AITestResult {
		return AITestResult{Answer: answer, Blocked: true, Reason: reason, Tools: []string{}}
	}
	if text != "" {
		if guard := InspectInput(text); !guard.Allowed {
			return fixed(SafeReply(), guard.Reason), nil
		}
	}
	req := AIRequest{Config: config, Message: Message{Platform: e.Platform, ChatType: "private", ChatID: "admin-test", Text: text}, Context: r.aiContext(ctx)}
	if len(images) > 0 {
		if !config.AllowImages {
			if text == "" {
				return fixed(AttachmentReply("image", false), "images_disabled"), nil
			}
			req.ImageNote = fmt.Sprintf("用户还发送了 %d 张图片，但图片识别未开启，你看不到图片内容；需要时请用户用文字描述", len(images))
		} else {
			req.Images = images
		}
	}
	out := r.runAI(ctx, e, req, false)
	tools := out.result.Tools
	if tools == nil {
		tools = []string{}
	}
	return AITestResult{
		Answer: out.answer, Blocked: out.blocked, Reason: out.reason, ErrorCode: out.errCode,
		LatencyMS: int(out.latency / time.Millisecond), InputTokens: out.result.InputTokens,
		OutputTokens: out.result.OutputTokens, Tools: tools, ImagesDropped: out.result.ImagesDropped,
	}, nil
}
