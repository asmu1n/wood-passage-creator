package http

import (
	"net/http"

	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/payment"
	"wood-passage-creator/internal/pkg/response"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	svc *payment.Service
}

func NewHandler(svc *payment.Service) *Handler {
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
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.CreateMockVIPSession(c.Request().Context(), uid)
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
	var req payment.MockCompleteRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	role, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	out, err := h.svc.CompleteMockVIP(c.Request().Context(), uid, role, req.SessionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(out))
}
