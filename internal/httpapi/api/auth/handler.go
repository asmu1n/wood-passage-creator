package authapi

import (
	"net/http"

	authapp "wood-passage-creator/internal/app/auth"
	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	_ "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/response"


	"github.com/labstack/echo/v5"
)

// Handler 认证 HTTP；依赖 app/auth，Me 依赖 app/user.GetByID。
type Handler struct {
	auth *authapp.Service
}

func NewHandler(authSvc *authapp.Service) *Handler {
	return &Handler{auth: authSvc}
}

// Register godoc
// @Summary      注册
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body authapp.RegisterRequest true "注册信息"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误/账号冲突"
// @Router       /auth/register [post]
func (h *Handler) Register(c *echo.Context) error {
	var req authapp.RegisterRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	u, err := h.auth.Register(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// Login godoc
// @Summary      登录
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body authapp.LoginRequest true "登录信息"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误/账号或密码错误"
// @Router       /auth/login [post]
func (h *Handler) Login(c *echo.Context) error {
	var req authapp.LoginRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	u, err := h.auth.Login(c.Request().Context(), req)
	if err != nil {
		return err
	}
	if err := middleware.SaveLoginUserID(c, u.ID); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// Logout godoc
// @Summary      登出
// @Tags         auth
// @Produce      json
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      401 {object} response.Response "未登录"
// @Security     SessionAuth
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *echo.Context) error {
	if err := middleware.ClearLoginSession(c); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// Me godoc
// @Summary      获取当前登录用户
// @Description  根据 session 从数据库加载最新用户信息（角色/配额等），用于前端刷新登录态。
// @Tags         auth
// @Produce      json
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      401 {object} response.Response "未登录"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /auth/me [get]
func (h *Handler) Me(c *echo.Context) error {
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	u, err := h.auth.Me(c.Request().Context(), actorID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}
