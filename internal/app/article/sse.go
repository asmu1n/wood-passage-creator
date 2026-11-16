package articleapp

import (
	"context"
	"errors"

	"wood-passage-creator/internal/app/article/workflow"
	module "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/pkg/sse"
)

func (s *Service) SubscribeProgress(ctx context.Context, taskID string, actor moduser.Actor) (<-chan sse.SSEEvent, func(), error) {
	if _, err := s.loadAccessibleByTaskID(ctx, taskID, actor); err != nil {
		return nil, nil, err
	}
	if s.progress == nil || s.progress.hub == nil {
		return nil, nil, response.NewBizErrorWithDetail(response.SystemError, "sse unavailable")
	}
	return s.progress.subscribe(ctx, taskID, func(ctx context.Context) (*module.Article, error) {
		return s.loadAccessibleByTaskID(ctx, taskID, actor)
	})
}

func publicGenerationError(ctx context.Context, err error) articleworkflow.TaskError {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return articleworkflow.TaskError{Code: "GENERATION_TIMEOUT", Message: "文章生成超时，请稍后重新创建"}
	}
	return articleworkflow.TaskError{Code: "GENERATION_FAILED", Message: "文章生成失败，请稍后重新创建"}
}

func (s *Service) publishSSEError(taskID string, phase module.ArticlePhase, payload articleworkflow.TaskError) {
	if err := s.progress.emit(taskID, phase, payload); err != nil {
		s.log.Error("publish progress failure", logger.FieldEvent, "article.progress.failed", logger.FieldErr, err, "task_id", taskID)
	}
}
