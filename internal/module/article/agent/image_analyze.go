package agent

import (
	"context"
	"fmt"
	"strings"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// ImageMethodGuide 描述一种可用配图方式（由上层/配图模块注入，避免 agent 依赖 infra）。
type ImageMethodGuide struct {
	Code        string // 如 PEXELS / NANO_BANANA
	Description string // 简短说明
	UsageGuide  string // 给模型的详细用法
}

// ImageAnalyzer 阶段 3b：分析配图需求并在正文插入占位符。
type ImageAnalyzer struct {
	base
	methods []ImageMethodGuide
}

func NewImageAnalyzer(llm port.ChatModel, methods []ImageMethodGuide) *ImageAnalyzer {
	return &ImageAnalyzer{base: newBase(llm), methods: methods}
}

func (a *ImageAnalyzer) Name() Name { return NameImageAnalyzer }

type imageAnalyzeResult struct {
	ContentWithPlaceholders string                    `json:"contentWithPlaceholders"`
	ImageRequirements       []article.ImageRequirement `json:"imageRequirements"`
}

func (a *ImageAnalyzer) Execute(ctx context.Context, state *article.ArticleState) error {
	if err := requireTitle(state); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if strings.TrimSpace(state.Content) == "" {
		return fmt.Errorf("%s: state.content is required", a.Name())
	}

	enabled := state.EnabledImageMethods
	guides := a.filterMethods(enabled)
	if len(guides) == 0 {
		// 无可用配图方式：跳过，占位正文=原正文
		state.ContentWithPlaceholders = state.Content
		state.ImageRequirements = nil
		a.log.Info("agent skip",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "agent.image_analyze.skip",
			"task_id", state.TaskID,
			"reason", "no enabled image methods",
		)
		return nil
	}

	p := prompt.ImageRequirements(
		state.Title.MainTitle,
		state.Content,
		formatAvailableMethods(guides),
		formatMethodUsage(guides),
	)

	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.image_analyze.start",
		"task_id", state.TaskID,
		"methods", enabled,
	)

	raw, err := a.generate(ctx, p, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	var result imageAnalyzeResult
	if err := unmarshalJSON(raw, &result); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	allowed := map[string]struct{}{}
	for _, g := range guides {
		allowed[g.Code] = struct{}{}
	}
	filtered := make([]article.ImageRequirement, 0, len(result.ImageRequirements))
	for _, req := range result.ImageRequirements {
		if _, ok := allowed[req.ImageSource]; !ok {
			continue
		}
		filtered = append(filtered, req)
	}

	if result.ContentWithPlaceholders != "" {
		state.ContentWithPlaceholders = result.ContentWithPlaceholders
	} else {
		state.ContentWithPlaceholders = state.Content
	}
	state.ImageRequirements = filtered

	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.image_analyze.done",
		"task_id", state.TaskID,
		"requirements", len(filtered),
	)
	return nil
}

func (a *ImageAnalyzer) filterMethods(enabled []string) []ImageMethodGuide {
	if len(a.methods) == 0 {
		return nil
	}
	if len(enabled) == 0 {
		// 未指定则全部可用
		return a.methods
	}
	set := map[string]struct{}{}
	for _, e := range enabled {
		set[e] = struct{}{}
	}
	out := make([]ImageMethodGuide, 0, len(enabled))
	for _, m := range a.methods {
		if _, ok := set[m.Code]; ok {
			out = append(out, m)
		}
	}
	return out
}

func formatAvailableMethods(guides []ImageMethodGuide) string {
	var b strings.Builder
	for i, g := range guides {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(g.Code)
		if g.Description != "" {
			b.WriteString(": ")
			b.WriteString(g.Description)
		}
	}
	return b.String()
}

func formatMethodUsage(guides []ImageMethodGuide) string {
	var b strings.Builder
	for i, g := range guides {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("### ")
		b.WriteString(g.Code)
		b.WriteByte('\n')
		b.WriteString(g.UsageGuide)
	}
	return b.String()
}
