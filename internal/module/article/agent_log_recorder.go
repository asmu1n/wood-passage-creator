package article

import (
	"context"
	"log/slog"

	"wood-passage-creator/internal/pkg/logger"
)

// asyncAgentLogRecorder 异步写入，失败只记 slog。
type asyncAgentLogRecorder struct {
	repo AgentLogRepository
	log  *slog.Logger
}

func NewAgentLogRecorder(repo AgentLogRepository) AgentLogRecorder {
	if repo == nil {
		return nopAgentLogRecorder{}
	}
	return &asyncAgentLogRecorder{
		repo: repo,
		log:  logger.Module("article.agent_log"),
	}
}

func (r *asyncAgentLogRecorder) SaveAsync(params CreateAgentLogParams) {
	if r == nil || r.repo == nil {
		return
	}
	go func() {
		ctx := context.Background()
		if err := r.repo.Create(ctx, params); err != nil {
			r.log.Error("save agent log failed",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "article.agent_log.save_failed",
				logger.FieldErr, err,
				"task_id", params.TaskID,
				"agent_name", params.AgentName,
				"status", string(params.Status),
			)
			return
		}
		r.log.Info("agent log saved",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.agent_log.saved",
			"task_id", params.TaskID,
			"agent_name", params.AgentName,
			"status", string(params.Status),
			"duration_ms", params.DurationMs,
		)
	}()
}

type nopAgentLogRecorder struct{}

func (nopAgentLogRecorder) SaveAsync(CreateAgentLogParams) {}
