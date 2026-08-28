package agent

import (
	"context"
	"fmt"
	"log/slog"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// Orchestrator 文章多智能体编排（确定性流水线，非 ReAct）。
type Orchestrator struct {
	log *slog.Logger

	title   agent
	outline agentWithModify
	content agent
	image   agent
	merge   agent
	images  port.ImageGenerator // 可选；nil 则跳过真实出图
}

func NewOrchestrator(llm port.ChatModel, imageGenerator port.ImageGenerator, imageMethods []ImageMethodGuide) article.AgentOrchestrator {
	return &Orchestrator{
		log:     logger.Module("article.orchestrator"),
		title:   NewTitleGenerator(llm),
		outline: NewOutlineGenerator(llm),
		content: NewContentGenerator(llm),
		image:   NewImageAnalyzer(llm, imageMethods),
		merge:   NewContentMerger(),
		images:  imageGenerator,
	}
}

// RunPhase1 生成标题方案 → 写入 state.TitleOptions。
func (o *Orchestrator) RunPhase1(ctx context.Context, state *article.ArticleState) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase1.start",
		"task_id", state.TaskID,
	)
	if err := o.title.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase1: %w", err)
	}
	return nil
}

// RunPhase2 生成大纲；onProgress 由 Service 注入（通常闭包 publish 到 SSE）。
func (o *Orchestrator) RunPhase2(ctx context.Context, state *article.ArticleState, onProgress article.ProgressFunc) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase2.start",
		"task_id", state.TaskID,
	)
	ctx = llmkit.WithStreamHandler(ctx, func(ctx context.Context, delta string) error {
		if delta == "" {
			return nil
		}
		onProgress(ctx, article.EventOutlineDelta, article.OutlineDeltaPayload{Delta: delta})
		return nil
	})
	if err := o.outline.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase2: %w", err)
	}
	return nil
}

// RunPhase3 正文 → 配图分析 → 出图 → 合成；按 SSE 规范通过 onProgress 推送。
func (o *Orchestrator) RunPhase3(ctx context.Context, state *article.ArticleState, onProgress article.ProgressFunc) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase3.start",
		"task_id", state.TaskID,
	)

	// 1) 正文流式
	ctx = llmkit.WithStreamHandler(ctx, func(ctx context.Context, delta string) error {
		if delta == "" {
			return nil
		}
		onProgress(ctx, article.EventContentDelta, article.ContentDeltaPayload{Delta: delta})
		return nil
	})
	if err := o.content.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 content: %w", err)
	}
	onProgress(ctx, article.EventContentGenerated, article.ContentGeneratedPayload{
		Phase:         state.Phase,
		ContentLength: len(state.Content),
	})

	// 2) 配图分析
	if err := o.image.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 image analyze: %w", err)
	}
	onProgress(ctx, article.EventImagesPlanned, article.ImagesPlannedPayload{
		Phase: state.Phase,
		Count: len(state.ImageRequirements),
	})

	// 3) 出图：逐张 image_complete + 整批 images_done
	if o.images != nil && len(state.ImageRequirements) > 0 {
		imgs, err := o.images.Generate(ctx, state.TaskID, state.ImageRequirements,
			func(ctx context.Context, done, total int, img port.ImageResult) {
				onProgress(ctx, article.EventImageComplete, article.ImageCompletePayload{
					Image: img,
					Done:  done,
					Total: total,
				})
			},
		)
		if err != nil {
			return fmt.Errorf("phase3 image generate: %w", err)
		}
		state.Images = imgs
		onProgress(ctx, article.EventImagesDone, article.ImagesDonePayload{
			Phase:  state.Phase,
			Count:  len(imgs),
			Images: imgs,
		})
	} else {
		state.Images = nil
		onProgress(ctx, article.EventImagesDone, article.ImagesDonePayload{
			Phase:  state.Phase,
			Count:  0,
			Images: nil,
		})
	}

	// 4) 合成
	if err := o.merge.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 merge: %w", err)
	}
	onProgress(ctx, article.EventMergeDone, article.MergeDonePayload{
		Phase:             state.Phase,
		FullContentLength: len(state.FullContent),
	})
	return nil
}

// ModifyOutline AI 按建议修改大纲（同步，不走 SSE 流）。
func (o *Orchestrator) ModifyOutline(ctx context.Context, state *article.ArticleState, modifySuggestion string) ([]article.OutlineSection, error) {
	o.log.Info("modify outline",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "modify_outline.start",
		"task_id", state.TaskID,
	)
	if err := o.outline.ExecuteWithModify(ctx, state, modifySuggestion); err != nil {
		return nil, fmt.Errorf("modify outline: %w", err)
	}
	return state.Outline, nil
}
