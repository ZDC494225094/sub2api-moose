package handler

import (
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestMediaAdmissionErrorPreservesStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &OpenAIGatewayHandler{}
	for _, err := range []error{infraerrors.Forbidden("CUSTOM_EXTENSION_DISABLED", "disabled"), infraerrors.ServiceUnavailable("CUSTOM_EXTENSION_STATE_UNAVAILABLE", "retry")} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		require.True(t, h.handleMediaAdmissionError(c, err))
		require.Equal(t, infraerrors.Code(err), recorder.Code)
		require.Contains(t, recorder.Body.String(), infraerrors.Reason(err))
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.False(t, h.handleMediaAdmissionError(c, errors.New("upstream failure")))
	require.False(t, c.Writer.Written())
}
