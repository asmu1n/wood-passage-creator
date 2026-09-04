package authapi

import (
	"wood-passage-creator/internal/app/auth"
	"wood-passage-creator/internal/httpapi/middleware"

	"github.com/labstack/echo/v5"
)

// Registrar 认证路由。
type Registrar struct {
	h *Handler
}

func NewRegistrar(authSvc *auth.Service) *Registrar {
	return &Registrar{h: NewHandler(authSvc)}
}

func (r *Registrar) RegisterRoutes(api *echo.Group) {
	g := api.Group("/auth")
	g.POST("/register", r.h.Register)
	g.POST("/login", r.h.Login)
	g.POST("/logout", r.h.Logout, middleware.AuthRequired())
	g.GET("/me", r.h.Me, middleware.AuthRequired())
}
