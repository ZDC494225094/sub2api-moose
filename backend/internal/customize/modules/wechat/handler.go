package wechat

import (
"context"
"fmt"
"net/url"
"os"
"strings"

"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
"github.com/Wei-Shaw/sub2api/internal/pkg/response"
"github.com/gin-gonic/gin"
)

// LotteryRecordReader loads a user's draw records (implemented by the lottery service).
type LotteryRecordReader interface {
ListUserDrawRecords(ctx context.Context, userID, activityID int64, params pagination.PaginationParams) ([]marketing.LotteryDrawRecord, *pagination.PaginationResult, error)
}

// SubjectResolver resolves the current authenticated user id.
type SubjectResolver func(c *gin.Context) (int64, bool)

// Handler exposes the WeChat OA lottery endpoints.
type Handler struct {
oauth *Service
bindings BindingRepository
records LotteryRecordReader
subject SubjectResolver
}

// NewHandler builds the handler. oauth/bindings may be nil (endpoints then report unconfigured).
func NewHandler(oauth *Service, bindings BindingRepository, records LotteryRecordReader, subject SubjectResolver) *Handler {
return &Handler{oauth: oauth, bindings: bindings, records: records, subject: subject}
}

func (h *Handler) requireUser(c *gin.Context) (int64, bool) {
if h.subject!= nil {
if id, ok:= h.subject(c); ok {
return id, true
}
}
response.Unauthorized(c, "User not authenticated")
return 0, false
}

func (h *Handler) requireOAuth(c *gin.Context) bool {
if h.oauth!= nil && h.oauth.Enabled() {
return true
}
response.InternalError(c, "wechat oa is not configured (set WECHAT_OA_APPID / WECHAT_OA_APPSECRET)")
return false
}

// baseURL derives the public scheme+host behind reverse proxies.
func baseURL(c *gin.Context) string {
scheme:= "https"
if proto:= c.GetHeader("X-Forwarded-Proto"); proto!= "" {
scheme = strings.Split(proto, ",")[0]
} else if c.Request.TLS == nil {
scheme = "http"
}
host:= c.GetHeader("X-Forwarded-Host")
if host == "" {
host = c.Request.Host
}
host = strings.Split(host, ",")[0]
return strings.TrimSuffix(scheme+"://"+strings.TrimSpace(host), "/")
}

// h5BaseURL is where the /wx-lottery page lives; override with env when the
// frontend origin differs from the API origin.
func h5BaseURL(c *gin.Context) string {
if v:= strings.TrimSpace(os.Getenv("WECHAT_OA_H5_BASE_URL")); v!= "" {
return strings.TrimSuffix(v, "/")
}
return baseURL(c)
}

func oauthCallbackURL(c *gin.Context) string {
return baseURL(c) + "/api/v1/payment/public/wechat/oauth/callback"
}

// GetOAuthURL returns the WeChat webpage-authorization URL for the H5 page.
// GET /wechat/oauth/url
func (h *Handler) GetOAuthURL(c *gin.Context) {
if!h.requireOAuth(c) {
return
}
back:= strings.TrimSpace(c.Query("back"))
if back == "" {
back = "/wx-lottery"
}
if!strings.HasPrefix(back, "/") {
back = "/wx-lottery"
}
// state carries where to return after authorization (path only, no open redirect).
state:= url.QueryEscape(back)
authURL, err:= h.oauth.BuildAuthorizeURL(oauthCallbackURL(c), state)
if err!= nil {
response.InternalError(c, err.Error())
return
}
response.Success(c, gin.H{"url": authURL})
}

