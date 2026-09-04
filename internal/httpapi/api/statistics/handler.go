package statisticsapi

import (
	"net/http"

	app "wood-passage-creator/internal/app/statistics"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) *Handler {
	return &Handler{svc: svc}
}

// Overview godoc
// @Summary      系统概览统计（管理员）
// @Tags         admin-statistics
// @Produce      json
// @Success      200 {object} response.Response{data=statistics.Overview} "成功"
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
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
