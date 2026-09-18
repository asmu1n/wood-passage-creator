package llm

import (
	"context"

	"wood-passage-creator/internal/config"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/eino-contrib/jsonschema"
)

// NewChatModel free-text model (Markdown body, tools).
func NewChatModel(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
}

// NewJSONObjectChatModel forces response_format=json_object (root must be object, not array).
func NewJSONObjectChatModel(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
}

// NewJSONSchemaChatModel 使用具体的结构化输出 DTO 构造 JSON Schema 模型。
func NewJSONSchemaChatModel(
	ctx context.Context,
	cfg config.LLMConfig,
	name string,
	description string,
	output any,
) (model.ToolCallingChatModel, error) {
	outputSchema := reflectJSONSchema(output)
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        name,
				Description: description,
				JSONSchema:  outputSchema,
				Strict:      true,
			},
		},
	})
}

// InjectJSONSchemaChatModel 绑定公共模型配置，调用方只需传入输出 DTO 指针。
func InjectJSONSchemaChatModel(ctx context.Context, cfg config.LLMConfig) func(string, string, any) (model.ToolCallingChatModel, error) {
	return func(name, description string, output any) (model.ToolCallingChatModel, error) {
		return NewJSONSchemaChatModel(ctx, cfg, name, description, output)
	}
}

// reflectJSONSchema 将输出 DTO 转换为 OpenAI response_format 接受的 schema。
func reflectJSONSchema(output any) *jsonschema.Schema {
	reflector := &jsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
		ExpandedStruct: true,
	}
	outputSchema := reflector.Reflect(output)
	outputSchema.Version = ""
	return outputSchema
}