// OAuthCallback exchanges the code for openid and redirects to the H5 page.
// GET /wechat/oauth/callback?code=...&state=...
func (h *Handler) OAuthCallback(c *gin.Context) {
if!h.requireOAuth(c) {
return
}
code:= strings.TrimSpace(c.Query("code"))
if code == "" {
c.String(400, "missing code")
return
}
openid, err:= h.oauth.ExchangeCode(c.Request.Context(), code)
if err!= nil {
c.String(502, "wechat authorization failed")
return
}
back:= "/wx-lottery"
if s, err:= url.QueryUnescape(strings.TrimSpace(c.Query("state"))); err == nil && strings.HasPrefix(s, "/") {
back = s
}
target:= h5BaseURL(c) + back
sep:= "?"
if strings.Contains(target, "?") {
sep = "&"
}
c.Redirect(302, target+sep+"openid="+url.QueryEscape(openid))
}

// GetBinding returns the current user's bound openid, if any.
// GET /wechat/binding
func (h *Handler) GetBinding(c *gin.Context) {
userID, ok:= h.requireUser(c)
if!ok {
return
}
if h.bindings == nil {
response.Success(c, gin.H{"openid": ""})
return
}
b, err:= h.bindings.GetByUserID(c.Request.Context(), userID)
if err!= nil {
response.ErrorFrom(c, err)
return
}
openid:= ""
if b!= nil {
openid = b.OpenID
}
response.Success(c, gin.H{"openid": openid})
}

func validOpenID(openid string) bool {
if len(openid) < 10 || len(openid) > 64 {
return false
}
for _, r:= range openid {
if!(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
return false
}
}
return true
}

// BindOpenID binds the authorized openid to the current user (one-to-one).
// POST /wechat/bind {openid}
func (h *Handler) BindOpenID(c *gin.Context) {
userID, ok:= h.requireUser(c)
if!ok {
return
}
if h.bindings == nil {
response.InternalError(c, "binding store is not configured")
return
}
var req struct {
OpenID string `json:"openid"`
}
if err:= c.ShouldBindJSON(&req); err!= nil {
response.BadRequest(c, "Invalid request: "+err.Error())
return
}
openid:= strings.TrimSpace(req.OpenID)
if!validOpenID(openid) {
response.BadRequest(c, "invalid openid")
return
}
b, err:= h.bindings.Upsert(c.Request.Context(), userID, openid)
if err!= nil {
response.ErrorFrom(c, err)
return
}
response.Success(c, gin.H{"openid": b.OpenID})
}

// NotifyDrawResult pushes the user's latest draw result of the activity to the
// OA conversation via customer-service message.
// POST /wechat/lottery-notify {activity_id}
func (h *Handler) NotifyDrawResult(c *gin.Context) {
userID, ok:= h.requireUser(c)
if!ok {
return
}
if!h.requireOAuth(c) {
return
}
if h.bindings == nil || h.records == nil {
response.InternalError(c, "wechat lottery notify is not configured")
return
}
var req struct {
ActivityID int64 `json:"activity_id" binding:"required,gt=0"`
}
if err:= c.ShouldBindJSON(&req); err!= nil {
response.BadRequest(c, "Invalid request: "+err.Error())
return
}
b, err:= h.bindings.GetByUserID(c.Request.Context(), userID)
if err!= nil {
response.ErrorFrom(c, err)
return
}
if b == nil || b.OpenID == "" {
response.BadRequest(c, "wechat openid is not bound")
return
}
records, _, err:= h.records.ListUserDrawRecords(c.Request.Context(), userID, req.ActivityID, pagination.PaginationParams{Page: 1, PageSize: 1})
if err!= nil {
response.ErrorFrom(c, err)
return
}
if len(records) == 0 {
response.BadRequest(c, "no draw record found")
return
}
rec:= records[0]
text:= fmt.Sprintf("%s\n%s", rec.PrizeName, drawResultHint(rec))
if err:= h.oauth.SendTextMessage(c.Request.Context(), b.OpenID, text); err!= nil {
response.ErrorFrom(c, err)
return
}
response.Success(c, gin.H{"sent": true})
}

func drawResultHint(rec marketing.LotteryDrawRecord) string {
switch rec.ResultCode {
case marketing.LotteryDrawResultWin:
return "恭喜中奖！奖品已发放到你的账户，请查收。"
default:
return "很遗憾这次没有中奖，再接再厉！"
}
}
