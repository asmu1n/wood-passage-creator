package http

import (
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/payment"
	"wood-passage-creator/internal/module/user"

	"github.com/labstack/echo/v5"
)

// Register 挂载开发态支付/Mock VIP 路由。
func Register(api *echo.Group, svc *payment.Service, userSvc *user.Service) {
	h := NewHandler(svc)

	{
		vip := api.Group("/payment", middleware.AuthWithRoleRequired(userSvc, false))
		vip.POST("/vip/mock-session", h.CreateMockVIPSession)
		vip.POST("/vip/mock-complete", h.CompleteMockVIP)
	}

	{
		admin := api.Group("/admin/payment", middleware.AuthWithRoleRequired(userSvc, true))

		admin.GET("/list", h.AdminList)
	}

}
