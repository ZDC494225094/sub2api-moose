package iqmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const reviewLock = 5 * time.Minute

func (r *Runtime) reviewLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(reviewEvery)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
		}
		// Drain due submissions, one at a time.
		for i := 0; i < 20 && r.ctx.Err() == nil; i++ {
			if !r.reviewOne(r.ctx) {
				break
			}
		}
	}
}

// reviewOne locks and reviews the oldest due submission. It returns false
// when there was nothing to do.
func (r *Runtime) reviewOne(ctx context.Context) bool {
	if !r.enabled(ctx) {
		return false
	}
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		r.report(err)
		return false
	}
	topic, err := r.Store.ClaimReview(ctx, r.NodeID, reviewLock)
	if err != nil {
		r.report(err)
		return false
	}
	if topic == nil {
		return false
	}
	reviewCtx, cancel := context.WithTimeout(ctx, reviewLock-30*time.Second)
	defer cancel()
	approved, reason, err := r.review(reviewCtx, cfg, topic.Topic)
	bg := context.WithoutCancel(ctx)
	if ctx.Err() != nil {
		// Shutdown: the expired lock lets another node retry it.
		return false
	}
	if err != nil {
		r.report(r.Store.FailReview(bg, *topic, r.NodeID, err.Error()))
		return true
	}
	r.report(r.Store.FinishReview(bg, topic.ID, r.NodeID, approved, reason))
	return true
}

func (r *Runtime) review(ctx context.Context, cfg Config, topic string) (bool, string, error) {
	s := cfg.Submissions
	if s.ReviewAPIKeyID == 0 || strings.TrimSpace(s.ReviewModel) == "" || cfg.ProbeUserID == 0 {
		return false, "", errors.New("未配置审核模型或令牌")
	}
	key, err := r.Host.APIKey(ctx, s.ReviewAPIKeyID, cfg.ProbeUserID)
	if err != nil {
		return false, "", errors.New("审核令牌不可用")
	}
	if !r.acquire(ctx) {
		return false, "", ctx.Err()
	}
	defer r.release()
	out, err := r.Caller.Call(ctx, CallRequest{APIKey: key, Model: s.ReviewModel, Prompt: renderPrompt(s.ReviewPrompt, "{topic}", topic),
		MaxOutputTokens: 0, Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second})
	if err != nil {
		return false, "", err
	}
	return ParseReview(out.Content)
}

var jsonObject = regexp.MustCompile(`(?s)\{.*\}`)

// ParseReview reads the reviewer's JSON verdict. An unparsable reply is an
// error (retried), never an approval.
func ParseReview(reply string) (bool, string, error) {
	text := strings.TrimSpace(stripThinking(reply))
	raw := jsonObject.FindString(text)
	if raw == "" {
		return false, "", errors.New("审核模型没有返回 JSON")
	}
	var v struct {
		Approved *bool  `json:"approved"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil || v.Approved == nil {
		return false, "", errors.New("审核模型返回的 JSON 无法解析")
	}
	reason := strings.TrimSpace(v.Reason)
	if !*v.Approved && reason == "" {
		reason = "主题不符合投稿规范"
	}
	if *v.Approved {
		reason = ""
	}
	return *v.Approved, truncateRunes(reason, 200), nil
}

// CleanTopic validates a submission before it is queued for review.
func CleanTopic(raw string, maxLen int) (string, error) {
	topic := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	n := utf8.RuneCountInString(topic)
	if n < 4 {
		return "", errors.New("主题至少 4 个字")
	}
	if n > maxLen {
		return "", errors.New("主题过长")
	}
	for _, r := range topic {
		if unicode.IsControl(r) {
			return "", errors.New("主题包含不可见字符")
		}
	}
	return topic, nil
}

// MaskName hides a submitter's name on public pages: first and last
// character kept, e.g. "hello" -> "h***o", "hg" -> "h*".
func MaskName(name string) string {
	r := []rune(strings.TrimSpace(name))
	switch len(r) {
	case 0:
		return "匿名用户"
	case 1:
		return string(r) + "*"
	case 2:
		return string(r[0]) + "*"
	}
	stars := len(r) - 2
	if stars > 4 {
		stars = 4
	}
	return string(r[0]) + strings.Repeat("*", stars) + string(r[len(r)-1])
}

// displayName picks the username, else the local part of the email.
func displayName(username, email string) string {
	if strings.TrimSpace(username) != "" {
		return username
	}
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at]
	}
	return email
}
