package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// ImageAgent 统一编排配图规划和生成：先生成配图需求与正文占位符，
// 再将紧凑的需求列表交给 ImageGenerator 执行有界 Tool Calling。
type ImageAgent struct {
	llm       model.BaseChatModel
	generator port.ImageGenerator
	log       *slog.Logger
}

func NewImageAgent(llm model.BaseChatModel, generator port.ImageGenerator) *ImageAgent {
	return &ImageAgent{
		llm:       llm,
		generator: generator,
		log:       logger.Module("article.agent"),
	}
}

func (a *ImageAgent) Name() Name { return NameImageGenerator }

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
	var providers []port.ImageProviderMetadata
	if a.generator != nil {
		providers = a.generator.AvailableProviders(enabled)
	}
	if len(providers) == 0 {
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
		formatAvailableProviders(providers),
		formatProviderUsage(providers),
		providers[0].Method.String(),
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

	var result article.AgentImageAnalyzeSchema
	if err := llmkit.UnmarshalJSON(raw, &result); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	filtered := lo.FilterMap(result.ImageRequirements, func(req port.ImageRequirement, i int) (port.ImageRequirement, bool) {
		src := req.ImageSource.Normalize()
		if !port.Allow(enabled, src) {
			return req, false
		}
		req.ImageSource = src
		return req, true
	})

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

func formatAvailableProviders(providers []port.ImageProviderMetadata) string {
	var b strings.Builder
	for i, provider := range providers {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(provider.Method.String())
		if provider.PlannerDescription != "" {
			b.WriteString(": ")
			b.WriteString(provider.PlannerDescription)
		}
	}
	return b.String()
}

func formatProviderUsage(providers []port.ImageProviderMetadata) string {
	var b strings.Builder
	for i, provider := range providers {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("### ")
		b.WriteString(provider.Method.String())
		b.WriteByte('\n')
		b.WriteString(provider.PlannerUsageGuide)
	}
	return b.String()
}
