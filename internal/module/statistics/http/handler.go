package http

import (
	"net/http"

	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/statistics"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	svc *statistics.Service
}

func NewHandler(svc *statistics.Service) *Handler {
	return &Handler{svc: svc}
}

// Overview godoc
// @Summary      系统统计概览（管理员）
// @Description  今日/本周/本月创作量、成功率、平均耗时、用户与 VIP、配额使用等。
// @Tags         admin-statistics
// @Produce      json
// @Success      200 {object} response.Response{data=statistics.Overview}
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Security     SessionAuth
// @Router       /admin/statistics/overview [get]
func (h *Handler) Overview(c *echo.Context) error {
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	out, err := h.svc.GetOverview(c.Request().Context(), actor)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(out))
}
