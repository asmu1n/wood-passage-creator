package articleworkflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

type FlowName string

const (
	FlowGenerateTitles  FlowName = "generateTitles"
	FlowGenerateOutline FlowName = "generateOutline"
	FlowReviseOutline   FlowName = "reviseOutline"
	FlowGenerateArticle FlowName = "generateArticle"
)

// RunContext is server-owned execution data, not model-generated prompt input.
// AllowedImageMethods is an explicit grant: nil and empty both enable no tools.
type RunContext struct {
	ArticleID           int64              `json:"articleId"`
	TaskID              string             `json:"taskId"`
	RunID               string             `json:"runId"`
	AllowedImageMethods []port.ImageMethod `json:"allowedImageMethods,omitempty"`
}

type EditorialContext struct {
	MainTitle       string                `json:"mainTitle"`
	SubTitle        string                `json:"subTitle"`
	Style           *article.ArticleStyle `json:"style,omitempty"`
	UserDescription string                `json:"userDescription"`
}

type GenerateTitlesInput struct {
	Run   RunContext            `json:"run"`
	Topic string                `json:"topic"`
	Style *article.ArticleStyle `json:"style,omitempty"`
}

type TitlesResult struct {
	Options []article.TitleOption `json:"options"`
}

type GenerateOutlineInput struct {
	Run       RunContext       `json:"run"`
	Editorial EditorialContext `json:"editorial"`
}

type OutlineResult struct {
	Sections []article.OutlineSection `json:"sections"`
}

type ReviseOutlineInput struct {
	Run              RunContext               `json:"run"`
	Editorial        EditorialContext         `json:"editorial"`
	Sections         []article.OutlineSection `json:"sections"`
	ModifySuggestion string                   `json:"modifySuggestion"`
}

type GenerateArticleInput struct {
	Run       RunContext               `json:"run"`
	Editorial EditorialContext         `json:"editorial"`
	Outline   []article.OutlineSection `json:"outline"`
}

type ArticleResult struct {
	Content     string                      `json:"content"`
	FullContent string                      `json:"fullContent"`
	Images      []GeneratedImage            `json:"images"`
	Warnings    []article.GenerationWarning `json:"warnings"`
}

type EmitProgress func(context.Context, ProgressPayload) error

type PlanningRunner interface {
	GenerateTitles(context.Context, GenerateTitlesInput) (TitlesResult, error)
	GenerateOutline(context.Context, GenerateOutlineInput, EmitProgress) (OutlineResult, error)
	ReviseOutline(context.Context, ReviseOutlineInput) (OutlineResult, error)
}

// Runner is the application boundary; Genkit handles stay inside its implementation.
type Runner interface {
	PlanningRunner
	GenerateArticle(context.Context, GenerateArticleInput, EmitProgress) (ArticleResult, error)
}

func validateCompletedResponse(response *ai.ModelResponse) error {
	if response == nil {
		return status.Errorf(status.ErrInvalidOutput, "model returned no response")
	}
	if response.FinishReason != ai.FinishReasonStop {
		return status.Errorf(status.ErrInvalidOutput, "model did not finish normally (%s)", response.FinishReason)
	}
	return nil
}

type stepRecorder struct {
	logs article.AgentLogRecorder
	log  *slog.Logger
}

func runWorkflowStep[Out any](
	p stepRecorder,
	ctx context.Context,
	run RunContext,
	name string,
	input any,
	summary func(Out) any,
	execute func(context.Context) (Out, error),
) (Out, error) {
	start := time.Now()
	var out Out
	var err error
	if core.FlowNameFromContext(ctx) != "" {
		out, err = genkit.RunWithContext(ctx, name, execute)
	} else {
		out, err = execute(ctx)
	}
	end := time.Now()
	status := article.AgentLogSuccess
	message := ""
	output := ""
	if err != nil {
		status, message = article.AgentLogFailed, err.Error()
	} else {
		output = p.logSummary(ctx, summary(out))
	}
	if p.logs != nil && run.ArticleID > 0 && run.TaskID != "" {
		p.logs.SaveAsync(article.CreateAgentLogParams{
			ArticleID: run.ArticleID, TaskID: run.TaskID, AgentName: name,
			StartTime: start, EndTime: end, DurationMs: int(end.Sub(start).Milliseconds()),
			Status: status, ErrorMessage: message, InputData: p.logSummary(ctx, input), OutputData: output,
		})
	}
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
	}
	p.log.Log(ctx, level, "planning step finished",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.workflow.done",
		"task_id", run.TaskID, "run_id", run.RunID, "step", name,
		"status", status, "duration_ms", end.Sub(start).Milliseconds(), logger.FieldErr, err,
	)
	return out, err
}

func (p stepRecorder) logSummary(ctx context.Context, value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		p.log.ErrorContext(ctx, "planning log summary failed",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "article.workflow.log_summary_failed",
			logger.FieldErr, err,
		)
		return ""
	}
	const limit = 4000
	runes := []rune(string(raw))
	if len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return string(raw)
}
