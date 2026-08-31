package user

import (
	"context"
	"errors"
	"time"
	"log/slog"

	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"golang.org/x/crypto/bcrypt"
)

// Service 用户用例。
type Service struct {
	repo Repository
	log  *slog.Logger
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		log:  logger.Module("user"),
	}
}

func (s *Service) CheckAndConsumeQuota(ctx context.Context, id int64) error {
	u, err := s.repo.FindByID(ctx, id)
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

// UpgradeToVIP 将用户设为 VIP 并记录 vip_time；已是 VIP 则幂等返回。
func (s *Service) UpgradeToVIP(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "用户不存在")
	}
	if u.UserRole == RoleVIP {
		return u, nil
	}
	if u.UserRole == RoleAdmin {
		// 管理员无需 VIP；直接返回
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
	s.log.Info("user upgraded to vip",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.vip.upgrade",
		"user_id", id,
	)
	return out, nil
}

// RevokeVIP 取消 VIP，角色回 user 并清空 vip_time（开发/退款用）。
func (s *Service) RevokeVIP(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
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
	s.log.Info("user vip revoked",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "user.vip.revoke",
		"user_id", id,
	)
	return out, nil
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
	return u, nil
}

func (s *Service) Login(ctx context.Context, in LoginRequest) (*User, error) {
	row, err := s.repo.FindByAccount(ctx, in.UserAccount)
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
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	return u, nil
}

func (s *Service) QueryList(ctx context.Context, params page.PageRequest) ([]*User, int, error) {
	users, total, err := s.repo.QueryList(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Service) Update(ctx context.Context, actorID, targetID int64, in UpdateRequest) (*User, error) {
	if actorID != targetID {
		actor, err := s.repo.FindByID(ctx, actorID)
		if err != nil {
			return nil, err
		}
		if actor == nil {
			return nil, response.NewBizError(response.NoAuth)
		}
		if actor.UserRole != "admin" {
			return nil, response.NewBizError(response.NoAuth)
		}
	}

	existing, err := s.repo.FindByID(ctx, targetID)
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

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
