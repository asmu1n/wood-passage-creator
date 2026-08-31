package http

import (
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/user"

	"github.com/labstack/echo/v5"
)

// Register 挂载用户/认证相关路由到 /api 组。
func Register(api *echo.Group, svc *user.Service) {
	h := NewHandler(svc)

	auth := api.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout, middleware.AuthRequired())
	}

	users := api.Group("/users")
	{
		users.GET("/:id", h.GetByID)
		users.PATCH("/:id", h.Update, middleware.AuthRequired())
		// 管理端：列表 / 删除需管理员
		users.GET("/list", h.List, middleware.AuthWithRoleRequired(svc, true))
		users.DELETE("/:id", h.Delete, middleware.AuthWithRoleRequired(svc, true))
		// 管理端：开发期直接升降 VIP（真支付上线后仍可保留）
		users.POST("/:id/upgrade-vip", h.UpgradeVIP, middleware.AuthWithRoleRequired(svc, true))
		users.POST("/:id/revoke-vip", h.RevokeVIP, middleware.AuthWithRoleRequired(svc, true))
	}
}
