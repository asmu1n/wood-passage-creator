package httpapi

import (
	"wood-passage-creator/internal/module/article"
	articlehttp "wood-passage-creator/internal/module/article/http"
	"wood-passage-creator/internal/module/payment"
	paymenthttp "wood-passage-creator/internal/module/payment/http"
	"wood-passage-creator/internal/module/statistics"
	statisticshttp "wood-passage-creator/internal/module/statistics/http"
	"wood-passage-creator/internal/module/user"
	userhttp "wood-passage-creator/internal/module/user/http"

	"github.com/labstack/echo/v5"
)

// RegisterRouter 注册全部 HTTP 路由。
func RegisterRouter(
	e *echo.Echo,
	userSvc *user.Service,
	articleSvc *article.Service,
	paymentSvc *payment.Service,
	statsSvc *statistics.Service,
) {
	registerHealth(e)

	api := e.Group("/api")
	userhttp.Register(api, userSvc)
	articlehttp.Register(api, articleSvc, userSvc)
	paymenthttp.Register(api, paymentSvc, userSvc)
	statisticshttp.Register(api, statsSvc, userSvc)
}
