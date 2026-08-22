package article

import (
	"context"
	"log/slog"
	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"

	"github.com/google/uuid"
)

type Service struct {
	repo        Repository
	userService user.Service
	log         *slog.Logger
}

func NewService(repo Repository, userService user.Service) *Service {
	return &Service{
		repo:        repo,
		userService: userService,
		log:         logger.Module("article"),
	}
}

// agent 大纲生成完毕，确认更新文章大纲
func (s *Service) ConfirmOutline(ctx context.Context, taskID string, actorID int64, isAdmin bool, outline []OutlineSection) error {
	article, err := s.repo.GetByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	if article == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}

	if !isAdmin && article.UserID != actorID {
		return response.NewBizErrorWithDetail(response.NoAuth, "无权限")
	}

	if article.Phase != PhaseOutlineEditing {
		return response.NewBizErrorWithDetail(response.ParamsError, "当前阶段不允许确认大纲")
	}

	if _, err := s.repo.Update(ctx, article.ID, UpdateArticleParams{
		Outline: outline,
	}); err != nil {
		return err
	}

	// TODO: agent async generate content
	return nil
}

// agent 标题生成完毕，确认更新文章标题和副标题
func (s *Service) ConfirmTitle(ctx context.Context, actorID int64, isAdmin bool, params ConfirmTitleRequest) error {
	article, err := s.repo.GetByTaskID(ctx, params.TaskID)
	if err != nil {
		return err
	}
	if article == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}
	if !isAdmin && article.UserID != actorID {
		return response.NewBizErrorWithDetail(response.NoAuth, "无权限")
	}
	if article.Phase != PhaseTitleSelecting {
		return response.NewBizErrorWithDetail(response.Forbidden, "当前阶段不允许确认标题")
	}
	if _, err = s.repo.Update(ctx, article.ID, UpdateArticleParams{
		MainTitle:       &params.SelectedMainTitle,
		SubTitle:        &params.SelectedSubTitle,
		UserDescription: params.UserDescription,
	}); err != nil {
		return err
	}

	// TODO: agent async generate content
	return nil
}

// 创建文章任务
func (s *Service) Create(ctx context.Context, params CreateArticleRequest) (*Article, error) {

	// TODO 检查用户配额-> 写入基础文章任务状态 -> 开始执行异步生成任务

	// 生成任务 ID
	taskID := uuid.NewString()

	return s.repo.Create(ctx, CreateArticleParams{
		TaskID:              taskID,
		Topic:               params.Topic,
		Style:               params.Style,
		EnabledImageMethods: params.EnabledImageMethods,
	})
}

func (s *Service) GetByTaskID(ctx context.Context, taskID string, actorID int64, isAdmin bool) (*Article, error) {
	article, err := s.repo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if article == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}

	if !isAdmin && article.UserID != actorID {
		return nil, response.NewBizErrorWithDetail(response.NoAuth, "无权限")
	}

	return article, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*Article, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, params UpdateArticleParams) (*Article, error) {
	return s.repo.Update(ctx, id, params)
}

func (s *Service) UpdateByTaskID(ctx context.Context, taskID string, params UpdateArticleParams) (*Article, error) {
	return s.repo.UpdateByTaskID(ctx, taskID, params)
}

func (s *Service) UpdateStatus(ctx context.Context, taskID string, status ArticleStatus) error {
	return s.repo.UpdateStatus(ctx, taskID, status)
}

func (s *Service) UpdatePhase(ctx context.Context, taskID string, phase ArticlePhase) error {
	return s.repo.UpdatePhase(ctx, taskID, phase)
}

func (s *Service) UpdateTitleOptions(ctx context.Context, taskID string, titleOptions []TitleOption) error {
	return s.repo.UpdateTitleOptions(ctx, taskID, titleOptions)
}

func (s *Service) ListByUser(ctx context.Context, userID int64, params page.PageRequest) ([]*Article, int, error) {
	return s.repo.ListByUser(ctx, userID, params)
}

func (s *Service) ListAll(ctx context.Context, params page.PageRequest) ([]*Article, int, error) {
	return s.repo.ListAll(ctx, params)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
