package articleworkflow

import (
	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

type EventName string

type GenerationStep string

const (
	StepTitles    GenerationStep = "titles"
	StepOutline   GenerationStep = "outline"
	StepContent   GenerationStep = "content"
	StepImagePlan GenerationStep = "image_plan"
	StepImages    GenerationStep = "images"
	StepMerge     GenerationStep = "merge"
	StepCompleted GenerationStep = "completed"
)

const (
	EventTaskSnapshot     EventName = "task_snapshot"
	EventTitlesDone       EventName = "titles_done"
	EventOutlineSnapshot  EventName = "outline_snapshot"
	EventOutlineDone      EventName = "outline_done"
	EventContentDelta     EventName = "content_delta"
	EventContentGenerated EventName = "content_generated"
	EventImagesPlanned    EventName = "images_planned"
	EventImageFinished    EventName = "image_finished"
	EventImagesDone       EventName = "images_done"
	EventContentDone      EventName = "content_done"
	EventTaskError        EventName = "task_error"
)

// ProgressPayload is closed to this package so event names cannot be paired
// with an unrelated payload. The application adds run metadata after emission.
type ProgressPayload interface {
	eventName() EventName
	terminal() bool
}

// Event is the JSON data of a named SSE frame; Name supplies its event: field.
// Seq is the run's revision; a first snapshot may have revision zero.
type Event[T ProgressPayload] struct {
	RunID string               `json:"runId"`
	Seq   uint64               `json:"seq"`
	Phase article.ArticlePhase `json:"phase"`
	Data  T                    `json:"data"`
}

func (e Event[T]) Name() EventName {
	return e.Data.eventName()
}
func (e Event[T]) IsTerminal() bool {
	return e.Data.terminal()
}

// Missing preview fields have not arrived yet; this is not a validated outline.
type OutlinePreviewSection struct {
	Section *int     `json:"section,omitempty"`
	Title   *string  `json:"title,omitempty"`
	Points  []string `json:"points,omitempty"`
}

// A snapshot replaces the client's projection, including clearing absent fields.
// Terminal also covers a phase waiting for user confirmation, not only completion.
type TaskSnapshot struct {
	Topic         string                      `json:"topic,omitempty"`
	Status        article.ArticleStatus       `json:"status"`
	Terminal      bool                        `json:"terminal"`
	TitleOptions  []article.TitleOption       `json:"titleOptions,omitempty"`
	Outline       []OutlinePreviewSection     `json:"outline,omitempty"`
	Content       string                      `json:"content,omitempty"`
	FullContent   string                      `json:"fullContent,omitempty"`
	Images        []GeneratedImage            `json:"images,omitempty"`
	ImageOutcomes []ImageOutcome              `json:"imageOutcomes,omitempty"`
	ImageSummary  ImageSummary                `json:"imageSummary"`
	Warnings      []article.GenerationWarning `json:"warnings,omitempty"`
	Error         *TaskError                  `json:"error,omitempty"`
	Editorial     *EditorialContext           `json:"editorial,omitempty"`
	Step          GenerationStep              `json:"step,omitempty"`
	// SavedImages preserve the existing persisted format, including historical providers.
	SavedImages []port.ImageResult `json:"savedImages,omitempty"`
}

type TitlesDone TitlesResult
type OutlineDone OutlineResult
type ContentDone ArticleResult

type OutlineSnapshot struct {
	Sections []OutlinePreviewSection `json:"sections"`
}

type ContentDelta struct {
	Delta string `json:"delta"`
}

type ContentGenerated struct {
	Content string `json:"content"`
}

type ImagesPlanned struct {
	Tasks []ImageTask `json:"tasks"`
}

type ImageFinished struct {
	Outcome ImageOutcome `json:"outcome"`
	Summary ImageSummary `json:"summary"`
}

type ImagesDone struct {
	Outcomes []ImageOutcome `json:"outcomes"`
	Summary  ImageSummary   `json:"summary"`
}

// TaskError contains a public error code and safe message, never raw provider errors.
type TaskError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (TaskSnapshot) eventName() EventName {
	return EventTaskSnapshot
}
func (s TaskSnapshot) terminal() bool {
	return s.Terminal
}
func (TitlesDone) eventName() EventName {
	return EventTitlesDone
}
func (TitlesDone) terminal() bool {
	return true
}
func (OutlineSnapshot) eventName() EventName {
	return EventOutlineSnapshot
}
func (OutlineSnapshot) terminal() bool {
	return false
}
func (OutlineDone) eventName() EventName {
	return EventOutlineDone
}
func (OutlineDone) terminal() bool {
	return true
}
func (ContentDelta) eventName() EventName {
	return EventContentDelta
}
func (ContentDelta) terminal() bool {
	return false
}
func (ContentGenerated) eventName() EventName {
	return EventContentGenerated
}
func (ContentGenerated) terminal() bool {
	return false
}
func (ImagesPlanned) eventName() EventName {
	return EventImagesPlanned
}
func (ImagesPlanned) terminal() bool {
	return false
}
func (ImageFinished) eventName() EventName {
	return EventImageFinished
}
func (ImageFinished) terminal() bool {
	return false
}
func (ImagesDone) eventName() EventName {
	return EventImagesDone
}
func (ImagesDone) terminal() bool {
	return false
}
func (ContentDone) eventName() EventName {
	return EventContentDone
}
func (ContentDone) terminal() bool {
	return true
}
func (TaskError) eventName() EventName {
	return EventTaskError
}
func (TaskError) terminal() bool {
	return true
}
