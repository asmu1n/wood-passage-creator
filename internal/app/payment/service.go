package payment

import (
	"context"
	"fmt"
	"log/slog"

	module "wood-passage-creator/internal/module/payment"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/port"

	"github.com/google/uuid"
)

// UserVIP 升级 VIP 的最小依赖。
type UserVIP interface {
	GetByID(ctx context.Context, id int64) (*moduser.User, error)
	GrantVIP(ctx context.Context, userID int64) (*moduser.User, error)
}

// Service 开发态 mock 支付 + 与 VIP 升级打通。
type Service struct {
	tx    port.TxManager
	repo  module.Repository
	users UserVIP
	log   *slog.Logger
}

func NewService(tx port.TxManager, repo module.Repository, users UserVIP) *Service {
	return &Service{
		tx:    tx,
		repo:  repo,
		users: users,
		log:   logger.Module("app.payment"),
	}
}

func (s *Service) CreateMockVIPSession(ctx context.Context, userID int64) (*module.MockSessionResult, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	if u.UserRole == moduser.RoleVIP || u.UserRole == moduser.RoleAdmin {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "当前账号已是 VIP 或管理员，无需购买")
	}

	sessionID := "mock_cs_" + uuid.NewString()
	desc := "Mock VIP permanent (dev only)"
	rec, err := s.repo.CreatePending(ctx, userID, sessionID, module.ProductVIPPermanent, module.MockVIPCurrency, module.MockVIPAmount, desc)
	if err != nil {
		return nil, err
	}

	s.log.Info("mock vip session created",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "payment.mock.session_created",
		"user_id", userID,
		"session_id", sessionID,
	)

	return &module.MockSessionResult{
		SessionID:   rec.StripeSessionID,
		CheckoutURL: fmt.Sprintf("https://mock.local/checkout/%s", rec.StripeSessionID),
		Amount:      rec.Amount,
		Currency:    rec.Currency,
		ProductType: rec.ProductType,
		Status:      rec.Status,
	}, nil
}

// CompleteMockVIP 模拟支付成功：标记记录 SUCCEEDED 并开通 VIP（幂等）。
// PENDING→SUCCEEDED 与 GrantVIP 在同一本地事务中。
func (s *Service) CompleteMockVIP(ctx context.Context, actor moduser.Actor, sessionID string) (*module.MockCompleteResult, error) {
	rec, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "支付会话不存在")
	}
	if err := actor.RequireSelfOrAdmin(rec.UserID); err != nil {
		return nil, err
	}
	if rec.ProductType != module.ProductVIPPermanent {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "不支持的产品类型")
	}

	// 已成功：只保证用户是 VIP（单写，不必与 MarkSucceeded 同事务）
	if rec.Status == module.StatusSucceeded {
		u, err := s.users.GrantVIP(ctx, rec.UserID)
		if err != nil {
			return nil, err
		}
		return &module.MockCompleteResult{
			Record: rec,
			UserID: rec.UserID,
			IsVIP:  u != nil && (u.UserRole == moduser.RoleVIP || u.UserRole == moduser.RoleAdmin),
		}, nil
	}
	if rec.Status != module.StatusPending {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "支付状态不允许完成: "+string(rec.Status))
	}
	if s.tx == nil {
		return nil, response.NewBizErrorWithDetail(response.SystemError, "tx manager unavailable")
	}

	intentID := "mock_pi_" + uuid.NewString()
	var updated *module.Record
	var u *moduser.User
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		updated, err = s.repo.MarkSucceeded(ctx, rec.ID, intentID)
		if err != nil {
			return err
		}
		u, err = s.users.GrantVIP(ctx, rec.UserID)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.log.Info("mock vip payment completed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "payment.mock.completed",
		"user_id", rec.UserID,
		"session_id", sessionID,
	)

	return &module.MockCompleteResult{
		Record: updated,
		UserID: rec.UserID,
		IsVIP:  u != nil && (u.UserRole == moduser.RoleVIP || u.UserRole == moduser.RoleAdmin),
	}, nil
}

func (s *Service) ListAll(ctx context.Context, actor moduser.Actor, req ListRequest) ([]*module.Record, int, error) {
	err := actor.RequireAdmin()
	if err != nil {
		return nil, 0, err
	}
	items, total, err := s.repo.List(ctx, module.ListParams{
		Status:      req.Status,
		UserID:      req.UserID,
		ProductType: req.ProductType,
		PageRequest: req.PageRequest,
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) ListByUser(ctx context.Context, actor moduser.Actor, userID int64, req ListByUserRequest) ([]*module.Record, int, error) {
	err := actor.RequireSelfOrAdmin(userID)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.List(ctx, module.ListParams{
		UserID:      &userID,
		Status:      req.Status,
		ProductType: req.ProductType,
		PageRequest: req.PageRequest,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
