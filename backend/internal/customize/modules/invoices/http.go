package invoices

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Handler serves the user and administrator invoice APIs. Both mount behind
// the host's authentication, rate-limit and audit chain and the module switch.
type Handler struct {
	Store *Store
}

const settingsKey = "invoices.settings"

// RegisterUser mounts /invoices under the authenticated user group. Overview is
// reachable by every signed-in user so the console can decide whether to show
// the entry; everything else additionally requires access under the settings.
func (h *Handler) RegisterUser(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/invoices", manager.Require(ID), noStore)
	g.GET("/overview", h.overview)
	allowed := g.Group("", h.requireAccess)
	allowed.GET("/orders", h.orders)
	allowed.GET("/titles", h.listTitles)
	allowed.POST("/titles", h.createTitle)
	allowed.PUT("/titles/:id", h.updateTitle)
	allowed.DELETE("/titles/:id", h.deleteTitle)
	allowed.GET("/applications", h.listOwnApplications)
	allowed.POST("/applications", h.apply)
	allowed.GET("/applications/:id", h.ownApplication)
	allowed.POST("/applications/:id/cancel", h.cancel)
	allowed.GET("/applications/:id/file", h.ownFile)
}

// RegisterAdmin mounts /invoices under the administrator group.
func (h *Handler) RegisterAdmin(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/invoices", manager.Require(ID), noStore)
	g.GET("/settings", h.getSettings)
	g.PUT("/settings", h.saveSettings)
	g.GET("/access", h.listAccess)
	g.POST("/access", h.addAccess)
	g.DELETE("/access/:user_id", h.removeAccess)
	g.GET("/applications", h.listApplications)
	g.GET("/applications/:id", h.adminApplication)
	g.POST("/applications/:id/process", h.process)
	g.POST("/applications/:id/issue", h.issue)
	g.POST("/applications/:id/file", h.uploadFile)
	g.GET("/applications/:id/file", h.adminFile)
	g.POST("/applications/:id/reject", h.reject)
	g.POST("/applications/:id/void", h.void)
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func subjectID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func pathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return id, true
}

func (h *Handler) requireAccess(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		c.Abort()
		return
	}
	settings, err := h.Store.Settings(c.Request.Context())
	if err == nil {
		var allowed bool
		allowed, err = h.Store.HasAccess(c.Request.Context(), userID, settings)
		if err == nil && !allowed {
			err = infraerrors.Forbidden("INVOICE_ACCESS_DENIED", "开票功能暂未对你的账户开放，如需开票请联系客服")
		}
	}
	if response.ErrorFrom(c, err) {
		c.Abort()
		return
	}
	c.Set(settingsKey, settings)
	c.Next()
}

func settingsFrom(c *gin.Context) Settings {
	if value, ok := c.Get(settingsKey); ok {
		if settings, ok := value.(Settings); ok {
			return settings
		}
	}
	return Settings{}
}

// publicSettings is what users see of the rules.
func publicSettings(s Settings) gin.H {
	return gin.H{
		"min_amount": s.MinAmount.StringFixed(2), "allow_special": s.AllowSpecial,
		"order_window_days": s.OrderWindowDays, "item_name": s.ItemName, "notice": s.Notice,
	}
}

// ---------- user ----------

