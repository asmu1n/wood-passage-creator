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

	subTitle := ""
	if state.SubTitle != nil {
		subTitle = *state.SubTitle
	}

	p := prompt.Outline(
		*state.MainTitle,
		subTitle,
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

	// 与 prompt 约定一致：顶层 JSON 数组
	var sections []article.OutlineSection
	if err := llmkit.UnmarshalJSON(raw, &sections); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(sections) == 0 {
		return fmt.Errorf("%s: empty outline", a.Name())
	}

	state.Outline = sections
	a.Log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.done",
		"task_id", state.TaskID,
		"sections", len(sections),
	)
	return nil
}
