package user

import (
	"context"
	"errors"
	"log/slog"

	"projecttemp/internal/pkg/logger"
	"projecttemp/internal/pkg/response"

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

func (s *Service) Register(ctx context.Context, in RegisterParams) (*User, error) {
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

func (s *Service) Login(ctx context.Context, in LoginParams) (*User, error) {
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

func (s *Service) Update(ctx context.Context, actorID, targetID int64, in UpdateParams) (*User, error) {
	if actorID != targetID {
		return nil, response.NewBizError(response.NoAuth)
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

func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
