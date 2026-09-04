package user

import (
	"context"
	"log/slog"
	"time"

	module "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"
)

// overviewCacheInvalidator 避免 user → statistics 具体类型依赖。
type overviewCacheInvalidator interface {
	InvalidateOverview(ctx context.Context)
}

// Service 用户应用用例（资料/管理/配额/VIP；注册登录见 app/auth）。
type Service struct {
	repo       module.Repository
	statsCache overviewCacheInvalidator
	log        *slog.Logger
}

func NewService(repo module.Repository, statsCache overviewCacheInvalidator) *Service {
	return &Service{
		repo:       repo,
		statsCache: statsCache,
		log:        logger.Module("app.user"),
	}
}

func (s *Service) invalidateStatsOverview(ctx context.Context) {
	if s.statsCache == nil {
		return
	}
	s.statsCache.InvalidateOverview(ctx)
}

func rolePtr(r module.UserRole) *module.UserRole { return &r }

func (s *Service) GrantVIP(ctx context.Context, userID int64) (*module.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "用户不存在")
	}
	if u.UserRole == module.RoleVIP || u.UserRole == module.RoleAdmin {
		return u, nil
	}
	now := time.Now()
	out, err := s.repo.Update(ctx, u.ID, module.UpdateRepoParams{
		UserRole: rolePtr(module.RoleVIP),
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
func (s *Service) AdminUpgradeVIP(ctx context.Context, actor module.Actor, targetID int64) (*module.User, error) {
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
func (s *Service) AdminRevokeVIP(ctx context.Context, actor module.Actor, targetID int64) (*module.User, error) {
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
	if u.UserRole == module.RoleAdmin {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "不能取消管理员身份")
	}
	if u.UserRole != module.RoleVIP {
		return u, nil
	}
	out, err := s.repo.Update(ctx, u.ID, module.UpdateRepoParams{
		UserRole:     rolePtr(module.RoleUser),
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

// CheckAndConsumeQuota 检查并消耗 1 次配额。
// VIP/Admin 不扣减，返回 consumed=false；普通用户原子扣减成功返回 consumed=true。
func (s *Service) CheckAndConsumeQuota(ctx context.Context, id int64) (consumed bool, err error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	if u == nil {
		return false, response.NewBizErrorWithDetail(response.ParamsError, "用户不存在")
	}
	// VIP / Admin 无限配额
	if u.UserRole == module.RoleVIP || u.UserRole == module.RoleAdmin {
		return false, nil
	}

	n, err := s.repo.DecrementQuota(ctx, id)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, response.NewBizErrorWithDetail(response.ParamsError, "配额不足，无法创建文章")
	}
	return true, nil
}

// RestoreQuota 创建任务失败时退回 1 次配额（仅应在 CheckAndConsumeQuota 返回 consumed=true 时调用）。
// 若用户已不存在则 no-op，避免回滚拖垮主错误返回。
func (s *Service) RestoreQuota(ctx context.Context, id int64) error {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if u == nil {
		return nil
	}
	// VIP/Admin 理论上不应走到这里（consumed=false）
	if u.UserRole == module.RoleVIP || u.UserRole == module.RoleAdmin {
		return nil
	}
	_, err = s.repo.IncrementQuota(ctx, id)
	return err
}

func (s *Service) GetByID(ctx context.Context, id int64) (*module.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	return u, nil
}

func (s *Service) ListAll(ctx context.Context, actor module.Actor, params page.PageRequest) ([]*module.User, int, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, 0, err
	}
	users, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Service) Update(ctx context.Context, actor module.Actor, targetID int64, in UpdateRequest) (*module.User, error) {
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

	repoIn := module.UpdateRepoParams{
		UserName:    in.UserName,
		UserAvatar:  in.UserAvatar,
		UserProfile: in.UserProfile,
	}
	if in.UserPassword != nil {
		hash, err := module.HashPassword(*in.UserPassword)
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

func (s *Service) AdminDelete(ctx context.Context, actor module.Actor, targetID int64) error {
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
	if u.UserRole == module.RoleAdmin {
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
