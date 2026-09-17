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

// contentGenerator 阶段 3a：按大纲流式生成 Markdown 正文。
type contentGenerator struct {
	llm model.BaseChatModel
	log *slog.Logger
}

func NewContentGenerator(llm model.BaseChatModel) streamingAgent {
	return &contentGenerator{llm: llm, log: logger.Module("article.agent")}
}

func (a *contentGenerator) Name() Name { return NameContentGenerator }

func (a *contentGenerator) Execute(ctx context.Context, state *article.ArticleState, onDelta func(string)) error {
	if err := requireTitle(state); err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	if len(state.Outline) == 0 {
		return fmt.Errorf("%s: state.outline is required", a.Name())
	}

	outlineJSON, err := json.Marshal(state.Outline)
	if err != nil {
		return fmt.Errorf("%s: marshal outline: %w", a.Name(), err)
	}
	p := prompt.Content(*state.MainTitle, *state.SubTitle, string(outlineJSON), state.Style)

	a.log.Info("agent start",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.content.start",
		"task_id", state.TaskID,
	)

	stream, err := a.llm.Stream(ctx, []*schema.Message{schema.UserMessage(p)})
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name(), err)
	}
	text, err := llmkit.CollectTextStream(stream, onDelta)
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
