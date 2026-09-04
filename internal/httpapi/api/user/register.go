package userapi

import (
	appuser "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

// Registrar 用户资料与管理端路由。
type Registrar struct {
	h *Handler
}

func NewRegistrar(svc *appuser.Service) *Registrar {
	return &Registrar{h: NewHandler(svc)}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	users := api.Group("/users", middleware.AuthWithRoleRequired(r.h.svc, false))
	users.GET("/:id", r.h.GetByID)
	users.PATCH("/:id", r.h.Update)

	admin := api.Group("/admin/users", middleware.AuthWithRoleRequired(r.h.svc, true))
	admin.GET("/list", r.h.AdminList)
	admin.DELETE("/:id", r.h.Delete)
	admin.POST("/:id/upgrade-vip", r.h.UpgradeVIP)
	admin.POST("/:id/revoke-vip", r.h.RevokeVIP)
}
