package paymentapi

import (
	apppay "wood-passage-creator/internal/app/payment"
	appuser "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

type Registrar struct {
	h       *Handler
	userSvc *appuser.Service
}

func NewRegistrar(svc *apppay.Service, userSvc *appuser.Service) *Registrar {
	return &Registrar{h: NewHandler(svc), userSvc: userSvc}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	pay := api.Group("/payment", middleware.AuthWithRoleRequired(r.userSvc, false))
	pay.POST("/vip/mock-session", r.h.CreateMockVIPSession)
	pay.POST("/vip/mock-complete", r.h.CompleteMockVIP)
	pay.GET("/list", r.h.ListBySelf)

	admin := api.Group("/admin/payment", middleware.AuthWithRoleRequired(r.userSvc, true))
	admin.GET("/list", r.h.AdminList)
}
