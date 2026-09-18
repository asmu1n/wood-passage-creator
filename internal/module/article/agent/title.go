package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
)

type titleGenerator struct {
	llm model.BaseChatModel
	log *slog.Logger
}

func NewTitleGenerator(llm model.BaseChatModel) agent {
	return &titleGenerator{llm: llm, log: logger.Module("article.agent")}
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

	response, err := a.llm.Generate(ctx, []*schema.Message{schema.UserMessage(p)})
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if response == nil {
		return fmt.Errorf("%s: empty model response", a.Name())
	}
	raw := response.Content

	options, err := parseTitleOptions(raw)
	if err != nil {
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

func parseTitleOptions(raw string) ([]article.TitleOption, error) {
	var envelope article.AgentTitleSchema
	if err := llmkit.UnmarshalJSON(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.Options, nil
}
