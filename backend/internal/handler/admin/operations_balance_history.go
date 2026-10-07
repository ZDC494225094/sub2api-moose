package admin

import (
	"log/slog"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *OperationsHandler) GetOperationsBalanceHistory(c *gin.Context) {
	months, err := strconv.Atoi(c.DefaultQuery("months", "12"))
	if err != nil || (months != 3 && months != 6 && months != 12) {
		response.BadRequest(c, "months must be 3, 6 or 12")
		return
	}
	points, err := h.balanceSnapshotService.GetHistory(c.Request.Context(), months)
	if err != nil {
		slog.Error("operations_balance_history_failed", "error", err)
		response.Error(c, 500, "Failed to get operations balance history")
		return
	}
	response.Success(c, points)
}
