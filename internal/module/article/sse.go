package article

import (
	"wood-passage-creator/internal/port"
)

// SSE event 名（B 范式：event: + data JSON）。
const (
	EventConnected = "connected"
	EventTaskError = "task_error"

	EventTitlesDone = "titles_done"

	EventOutlineDelta = "outline_delta"
	EventOutlineDone  = "outline_done"

	EventContentDelta     = "content_delta"
	EventContentGenerated = "content_generated"
	EventImagesPlanned    = "images_planned"
	EventImageComplete    = "image_complete"
	EventImagesDone       = "images_done"
	EventMergeDone        = "merge_done"
	EventContentDone      = "content_done"
)

// IsTerminalSSEEvent 本段进度流应结束的事件。
func IsTerminalSSEEvent(name string) bool {
	switch name {
	case EventTitlesDone, EventOutlineDone, EventContentDone, EventTaskError:
		return true
	default:
		return false
	}
}

type ConnectedPayload struct {
	Phase  ArticlePhase  `json:"phase"`
	Status ArticleStatus `json:"status"`
}

type TaskErrorPayload struct {
	Message string       `json:"message"`
	Phase   ArticlePhase `json:"phase,omitempty"`
}

type TitlesDonePayload struct {
	Phase        ArticlePhase  `json:"phase"`
	TitleOptions []TitleOption `json:"titleOptions"`
}

type OutlineDeltaPayload struct {
	Delta string `json:"delta"`
}

type OutlineDonePayload struct {
	Phase   ArticlePhase     `json:"phase"`
	Outline []OutlineSection `json:"outline"`
}

type ContentDeltaPayload struct {
	Delta string `json:"delta"`
}

type ContentGeneratedPayload struct {
	Phase         ArticlePhase `json:"phase"`
	ContentLength int          `json:"contentLength"`
}

type ImagesPlannedPayload struct {
	Phase ArticlePhase `json:"phase"`
	Count int          `json:"count"`
}

type ImageCompletePayload struct {
	Image port.ImageResult `json:"image"`
	Done  int              `json:"done"`
	Total int              `json:"total"`
}

type ImagesDonePayload struct {
	Phase  ArticlePhase       `json:"phase"`
	Count  int                `json:"count"`
	Images []port.ImageResult `json:"images"`
}

type MergeDonePayload struct {
	Phase             ArticlePhase `json:"phase"`
	FullContentLength int          `json:"fullContentLength"`
}

type ContentDonePayload struct {
	Phase  ArticlePhase  `json:"phase"`
	Status ArticleStatus `json:"status"`
}
