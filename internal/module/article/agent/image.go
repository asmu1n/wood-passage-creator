package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// ImageMethodGuide 描述一种可用配图方式，供 ImageAgent 构造规划提示词。
type ImageMethodGuide struct {
	Code        port.ImageMethod // 如 PEXELS / NANO_BANANA
	Description string           // 简短说明
	UsageGuide  string           // 给模型的详细用法
}

var ImageMethodGuides = [6]ImageMethodGuide{
	{Code: port.MethodPexels, Description: "Pexels 免费图库，适合真实照片", UsageGuide: "imageSource=PEXELS；keywords 填英文检索词；无需 prompt。"},
	{Code: port.MethodIconify, Description: "Iconify 开源图标库，适合简洁图标", UsageGuide: "imageSource=ICONIFY；keywords 填图标语义（如 rocket、chart）。"},
	{Code: port.MethodEmojiPack, Description: "网络表情包检索", UsageGuide: "imageSource=EMOJI_PACK；keywords 填中文主题词。"},
	{Code: port.MethodMermaid, Description: "Mermaid 流程图/时序图", UsageGuide: "imageSource=MERMAID；prompt 填完整 mermaid 源码。"},
	{Code: port.MethodSVGDiagram, Description: "LLM 生成 SVG 示意图（VIP）", UsageGuide: "imageSource=SVG_DIAGRAM；prompt 描述示意图内容。"},
	{Code: port.MethodNanoBanana, Description: "AI 生图（VIP）", UsageGuide: "imageSource=NANO_BANANA；prompt 填画面描述。"},
}

// ImageAgent 统一编排配图规划和生成：先生成配图需求与正文占位符，
// 再将紧凑的需求列表交给 ImageGenerator 执行有界 Tool Calling。
type ImageAgent struct {
	methods   []ImageMethodGuide
	llm       model.BaseChatModel
	generator port.ImageGenerator
	log       *slog.Logger
}

func NewImageAgent(llm model.BaseChatModel, generator port.ImageGenerator) *ImageAgent {
	return &ImageAgent{
		methods:   ImageMethodGuides[:],
		llm:       llm,
		generator: generator,
		log:       logger.Module("article.agent"),
	}
}

func (a *ImageAgent) Name() Name { return NameImageGenerator }

type imageAnalyzeResult struct {
	ContentWithPlaceholders string                  `json:"contentWithPlaceholders"`
	ImageRequirements       []port.ImageRequirement `json:"imageRequirements"`
}

func (a *ImageAgent) Execute(
	ctx context.Context,
	state *article.ArticleState,
	onPlanned func([]port.ImageRequirement),
	onImage port.ImageProgressFunc,
) error {
	// 1. 分析正文并生成配图规划。
	if err := a.analyze(ctx, state); err != nil {
		return err
	}

	if onPlanned != nil {
		onPlanned(state.ImageRequirements)
	}

	// 2. 没有配图需求时直接完成
	if len(state.ImageRequirements) == 0 {
		state.Images = nil
		return nil
	}

	// 3. 执行现有 Tool Calling Generator
	images, err := a.generator.Generate(
		ctx,
		state.TaskID,
		state.ImageRequirements,
		state.EnabledImageMethods,
		onImage,
	)
	if err != nil {
		return err
	}

	state.Images = images
	return nil
}

func (a *ImageAgent) analyze(ctx context.Context, state *article.ArticleState) error {
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
		*state.MainTitle,
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

	response, err := a.llm.Generate(ctx, []*schema.Message{schema.UserMessage(p)})
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if response == nil {
		return fmt.Errorf("%s: empty model response", a.Name())
	}
	raw := response.Content

	var result imageAnalyzeResult
	if err := llmkit.UnmarshalJSON(raw, &result); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	filtered := make([]port.ImageRequirement, 0, len(result.ImageRequirements))
	for _, req := range result.ImageRequirements {
		src := req.ImageSource.Normalize()
		if !port.Allow(enabled, src) {
			continue
		}
		req.ImageSource = src
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

func (a *ImageAgent) filterMethods(enabled []port.ImageMethod) []ImageMethodGuide {
	if len(a.methods) == 0 {
		return nil
	}
	if len(enabled) == 0 {
		return a.methods // 空 = 不限制
	}
	out := make([]ImageMethodGuide, 0, len(a.methods))
	for _, g := range a.methods {
		if port.Allow(enabled, g.Code) {
			out = append(out, g)
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
		b.WriteString(g.Code.String())
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
		b.WriteString(g.Code.String())
		b.WriteByte('\n')
		b.WriteString(g.UsageGuide)
	}
	return b.String()
}
