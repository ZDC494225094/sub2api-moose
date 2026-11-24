package iqmonitor

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Drawing previews are model-written HTML. They are served from their own
// URL so they get their own CSP instead of inheriting the console's (a
// srcdoc frame would inherit the console's nonce-based script policy). The
// CSP sandbox puts the document in an opaque origin even when opened
// directly, so it can never read this site's storage or cookies.
const PreviewCSP = "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:; font-src data:; media-src data:; base-uri 'none'; form-action 'none'; sandbox allow-scripts; frame-ancestors 'self'"

// previewWindow is the URL lifetime bucket: a URL stays identical for a
// whole window so periodic page refreshes do not reload every frame.
const previewWindow = 10 * time.Minute

type previewSigner struct {
	mu     sync.Mutex
	secret []byte
}

// PreviewSecret returns the cluster-wide signing key, creating it once.
func (s *Store) PreviewSecret(ctx context.Context) ([]byte, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	// Only the first writer's key sticks; everyone then reads it back.
	if _, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_settings SET preview_secret = $1 WHERE id = 1 AND preview_secret = ''`, hex.EncodeToString(b)); err != nil {
		return nil, err
	}
	var raw string
	if err := s.DB.QueryRowContext(ctx, `SELECT preview_secret FROM iq_monitor_settings WHERE id = 1`).Scan(&raw); err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) < 16 {
		return nil, errors.New("iq-monitor: invalid preview secret")
	}
	return key, nil
}

func (r *Runtime) previewKey(ctx context.Context) ([]byte, error) {
	r.signer.mu.Lock()
	defer r.signer.mu.Unlock()
	if r.signer.secret != nil {
		return r.signer.secret, nil
	}
	key, err := r.Store.PreviewSecret(ctx)
	if err != nil {
		return nil, err
	}
	r.signer.secret = key
	return key, nil
}

func previewSig(key []byte, id, exp int64) string {
	m := hmac.New(sha256.New, key)
	_, _ = fmt.Fprintf(m, "iq-preview:%d:%d", id, exp)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// PreviewURL signs a preview link valid until the end of the next window.
func (r *Runtime) PreviewURL(ctx context.Context, resultID int64) string {
	key, err := r.previewKey(ctx)
	if err != nil {
		r.report(err)
		return ""
	}
	now := r.Store.now()
	exp := now.Truncate(previewWindow).Add(2 * previewWindow).Unix()
	return fmt.Sprintf("/api/v1/iq-monitor/preview/%d?exp=%d&sig=%s", resultID, exp, previewSig(key, resultID, exp))
}

func (r *Runtime) verifyPreview(ctx context.Context, id int64, expRaw, sig string) bool {
	exp, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || r.Store.now().Unix() > exp {
		return false
	}
	key, err := r.previewKey(ctx)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(previewSig(key, id, exp)), []byte(sig)) == 1
}

const previewFallback = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body style="margin:0;display:grid;place-items:center;height:100vh;font:14px sans-serif;color:#888">没有可预览的 SVG</body></html>`

// ServePreview is mounted without login: the signature is the credential.
func (h *Handler) ServePreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || !h.Runtime.verifyPreview(c.Request.Context(), id, c.Query("exp"), c.Query("sig")) {
		c.Header("Cache-Control", "no-store")
		c.String(http.StatusForbidden, "预览链接无效或已过期")
		return
	}
	d, err := h.Runtime.Detail(c.Request.Context(), id)
	if err != nil || d.Result.Kind != KindDrawing {
		c.Header("Cache-Control", "no-store")
		c.String(http.StatusNotFound, "记录不存在或已被清理")
		return
	}
	doc := d.PreviewHTML
	if doc == "" {
		doc = previewFallback
	}
	c.Header("Content-Security-Policy", PreviewCSP)
	// The host sets DENY on every route; previews must frame in the console.
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("Cache-Control", "private, max-age=600")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(doc))
}
