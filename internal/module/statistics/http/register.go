package http

import (
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/statistics"
	"wood-passage-creator/internal/module/user"

	"github.com/labstack/echo/v5"
)

// Register 挂载管理端统计路由。
func Register(api *echo.Group, svc *statistics.Service, userSvc *user.Service) {
	h := NewHandler(svc)

	{
		admin := api.Group("/admin/statistics", middleware.AuthWithRoleRequired(userSvc, true))

		admin.GET("/overview", h.Overview)
	}
}
