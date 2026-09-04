package paymentapi

import (
	"net/http"

	app "wood-passage-creator/internal/app/payment"
	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) *Handler {
	return &Handler{svc: svc}
}

// CreateMockVIPSession godoc
// @Summary      [Dev] 创建 Mock VIP 支付会话
// @Description  不连接 Stripe；写入 PENDING 支付记录并返回假 checkoutUrl。
// @Tags         payment
// @Produce      json
// @Success      200 {object} response.Response{data=payment.MockSessionResult}
// @Security     SessionAuth
// @Router       /payment/vip/mock-session [post]
func (h *Handler) CreateMockVIPSession(c *echo.Context) error {
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.CreateMockVIPSession(c.Request().Context(), actorID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(out))
}

// CompleteMockVIP godoc
// @Summary      [Dev] Mock 支付成功回调
// @Description  将 PENDING 会话标为 SUCCEEDED 并为对应用户开通 VIP（幂等）。
// @Tags         payment
// @Accept       json
// @Produce      json
// @Param        body body payment.MockCompleteRequest true "sessionId"
// @Success      200 {object} response.Response{data=payment.MockCompleteResult}
// @Security     SessionAuth
// @Router       /payment/vip/mock-complete [post]
func (h *Handler) CompleteMockVIP(c *echo.Context) error {
	var req app.MockCompleteRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	out, err := h.svc.CompleteMockVIP(c.Request().Context(), actor, req.SessionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(out))
}

// AdminList godoc
// @Summary      分页查询支付记录（管理员）
// @Description  全站支付记录列表，仅管理员；支持 pageNum/pageSize/status/userId/productType 查询参数。
// @Tags         admin-payment
// @Produce      json
// @Param        pageNum     query int    false "页码，默认 1"
// @Param        pageSize    query int    false "每页条数，默认 10，最大 100"
// @Param        status      query string false "状态筛选" Enums(PENDING, SUCCEEDED, FAILED, REFUNDED)
// @Param        userId      query int    false "用户 ID"
// @Param        productType query string false "产品类型，如 VIP_PERMANENT"
// @Success      200 {object} response.Response{data=payment.RecordListData} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Security     SessionAuth
// @Router       /admin/payment/list [get]
func (h *Handler) AdminList(c *echo.Context) error {
	var req app.ListRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	out, count, err := h.svc.ListAll(c.Request().Context(), actor, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(page.NewPageResponse(out, count, req.PageRequest)))
}

// ListBySelf godoc
// @Summary      我的支付记录
// @Tags         payment
// @Produce      json
// @Param        pageNum     query int    false "页码，默认 1"
// @Param        pageSize    query int    false "每页条数，默认 10，最大 100"
// @Param        status      query string false "状态筛选" Enums(PENDING, SUCCEEDED, FAILED, REFUNDED)
// @Param        productType query string false "产品类型"
// @Success      200 {object} response.Response{data=payment.RecordListData} "成功"
// @Security     SessionAuth
// @Router       /payment/list [get]
func (h *Handler) ListBySelf(c *echo.Context) error {
	var req app.ListByUserRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actor, err := middleware.GetLoginActor(c)
	if err != nil {
		return err
	}
	out, count, err := h.svc.ListByUser(c.Request().Context(), actor, actor.ID, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(page.NewPageResponse(out, count, req.PageRequest)))
}