func (h *Handler) overview(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	settings, err := h.Store.Settings(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	allowed, err := h.Store.HasAccess(c.Request.Context(), userID, settings)
	if response.ErrorFrom(c, err) {
		return
	}
	if !allowed {
		response.Success(c, gin.H{"allowed": false})
		return
	}
	response.Success(c, gin.H{"allowed": true, "settings": publicSettings(settings)})
}

func (h *Handler) orders(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	settings := settingsFrom(c)
	orders, truncated, err := h.Store.InvoiceableOrders(c.Request.Context(), userID, settings)
	if response.ErrorFrom(c, err) {
		return
	}
	total := decimal.Zero
	for _, o := range orders {
		total = total.Add(o.Amount)
	}
	response.Success(c, gin.H{"items": orders, "total_amount": total.StringFixed(2), "truncated": truncated})
}

func (h *Handler) listTitles(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	items, err := h.Store.ListTitles(c.Request.Context(), userID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) saveTitle(c *gin.Context, id int64) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	var req TitleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	in, err := NormalizeTitle(req)
	if response.ErrorFrom(c, err) {
		return
	}
	title, err := h.Store.SaveTitle(c.Request.Context(), userID, id, in)
	if !response.ErrorFrom(c, err) {
		response.Success(c, title)
	}
}

func (h *Handler) createTitle(c *gin.Context) { h.saveTitle(c, 0) }

func (h *Handler) updateTitle(c *gin.Context) {
	id, ok := pathID(c, "id")
	if ok {
		h.saveTitle(c, id)
	}
}

func (h *Handler) deleteTitle(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, h.Store.DeleteTitle(c.Request.Context(), userID, id)) {
		response.Success(c, gin.H{"id": id})
	}
}

func listFilter(c *gin.Context) ApplicationFilter {
	page, pageSize := response.ParsePagination(c)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return ApplicationFilter{Status: validStatus(c.Query("status")), Query: c.Query("q"), Page: page, PageSize: pageSize}
}

func validStatus(status string) string {
	switch status {
	case StatusPending, StatusProcessing, StatusIssued, StatusRejected, StatusCancelled, StatusVoided:
		return status
	default:
		return ""
	}
}

// forOwner hides review internals from the applicant.
func forOwner(a Application) Application {
	a.AdminNote = ""
	a.RefundAlert = false
	a.UserEmail = ""
	if a.Status != StatusIssued {
		a.HasFile = false
		a.FileName = ""
	}
	for i := range a.Orders {
		a.Orders[i].Refunded = false
	}
	return a
}

