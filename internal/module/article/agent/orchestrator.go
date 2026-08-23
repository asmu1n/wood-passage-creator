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

// ImageGenerator 并行/串行拉图能力由上层实现（Pexels/COS 等），agent 包不依赖 infra。
// 入参为 state.ImageRequirements，写回 state.Images。
type ImageGenerator interface {
	Generate(ctx context.Context, state *article.ArticleState) error
}

// Orchestrator 文章多智能体编排（确定性流水线，非 ReAct）。
type Orchestrator struct {
	log *slog.Logger

	title   agent
	outline agent
	content agent
	image   agent
	merge   agent
	images  ImageGenerator // 可选；nil 则跳过真实出图
}

func NewOrchestrator(llm port.ChatModel, imageGenerator ImageGenerator, imageMethods []ImageMethodGuide) article.AgentOrchestrator {
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

// RunPhase2 生成大纲；onDelta 可选，用于 SSE。
func (o *Orchestrator) RunPhase2(ctx context.Context, state *article.ArticleState, onDelta port.StreamHandler) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase2.start",
		"task_id", state.TaskID,
	)
	ctx = llmkit.WithStreamHandler(ctx, onDelta)
	if err := o.outline.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase2: %w", err)
	}
	return nil
}

// RunPhase3 正文 → 配图分析 →（可选）出图 → 图文合成。
func (o *Orchestrator) RunPhase3(ctx context.Context, state *article.ArticleState, onDelta port.StreamHandler) error {
	o.log.Info("phase start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "phase3.start",
		"task_id", state.TaskID,
	)
	ctx = llmkit.WithStreamHandler(ctx, onDelta)

	if err := o.content.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 content: %w", err)
	}
	if err := o.image.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 image analyze: %w", err)
	}
	if o.images != nil && len(state.ImageRequirements) > 0 {
		if err := o.images.Generate(ctx, state); err != nil {
			return fmt.Errorf("phase3 image generate: %w", err)
		}
	}
	if err := o.merge.Execute(ctx, state); err != nil {
		return fmt.Errorf("phase3 merge: %w", err)
	}
	return nil
}
