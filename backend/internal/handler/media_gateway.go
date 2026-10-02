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
