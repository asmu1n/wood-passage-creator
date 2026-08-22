package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// contentGenerator 阶段 3a：按大纲流式生成 Markdown 正文。
type contentGenerator struct {
	base
}

func NewContentGenerator(llm port.ChatModel) Agent {
	return &contentGenerator{base: newBase(llm)}
}

func (a *contentGenerator) Name() Name { return NameContentGenerator }

func (a *contentGenerator) Execute(ctx context.Context, state *article.ArticleState) error {
	if err := requireTitle(state); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if state.Outline == nil || len(state.Outline.Sections) == 0 {
		return fmt.Errorf("%s: state.outline is required", a.Name())
	}

	outlineJSON, err := json.Marshal(state.Outline.Sections)
	if err != nil {
		return fmt.Errorf("%s: marshal outline: %w", a.Name(), err)
	}
	p := prompt.Content(state.Title.MainTitle, state.Title.SubTitle, string(outlineJSON), state.Style)

	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.content.start",
		"task_id", state.TaskID,
	)

	text, err := a.stream(ctx, p, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if text == "" {
		return fmt.Errorf("%s: empty content", a.Name())
	}

	state.Content = text
	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.content.done",
		"task_id", state.TaskID,
		"chars", len(text),
	)
	return nil
}
