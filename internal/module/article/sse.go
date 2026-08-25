package article

import (
	"context"
	"encoding/json"
	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/port"
)

type SSEEventType string

const (
	EventOutlineDelta SSEEventType = "outline_delta"
	EventOutlineDone  SSEEventType = "outline_done"
	EventContentDelta SSEEventType = "content_delta"
	EventContentDone  SSEEventType = "content_done"
	EventError        SSEEventType = "task_error"
	EventConnected    SSEEventType = "connected"
)

type OutlineDeltaPayload struct {
	Delta string `json:"delta"`
}

type OutlineDonePayload struct {
	Phase   ArticlePhase     `json:"phase"`
	Outline []OutlineSection `json:"outline"`
}

type ErrorPayload struct {
	Message string       `json:"message"`
	Phase   ArticlePhase `json:"phase,omitempty"`
}

type ConnectedPayload struct {
	Phase  ArticlePhase  `json:"phase"`
	Status ArticleStatus `json:"status"`
}

// SubscribeProgress 校验访问权并订阅 task 进度事件（Hub fan-out：同 task 多连接互不踢除）。
// 调用方负责 cancel（通常 defer），并将 ch 中的事件写成 HTTP SSE 帧（event + data）。
// err != nil 时未建立订阅（或已保证无泄漏），cancel 为 nil。
func (s *Service) SubscribeProgress(
	ctx context.Context,
	taskID string,
	actorID int64,
	role user.UserRole,
) (ch <-chan port.SSEEvent, cancel func(), err error) {
	art, err := s.loadAccessibleByTaskID(ctx, taskID, actorID, role)
	if err != nil {
		return nil, nil, err
	}
	if s.sse == nil {
		return nil, nil, response.NewBizErrorWithDetail(response.SystemError, "sse unavailable")
	}

	ch, unsub := s.sse.Subscribe(taskID)
	s.PublishConnected(taskID, art.Phase, art.Status)
	return ch, unsub, nil
}

func (s *Service) publish(taskID string, typ SSEEventType, data any) {
	if s.sse == nil || taskID == "" || typ == "" {
		return
	}

	b, err := json.Marshal(data)

	if err != nil {
		s.log.Error("sse marshal failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.sse.marshal_failed",
			logger.FieldErr, err,
			"task_id", taskID,
			"type", string(typ))
		return
	}

	s.sse.Publish(port.SSEEvent{
		Topic: taskID,
		Name:  string(typ),
		Data:  b,
	})
}

func (s *Service) publishOutlineDelta(taskID string, delta string) {
	s.publish(taskID, EventOutlineDelta, OutlineDeltaPayload{
		Delta: delta,
	})
}

func (s *Service) publishOutlineDone(taskID string, outline []OutlineSection) {
	s.publish(taskID, EventOutlineDone, OutlineDonePayload{
		Phase:   PhaseOutlineEditing,
		Outline: outline,
	})
}

func (s *Service) publishSSEError(taskID, msg string) {
	s.publish(taskID, EventError, ErrorPayload{
		Message: msg,
	})
}

// PublishConnected 在客户端 Subscribe 成功后推送当前任务快照。
func (s *Service) PublishConnected(taskID string, phase ArticlePhase, status ArticleStatus) {
	s.publish(taskID, EventConnected, ConnectedPayload{
		Phase:  phase,
		Status: status,
	})
}
