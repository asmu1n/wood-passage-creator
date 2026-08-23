package llmkit

import (
	"context"
	"fmt"
	"log/slog"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// Helper 公共依赖。
type Helper struct {
	llm port.ChatModel
	Log *slog.Logger
}

func NewHelper(llm port.ChatModel, name string) Helper {
	return Helper{
		llm: llm,
		Log: logger.Module(name),
	}
}

func (h Helper) Generate(ctx context.Context, userPrompt string, opt *port.ChatOptions) (string, error) {
	if h.llm == nil {
		return "", fmt.Errorf("chat model is nil")
	}
	return h.llm.Generate(ctx, []port.Message{
		{Role: port.RoleUser, Content: userPrompt},
	}, opt)
}

func (h Helper) StreamWithContext(ctx context.Context, userPrompt string, opt *port.ChatOptions) (string, error) {
	if h.llm == nil {
		return "", fmt.Errorf("chat model is nil")
	}
	handler := streamHandlerFrom(ctx)
	return h.llm.Stream(ctx, []port.Message{
		{Role: port.RoleUser, Content: userPrompt},
	}, opt, handler)
}

func (h Helper) StreamWithHandler(ctx context.Context, userPrompt string, opt *port.ChatOptions) (string, error) {
	if h.llm == nil {
		return "", fmt.Errorf("chat model is nil")
	}
	handler := streamHandlerFrom(ctx)
	return h.llm.Stream(ctx, []port.Message{
		{Role: port.RoleUser, Content: userPrompt},
	}, opt, handler)
}
