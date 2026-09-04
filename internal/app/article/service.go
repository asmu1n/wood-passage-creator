package article

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	module "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/pkg/sse"
	"wood-passage-creator/internal/port"

	"github.com/google/uuid"
)

// UserQuota 文章创建时的配额扣减（由 app/user.Service 实现；须在同一 ctx/事务内调用）。
// RestoreQuota 保留给非事务场景或运维补偿，Create 已改 WithinTx 后不再依赖回滚。
type UserQuota interface {
	CheckAndConsumeQuota(ctx context.Context, id int64) (consumed bool, err error)
	RestoreQuota(ctx context.Context, id int64) error
}

type overviewCacheInvalidator interface {
	InvalidateOverview(ctx context.Context)
}

// Service 文章应用用例（编排 agent 仍在 module/article）。
type Service struct {
	tx           port.TxManager
	repo         module.Repository
	agentLogs    module.AgentLogRepository
	userQuota    UserQuota
	orchestrator module.AgentOrchestrator
	sse          sse.SSEHub
	statsCache   overviewCacheInvalidator
	log          *slog.Logger
}

func NewService(
	tx port.TxManager,
	repo module.Repository,
	agentLogs module.AgentLogRepository,
	userQuota UserQuota,
	orch module.AgentOrchestrator,
	sseHub sse.SSEHub,
	statsCache overviewCacheInvalidator,
) *Service {
	return &Service{
		tx:           tx,
		repo:         repo,
		agentLogs:    agentLogs,
		userQuota:    userQuota,
		orchestrator: orch,
		sse:          sseHub,
		statsCache:   statsCache,
		log:          logger.Module("app.article"),
	}
}

func (s *Service) invalidateStatsOverview(ctx context.Context) {
	if s.statsCache == nil {
		return
	}
	s.statsCache.InvalidateOverview(ctx)
}

// ensureArticleAccess 校验登录用户是否可访问文章（管理员或作者）。
func (s *Service) ensureArticleAccess(article *module.Article, actor moduser.Actor) error {
	if article == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}
	return actor.RequireSelfOrAdmin(article.UserID)
}

// loadAccessibleByTaskID 按 taskID 加载文章并校验访问权。
// 不存在 → NotFound；非管理员且非作者 → NoAuth。
func (s *Service) loadAccessibleByTaskID(ctx context.Context, taskID string, actor moduser.Actor) (*module.Article, error) {
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
	if err := s.ensureArticleAccess(article, actor); err != nil {
		return nil, err
	}
	return article, nil
}

// loadAccessibleByID 按 ID 加载文章并校验访问权。
// 不存在 → NotFound；非管理员且非作者 → NoAuth。
func (s *Service) loadAccessibleByID(ctx context.Context, id int64, actor moduser.Actor) (*module.Article, error) {
	if id == 0 {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "文章ID不能为空")
	}
	article, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "文章不存在")
	}
	if err := s.ensureArticleAccess(article, actor); err != nil {
		return nil, err
	}
	return article, nil
}

// ConfirmOutline agent 大纲生成完毕后，确认并更新文章大纲。
func (s *Service) ConfirmOutline(ctx context.Context, actor moduser.Actor, taskID string, outline []module.OutlineSection) error {
	article, err := s.loadAccessibleByTaskID(ctx, taskID, actor)
	if err != nil {
		return err
	}
	if article.Phase != module.PhaseOutlineEditing {
		return response.NewBizErrorWithDetail(response.ParamsError, "当前阶段不允许确认大纲")
	}

	article, err = s.repo.Update(ctx, article.ID, module.UpdateArticleParams{
		Outline: outline,
	})
	if err != nil {
		s.log.Error("confirm outline update failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.confirm_outline.failed",
			logger.FieldErr, err,
			"task_id", taskID,
			"user_id", actor.ID,
		)
		return err
	}

	s.log.Info("outline confirmed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.confirm_outline.ok",
		"task_id", taskID,
		"user_id", actor.ID,
		"sections", len(outline),
	)

	go s.runPhase3Async(article, actor.Role)

	return nil
}

