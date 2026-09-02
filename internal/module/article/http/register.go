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

	{
		article := api.Group("/article", middleware.AuthWithRoleRequired(usersvc, false))

		article.POST("/create", h.Create)
		article.POST("/delete", h.Delete)

		article.POST("/confirm-title", h.ConfirmTitle)
		article.POST("/confirm-outline", h.ConfirmOutline)
		article.POST("/modify-outline", h.ModifyOutline)

		article.GET("/progress/:taskId", h.GetProgress)
		article.GET("/execution-logs/:taskId", h.GetExecutionLogs)

		article.GET("/list/self", h.ListBySelf)
		article.GET("/:taskId", h.GetByTaskID)

	}

	{
		article := api.Group("/article", middleware.AuthWithRoleRequired(usersvc, true))

		article.GET("/list", h.ListAll)
	}
}
