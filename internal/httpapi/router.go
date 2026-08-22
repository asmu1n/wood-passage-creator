package httpapi

import (
	"wood-passage-creator/internal/module/article"
	articlehttp "wood-passage-creator/internal/module/article/http"
	"wood-passage-creator/internal/module/user"
	userhttp "wood-passage-creator/internal/module/user/http"

	"github.com/labstack/echo/v5"
)

// RegisterRouter 注册全部 HTTP 路由。
func RegisterRouter(e *echo.Echo, userSvc *user.Service, articleSvc *article.Service) {
	registerHealth(e)

	api := e.Group("/api")
	userhttp.Register(api, userSvc)
	articlehttp.Register(api, articleSvc, userSvc)
}