// ConfirmTitle agent 标题生成完毕后，确认主/副标题。
func (s *Service) ConfirmTitle(ctx context.Context, actor moduser.Actor, params ConfirmTitleRequest) error {
	article, err := s.loadAccessibleByTaskID(ctx, params.TaskID, actor)
	if err != nil {
		return err
	}
	if article.Phase != module.PhaseTitleSelecting {
		return response.NewBizErrorWithDetail(response.Forbidden, "当前阶段不允许确认标题")
	}

	article, err = s.repo.Update(ctx, article.ID, module.UpdateArticleParams{
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
			"user_id", actor.ID,
		)
		return err
	}

	s.log.Info("title confirmed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.confirm_title.ok",
		"task_id", params.TaskID,
		"user_id", actor.ID,
	)

	go s.runPhase2Async(article)
	return nil
}

// Create 创建文章任务并异步启动 Phase1（生成标题方案）。
// 扣配额与插入文章在同一本地事务中；失败整笔回滚，无需 RestoreQuota。
func (s *Service) Create(ctx context.Context, actor moduser.Actor, req CreateArticleRequest) (string, error) {
	taskID := uuid.NewString()

	enabledMethods, err := resolveEnabledImageMethods(req.EnabledImageMethods, actor)
	if err != nil {
		return "", err
	}
	if s.tx == nil {
		return "", response.NewBizErrorWithDetail(response.SystemError, "tx manager unavailable")
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.userQuota.CheckAndConsumeQuota(ctx, actor.ID); err != nil {
			return err
		}
		if _, err := s.repo.Create(ctx, module.CreateArticleParams{
			UserID:              actor.ID,
			TaskID:              taskID,
			Topic:               req.Topic,
			Style:               req.Style,
			EnabledImageMethods: enabledMethods,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.log.Error("create article failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.create.failed",
			logger.FieldErr, err,
			"user_id", actor.ID,
			"task_id", taskID,
			"topic", req.Topic,
		)
		return "", err
	}

	s.log.Info("article task created",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.create.ok",
		"task_id", taskID,
		"user_id", actor.ID,
		"topic", req.Topic,
	)

	// 提交后再失效缓存 / 启动异步，避免长事务与脏缓存
	s.invalidateStatsOverview(ctx)
	go s.runPhase1Async(taskID, req.Topic, req.Style)

	return taskID, nil
}

func (s *Service) GetByTaskID(ctx context.Context, taskID string, actor moduser.Actor) (*module.Article, error) {
	return s.loadAccessibleByTaskID(ctx, taskID, actor)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*module.Article, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, params module.UpdateArticleParams) (*module.Article, error) {
	return s.repo.Update(ctx, id, params)
}

func (s *Service) UpdateByTaskID(ctx context.Context, taskID string, params module.UpdateArticleParams) (*module.Article, error) {
	return s.repo.UpdateByTaskID(ctx, taskID, params)
}

func (s *Service) UpdateStatus(ctx context.Context, taskID string, status module.ArticleStatus) error {
	return s.repo.UpdateStatus(ctx, taskID, status)
}

func (s *Service) UpdatePhase(ctx context.Context, taskID string, phase module.ArticlePhase) error {
	return s.repo.UpdatePhase(ctx, taskID, phase)
}

func (s *Service) UpdateTitleOptions(ctx context.Context, taskID string, titleOptions []module.TitleOption) error {
	return s.repo.UpdateTitleOptions(ctx, taskID, titleOptions)
}

func (s *Service) ListByUser(ctx context.Context, actor moduser.Actor, req QueryArticleRequest) ([]*module.Article, int, error) {
	// 仅查询执行者自己的文章（admin 也走 list/self 时同样只看自己）
	return s.repo.ListByUser(ctx, actor.ID, module.ListArticlesParams{Status: req.Status, PageRequest: req.PageRequest})
}

func (s *Service) ListAll(ctx context.Context, actor moduser.Actor, req QueryArticleRequest) ([]*module.Article, int, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, module.ListArticlesParams{Status: req.Status, PageRequest: req.PageRequest})
}

func (s *Service) Delete(ctx context.Context, actor moduser.Actor, id int64) error {
	if _, err := s.loadAccessibleByID(ctx, id, actor); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateStatsOverview(ctx)
	return nil
}

// runPhase1Async 异步执行阶段 1：生成标题方案。
// 成功：TitleOptions + phase=TITLE_SELECTING。
// 失败/panic：status=FAILED + errorMessage；成功路径不会标记失败。
func (s *Service) runPhase1Async(taskID string, topic string, style *module.ArticleStyle) {
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

	if err := s.repo.UpdateStatus(ctx, taskID, module.StatusProcessing); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update status to processing: %w", err))
		return
	}
	if err := s.repo.UpdatePhase(ctx, taskID, module.PhaseTitleGenerating); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update phase to title_generating: %w", err))
		return
	}

	art, err := s.repo.GetByTaskID(ctx, taskID)
	if err != nil || art == nil {
		if err == nil {
			err = fmt.Errorf("article not found")
		}
		s.failPhase1(ctx, taskID, fmt.Errorf("load article: %w", err))
		return
	}

	state := &module.ArticleState{
		ArticleID: art.ID,
		TaskID:    taskID,
		Topic:     topic,
		Style:     style,
		Phase:     module.PhaseTitleGenerating,
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
	if err := s.repo.UpdatePhase(ctx, taskID, module.PhaseTitleSelecting); err != nil {
		s.failPhase1(ctx, taskID, fmt.Errorf("update phase to title_selecting: %w", err))
		return
	}

	s.publish(taskID, module.EventTitlesDone, module.TitlesDonePayload{
		Phase:        module.PhaseTitleSelecting,
		TitleOptions: state.TitleOptions,
	})

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
	status := module.StatusFailed
	if _, uerr := s.repo.UpdateByTaskID(ctx, taskID, module.UpdateArticleParams{
		Status:       &status,
		ErrorMessage: &msg,
	}); uerr != nil {
		s.log.Error("phase1 persist failure state failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase1.fail_persist_error",
			logger.FieldErr, uerr,
			"task_id", taskID,
		)
	} else {
		s.invalidateStatsOverview(ctx)
	}
	s.publishSSEError(taskID, msg)
}

