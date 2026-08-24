package article

import (
	"encoding/json"
	"time"
	"wood-passage-creator/internal/pkg/logger"
)

type SSEEventType string

const (
	EventOutlineDelta SSEEventType = "OUTLINE_DELTA"
	EventOutlineDone  SSEEventType = "OUTLINE_DONE"
	EventError        SSEEventType = "ERROR"
	EventConnected    SSEEventType = "CONNECTED"
)

// sse 事件推送数据结构
type sseEnvelope struct {
	Type    SSEEventType `json:"type"`
	TaskID  string       `json:"taskId"`
	TS      int64        `json:"ts"`
	Payload any          `json:"payload,omitempty"`
}

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

func (s *Service) publish(taskID string, typ SSEEventType, payload any) {
	if s.sse == nil || taskID == "" || typ == "" {
		return
	}

	env := sseEnvelope{
		Type:    typ,
		TaskID:  taskID,
		TS:      time.Now().UnixMilli(),
		Payload: payload,
	}

	b, err := json.Marshal(env)

	if err != nil {
		s.log.Error("sse marshal failed",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "article.sse.marshal_failed",
			logger.FieldErr, err,
			"task_id", taskID,
			"type", string(typ))
		return
	}

	s.sse.Publish(taskID, b)
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

