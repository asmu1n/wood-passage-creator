package http

import (
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/user"

	"github.com/labstack/echo/v5"
)

// Register 挂载文章相关路由到 /api 组。
func Register(api *echo.Group, svc *article.Service, usersvc *user.Service) {
	h := NewHandler(svc)

	article := api.Group("/article", middleware.AuthWithRoleRequired(usersvc, false))
	{
		article.POST("/create", h.Create)
		article.POST("/confirm-title", h.ConfirmTitle)
		article.POST("/confirm-outline", h.ConfirmOutline)

		// 具体 path 须在 /:taskId 之前
		article.GET("/progress/:taskId", h.GetProgress)

		article.GET("/:taskId", h.GetByTaskID)
		article.POST("/list", h.ListAll)
		article.POST("/delete", h.Delete)
	}
}
