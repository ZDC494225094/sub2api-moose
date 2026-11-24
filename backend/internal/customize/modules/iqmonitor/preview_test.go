package iqmonitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSignedPreviewServesSandboxedDrawing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, _, _, c := newTestRuntime(t)
	addTarget(t, r.Store, "smart", "")
	round := runOnce(t, r)
	res := resultsByModel(t, r.Store, round.ID, KindDrawing)["smart"]
	url := r.PreviewURL(context.Background(), res.ID)
	require.NotEmpty(t, url)

	engine := gin.New()
	h := &Handler{Runtime: r}
	engine.GET("/api/v1/iq-monitor/preview/:id", h.ServePreview)
	get := func(u string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, u, nil))
		return w
	}
	w := get(url)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "<svg")
	require.NotContains(t, w.Body.String(), "```", "the fenced block is unwrapped")
	csp := w.Header().Get("Content-Security-Policy")
	require.Contains(t, csp, "default-src 'none'")
	require.Contains(t, csp, "sandbox allow-scripts")
	require.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))

	require.Equal(t, http.StatusForbidden, get(strings.Replace(url, "sig=", "sig=x", 1)).Code)
	other := resultsByModel(t, r.Store, round.ID, KindLogic)["smart"]
	require.Equal(t, http.StatusForbidden, get(strings.Replace(url, "/preview/"+itoa(res.ID), "/preview/"+itoa(other.ID), 1)).Code, "a signature is bound to its result")
	c.advance(time.Hour)
	require.Equal(t, http.StatusForbidden, get(url).Code, "links expire")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
