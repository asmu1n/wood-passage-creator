package llm

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type chatModel struct {
	cm model.ToolCallingChatModel
}

func NewChatModel(ctx context.Context, cfg config.LLMConfig) (port.ChatModel, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})

	if err != nil {
		return nil, err
	}
	return &chatModel{cm: cm}, nil
}

func (m *chatModel) Generate(ctx context.Context, messages []port.Message, opt *port.ChatOptions) (string, error) {
	opts := make([]model.Option, 0, 1)
	if opt != nil && opt.Temperature != nil {
		opts = append(opts, model.WithTemperature(*opt.Temperature))
	}
	out, err := m.cm.Generate(ctx, toSchema(messages), opts...)
	if err != nil {
		return "", err
	}
	return out.Content, nil
}

func (m *chatModel) Stream(ctx context.Context, messages []port.Message, opt *port.ChatOptions, onDelta port.StreamHandler) (string, error) {
	opts := make([]model.Option, 0, 1)
	if opt != nil && opt.Temperature != nil {
		opts = append(opts, model.WithTemperature(*opt.Temperature))
	}
	sr, err := m.cm.Stream(ctx, toSchema(messages), opts...)
	if err != nil {
		return "", err
	}
	defer sr.Close()

	var b strings.Builder
	for {
		chunk, err := sr.Recv()
		// 先确定错误是否为 EOF，再处理其他错误（EOF 代表流结束）
		if err == io.EOF {
			break
		}
		if err != nil {
			return b.String(), err
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}
		b.WriteString(chunk.Content)
		if onDelta != nil {
			if err := onDelta(ctx, chunk.Content); err != nil {
				return b.String(), err
			}
		}
	}
	return b.String(), nil
}

func toSchema(in []port.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(in))
	for i := range in {
		m := in[i]
		switch m.Role {
		case port.RoleSystem:
			out = append(out, schema.SystemMessage(m.Content))
		case port.RoleAssistant:
			out = append(out, schema.AssistantMessage(m.Content, nil))
		default:
			out = append(out, schema.UserMessage(m.Content))
		}
	}
	return out
}
