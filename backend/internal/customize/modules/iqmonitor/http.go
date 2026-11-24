package iqmonitor

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// Handler serves the public page API (authenticated users) and the admin
// console. Both mount behind the host's auth, rate-limit and audit chain.
type Handler struct{ Runtime *Runtime }

func badRequest(msg string) error { return infraerrors.BadRequest("IQ_MONITOR_INVALID", msg) }

func notFound() error {
	return infraerrors.NotFound("IQ_MONITOR_NOT_FOUND", "记录不存在或已被清理")
}

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return notFound()
	case errors.Is(err, ErrQuotaExceeded):
		return infraerrors.TooManyRequests("IQ_MONITOR_QUOTA", err.Error())
	case errors.Is(err, ErrRoundRunning):
		return infraerrors.Conflict("IQ_MONITOR_RUNNING", err.Error())
	}
	return err
}

func (h *Handler) RegisterUser(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/iq-monitor", manager.Require(ID))
	g.GET("/access", h.access)
	allowed := g.Group("", h.requireViewer)
	allowed.GET("/overview", h.overview)
	allowed.GET("/rounds/:id/wall", h.roundWall)
	allowed.GET("/results/:id", h.detail)
	allowed.GET("/submissions", h.mySubmissions)
	allowed.POST("/submissions", h.submit)
}

// RegisterPublic mounts the signed drawing preview, which iframes load
// without the console's bearer token.
func (h *Handler) RegisterPublic(group *gin.RouterGroup, manager *customize.Manager) {
	group.GET("/iq-monitor/preview/:id", manager.Require(ID), h.ServePreview)
}

func (h *Handler) RegisterAdmin(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/iq-monitor", manager.Require(ID))
	g.GET("/config", h.getConfig)
	g.PUT("/config", h.saveConfig)
	g.GET("/status", h.status)
	g.GET("/problems", h.problems)
	g.POST("/trigger", h.trigger)
	g.GET("/estimate", h.estimate)
	g.POST("/history/clear", h.clearHistory)
	g.GET("/options", h.options)
	g.GET("/options/models", h.models)
	g.GET("/options/efforts", h.efforts)
	g.GET("/options/users", h.users)
	g.GET("/targets", h.listTargets)
	g.POST("/targets", h.createTarget)
	g.PUT("/targets/:id", h.updateTarget)
	g.DELETE("/targets/:id", h.deleteTarget)
	g.PUT("/targets-order", h.reorderTargets)
	g.GET("/questions", h.listQuestions)
	g.POST("/questions", h.createQuestion)
	g.PUT("/questions/:id", h.updateQuestion)
	g.DELETE("/questions/:id", h.deleteQuestion)
	g.GET("/topics", h.listTopics)
	g.DELETE("/topics/:id", h.deleteTopic)
	g.GET("/viewers", h.listViewers)
	g.POST("/viewers", h.addViewer)
	g.DELETE("/viewers/:user_id", h.removeViewer)
	g.GET("/results/:id", h.detail)
}

func userID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID, ok && subject.UserID > 0
}

func isAdmin(c *gin.Context) bool {
	role, _ := middleware.GetUserRoleFromContext(c)
	return role == "admin"
}

func (h *Handler) canView(ctx context.Context, c *gin.Context) (bool, error) {
	if isAdmin(c) {
		return true, nil
	}
	uid, ok := userID(c)
	if !ok {
		return false, nil
	}
	cfg, _, err := h.Runtime.Store.Config(ctx)
	if err != nil {
		return false, err
	}
	if cfg.Visibility == VisibilityAll {
		return true, nil
	}
	return h.Runtime.Store.IsViewer(ctx, uid)
}

func (h *Handler) access(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ok, err := h.canView(c.Request.Context(), c)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"allowed": ok})
	}
}

func (h *Handler) requireViewer(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ok, err := h.canView(c.Request.Context(), c)
	if err == nil && !ok {
		err = infraerrors.Forbidden("IQ_MONITOR_FORBIDDEN", "你没有查看降智监测的权限")
	}
	if response.ErrorFrom(c, err) {
		c.Abort()
		return
	}
	c.Next()
}

func (h *Handler) overview(c *gin.Context) {
	ov, err := h.Runtime.Overview(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, ov)
	}
}

