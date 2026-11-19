package httpapi

import (
	"github.com/labstack/echo/v5"
)

// RouteRegistrar 由 internal/httpapi/api/* 提供：各资源/用例自注册路由。
type RouteRegistrar interface {
	RegisterRoutes(api *echo.Group)
}

// RegisterRouter 挂载健康检查与全部 /api 路由。
func RegisterRouter(e *echo.Echo, registrars ...RouteRegistrar) {
	registerHealth(e)

	api := e.Group("/api")
	for _, r := range registrars {
		if r != nil {
			r.RegisterRoutes(api)
		}
	}
}
