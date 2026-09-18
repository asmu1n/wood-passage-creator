package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/module/article/prompt"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
)

type outlineGenerator struct {
	llm model.BaseChatModel
	log *slog.Logger
}

func NewOutlineGenerator(llm model.BaseChatModel) agentWithModify {
	return &outlineGenerator{llm: llm, log: logger.Module("article.agent")}
}

func (a *outlineGenerator) Name() Name { return NameOutlineGenerator }

func (a *outlineGenerator) Execute(ctx context.Context, state *article.ArticleState, onDelta func(string)) error {
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

	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.start",
		"task_id", state.TaskID,
	)

	stream, err := a.llm.Stream(ctx, []*schema.Message{schema.UserMessage(p)})
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	raw, err := llmkit.CollectTextStream(stream, onDelta)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	sections, err := parseOutlineSections(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(sections) == 0 {
		return fmt.Errorf("%s: empty outline", a.Name())
	}

	state.Outline = sections
	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.done",
		"task_id", state.TaskID,
		"sections", len(sections),
	)
	return nil
}

func (a *outlineGenerator) ExecuteWithModify(ctx context.Context, state *article.ArticleState, modifySuggestion string) error {
	if err := requireTitle(state); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}

	outlineJSON, err := json.Marshal(article.AgentOutlineSchema{Sections: state.Outline})
	if err != nil {
		return fmt.Errorf("%s: marshal outline: %w", a.Name(), err)
	}

	p := prompt.ModifyOutline(
		*state.MainTitle,
		*state.SubTitle,
		string(outlineJSON),
		modifySuggestion,
	)

	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.modify.start",
		"task_id", state.TaskID,
	)

	response, err := a.llm.Generate(ctx, []*schema.Message{schema.UserMessage(p)})
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if response == nil {
		return fmt.Errorf("%s: empty model response", a.Name())
	}

	sections, err := parseOutlineSections(response.Content)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(sections) == 0 {
		return fmt.Errorf("%s: empty outline", a.Name())
	}

	state.Outline = sections
	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.outline.modify.done",
		"task_id", state.TaskID,
		"sections", len(sections),
	)
	return nil
}

func parseOutlineSections(raw string) ([]article.OutlineSection, error) {
	var envelope article.AgentOutlineSchema
	if err := llmkit.UnmarshalJSON(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.Sections, nil
}