func (s *Service) runPhase2Async(article *module.Article) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			s.failPhase2(ctx, article.TaskID, fmt.Errorf("panic: %v", r))
		}
	}()

	if err := s.repo.UpdatePhase(ctx, article.TaskID, module.PhaseOutlineGenerating); err != nil {
		s.failPhase2(ctx, article.TaskID, fmt.Errorf("update phase to outline_generating: %w", err))
		return
	}

	state := &module.ArticleState{
		ArticleID: article.ID,
		TaskID:    article.TaskID,
		Style:     article.Style,
		Phase:     module.PhaseOutlineGenerating,
		MainTitle: article.MainTitle,
		SubTitle:  article.SubTitle,
	}

	if article.UserDescription != nil {
		state.UserDescription = *article.UserDescription
	}

	taskID := article.TaskID
	onProgress := func(ctx context.Context, name string, data any) {
		s.publish(taskID, name, data)
	}
	if err := s.orchestrator.RunPhase2(ctx, state, onProgress); err != nil {
		s.failPhase2(ctx, taskID, err)
		return
	}

	if err := s.repo.UpdateOutline(ctx, taskID, state.Outline); err != nil {
		s.failPhase2(ctx, taskID, err)
		return
	}

	if err := s.repo.UpdatePhase(ctx, taskID, module.PhaseOutlineEditing); err != nil {
		s.failPhase2(ctx, taskID, err)
		return
	}

	s.publish(taskID, module.EventOutlineDone, module.OutlineDonePayload{
		Phase:   module.PhaseOutlineEditing,
		Outline: state.Outline,
	})

	s.log.Info("phase2 done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase2.done",
		"task_id", article.TaskID,
	)

}

