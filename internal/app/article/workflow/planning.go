package articleworkflow

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/core/tracing"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
)

type PlanningFlows struct {
	titles  *core.Flow[GenerateTitlesInput, TitlesResult, struct{}]
	outline *core.Flow[GenerateOutlineInput, OutlineResult, OutlineSnapshot]
	revise  *core.Flow[ReviseOutlineInput, OutlineResult, struct{}]
	stepRecorder
}

var _ PlanningRunner = (*PlanningFlows)(nil)

func NewPlanningFlows(g *genkit.Genkit, logs article.AgentLogRecorder) (*PlanningFlows, error) {
	if g == nil {
		return nil, fmt.Errorf("Genkit runtime is required")
	}
	registerPromptSchemas(g)
	titles := genkit.LookupDataPrompt[TitlesPromptInput, *TitlesResult](g, PromptTitles)
	outline := genkit.LookupDataPrompt[OutlinePromptInput, *OutlineResult](g, PromptOutline)
	revise := genkit.LookupDataPrompt[ReviseOutlinePromptInput, *OutlineResult](g, PromptReviseOutline)
	if titles == nil || outline == nil || revise == nil {
		return nil, fmt.Errorf("required planning prompts are missing")
	}
	p := &PlanningFlows{stepRecorder: stepRecorder{logs: logs, log: logger.Module("article.workflow")}}

	p.titles = genkit.DefineFlow(g, string(FlowGenerateTitles),
		func(ctx context.Context, input GenerateTitlesInput) (TitlesResult, error) {
			p.start(ctx, input.Run)
			if err := validateTitlesInput(input); err != nil {
				return TitlesResult{}, status.Errorf(status.ErrInvalidInput, "%w", err)
			}
			result, response, err := titles.Execute(ctx, TitlesPromptInput{Topic: input.Topic, Style: promptStyle(input.Style)})
			if err != nil {
				return TitlesResult{}, err
			}
			if err := validateCompletedResponse(response); err != nil {
				return TitlesResult{}, err
			}
			if result == nil {
				return TitlesResult{}, status.Errorf(status.ErrInvalidOutput, "title generation returned no data")
			}
			if err := validateTitles(*result); err != nil {
				return TitlesResult{}, status.Errorf(status.ErrInvalidOutput, "%w", err)
			}
			return *result, nil
		})

	p.outline = genkit.DefineStreamingFlow(g, string(FlowGenerateOutline),
		func(ctx context.Context, input GenerateOutlineInput, send core.StreamCallback[OutlineSnapshot]) (OutlineResult, error) {
			p.start(ctx, input.Run)
			if err := validateEditorial(input.Editorial); err != nil {
				return OutlineResult{}, status.Errorf(status.ErrInvalidInput, "%w", err)
			}
			for value, err := range outline.ExecuteStream(ctx, OutlinePromptInput{Editorial: promptEditorial(input.Editorial)}) {
				if err != nil {
					return OutlineResult{}, err
				}
				if value.Done {
					if err := validateCompletedResponse(value.Response); err != nil {
						return OutlineResult{}, err
					}
					if value.Output == nil {
						return OutlineResult{}, status.Errorf(status.ErrInvalidOutput, "outline generation returned no data")
					}
					if err := validateOutline(*value.Output); err != nil {
						return OutlineResult{}, status.Errorf(status.ErrInvalidOutput, "%w", err)
					}
					return *value.Output, nil
				}
				if value.Chunk != nil && len(value.Chunk.Sections) > 0 {
					snapshot := outlinePreview(*value.Chunk)
					if err := send(ctx, snapshot); err != nil {
						return OutlineResult{}, err
					}
				}
			}
			return OutlineResult{}, status.Errorf(status.ErrInvalidOutput, "outline stream ended without a final result")
		})

	p.revise = genkit.DefineFlow(g, string(FlowReviseOutline),
		func(ctx context.Context, input ReviseOutlineInput) (OutlineResult, error) {
			p.start(ctx, input.Run)
			if err := validateRevisionInput(input); err != nil {
				return OutlineResult{}, status.Errorf(status.ErrInvalidInput, "%w", err)
			}
			result, response, err := revise.Execute(ctx, ReviseOutlinePromptInput{
				Editorial: promptEditorial(input.Editorial), Sections: input.Sections, ModifySuggestion: input.ModifySuggestion,
			})
			if err != nil {
				return OutlineResult{}, err
			}
			if err := validateCompletedResponse(response); err != nil {
				return OutlineResult{}, err
			}
			if result == nil {
				return OutlineResult{}, status.Errorf(status.ErrInvalidOutput, "outline revision returned no data")
			}
			if err := validateOutline(*result); err != nil {
				return OutlineResult{}, status.Errorf(status.ErrInvalidOutput, "%w", err)
			}
			return *result, nil
		})
	return p, nil
}

