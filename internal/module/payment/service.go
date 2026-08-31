package payment

import (
	"context"
	"fmt"
	"log/slog"

	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"

	"github.com/google/uuid"
)

// UserVIP 升级 VIP 的最小依赖（避免 payment → user 具体方法膨胀时难测）。
type UserVIP interface {
	GetByID(ctx context.Context, id int64) (*user.User, error)
	UpgradeToVIP(ctx context.Context, id int64) (*user.User, error)
}

// Service 开发态 mock 支付 + 与 VIP 升级打通。
type Service struct {
	repo  Repository
	users UserVIP
	log   *slog.Logger
}

func NewService(repo Repository, users UserVIP) *Service {
	return &Service{
		repo:  repo,
		users: users,
		log:   logger.Module("payment"),
	}
}

// CreateMockVIPSession 为当前用户创建一笔 PENDING 的 VIP 支付记录（不连 Stripe）。
func (s *Service) CreateMockVIPSession(ctx context.Context, userID int64) (*MockSessionResult, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	if u.UserRole == user.RoleVIP || u.UserRole == user.RoleAdmin {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "当前账号已是 VIP 或管理员，无需购买")
	}

	sessionID := "mock_cs_" + uuid.NewString()
	desc := "Mock VIP permanent (dev only)"
	rec, err := s.repo.CreatePending(ctx, userID, sessionID, ProductVIPPermanent, MockVIPCurrency, MockVIPAmount, desc)
	if err != nil {
		return nil, err
	}

	s.log.Info("mock vip session created",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "payment.mock.session_created",
		"user_id", userID,
		"session_id", sessionID,
	)

	return &MockSessionResult{
		SessionID:   rec.StripeSessionID,
		CheckoutURL: fmt.Sprintf("https://mock.local/checkout/%s", rec.StripeSessionID),
		Amount:      rec.Amount,
		Currency:    rec.Currency,
		ProductType: rec.ProductType,
		Status:      rec.Status,
	}, nil
}

// CompleteMockVIP 模拟支付成功：标记记录 SUCCEEDED 并 UpgradeToVIP（幂等）。
func (s *Service) CompleteMockVIP(ctx context.Context, actorID int64, actorRole user.UserRole, sessionID string) (*MockCompleteResult, error) {
	rec, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "支付会话不存在")
	}
	// 仅本人或管理员可 complete
	if actorRole != user.RoleAdmin && rec.UserID != actorID {
		return nil, response.NewBizError(response.NoAuth)
	}
	if rec.ProductType != ProductVIPPermanent {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "不支持的产品类型")
	}

	// 已成功：只保证用户是 VIP，再返回
	if rec.Status == StatusSucceeded {
		u, err := s.users.UpgradeToVIP(ctx, rec.UserID)
		if err != nil {
			return nil, err
		}
		return &MockCompleteResult{
			Record: rec,
			UserID: rec.UserID,
			IsVIP:  u != nil && (u.UserRole == user.RoleVIP || u.UserRole == user.RoleAdmin),
		}, nil
	}
	if rec.Status != StatusPending {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "支付状态不允许完成: "+rec.Status)
	}

	intentID := "mock_pi_" + uuid.NewString()
	updated, err := s.repo.MarkSucceeded(ctx, rec.ID, intentID)
	if err != nil {
		return nil, err
	}
	u, err := s.users.UpgradeToVIP(ctx, rec.UserID)
	if err != nil {
		return nil, err
	}

	s.log.Info("mock vip payment completed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "payment.mock.completed",
		"user_id", rec.UserID,
		"session_id", sessionID,
	)

	return &MockCompleteResult{
		Record: updated,
		UserID: rec.UserID,
		IsVIP:  u != nil && (u.UserRole == user.RoleVIP || u.UserRole == user.RoleAdmin),
	}, nil
}
