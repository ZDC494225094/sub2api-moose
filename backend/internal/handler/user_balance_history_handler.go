package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (handler *UserHandler) GetBalanceHistory(ctx *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(ctx)
	if !ok {
		response.Unauthorized(ctx, "User not authenticated")
		return
	}
	months, _ := strconv.Atoi(ctx.DefaultQuery("months", "12"))
	points, err := handler.balanceSnapshotService.GetHistory(ctx.Request.Context(), subject.UserID, months)
	if err != nil {
		response.ErrorFrom(ctx, err)
		return
	}
	response.Success(ctx, points)
}