func (p *PlanningFlows) start(ctx context.Context, run RunContext) {
	tracing.SetSpanMetadata(ctx, map[string]string{"taskId": run.TaskID, "runId": run.RunID})
	p.log.InfoContext(ctx, "planning flow started",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "article.workflow.start",
		"task_id", run.TaskID, "run_id", run.RunID,
		"trace_id", tracing.SpanTraceInfo(ctx).TraceID,
	)
}

func (p *PlanningFlows) GenerateTitles(ctx context.Context, input GenerateTitlesInput) (TitlesResult, error) {
	return runWorkflowStep(p.stepRecorder, ctx, input.Run, "title_generator",
		map[string]any{"topic": input.Topic, "style": input.Style},
		func(out TitlesResult) any { return map[string]any{"options": len(out.Options)} },
		func(ctx context.Context) (TitlesResult, error) { return p.titles.Run(ctx, input) },
	)
}

func (p *PlanningFlows) GenerateOutline(ctx context.Context, input GenerateOutlineInput, emit EmitProgress) (OutlineResult, error) {
	return runWorkflowStep(p.stepRecorder, ctx, input.Run, "outline_generator",
		map[string]any{"mainTitle": input.Editorial.MainTitle, "subTitle": input.Editorial.SubTitle},
		func(out OutlineResult) any { return map[string]any{"sections": len(out.Sections)} },
		func(ctx context.Context) (OutlineResult, error) {
			return p.outline.Action.Run(ctx, input, func(ctx context.Context, snapshot OutlineSnapshot) error {
				if emit != nil {
					return emit(ctx, snapshot)
				}
				return nil
			})
		},
	)
}

func (p *PlanningFlows) ReviseOutline(ctx context.Context, input ReviseOutlineInput) (OutlineResult, error) {
	return runWorkflowStep(p.stepRecorder, ctx, input.Run, "outline_modifier",
		map[string]any{"sections": len(input.Sections), "suggestionLength": len(input.ModifySuggestion)},
		func(out OutlineResult) any { return map[string]any{"sections": len(out.Sections)} },
		func(ctx context.Context) (OutlineResult, error) { return p.revise.Run(ctx, input) },
	)
}

func outlinePreview(result OutlineResult) OutlineSnapshot {
	sections := make([]OutlinePreviewSection, len(result.Sections))
	for i, section := range result.Sections {
		if section.Section > 0 {
			sections[i].Section = new(section.Section)
		}
		if section.Title != "" {
			sections[i].Title = new(section.Title)
		}
		sections[i].Points = append([]string(nil), section.Points...)
	}
	return OutlineSnapshot{Sections: sections}
}

func validateTitlesInput(input GenerateTitlesInput) error {
	if strings.TrimSpace(input.Topic) == "" {
		return fmt.Errorf("topic is required")
	}
	return validateStyle(input.Style)
}

func validateStyle(style *article.ArticleStyle) error {
	if style == nil || *style == "" {
		return nil
	}
	switch *style {
	case article.StyleTech, article.StyleEmotional, article.StyleEducational, article.StyleHumorous:
		return nil
	default:
		return fmt.Errorf("unknown article style %q", *style)
	}
}

func validateEditorial(input EditorialContext) error {
	if strings.TrimSpace(input.MainTitle) == "" || strings.TrimSpace(input.SubTitle) == "" {
		return fmt.Errorf("main title and subtitle are required")
	}
	return validateStyle(input.Style)
}

func validateRevisionInput(input ReviseOutlineInput) error {
	if err := validateEditorial(input.Editorial); err != nil {
		return err
	}
	if strings.TrimSpace(input.ModifySuggestion) == "" {
		return fmt.Errorf("outline modification suggestion is required")
	}
	return validateOutline(OutlineResult{Sections: input.Sections})
}

func validateTitles(result TitlesResult) error {
	if len(result.Options) < 3 || len(result.Options) > 5 {
		return fmt.Errorf("title generation must return 3-5 options")
	}
	for i, option := range result.Options {
		if !singleLine(option.MainTitle) || !singleLine(option.SubTitle) {
			return fmt.Errorf("title option %d requires nonempty single-line titles", i)
		}
		if utf8.RuneCountInString(option.MainTitle) > 30 {
			return fmt.Errorf("title option %d exceeds 30 characters", i)
		}
	}
	return nil
}

func validateOutline(result OutlineResult) error {
	if len(result.Sections) == 0 {
		return fmt.Errorf("outline must contain at least one section")
	}
	for i, section := range result.Sections {
		if section.Section != i+1 || !singleLine(section.Title) || len(section.Points) == 0 {
			return fmt.Errorf("outline section %d requires sequential numbering, a title and points", i+1)
		}
		for _, point := range section.Points {
			if !singleLine(point) {
				return fmt.Errorf("outline section %d contains an empty or multiline point", i+1)
			}
		}
	}
	return nil
}

func singleLine(text string) bool {
	return strings.TrimSpace(text) != "" && !strings.ContainsAny(text, "\r\n")
}
