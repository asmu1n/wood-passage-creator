package article

import (
	"context"
	"encoding/json"

	modart "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/pkg/sse"
)

// SubscribeProgress 校验访问权并订阅 task 进度事件（Hub fan-out）。
func (s *Service) SubscribeProgress(
	ctx context.Context,
	taskID string,
	actor moduser.Actor,
) (ch <-chan sse.SSEEvent, cancel func(), err error) {
	art, err := s.loadAccessibleByTaskID(ctx, taskID, actor)
	if err != nil {
		return nil, nil, err
	}
	if s.sse == nil {
		return nil, nil, response.NewBizErrorWithDetail(response.SystemError, "sse unavailable")
	}

	ch, unsub := s.sse.Subscribe(taskID)
	s.publish(taskID, modart.EventConnected, modart.ConnectedPayload{
		Phase:  art.Phase,
		Status: art.Status,
	})
	return ch, unsub, nil
}

func (s *Service) publish(taskID string, name string, data any) {
	if s.sse == nil || taskID == "" || name == "" {
		return
	}
	b, err := json.Marshal(data)
	if err != nil {
		s.log.Error("sse marshal failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.sse.marshal_failed",
			logger.FieldErr, err,
			"task_id", taskID,
			"name", name)
		return
	}
	s.sse.Publish(sse.SSEEvent{
		Topic: taskID,
		Name:  name,
		Data:  b,
	})
}

func (s *Service) publishSSEError(taskID, msg string) {
	s.publish(taskID, modart.EventTaskError, modart.TaskErrorPayload{Message: msg})
}
