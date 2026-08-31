package http

import (
	"net/http"
	"strconv"

	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

// Handler 用户 HTTP 传输层。
type Handler struct {
	svc *user.Service
}

func NewHandler(svc *user.Service) *Handler {
	return &Handler{svc: svc}
}

// Register godoc
// @Summary      注册
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body user.RegisterRequest true "注册信息"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误/账号冲突"
// @Router       /auth/register [post]
func (h *Handler) Register(c *echo.Context) error {
	var req user.RegisterRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	u, err := h.svc.Register(c.Request().Context(), req)
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
// @Param        body body user.LoginRequest true "登录信息"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误/账号或密码错误"
// @Router       /auth/login [post]
func (h *Handler) Login(c *echo.Context) error {
	var req user.LoginRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	u, err := h.svc.Login(c.Request().Context(), req)
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

// List godoc
// @Summary      分页查询用户列表（管理员）
// @Tags         users
// @Produce      json
// @Param        pageNum  query int false "页码，默认 1"
// @Param        pageSize query int false "每页条数，默认 10，最大 100"
// @Success      200 {object} response.Response{data=user.UserListData} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Security     SessionAuth
// @Router       /users/list [get]
func (h *Handler) List(c *echo.Context) error {
	var req page.PageRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	users, total, err := h.svc.QueryList(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(page.NewPageResponse(users, total, req)))
}

// GetByID godoc
// @Summary      按 ID 查询用户
// @Tags         users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      404 {object} response.Response "用户不存在"
// @Router       /users/{id} [get]
func (h *Handler) GetByID(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	u, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// Update godoc
// @Summary      部分更新用户（仅本人）
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id path int true "用户 ID"
// @Param        body body user.UpdateRequest true "更新字段"
// @Success      200 {object} response.Response{data=user.User} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /users/{id} [patch]
func (h *Handler) Update(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	var req user.UpdateRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	if !req.HasUpdates() {
		return response.NewBizErrorWithDetail(response.ParamsError, "请至少提供一个更新字段")
	}
	u, err := h.svc.Update(c.Request().Context(), actorID, id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// Delete godoc
// @Summary      删除用户（软删除，管理员）
// @Tags         users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /users/{id} [delete]
func (h *Handler) Delete(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// UpgradeVIP godoc
// @Summary      [Admin] 升级用户为 VIP
// @Tags         users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response{data=user.User}
// @Security     SessionAuth
// @Router       /users/{id}/upgrade-vip [post]
func (h *Handler) UpgradeVIP(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	u, err := h.svc.UpgradeToVIP(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// RevokeVIP godoc
// @Summary      [Admin] 取消用户 VIP
// @Tags         users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response{data=user.User}
// @Security     SessionAuth
// @Router       /users/{id}/revoke-vip [post]
func (h *Handler) RevokeVIP(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	u, err := h.svc.RevokeVIP(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}
