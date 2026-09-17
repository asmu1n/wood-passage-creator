package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/components/model"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// Orchestrator 文章阶段采用确定性编排；配图执行内部包含有界 tool-calling 循环。
type Orchestrator struct {
	log  *slog.Logger
	logs article.AgentLogRecorder

	title   agent
	outline agentWithModify
	content streamingAgent
	image   imageAgent
	merge   agent
}

// NewOrchestrator(textLLM, jsonLLM, ...). jsonLLM nil → textLLM.
// jsonLLM: title / outline / image plan (json_object). textLLM: content + tools.
func NewOrchestrator(
	textLLM model.BaseChatModel,
	jsonLLM model.BaseChatModel,
	imageGenerator port.ImageGenerator,
	logs article.AgentLogRecorder,
) article.AgentOrchestrator {
	if jsonLLM == nil {
		jsonLLM = textLLM
	}
	return &Orchestrator{
		log:     logger.Module("article.orchestrator"),
		logs:    logs,
		title:   NewTitleGenerator(jsonLLM),
		outline: NewOutlineGenerator(jsonLLM),
		content: NewContentGenerator(textLLM),
		image:   NewImageAgent(jsonLLM, imageGenerator),
		merge:   NewContentMerger(),
	}
}

func emit(ctx context.Context, onProgress article.ProgressFunc, name string, data any) {
	if onProgress == nil {
		return
	}
	onProgress(ctx, name, data)
}

func summaryJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	const max = 4000
	s := string(b)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// trace 在编排步骤外包一层 agent_log（方案 B）。
func (o *Orchestrator) trace(
	state *article.ArticleState,
	agentName string,
	input any,
	run func() error,
	output func() any,
) error {
	start := time.Now()
	err := run()
	end := time.Now()
	dur := int(end.Sub(start).Milliseconds())

	status := article.AgentLogSuccess
	errMsg := ""
	if err != nil {
		status = article.AgentLogFailed
		errMsg = err.Error()
	}
	out := ""
	if output != nil && err == nil {
		out = summaryJSON(output())
	}
	in := ""
	if input != nil {
		in = summaryJSON(input)
	}

	if o.logs != nil && state != nil && state.ArticleID > 0 && state.TaskID != "" {
		o.logs.SaveAsync(article.CreateAgentLogParams{
			ArticleID:    state.ArticleID,
			TaskID:       state.TaskID,
			AgentName:    agentName,
			StartTime:    start,
			EndTime:      end,
			DurationMs:   dur,
			Status:       status,
			ErrorMessage: errMsg,
			InputData:    in,
			OutputData:   out,
		})
	}
	return err
}

// RunPhase1 生成标题方案。
func (o *Orchestrator) RunPhase1(ctx context.Context, state *article.ArticleState) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase1.start",
		"task_id", state.TaskID,
	)
	return o.trace(state, string(NameTitleGenerator),
		map[string]any{"topic": state.Topic, "style": state.Style},
		func() error {
			if err := o.title.Execute(ctx, state); err != nil {
				return fmt.Errorf("phase1: %w", err)
			}
			return nil
		},
		func() any {
			return map[string]any{"options": len(state.TitleOptions)}
		},
	)
}

// RunPhase2 生成大纲。
func (o *Orchestrator) RunPhase2(ctx context.Context, state *article.ArticleState, onProgress article.ProgressFunc) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase2.start",
		"task_id", state.TaskID,
	)
	onDelta := func(delta string) {
		if delta == "" {
			return
		}
		emit(ctx, onProgress, article.EventOutlineDelta, article.OutlineDeltaPayload{Delta: delta})
	}
	return o.trace(state, string(NameOutlineGenerator),
		map[string]any{"mainTitle": state.MainTitle, "subTitle": state.SubTitle},
		func() error {
			if err := o.outline.Execute(ctx, state, onDelta); err != nil {
				return fmt.Errorf("phase2: %w", err)
			}
			return nil
		},
		func() any {
			return map[string]any{"sections": len(state.Outline)}
		},
	)
}

// RunPhase3 正文 → 配图分析 → 出图 → 合成。
func (o *Orchestrator) RunPhase3(ctx context.Context, state *article.ArticleState, onProgress article.ProgressFunc) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase3.start",
		"task_id", state.TaskID,
	)

	// 1) 正文
	onDelta := func(delta string) {
		if delta == "" {
			return
		}
		emit(ctx, onProgress, article.EventContentDelta, article.ContentDeltaPayload{Delta: delta})
	}
	if err := o.trace(state, string(NameContentGenerator),
		map[string]any{"outlineSections": len(state.Outline)},
		func() error {
			if err := o.content.Execute(ctx, state, onDelta); err != nil {
				return fmt.Errorf("phase3 content: %w", err)
			}
			return nil
		},
		func() any {
			return map[string]any{"contentLength": len(state.Content)}
		},
	); err != nil {
		return err
	}
	emit(ctx, onProgress, article.EventContentGenerated, article.ContentGeneratedPayload{
		Phase:         state.Phase,
		ContentLength: len(state.Content),
	})

	// 2) 配图分析 -> 出图
	if err := o.trace(
		state,
		string(NameImageGenerator),
		map[string]any{
			"contentLength": len(state.Content),
		},
		func() error {
			return o.image.Execute(
				ctx,
				state,
				func(requirements []port.ImageRequirement) {
					emit(ctx, onProgress, article.EventImagesPlanned, article.ImagesPlannedPayload{
						Phase: state.Phase,
						Count: len(requirements),
					})
				},
				func(ctx context.Context, done, total int, img port.ImageResult) {
					emit(ctx, onProgress, article.EventImageComplete, article.ImageCompletePayload{
						Image: img,
						Done:  done,
						Total: total,
					})
				},
			)
		},
		func() any {
			return map[string]any{
				"requirements": len(state.ImageRequirements),
				"images":       len(state.Images),
			}
		},
	); err != nil {
		return err
	}
	emit(ctx, onProgress, article.EventImagesDone, article.ImagesDonePayload{
		Phase:  state.Phase,
		Count:  len(state.Images),
		Images: state.Images,
	})

	// 3) 合成
	if err := o.trace(state, string(NameContentMerger),
		map[string]any{"images": len(state.Images)},
		func() error {
			if err := o.merge.Execute(ctx, state); err != nil {
				return fmt.Errorf("phase3 merge: %w", err)
			}
			return nil
		},
		func() any {
			return map[string]any{"fullContentLength": len(state.FullContent)}
		},
	); err != nil {
		return err
	}
	emit(ctx, onProgress, article.EventMergeDone, article.MergeDonePayload{
		Phase:             state.Phase,
		FullContentLength: len(state.FullContent),
	})
	return nil
}

// ModifyOutline AI 按建议修改大纲。
func (o *Orchestrator) ModifyOutline(ctx context.Context, state *article.ArticleState, modifySuggestion string) ([]article.OutlineSection, error) {
	o.log.Info("modify outline",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "modify_outline.start",
		"task_id", state.TaskID,
	)
	err := o.trace(state, "outline_modifier",
		map[string]any{
			"sections":         len(state.Outline),
			"suggestionLength": len(modifySuggestion),
		},
		func() error {
			if err := o.outline.ExecuteWithModify(ctx, state, modifySuggestion); err != nil {
				return fmt.Errorf("modify outline: %w", err)
			}
			return nil
		},
		func() any {
			return map[string]any{"sections": len(state.Outline)}
		},
	)
	if err != nil {
		return nil, err
	}
	return state.Outline, nil
}
