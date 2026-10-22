package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestVideoHTTPErrorDetails(t *testing.T) {
	for _, body := range []string{`{"error":"unsupported model"}`, `{"error":{"message":"unsupported model"}}`, `{"error":["unsupported model"]}`} {
		t.Run(body, func(t *testing.T) {
			require.Contains(t, extractUpstreamErrorMessage([]byte(body)), "unsupported model")
			service := &OpenAIGatewayService{cfg: &config.Config{}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil).WithContext(context.Background())
			response := &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
			_, err := service.handleCompatErrorResponse(response, c, &Account{ID: 1, Platform: PlatformOpenAI}, writeGrokMediaErrorResponse, "seedance2.0-fast")
			require.Error(t, err)
			require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
			require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String(), "unsupported model")
		})
	}
}
