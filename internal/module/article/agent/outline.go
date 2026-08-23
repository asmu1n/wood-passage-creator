package agent

import (
	"context"
	"fmt"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// outlineGenerator 阶段 2：根据已选标题生成大纲（可流式）。
type outlineGenerator struct {
	llmkit.Helper
}

func NewOutlineGenerator(llm port.ChatModel) agent {
	return &outlineGenerator{llmkit.NewHelper(llm, "article.agent")}
}

func (a *outlineGenerator) Name() Name { return NameOutlineGenerator }

func (a *outlineGenerator) Execute(ctx context.Context, state *article.ArticleState) error {
	if err := requireTitle(state); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	p := prompt.Outline(
		state.Title.MainTitle,
		state.Title.SubTitle,
		state.UserDescription,
		state.Style,
	)

	a.Log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.start",
		"task_id", state.TaskID,
	)

	raw, err := a.StreamWithContext(ctx, p, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	var outline article.OutlineResult
	if err := llmkit.UnmarshalJSON(raw, &outline); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(outline.Sections) == 0 {
		return fmt.Errorf("%s: empty outline sections", a.Name())
	}

	state.Outline = &outline
	a.Log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.done",
		"task_id", state.TaskID,
		"sections", len(outline.Sections),
	)
	return nil
}
