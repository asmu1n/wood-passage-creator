package statisticsapi

import (
	statsapp "wood-passage-creator/internal/app/statistics"
	userapp "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

type Registrar struct {
	h       *Handler
	userSvc *userapp.Service
}

func NewRegistrar(svc *statsapp.Service, userSvc *userapp.Service) *Registrar {
	return &Registrar{h: NewHandler(svc), userSvc: userSvc}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	admin := api.Group("/admin/statistics", middleware.AuthWithRoleRequired(r.userSvc, true))
	admin.GET("/overview", r.h.Overview)
}
