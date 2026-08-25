package article

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/port"

	"github.com/google/uuid"
)

type Service struct {
	repo         Repository
	userService  *user.Service
	orchestrator AgentOrchestrator
	sse          port.SSEHub
	log          *slog.Logger
}

func NewService(repo Repository, userService *user.Service, orch AgentOrchestrator, sse port.SSEHub) *Service {
	return &Service{
		repo:         repo,
		userService:  userService,
		orchestrator: orch,
		sse:          sse,
		log:          logger.Module("article"),
	}
}

// ensureArticleAccess 校验登录用户是否可访问文章（管理员或作者）。
func (s *Service) ensureArticleAccess(article *Article, actorID int64, role user.UserRole) error {
	if article == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}
	if role == user.RoleAdmin || article.UserID == actorID {
		return nil
	}
	return response.NewBizErrorWithDetail(response.NoAuth, "无权限")
}

// loadAccessibleByTaskID 按 taskID 加载文章并校验访问权。
// 不存在 → NotFound；非管理员且非作者 → NoAuth。
func (s *Service) loadAccessibleByTaskID(ctx context.Context, taskID string, actorID int64, role user.UserRole) (*Article, error) {
	if taskID == "" {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "任务ID不能为空")
	}
	article, err := s.repo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}
	if err := s.ensureArticleAccess(article, actorID, role); err != nil {
		return nil, err
	}
	return article, nil
}

// ConfirmOutline agent 大纲生成完毕后，确认并更新文章大纲。
func (s *Service) ConfirmOutline(ctx context.Context, taskID string, actorID int64, role user.UserRole, outline []OutlineSection) error {
	article, err := s.loadAccessibleByTaskID(ctx, taskID, actorID, role)
	if err != nil {
		return err
	}
	if article.Phase != PhaseOutlineEditing {
		return response.NewBizErrorWithDetail(response.ParamsError, "当前阶段不允许确认大纲")
	}

	if _, err := s.repo.Update(ctx, article.ID, UpdateArticleParams{
		Outline: outline,
	}); err != nil {
		s.log.Error("confirm outline update failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.confirm_outline.failed",
			logger.FieldErr, err,
			"task_id", taskID,
			"user_id", actorID,
		)
		return err
	}

	s.log.Info("outline confirmed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.confirm_outline.ok",
		"task_id", taskID,
		"user_id", actorID,
		"sections", len(outline),
	)

	// TODO: agent async generate content
	return nil
}

// ConfirmTitle agent 标题生成完毕后，确认主/副标题。
func (s *Service) ConfirmTitle(ctx context.Context, actorID int64, role user.UserRole, params ConfirmTitleRequest) error {
	article, err := s.loadAccessibleByTaskID(ctx, params.TaskID, actorID, role)
	if err != nil {
		return err
	}
	if article.Phase != PhaseTitleSelecting {
		return response.NewBizErrorWithDetail(response.Forbidden, "当前阶段不允许确认标题")
	}

	article, err = s.repo.Update(ctx, article.ID, UpdateArticleParams{
		MainTitle:       &params.SelectedMainTitle,
		SubTitle:        &params.SelectedSubTitle,
		UserDescription: params.UserDescription,
	})
	if err != nil {
		s.log.Error("confirm title update failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.confirm_title.failed",
			logger.FieldErr, err,
			"task_id", params.TaskID,
			"user_id", actorID,
		)
		return err
	}

	s.log.Info("title confirmed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.confirm_title.ok",
		"task_id", params.TaskID,
		"user_id", actorID,
	)

	onDelta := func(ctx context.Context, delta string) error {
		if delta == "" {
			return nil
		}
		s.publishOutlineDelta(article.TaskID, delta)
		return nil
	}

	go s.runPhase2Async(article, onDelta)
	return nil
}

// Create 创建文章任务并异步启动 Phase1（生成标题方案）。
func (s *Service) Create(ctx context.Context, actorID int64, params CreateArticleRequest) (string, error) {
	if err := s.userService.CheckAndConsumeQuota(ctx, actorID); err != nil {
		return "", err
	}

	taskID := uuid.NewString()

	if _, err := s.repo.Create(ctx, CreateArticleParams{
		UserID:              actorID,
		TaskID:              taskID,
		Topic:               params.Topic,
		Style:               params.Style,
		EnabledImageMethods: params.EnabledImageMethods,
	}); err != nil {
		s.log.Error("create article failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.create.failed",
			logger.FieldErr, err,
			"user_id", actorID,
			"topic", params.Topic,
		)
		return "", err
	}

	s.log.Info("article task created",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.create.ok",
		"task_id", taskID,
		"user_id", actorID,
		"topic", params.Topic,
	)

	go s.runPhase1Async(taskID, params.Topic, params.Style)

	return taskID, nil
}

