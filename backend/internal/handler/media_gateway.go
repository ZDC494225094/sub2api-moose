package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/mediagateway"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

func (h *OpenAIGatewayHandler) admitMediaGateway(c *gin.Context, operation mediagateway.Operation) bool {
	if err := h.gatewayService.CheckMediaGatewayAdmission(c.Request.Context(), operation); err != nil {
		h.errorResponse(c, infraerrors.Code(err), infraerrors.Reason(err), infraerrors.Message(err))
		return false
	}
	return true
}

// Preserve local admission errors without retrying or penalizing an account.
func (h *OpenAIGatewayHandler) handleMediaAdmissionError(c *gin.Context, err error) bool {
	switch infraerrors.Reason(err) {
	case "CUSTOM_EXTENSION_DISABLED", "CUSTOM_EXTENSION_STATE_UNAVAILABLE":
		h.errorResponse(c, infraerrors.Code(err), infraerrors.Reason(err), infraerrors.Message(err))
		return true
	default:
		return false
	}
}