// failPhase2 将任务标为失败并写入错误信息；仅用于异步 Phase2 的失败/panic 路径。
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
	status := module.StatusFailed
	if _, uerr := s.repo.UpdateByTaskID(ctx, taskID, module.UpdateArticleParams{
		Status:       &status,
		ErrorMessage: &msg,
	}); uerr != nil {
		s.log.Error("phase2 persist failure state failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase2.fail_persist_error",
			logger.FieldErr, uerr,
			"task_id", taskID,
		)
	} else {
		s.invalidateStatsOverview(ctx)
	}
	s.publishSSEError(taskID, msg)
}

func (s *Service) runPhase3Async(article *module.Article, userRole moduser.UserRole) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			s.failPhase3(ctx, article.TaskID, fmt.Errorf("panic: %v", r))
		}
	}()

	if err := s.UpdatePhase(ctx, article.TaskID, module.PhaseContentGenerating); err != nil {
		s.failPhase3(ctx, article.TaskID, err)
		return
	}

	state := &module.ArticleState{
		ArticleID:           article.ID,
		TaskID:              article.TaskID,
		MainTitle:           article.MainTitle,
		SubTitle:            article.SubTitle,
		Style:               article.Style,
		Outline:             article.Outline,
		Phase:               module.PhaseContentGenerating,
		EnabledImageMethods: article.EnabledImageMethods,
	}

	taskID := article.TaskID
	onProgress := func(ctx context.Context, name string, data any) {
		s.publish(taskID, name, data)
	}
	if err := s.orchestrator.RunPhase3(ctx, state, onProgress); err != nil {
		s.failPhase3(ctx, taskID, err)
		return
	}

	if _, err := s.Update(ctx, article.ID, module.UpdateArticleParams{
		Content:       &state.Content,
		FullContent:   &state.FullContent,
		Status:        new(module.StatusCompleted),
		Phase:         new(module.PhaseCompleted),
		Images:        state.Images,
		CompletedTime: new(time.Now()),
	}); err != nil {
		s.log.Error("phase3 persist content failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase3.content_persist_error",
			logger.FieldErr, err,
			"task_id", article.TaskID,
		)
		s.failPhase3(ctx, state.TaskID, err)
		return
	}

	s.publish(taskID, module.EventContentDone, module.ContentDonePayload{
		Phase:  module.PhaseCompleted,
		Status: module.StatusCompleted,
	})

	s.invalidateStatsOverview(ctx)

	s.log.Info("phase3 completed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase3.completed",
		"task_id", article.TaskID,
	)
}

// failPhase3 将任务标为失败并写入错误信息；仅用于异步 Phase3 的失败/panic 路径。
func (s *Service) failPhase3(ctx context.Context, taskID string, err error) {
	if err == nil {
		return
	}

	s.log.Error("phase3 failed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "article.phase3.failed",
		logger.FieldErr, err,
		"task_id", taskID,
	)

	msg := truncateErr(err, 1000)
	status := module.StatusFailed
	if _, uerr := s.repo.UpdateByTaskID(ctx, taskID, module.UpdateArticleParams{
		Status:       &status,
		ErrorMessage: &msg,
	}); uerr != nil {
		s.log.Error("phase3 persist failure state failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.phase3.fail_persist_error",
			logger.FieldErr, uerr,
			"task_id", taskID,
		)
	} else {
		s.invalidateStatsOverview(ctx)
	}
	s.publishSSEError(taskID, msg)
}

