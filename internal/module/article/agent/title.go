package agent

import (
	"context"
	"fmt"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// titleGenerator 阶段 1：根据选题生成多个标题方案。
type titleGenerator struct {
	base
}

func NewTitleGenerator(llm port.ChatModel) Agent {
	return &titleGenerator{base: newBase(llm)}
}

func (a *titleGenerator) Name() Name { return NameTitleGenerator }

func (a *titleGenerator) Execute(ctx context.Context, state *article.ArticleState) error {
	if state == nil {
		return fmt.Errorf("state is nil")
	}
	if state.Topic == "" {
		return fmt.Errorf("state.topic is required")
	}

	p := prompt.TitleOptions(state.Topic, state.Style)
	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.title.start",
		"task_id", state.TaskID,
		"topic", state.Topic,
	)

	raw, err := a.generate(ctx, p, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	var options []article.TitleOption
	if err := unmarshalJSON(raw, &options); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(options) == 0 {
		return fmt.Errorf("%s: empty title options", a.Name())
	}

	state.TitleOptions = options
	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.title.done",
		"task_id", state.TaskID,
		"options", len(options),
	)
	return nil
}