func (s *Service) GetByTaskID(ctx context.Context, taskID string, actorID int64, role user.UserRole) (*Article, error) {
	return s.loadAccessibleByTaskID(ctx, taskID, actorID, role)
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

// runPhase1Async 异步执行阶段 1：生成标题方案。
// 成功：TitleOptions + phase=TITLE_SELECTING。
// 失败/panic：status=FAILED + errorMessage；成功路径不会标记失败。
func (s *Service) runPhase1Async(taskID string, topic string, style *ArticleStyle) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			s.failPhase1(ctx, taskID, fmt.Errorf("panic: %v", r))
		}
	}()

	s.log.Info("phase1 started",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase1.start",
		"task_id", taskID,
		"topic", topic,
	)

	if err := s.repo.UpdateStatus(ctx, taskID, StatusProcessing); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update status to processing: %w", err))
		return
	}
	if err := s.repo.UpdatePhase(ctx, taskID, PhaseTitleGenerating); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update phase to title_generating: %w", err))
		return
	}

	state := &ArticleState{
		TaskID: taskID,
		Topic:  topic,
		Style:  style,
		Phase:  PhaseTitleGenerating,
	}

	if err := s.orchestrator.RunPhase1(ctx, state); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("run phase1: %w", err))
		return
	}
	if len(state.TitleOptions) == 0 {
		s.failPhase1(ctx, taskID, fmt.Errorf("run phase1: empty title options"))
		return
	}

	if err := s.repo.UpdateTitleOptions(ctx, taskID, state.TitleOptions); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("save title options: %w", err))
		return
	}
	if err := s.repo.UpdatePhase(ctx, taskID, PhaseTitleSelecting); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update phase to title_selecting: %w", err))
		return
	}

	s.log.Info("phase1 completed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase1.done",
		"task_id", taskID,
		"options", len(state.TitleOptions),
	)
}

// failPhase1 将任务标为失败并写入错误信息；仅用于异步 Phase1 的失败/panic 路径。
func (s *Service) failPhase1(ctx context.Context, taskID string, err error) {
	if err == nil {
		return
	}

	s.log.Error("phase1 failed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase1.failed",
		logger.FieldErr, err,
		"task_id", taskID,
	)

	msg := truncateErr(err, 1000)
	status := StatusFailed
	if _, uerr := s.repo.UpdateByTaskID(ctx, taskID, UpdateArticleParams{
		Status:       &status,
		ErrorMessage: &msg,
	}); uerr != nil {
		s.log.Error("phase1 persist failure state failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase1.fail_persist_error",
			logger.FieldErr, uerr,
			"task_id", taskID,
		)
	}
}

func (s *Service) runPhase2Async(article *Article, onDelta port.StreamHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			s.failPhase2(ctx, article.TaskID, fmt.Errorf("panic: %v", r))
		}
	}()

	if err := s.repo.UpdatePhase(ctx, article.TaskID, PhaseOutlineGenerating); err != nil {
		s.failPhase2(ctx, article.TaskID, fmt.Errorf("update phase to outline_generating: %w", err))
		return
	}

	state := &ArticleState{
		TaskID:    article.TaskID,
		Style:     article.Style,
		Phase:     PhaseOutlineGenerating,
		MainTitle: article.MainTitle,
		SubTitle:  article.SubTitle,
	}

	if article.UserDescription != nil {
		state.UserDescription = *article.UserDescription
	}

	if err := s.orchestrator.RunPhase2(ctx, state, onDelta); err != nil {
		s.failPhase2(ctx, article.TaskID, err)
		return
	}

	if err := s.repo.UpdateOutline(ctx, article.TaskID, state.Outline); err != nil {
		s.failPhase2(ctx, article.TaskID, err)
		return
	}

	if err := s.repo.UpdatePhase(ctx, article.TaskID, PhaseOutlineEditing); err != nil {
		s.failPhase2(ctx, article.TaskID, err)
		return
	}

	s.publishOutlineDone(article.TaskID, state.Outline)

	s.log.Info("phase2 done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase2.done",
		"task_id", article.TaskID,
	)

}

// failPhase1 将任务标为失败并写入错误信息；仅用于异步 Phase2 的失败/panic 路径。
func (s *Service) failPhase2(ctx context.Context, taskID string, err error) {
	if err == nil {
		return
	}

	s.log.Error("phase2 failed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase2.failed",
		logger.FieldErr, err,
		"task_id", taskID,
	)

	msg := truncateErr(err, 1000)
	status := StatusFailed
	if _, uerr := s.repo.UpdateByTaskID(ctx, taskID, UpdateArticleParams{
		Status:       &status,
		ErrorMessage: &msg,
	}); uerr != nil {
		s.log.Error("phase2 persist failure state failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase2.fail_persist_error",
			logger.FieldErr, uerr,
			"task_id", taskID,
		)
	}
	s.publishSSEError(taskID, msg)

}

func truncateErr(err error, max int) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if max <= 0 || len(msg) <= max {
		return msg
	}
	return msg[:max] + "..."
}