func (h *Handler) listOwnApplications(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	filter := listFilter(c)
	filter.UserID = userID
	filter.Query = ""
	items, total, err := h.Store.ListApplications(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	for i := range items {
		items[i] = forOwner(items[i])
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *Handler) apply(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	var req ApplyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	in, err := NormalizeApply(req)
	if response.ErrorFrom(c, err) {
		return
	}
	settings := settingsFrom(c)
	title, err := h.Store.Title(c.Request.Context(), userID, in.TitleID)
	if response.ErrorFrom(c, err) {
		return
	}
	if response.ErrorFrom(c, ValidateTitleFor(title, in.InvoiceType, settings)) {
		return
	}
	app, err := h.Store.CreateApplication(c.Request.Context(), userID, in, title, settings)
	if !response.ErrorFrom(c, err) {
		response.Created(c, forOwner(app))
	}
}

func (h *Handler) ownApplication(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	app, err := h.Store.Application(c.Request.Context(), id, userID, false)
	if !response.ErrorFrom(c, err) {
		response.Success(c, forOwner(app))
	}
}

func (h *Handler) cancel(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if !response.ErrorFrom(c, h.Store.Cancel(c.Request.Context(), userID, id)) {
		response.Success(c, gin.H{"id": id, "status": StatusCancelled})
	}
}

func (h *Handler) ownFile(c *gin.Context) {
	userID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if ok {
		h.sendFile(c, id, userID)
	}
}

// ---------- admin ----------

func (h *Handler) getSettings(c *gin.Context) {
	settings, err := h.Store.Settings(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, adminSettings(settings))
	}
}

func adminSettings(s Settings) gin.H {
	out := publicSettings(s)
	out["access_mode"] = s.AccessMode
	out["updated_at"] = s.UpdatedAt
	return out
}

type settingsRequest struct {
	MinAmount       string `json:"min_amount"`
	AllowSpecial    bool   `json:"allow_special"`
	OrderWindowDays int    `json:"order_window_days"`
	AccessMode      string `json:"access_mode"`
	ItemName        string `json:"item_name"`
	Notice          string `json:"notice"`
}

func (h *Handler) saveSettings(c *gin.Context) {
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	minAmount := decimal.Zero
	if raw := strings.TrimSpace(req.MinAmount); raw != "" {
		parsed, err := decimal.NewFromString(raw)
		if err != nil {
			response.ErrorFrom(c, badRequest("INVOICE_SETTINGS_INVALID", "开票门槛必须是金额"))
			return
		}
		minAmount = parsed
	}
	settings, err := NormalizeSettings(Settings{
		MinAmount: minAmount, AllowSpecial: req.AllowSpecial, OrderWindowDays: req.OrderWindowDays,
		AccessMode: req.AccessMode, ItemName: req.ItemName, Notice: req.Notice,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	saved, err := h.Store.SaveSettings(c.Request.Context(), settings)
	if !response.ErrorFrom(c, err) {
		response.Success(c, adminSettings(saved))
	}
}

func (h *Handler) listAccess(c *gin.Context) {
	items, err := h.Store.ListAccess(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

type accessRequest struct {
	// Users holds e-mail addresses or numeric user IDs.
	Users []string `json:"users"`
	Note  string   `json:"note"`
}

func (h *Handler) addAccess(c *gin.Context) {
	var req accessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if len(req.Users) == 0 || len(req.Users) > 200 {
		response.ErrorFrom(c, badRequest("INVOICE_ACCESS_INVALID", "请填写 1 到 200 个用户邮箱或用户 ID"))
		return
	}
	note := clean(req.Note)
	if tooLong(note, 200) {
		response.ErrorFrom(c, badRequest("INVOICE_ACCESS_INVALID", "备注最多 200 个字符"))
		return
	}
	found, missing, err := h.Store.ResolveUsers(c.Request.Context(), req.Users)
	if response.ErrorFrom(c, err) {
		return
	}
	if len(found) > 0 {
		if response.ErrorFrom(c, h.Store.AddAccess(c.Request.Context(), found, note)) {
			return
		}
	}
	if missing == nil {
		missing = []string{}
	}
	response.Success(c, gin.H{"added": len(found), "missing": missing})
}

func (h *Handler) removeAccess(c *gin.Context) {
	id, ok := pathID(c, "user_id")
	if ok && !response.ErrorFrom(c, h.Store.RemoveAccess(c.Request.Context(), id)) {
		response.Success(c, gin.H{"user_id": id})
	}
}

func (h *Handler) listApplications(c *gin.Context) {
	filter := listFilter(c)
	items, total, err := h.Store.ListApplications(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	counts, err := h.Store.StatusCounts(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	pages := int((total + int64(filter.PageSize) - 1) / int64(max(filter.PageSize, 1)))
	response.Success(c, gin.H{"items": items, "total": total, "page": filter.Page, "page_size": filter.PageSize, "pages": pages, "counts": counts})
}

func (h *Handler) adminApplication(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	app, err := h.Store.Application(c.Request.Context(), id, 0, true)
	if !response.ErrorFrom(c, err) {
		response.Success(c, app)
	}
}

// respondApplication answers an admin action with the updated application.
func (h *Handler) respondApplication(c *gin.Context, id int64, err error) {
	if response.ErrorFrom(c, err) {
		return
	}
	app, err := h.Store.Application(c.Request.Context(), id, 0, true)
	if !response.ErrorFrom(c, err) {
		response.Success(c, app)
	}
}

func (h *Handler) process(c *gin.Context) {
	adminID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if ok {
		h.respondApplication(c, id, h.Store.MarkProcessing(c.Request.Context(), adminID, id))
	}
}

// readPDF reads an optional "file" part from a multipart request.
func readPDF(c *gin.Context, required bool) (*File, error) {
	header, err := c.FormFile("file")
	if errors.Is(err, http.ErrMissingFile) {
		if required {
			return nil, badRequest("INVOICE_FILE_REQUIRED", "请选择要上传的 PDF 发票")
		}
		return nil, nil
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, badRequest("INVOICE_FILE_TOO_LARGE", "发票文件不能超过 10MB")
		}
		return nil, badRequest("INVOICE_FILE_INVALID", "无法读取上传的文件")
	}
	if header.Size > MaxFileBytes {
		return nil, badRequest("INVOICE_FILE_TOO_LARGE", "发票文件不能超过 10MB")
	}
	src, err := header.Open()
	if err != nil {
		return nil, badRequest("INVOICE_FILE_INVALID", "无法读取上传的文件")
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(io.LimitReader(src, MaxFileBytes+1))
	if err != nil {
		return nil, badRequest("INVOICE_FILE_INVALID", "无法读取上传的文件")
	}
	name, err := ValidatePDF(header.Filename, data)
	if err != nil {
		return nil, err
	}
	return &File{Name: name, Data: data}, nil
}

// limitUpload caps multipart bodies a little above the file limit.
func limitUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileBytes+(1<<20))
}

func (h *Handler) issue(c *gin.Context) {
	adminID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	limitUpload(c)
	var invoiceNo, note string
	var file *File
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		invoiceNo, note = c.PostForm("invoice_no"), c.PostForm("admin_note")
		var err error
		if file, err = readPDF(c, false); response.ErrorFrom(c, err) {
			return
		}
	} else {
		var req struct {
			InvoiceNo string `json:"invoice_no"`
			AdminNote string `json:"admin_note"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "invalid request body")
			return
		}
		invoiceNo, note = req.InvoiceNo, req.AdminNote
	}
	invoiceNo, err := NormalizeInvoiceNo(invoiceNo)
	if response.ErrorFrom(c, err) {
		return
	}
	note = clean(note)
	if tooLong(note, 500) {
		response.ErrorFrom(c, badRequest("INVOICE_NOTE_TOO_LONG", "内部备注最多 500 个字符"))
		return
	}
	h.respondApplication(c, id, h.Store.Issue(c.Request.Context(), adminID, id, invoiceNo, note, file))
}

func (h *Handler) uploadFile(c *gin.Context) {
	adminID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	limitUpload(c)
	file, err := readPDF(c, true)
	if response.ErrorFrom(c, err) {
		return
	}
	h.respondApplication(c, id, h.Store.ReplaceFile(c.Request.Context(), adminID, id, file))
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) closeWith(c *gin.Context, act func(adminID, id int64, reason string) error) {
	adminID, ok := subjectID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req reasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	reason, err := NormalizeReason(req.Reason)
	if response.ErrorFrom(c, err) {
		return
	}
	h.respondApplication(c, id, act(adminID, id, reason))
}

func (h *Handler) reject(c *gin.Context) {
	h.closeWith(c, func(adminID, id int64, reason string) error {
		return h.Store.Reject(c.Request.Context(), adminID, id, reason)
	})
}

func (h *Handler) void(c *gin.Context) {
	h.closeWith(c, func(adminID, id int64, reason string) error {
		return h.Store.Void(c.Request.Context(), adminID, id, reason)
	})
}

func (h *Handler) adminFile(c *gin.Context) {
	id, ok := pathID(c, "id")
	if ok {
		h.sendFile(c, id, 0)
	}
}

func (h *Handler) sendFile(c *gin.Context, id, userID int64) {
	file, invoiceNo, err := h.Store.File(c.Request.Context(), id, userID)
	if response.ErrorFrom(c, err) {
		return
	}
	name := "invoice-" + strconv.FormatInt(id, 10) + ".pdf"
	if invoiceNo != "" {
		name = "发票-" + invoiceNo + ".pdf"
	}
	ascii := "invoice-" + strconv.FormatInt(id, 10) + ".pdf"
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(name)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, "application/pdf", file.Data)
}
