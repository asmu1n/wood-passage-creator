package userapi

import (
	"net/http"
	"strconv"

	app "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

// Handler 用户资料/管理端 HTTP；只依赖 app/user。
type Handler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) *Handler {
	return &Handler{svc: svc}
}

// AdminList godoc
// @Summary      分页查询用户列表（管理员）
// @Tags         admin-users
// @Produce      json
// @Param        pageNum  query int false "页码，默认 1"
// @Param        pageSize query int false "每页条数，默认 10，最大 100"
// @Success      200 {object} response.Response{data=appuser.UserListData} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Security     SessionAuth
// @Router       /admin/users/list [get]
func (h *Handler) AdminList(c *echo.Context) error {
	var req page.PageRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	users, total, err := h.svc.ListAll(c.Request().Context(), actor, req)
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
// @Success      200 {object} response.Response{data=moduser_placeholder "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
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
// @Param        body body appuser.UpdateRequest true "更新字段"
// @Success      200 {object} response.Response{data=moduser_placeholder "成功"
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
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	var req app.UpdateRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	if !req.HasUpdates() {
		return response.NewBizErrorWithDetail(response.ParamsError, "请至少提供一个更新字段")
	}
	u, err := h.svc.Update(c.Request().Context(), actor, id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// Delete godoc
// @Summary      删除用户（软删除，管理员）
// @Tags         admin-users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /admin/users/{id} [delete]
func (h *Handler) Delete(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	if err := h.svc.AdminDelete(c.Request().Context(), actor, id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// UpgradeVIP godoc
// @Summary      升级用户为 VIP（管理员）
// @Tags         admin-users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response{data=moduser_placeholder
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /admin/users/{id}/upgrade-vip [post]
func (h *Handler) UpgradeVIP(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	u, err := h.svc.AdminUpgradeVIP(c.Request().Context(), actor, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// RevokeVIP godoc
// @Summary      取消用户 VIP（管理员）
// @Tags         admin-users
// @Produce      json
// @Param        id path int true "用户 ID"
// @Success      200 {object} response.Response{data=moduser_placeholder
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "用户不存在"
// @Security     SessionAuth
// @Router       /admin/users/{id}/revoke-vip [post]
func (h *Handler) RevokeVIP(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的用户 ID")
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	u, err := h.svc.AdminRevokeVIP(c.Request().Context(), actor, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}
