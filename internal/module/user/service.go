package user

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"golang.org/x/crypto/bcrypt"
)

// Service 用户用例。
type Service struct {
	repo       Repository
	statsCache overviewCacheInvalidator // 可选：用户/VIP 变更时失效统计概览
	log        *slog.Logger
}

// overviewCacheInvalidator 避免 user → statistics 具体类型依赖。
type overviewCacheInvalidator interface {
	InvalidateOverview(ctx context.Context)
}

func NewService(repo Repository, statsCache overviewCacheInvalidator) *Service {
	return &Service{
		repo:       repo,
		statsCache: statsCache,
		log:        logger.Module("user"),
	}
}

func (s *Service) invalidateStatsOverview(ctx context.Context) {
	if s.statsCache == nil {
		return
	}
	s.statsCache.InvalidateOverview(ctx)
}

// GrantVIP 领域授 VIP（支付成功、系统任务等调用；不做 admin 校验）。
// 已是 VIP 幂等返回；admin 账号无需 VIP，原样返回。
func (s *Service) GrantVIP(ctx context.Context, userID int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "用户不存在")
	}
	if u.UserRole == RoleVIP || u.UserRole == RoleAdmin {
		return u, nil
	}
	now := time.Now()
	out, err := s.repo.Update(ctx, u.ID, UpdateRepoParams{
		UserRole: new(RoleVIP),
		VipTime:  &now,
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("user granted vip",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.vip.grant",
		"user_id", userID,
	)
	s.invalidateStatsOverview(ctx)
	return out, nil
}

// AdminUpgradeVIP 管理员人工开通 VIP。
func (s *Service) AdminUpgradeVIP(ctx context.Context, actor Actor, targetID int64) (*User, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, err
	}
	out, err := s.GrantVIP(ctx, targetID)
	if err != nil {
		return nil, err
	}
	s.log.Info("admin upgraded user vip",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.vip.admin_upgrade",
		"user_id", targetID,
		"actor_id", actor.ID,
	)
	return out, nil
}

// AdminRevokeVIP 管理员取消 VIP（开发/人工处理；支付退款也可走此入口并带 actor）。
func (s *Service) AdminRevokeVIP(ctx context.Context, actor Actor, targetID int64) (*User, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "用户不存在")
	}
	if u.UserRole == RoleAdmin {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "不能取消管理员身份")
	}
	if u.UserRole != RoleVIP {
		return u, nil
	}
	out, err := s.repo.Update(ctx, u.ID, UpdateRepoParams{
		UserRole:     new(RoleUser),
		ClearVipTime: true,
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("admin revoked user vip",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.vip.admin_revoke",
		"user_id", targetID,
		"actor_id", actor.ID,
	)
	s.invalidateStatsOverview(ctx)
	return out, nil
}

func (s *Service) CheckAndConsumeQuota(ctx context.Context, id int64) error {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if u == nil {
		return response.NewBizErrorWithDetail(response.ParamsError, "用户不存在")
	}
	if u.Quota <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "配额不足")
	}
	u.Quota--
	_, err = s.repo.Update(ctx, u.ID, UpdateRepoParams{
		Quota: &u.Quota,
	})
	return err
}

func (s *Service) Register(ctx context.Context, in RegisterRequest) (*User, error) {
	exists, err := s.repo.ExistsAccount(ctx, in.UserAccount)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号已存在")
	}

	hash, err := hashPassword(in.UserPassword)
	if err != nil {
		return nil, err
	}

	u, err := s.repo.Create(ctx, CreateRepoParams{
		UserAccount:  in.UserAccount,
		UserPassword: hash,
		UserName:     in.UserName,
		UserAvatar:   in.UserAvatar,
		UserProfile:  in.UserProfile,
		UserRole:     RoleUser,
		Quota:        5,
	})
	if err != nil {
		if errors.Is(err, ErrAccountConflict) {
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

func (s *Service) Login(ctx context.Context, in LoginRequest) (*User, error) {
	row, err := s.repo.GetByAccount(ctx, in.UserAccount)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(row.UserPassword), []byte(in.UserPassword)); err != nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}

	s.log.Info("user login",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "user.login",
		"user_id", row.ID,
		"user_account", row.UserAccount,
	)
	u := row.User
	return &u, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	return u, nil
}

func (s *Service) ListAll(ctx context.Context, actor Actor, params page.PageRequest) ([]*User, int, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, 0, err
	}
	users, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Service) Update(ctx context.Context, actor Actor, targetID int64, in UpdateRequest) (*User, error) {
	if err := actor.RequireSelfOrAdmin(targetID); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, response.NewBizError(response.NotFound)
	}

	repoIn := UpdateRepoParams{
		UserName:    in.UserName,
		UserAvatar:  in.UserAvatar,
		UserProfile: in.UserProfile,
	}
	if in.UserPassword != nil {
		hash, err := hashPassword(*in.UserPassword)
		if err != nil {
			return nil, err
		}
		repoIn.UserPassword = &hash
	}

	u, err := s.repo.Update(ctx, targetID, repoIn)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}

	s.log.Info("user updated",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.updated",
		"user_id", targetID,
	)
	return u, nil
}

func (s *Service) AdminDelete(ctx context.Context, actor Actor, targetID int64) error {
	if err := actor.RequireAdmin(); err != nil {
		return err
	}
	if actor.ID == targetID {
		return response.NewBizErrorWithDetail(response.ParamsError, "不能删除自己")
	}
	u, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if u == nil {
		return response.NewBizError(response.NotFound)
	}
	if u.UserRole == RoleAdmin {
		return response.NewBizErrorWithDetail(response.ParamsError, "不能删除管理员")
	}
	if err := s.repo.Delete(ctx, targetID); err != nil {
		return err
	}
	s.log.Info("admin deleted user",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.admin_delete",
		"user_id", targetID,
		"actor_id", actor.ID,
	)
	s.invalidateStatsOverview(ctx)
	return nil
}

func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