func (s *Service) ModifyOutline(ctx context.Context, actor moduser.Actor, req AiModifyOutlineRequest) ([]module.OutlineSection, error) {
	if err := actor.RequireVipOrAdmin(); err != nil {
		return nil, err
	}

	article, err := s.loadAccessibleByTaskID(ctx, req.TaskID, actor)
	if err != nil {
		s.log.Error("get article failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.modify_outline.get_failed",
			logger.FieldErr, err,
			"task_id", req.TaskID,
			"actor_id", actor.ID,
		)
		return nil, err
	}

	if article.Phase != module.PhaseOutlineEditing {
		return nil, response.NewBizErrorWithDetail(response.Forbidden, "当前阶段不允许修改大纲")
	}

	state := &module.ArticleState{
		ArticleID: article.ID,
		TaskID:    article.TaskID,
		MainTitle: article.MainTitle,
		SubTitle:  article.SubTitle,
		Outline:   article.Outline,
	}

	newOutline, err := s.orchestrator.ModifyOutline(ctx, state, req.ModifySuggestion)
	if err != nil {
		s.log.Error("modify outline failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.modify_outline.failed",
			logger.FieldErr, err,
			"task_id", req.TaskID,
			"actor_id", actor.ID,
		)
		return nil, err
	}

	// 写回修改后的大纲
	if _, err := s.repo.Update(ctx, article.ID, module.UpdateArticleParams{Outline: newOutline}); err != nil {
		return nil, err
	}

	return newOutline, nil
}

// GetExecutionLogs 按 taskId 返回 agent 执行日志与汇总（需能访问该任务）。
func (s *Service) GetExecutionLogs(ctx context.Context, actor moduser.Actor, taskID string) (*module.AgentExecutionStats, error) {
	if _, err := s.loadAccessibleByTaskID(ctx, taskID, actor); err != nil {
		return nil, err
	}
	if s.agentLogs == nil {
		return &module.AgentExecutionStats{
			TaskID:         taskID,
			OverallStatus:  "NOT_FOUND",
			AgentDurations: map[string]int{},
			Logs:           []*module.AgentLog{},
		}, nil
	}
	logs, err := s.agentLogs.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return &module.AgentExecutionStats{
			TaskID:         taskID,
			OverallStatus:  "NOT_FOUND",
			AgentDurations: map[string]int{},
			Logs:           []*module.AgentLog{},
		}, nil
	}

	total := 0
	durations := make(map[string]int, len(logs))
	overall := "SUCCESS"
	for _, entry := range logs {
		if entry.DurationMs != nil {
			total += *entry.DurationMs
			durations[entry.AgentName] = *entry.DurationMs
		}
		switch entry.Status {
		case module.AgentLogFailed:
			overall = "FAILED"
		case module.AgentLogRunning:
			if overall != "FAILED" {
				overall = "RUNNING"
			}
		}
	}
	return &module.AgentExecutionStats{
		TaskID:          taskID,
		TotalDurationMs: total,
		AgentCount:      len(logs),
		AgentDurations:  durations,
		OverallStatus:   overall,
		Logs:            logs,
	}, nil
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

// resolveEnabledImageMethods 处理默认值、用户可选枚举与 VIP 权限。
// 空：VIP/Admin → nil（不限制）；普通 → FreeImageMethods。
// 非空：须为 IsUserMethod；普通用户不得含 VIP 项；返回已 Normalize 的列表。
func resolveEnabledImageMethods(methods []port.ImageMethod, actor moduser.Actor) ([]port.ImageMethod, error) {
	vip := actor.Role == moduser.RoleVIP || actor.Role == moduser.RoleAdmin
	if len(methods) == 0 {
		if vip {
			return nil, nil
		}
		out := make([]port.ImageMethod, len(port.FreeImageMethods))
		copy(out, port.FreeImageMethods)
		return out, nil
	}

	out := make([]port.ImageMethod, 0, len(methods))
	for _, m := range methods {
		m = m.Normalize()
		if !m.IsUserMethod() {
			return nil, response.NewBizErrorWithDetail(response.ParamsError, "不支持的配图方式: "+string(m))
		}
		if !vip && m.IsVIPMethod() {
			return nil, response.NewBizErrorWithDetail(response.Forbidden, "高级配图功能（AI 生图、SVG 图表）仅限 VIP 会员使用")
		}
		out = append(out, m)
	}
	return out, nil
}
