package auth

import (
	"context"
	"errors"
	"log/slog"

	module "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
)

type overviewCacheInvalidator interface {
	InvalidateOverview(ctx context.Context)
}

// Service 认证用例：注册 / 登录。
type Service struct {
	repo       module.Repository
	statsCache overviewCacheInvalidator
	log        *slog.Logger
}

func NewService(repo module.Repository, statsCache overviewCacheInvalidator) *Service {
	return &Service{
		repo:       repo,
		statsCache: statsCache,
		log:        logger.Module("app.auth"),
	}
}

func (s *Service) invalidateStatsOverview(ctx context.Context) {
	if s.statsCache == nil {
		return
	}
	s.statsCache.InvalidateOverview(ctx)
}

func (s *Service) Register(ctx context.Context, in RegisterRequest) (*module.User, error) {
	exists, err := s.repo.ExistsAccount(ctx, in.UserAccount)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号已存在")
	}

	hash, err := module.HashPassword(in.UserPassword)
	if err != nil {
		return nil, err
	}

	u, err := s.repo.Create(ctx, module.CreateRepoParams{
		UserAccount:  in.UserAccount,
		UserPassword: hash,
		UserName:     in.UserName,
		UserAvatar:   in.UserAvatar,
		UserProfile:  in.UserProfile,
		UserRole:     module.RoleUser,
		Quota:        5,
	})
	if err != nil {
		if errors.Is(err, module.ErrAccountConflict) {
			return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号已存在")
		}
		return nil, err
	}

	s.log.Info("user registered",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.registered",
		"user_id", u.ID,
		"user_account", u.UserAccount,
	)
	s.invalidateStatsOverview(ctx)
	return u, nil
}

func (s *Service) Login(ctx context.Context, in LoginRequest) (*module.User, error) {
	row, err := s.repo.GetByAccount(ctx, in.UserAccount)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}
	if err := module.CheckPassword(row.UserPassword, in.UserPassword); err != nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}

	s.log.Info("user login",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "user.login",
		"user_id", row.ID,
		"user_account", row.UserAccount,
	)
	return &row.User, nil
}

func (s *Service) Me(ctx context.Context, actorID int64) (*module.User, error) {
	u, err := s.repo.GetByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "用户不存在")
	}
	return u, nil
}
