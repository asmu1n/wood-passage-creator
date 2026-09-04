package articleapi

import (
	appart "wood-passage-creator/internal/app/article"
	appuser "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

type Registrar struct {
	h       *Handler
	userSvc *appuser.Service
}

func NewRegistrar(svc *appart.Service, userSvc *appuser.Service) *Registrar {
	return &Registrar{h: NewHandler(svc), userSvc: userSvc}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	a := api.Group("/article", middleware.AuthWithRoleRequired(r.userSvc, false))
	a.POST("/create", r.h.Create)
	a.POST("/delete", r.h.Delete)
	a.POST("/confirm-title", r.h.ConfirmTitle)
	a.POST("/confirm-outline", r.h.ConfirmOutline)
	a.POST("/modify-outline", r.h.ModifyOutline)
	a.GET("/progress/:taskId", r.h.GetProgress)
	a.GET("/execution-logs/:taskId", r.h.GetExecutionLogs)
	a.GET("/list/self", r.h.ListBySelf)
	a.GET("/:taskId", r.h.GetByTaskID)

	admin := api.Group("/admin/article", middleware.AuthWithRoleRequired(r.userSvc, true))
	admin.GET("/list", r.h.ListAll)
}