func paramID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, badRequest("无效的编号"))
		return 0, false
	}
	return id, true
}

func (h *Handler) roundWall(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	wall, err := h.Runtime.Wall(c.Request.Context(), id)
	if !response.ErrorFrom(c, mapErr(err)) {
		response.Success(c, wall)
	}
}

func (h *Handler) detail(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	d, err := h.Runtime.Detail(c.Request.Context(), id)
	if response.ErrorFrom(c, mapErr(err)) {
		return
	}
	if d.Result.Kind == KindDrawing && len(d.Responses) > 0 {
		d.PreviewURL = h.Runtime.PreviewURL(c.Request.Context(), id)
	}
	response.Success(c, d)
}

func (h *Handler) mySubmissions(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		response.ErrorFrom(c, infraerrors.Unauthorized("IQ_MONITOR_AUTH", "请先登录"))
		return
	}
	out, err := h.Runtime.Submissions(c.Request.Context(), uid)
	if !response.ErrorFrom(c, err) {
		response.Success(c, out)
	}
}

func (h *Handler) submit(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		response.ErrorFrom(c, infraerrors.Unauthorized("IQ_MONITOR_AUTH", "请先登录"))
		return
	}
	var req struct {
		Topic string `json:"topic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return
	}
	t, err := h.Runtime.Submit(c.Request.Context(), uid, req.Topic)
	if err != nil && !errors.Is(err, ErrQuotaExceeded) && (errors.Is(err, errSubmissionsClosed) || !strings.HasPrefix(err.Error(), "iq-monitor:")) {
		err = badRequest(err.Error())
	}
	if !response.ErrorFrom(c, mapErr(err)) {
		response.Success(c, t)
	}
}

// configView is the admin config plus derived facts.
type configView struct {
	Config
	DefaultLogicPrompt  string `json:"default_logic_prompt"`
	DefaultDrawPrompt   string `json:"default_draw_prompt"`
	DefaultReviewPrompt string `json:"default_review_prompt"`
	DefaultTopics       string `json:"default_topics"`
}

func (h *Handler) getConfig(c *gin.Context) {
	cfg, _, err := h.Runtime.Store.Config(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, configView{Config: cfg, DefaultLogicPrompt: DefaultLogicPrompt, DefaultDrawPrompt: DefaultDrawPrompt,
		DefaultReviewPrompt: DefaultReviewPrompt, DefaultTopics: strings.Join(DefaultTopics, "\n")})
}

func (h *Handler) saveConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return
	}
	if cfg.Notify.BotIDs == nil {
		cfg.Notify.BotIDs = []int64{}
	}
	if err := cfg.Validate(); err != nil {
		response.ErrorFrom(c, badRequest(err.Error()))
		return
	}
	ctx := c.Request.Context()
	if cfg.ProbeUserID != 0 {
		if users, err := h.Runtime.Host.Users(ctx, "", []int64{cfg.ProbeUserID}); err != nil || len(users) == 0 {
			response.ErrorFrom(c, badRequest("探测账号不存在"))
			return
		}
	}
	if s := cfg.Submissions; s.ReviewAPIKeyID != 0 {
		if cfg.ProbeUserID == 0 {
			response.ErrorFrom(c, badRequest("请先选择探测账号"))
			return
		}
		if _, err := h.Runtime.Host.APIKey(ctx, s.ReviewAPIKeyID, cfg.ProbeUserID); err != nil {
			response.ErrorFrom(c, badRequest("审核令牌不属于探测账号或未启用"))
			return
		}
		if strings.TrimSpace(s.ReviewModel) == "" {
			response.ErrorFrom(c, badRequest("请填写审核模型"))
			return
		}
	}
	cfg.Submissions.ReviewModel = strings.TrimSpace(cfg.Submissions.ReviewModel)
	if response.ErrorFrom(c, h.Runtime.Store.SaveConfig(ctx, cfg)) {
		return
	}
	h.Runtime.signal()
	h.getConfig(c)
}

func (h *Handler) status(c *gin.Context) {
	st, err := h.Runtime.Status(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, st)
	}
}

func (h *Handler) problems(c *gin.Context) {
	items, err := h.Runtime.Store.RecentProblems(c.Request.Context(), 30)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) trigger(c *gin.Context) {
	id, err := h.Runtime.TriggerNow(c.Request.Context())
	if err != nil && !errors.Is(err, ErrRoundRunning) && !strings.HasPrefix(err.Error(), "iq-monitor:") {
		err = badRequest(err.Error())
	}
	if !response.ErrorFrom(c, mapErr(err)) {
		response.Success(c, gin.H{"round_id": id})
	}
}

func (h *Handler) estimate(c *gin.Context) {
	est, err := h.Runtime.EstimateDaily(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, est)
	}
}

func (h *Handler) clearHistory(c *gin.Context) {
	var req struct {
		All     bool   `json:"all"`
		GroupID int64  `json:"group_id"`
		Model   string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if !req.All && (req.GroupID <= 0 || req.Model == "") {
		response.ErrorFrom(c, badRequest("请选择要清空的分组和模型"))
		return
	}
	n, err := h.Runtime.Store.ClearHistory(c.Request.Context(), req.All, req.GroupID, req.Model)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"deleted": n})
	}
}

func (h *Handler) probeUser(ctx context.Context) int64 {
	cfg, _, err := h.Runtime.Store.Config(ctx)
	if err != nil {
		return 0
	}
	return cfg.ProbeUserID
}

func (h *Handler) options(c *gin.Context) {
	ctx := c.Request.Context()
	out := gin.H{"groups": []GroupOption{}, "keys": []KeyOption{}, "bots": []BotOption{}, "probe_user": nil}
	groups, err := h.Runtime.Host.Groups(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	out["groups"] = groups
	if uid := h.probeUser(ctx); uid != 0 {
		keys, err := h.Runtime.Host.Keys(ctx, uid)
		if response.ErrorFrom(c, err) {
			return
		}
		out["keys"] = keys
		if users, err := h.Runtime.Host.Users(ctx, "", []int64{uid}); err == nil && len(users) > 0 {
			out["probe_user"] = users[0]
		}
	}
	if h.Runtime.Host.Bots != nil {
		if bots, err := h.Runtime.Host.Bots(ctx); err == nil {
			out["bots"] = bots
		}
	}
	response.Success(c, out)
}

func (h *Handler) models(c *gin.Context) {
	keyID, _ := strconv.ParseInt(c.Query("key_id"), 10, 64)
	uid := h.probeUser(c.Request.Context())
	if keyID <= 0 || uid == 0 || h.Runtime.Host.Models == nil {
		response.Success(c, gin.H{"models": []string{}})
		return
	}
	models, err := h.Runtime.Host.Models(c.Request.Context(), keyID, uid)
	if err != nil {
		response.Success(c, gin.H{"models": []string{}, "error": err.Error()})
		return
	}
	response.Success(c, gin.H{"models": models})
}

// efforts lists the levels a model supports and what the gateway forwards
// for each under the group's policy.
func (h *Handler) efforts(c *gin.Context) {
	ctx := c.Request.Context()
	groupID, _ := strconv.ParseInt(c.Query("group_id"), 10, 64)
	model := strings.TrimSpace(c.Query("model"))
	platform := ""
	if groupID > 0 {
		for _, g := range mustGroups(ctx, h.Runtime) {
			if g.ID == groupID {
				platform = g.Platform
			}
		}
	}
	items := []EffortOption{}
	for _, v := range EffortLevels(platform, model) {
		opt := EffortOption{Value: v, Preview: EffortPreview{Sent: v}}
		if h.Runtime.Host.Effort != nil && groupID > 0 && model != "" {
			opt.Preview = h.Runtime.Host.Effort(ctx, groupID, model, v)
		}
		items = append(items, opt)
	}
	response.Success(c, gin.H{"platform": platform, "items": items})
}

func mustGroups(ctx context.Context, r *Runtime) []GroupOption {
	if r.Host.Groups == nil {
		return nil
	}
	list, _ := r.Host.Groups(ctx)
	return list
}

func (h *Handler) users(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) > 100 {
		q = string([]rune(q)[:100])
	}
	users, err := h.Runtime.Host.Users(c.Request.Context(), q, nil)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": users})
	}
}

func (h *Handler) listTargets(c *gin.Context) {
	items, err := h.Runtime.Store.Targets(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	type view struct {
		Target
		SentEffort string `json:"sent_effort"`
		EffortNote string `json:"effort_note,omitempty"`
		Denied     bool   `json:"effort_denied,omitempty"`
	}
	out := make([]view, 0, len(items))
	for _, t := range items {
		v := view{Target: t}
		if t.ReasoningEffort != "" && h.Runtime.Host.Effort != nil {
			p := h.Runtime.Host.Effort(c.Request.Context(), t.GroupID, t.Model, t.ReasoningEffort)
			v.SentEffort, v.EffortNote, v.Denied = p.Sent, p.Note, p.Denied
		}
		out = append(out, v)
	}
	response.Success(c, gin.H{"items": out, "max": MaxTargets})
}

func (h *Handler) bindTarget(c *gin.Context) (Target, bool) {
	var t Target
	if err := c.ShouldBindJSON(&t); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return t, false
	}
	t.Model, t.DisplayName = strings.TrimSpace(t.Model), strings.TrimSpace(t.DisplayName)
	t.ReasoningEffort = strings.ToLower(strings.TrimSpace(t.ReasoningEffort))
	ctx := c.Request.Context()
	fail := func(msg string) (Target, bool) {
		response.ErrorFrom(c, badRequest(msg))
		return t, false
	}
	if t.Model == "" || utf8.RuneCountInString(t.Model) > 200 {
		return fail("请填写模型（不超过 200 字）")
	}
	if t.DisplayName == "" {
		t.DisplayName = t.Model
	}
	if utf8.RuneCountInString(t.DisplayName) > 100 {
		return fail("显示名称不超过 100 字")
	}
	var group *GroupOption
	for _, g := range mustGroups(ctx, h.Runtime) {
		if g.ID == t.GroupID {
			g := g
			group = &g
		}
	}
	if group == nil {
		return fail("请选择有效的分组")
	}
	if group.ClaudeCodeOnly {
		return fail("该分组仅允许 Claude Code 客户端且没有降级分组，网关会以 403 拒绝监测请求；请选择其他分组或为它设置降级分组")
	}
	if !validEffort(group.Platform, t.Model, t.ReasoningEffort) {
		return fail("该模型不支持所选思考深度")
	}
	uid := h.probeUser(ctx)
	if uid == 0 {
		return fail("请先在基本设置中选择探测账号并保存")
	}
	if _, err := h.Runtime.Host.APIKey(ctx, t.APIKeyID, uid); err != nil {
		return fail("所选令牌不属于探测账号或未启用")
	}
	keys, err := h.Runtime.Host.Keys(ctx, uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return t, false
	}
	for _, k := range keys {
		if k.ID != t.APIKeyID {
			continue
		}
		for _, gid := range k.GroupIDs {
			if gid == t.GroupID {
				return t, true
			}
		}
		return fail("所选令牌没有绑定这个分组")
	}
	return fail("所选令牌不属于探测账号或未启用")
}

func (h *Handler) createTarget(c *gin.Context) {
	t, ok := h.bindTarget(c)
	if !ok {
		return
	}
	id, err := h.Runtime.Store.CreateTarget(c.Request.Context(), t)
	if err != nil && strings.HasPrefix(err.Error(), "最多") {
		err = badRequest(err.Error())
	}
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"id": id})
	}
}

func (h *Handler) updateTarget(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	t, ok := h.bindTarget(c)
	if !ok {
		return
	}
	t.ID = id
	if !response.ErrorFrom(c, mapErr(h.Runtime.Store.UpdateTarget(c.Request.Context(), t))) {
		response.Success(c, gin.H{"id": id})
	}
}

func (h *Handler) deleteTarget(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, mapErr(h.Runtime.Store.DeleteTarget(c.Request.Context(), id))) {
		response.Success(c, gin.H{"deleted": true})
	}
}

func (h *Handler) reorderTargets(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return
	}
	err := h.Runtime.Store.ReorderTargets(c.Request.Context(), req.IDs)
	if err != nil && !strings.HasPrefix(err.Error(), "iq-monitor:") {
		err = badRequest(err.Error())
	}
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"ok": true})
	}
}

func (h *Handler) listQuestions(c *gin.Context) {
	items, err := h.Runtime.Store.Questions(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

const maxQuestions = 1000

func (h *Handler) bindQuestion(c *gin.Context) (Question, bool) {
	var q Question
	if err := c.ShouldBindJSON(&q); err != nil {
		response.ErrorFrom(c, badRequest("请求格式错误"))
		return q, false
	}
	q.Question = strings.TrimSpace(q.Question)
	if q.Question == "" || utf8.RuneCountInString(q.Question) > 4000 {
		response.ErrorFrom(c, badRequest("题目不能为空且不超过 4000 字"))
		return q, false
	}
	answers := []string{}
	seen := map[string]bool{}
	for _, a := range q.Answers {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] {
			continue
		}
		if utf8.RuneCountInString(a) > 200 {
			response.ErrorFrom(c, badRequest("单个答案不超过 200 字"))
			return q, false
		}
		if NormalizeAnswer(a) == "" {
			response.ErrorFrom(c, badRequest("答案「"+a+"」归一化后为空，无法判定"))
			return q, false
		}
		seen[a] = true
		answers = append(answers, a)
	}
	if len(answers) == 0 || len(answers) > 20 {
		response.ErrorFrom(c, badRequest("每题需要 1 到 20 个可接受答案"))
		return q, false
	}
	q.Answers = answers
	return q, true
}

func (h *Handler) createQuestion(c *gin.Context) {
	q, ok := h.bindQuestion(c)
	if !ok {
		return
	}
	if n, err := h.Runtime.Store.CountQuestions(c.Request.Context()); err == nil && n >= maxQuestions {
		response.ErrorFrom(c, badRequest("题库最多 1000 题"))
		return
	}
	q.ID = 0
	id, err := h.Runtime.Store.SaveQuestion(c.Request.Context(), q)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"id": id})
	}
}

func (h *Handler) updateQuestion(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	q, ok := h.bindQuestion(c)
	if !ok {
		return
	}
	q.ID = id
	_, err := h.Runtime.Store.SaveQuestion(c.Request.Context(), q)
	if !response.ErrorFrom(c, mapErr(err)) {
		response.Success(c, gin.H{"id": id})
	}
}

func (h *Handler) deleteQuestion(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, mapErr(h.Runtime.Store.DeleteQuestion(c.Request.Context(), id))) {
		response.Success(c, gin.H{"deleted": true})
	}
}

func (h *Handler) listTopics(c *gin.Context) {
	page, size := response.ParsePagination(c)
	status := c.Query("status")
	switch status {
	case "", TopicPending, TopicReviewing, TopicApproved, TopicRejected, TopicFailed, TopicUsed:
	default:
		response.ErrorFrom(c, badRequest("无效的状态"))
		return
	}
	items, total, err := h.Runtime.Store.AdminTopics(c.Request.Context(), status, size, (page-1)*size)
	if !response.ErrorFrom(c, err) {
		response.Paginated(c, items, int64(total), page, size)
	}
}

func (h *Handler) deleteTopic(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, mapErr(h.Runtime.Store.DeleteTopic(c.Request.Context(), id))) {
		response.Success(c, gin.H{"deleted": true})
	}
}

func (h *Handler) listViewers(c *gin.Context) {
	items, err := h.Runtime.Store.Viewers(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) addViewer(c *gin.Context) {
	var req struct {
		UserID int64  `json:"user_id"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		response.ErrorFrom(c, badRequest("请选择用户"))
		return
	}
	if users, err := h.Runtime.Host.Users(c.Request.Context(), "", []int64{req.UserID}); err != nil || len(users) == 0 {
		response.ErrorFrom(c, badRequest("用户不存在"))
		return
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 200 {
		note = string([]rune(note)[:200])
	}
	if !response.ErrorFrom(c, h.Runtime.Store.AddViewer(c.Request.Context(), req.UserID, note)) {
		response.Success(c, gin.H{"ok": true})
	}
}

func (h *Handler) removeViewer(c *gin.Context) {
	id, ok := paramID(c, "user_id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, h.Runtime.Store.RemoveViewer(c.Request.Context(), id)) {
		response.Success(c, gin.H{"ok": true})
	}
}
