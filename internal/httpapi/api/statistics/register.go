package statisticsapi

import (
	appstat "wood-passage-creator/internal/app/statistics"
	appuser "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

type Registrar struct {
	h       *Handler
	userSvc *appuser.Service
}

func NewRegistrar(svc *appstat.Service, userSvc *appuser.Service) *Registrar {
	return &Registrar{h: NewHandler(svc), userSvc: userSvc}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	admin := api.Group("/admin/statistics", middleware.AuthWithRoleRequired(r.userSvc, true))
	admin.GET("/overview", r.h.Overview)
}
