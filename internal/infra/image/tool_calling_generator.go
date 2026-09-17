package image

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const maxImageToolRounds = 6

// ToolCallingGenerator 让模型通过原生 tool calling 选择并执行图片 Provider。
// ProviderExecutor 是确定性的工具执行层；本类型只负责对话循环、权限边界与结果收敛。
type ToolCallingGenerator struct {
	model model.ToolCallingChatModel
	tools *ProviderExecutor
}

func NewToolCallingGenerator(
	cfg *config.Config,
	toolModel model.ToolCallingChatModel,
	store port.ObjectStore,
) port.ImageGenerator {
	return &ToolCallingGenerator{
		model: toolModel,
		tools: NewProviderExecutor(cfg, toolModel, store),
	}
}

type imageToolArgs struct {
	RequirementIndex int    `json:"requirementIndex"`
	Keywords         string `json:"keywords"`
	Prompt           string `json:"prompt"`
}

type imageToolFeedback struct {
	OK               bool             `json:"ok"`
	Generated        bool             `json:"generated,omitempty"`
	RequirementIndex int              `json:"requirementIndex"`
	Position         int              `json:"position,omitempty"`
	Method           port.ImageMethod `json:"method,omitempty"`
	Message          string           `json:"message"`
}

func (g *ToolCallingGenerator) Generate(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	allowedMethods []port.ImageMethod,
	onProgress port.ImageProgressFunc,
) ([]port.ImageResult, error) {
	if g == nil || g.model == nil || g.tools == nil {
		return nil, fmt.Errorf("tool calling image generator is not initialized")
	}

	if len(reqs) == 0 {
		return nil, nil
	}

	// 构建可用的 image tools
	tools, toolMethods := g.buildTools(allowedMethods)
	if len(tools) == 0 {
		return nil, fmt.Errorf("no permitted image tools are available")
	}

	requirementsJSON, err := json.Marshal(reqs)
	if err != nil {
		return nil, fmt.Errorf("marshal image requirements: %w", err)
	}
	messages := []*schema.Message{
		schema.SystemMessage(imageToolSystemPrompt),
		schema.UserMessage(fmt.Sprintf(
			"请为下面 JSON 数组中的每一项生成一张图片。数组下标就是 requirementIndex。优先使用每项的 imageSource；工具失败时可改参数或换用其他可用工具。\n%s",
			string(requirementsJSON),
		)),
	}

	results := make(map[int]port.ImageResult, len(reqs))
	maxCalls := len(reqs) * 3
	if maxCalls < 4 {
		maxCalls = 4
	}
	if maxCalls > 24 {
		maxCalls = 24
	}
	callCount := 0

	toolModel, err := g.model.WithTools(tools)
	if err != nil {
		return nil, fmt.Errorf("bind image tools: %w", err)
	}
	for round := 0; round < maxImageToolRounds; round++ {
		response, callErr := toolModel.Generate(ctx, messages)
		if callErr != nil {
			if len(results) == len(reqs) {
				return sortedImageResults(results), nil
			}
			g.tools.log.Warn("image tool model call failed; falling back",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.model_failed",
				logger.FieldErr, callErr,
				"task_id", taskID,
			)
			break
		}
		if response == nil {
			g.tools.log.Warn("image tool model returned nil response; falling back",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.empty_response",
				"task_id", taskID,
			)
			break
		}
		if len(response.ToolCalls) == 0 {
			messages = append(messages, response)
			if len(results) == len(reqs) {
				return sortedImageResults(results), nil
			}
			messages = append(messages, schema.UserMessage(
				fmt.Sprintf("还有 %d 项没有生成。请继续调用工具，不要只返回文字。", len(reqs)-len(results)),
			))
			continue
		}
		for i := range response.ToolCalls {
			if response.ToolCalls[i].ID == "" {
				response.ToolCalls[i].ID = fmt.Sprintf("image-call-%d-%d", round, i)
			}
		}
		messages = append(messages, response)

		// 开始执行 LLM 输出的 ToolCalls 指令
		for _, call := range response.ToolCalls {
			if callCount >= maxCalls {
				break
			}
			callCount++
			// 执行工具调用
			feedback := g.executeToolCall(ctx, taskID, reqs, results, toolMethods, call)
			payload, marshalErr := json.Marshal(feedback)
			if marshalErr != nil {
				payload = []byte(`{"ok":false,"message":"encode tool result failed"}`)
			}
			messages = append(messages, schema.ToolMessage(
				string(payload),
				call.ID,
				schema.WithToolName(call.Function.Name),
			))

			// 如果生成成功，回调进度
			if feedback.Generated {
				if result, ok := results[feedback.RequirementIndex]; ok && onProgress != nil {
					onProgress(ctx, len(results), len(reqs), result)
				}
			}
		}

		if callCount >= maxCalls {
			break
		}
	}

	// 模型未完成所有项时，仅对缺失项使用原有确定性 fallback，避免整篇文章失败。
	for i, req := range reqs {
		if _, ok := results[i]; ok {
			continue
		}
		result, fetchErr := g.tools.ExecuteWithFallback(ctx, taskID, req)
		if fetchErr != nil {
			g.tools.log.Warn("image tool fallback skipped",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.fallback_failed",
				logger.FieldErr, fetchErr,
				"task_id", taskID,
				"requirement_index", i,
			)
			continue
		}
		results[i] = result
		if onProgress != nil {
			onProgress(ctx, len(results), len(reqs), result)
		}
	}

	g.tools.log.Info("image tool calling done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.done",
		"task_id", taskID,
		"tool_calls", callCount,
		"requested", len(reqs),
		"generated", len(results),
	)
	return sortedImageResults(results), nil
}

func (g *ToolCallingGenerator) executeToolCall(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	results map[int]port.ImageResult,
	toolMethods map[string]port.ImageMethod,
	call schema.ToolCall,
) imageToolFeedback {
	method, ok := toolMethods[call.Function.Name]
	if !ok {
		return imageToolFeedback{Message: "unknown or unauthorized tool"}
	}
	var args imageToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return imageToolFeedback{Message: "invalid arguments: " + llmkit.Truncate(err.Error(), 200)}
	}
	if args.RequirementIndex < 0 || args.RequirementIndex >= len(reqs) {
		return imageToolFeedback{RequirementIndex: args.RequirementIndex, Message: "requirementIndex out of range"}
	}
	if existing, exists := results[args.RequirementIndex]; exists {
		return imageToolFeedback{
			OK:               true,
			RequirementIndex: args.RequirementIndex,
			Position:         existing.Position,
			Method:           existing.Method,
			Message:          "already generated; do not call another tool for this item",
		}
	}

	req := reqs[args.RequirementIndex]
	if strings.TrimSpace(args.Keywords) != "" {
		req.Keywords = strings.TrimSpace(args.Keywords)
	}
	if strings.TrimSpace(args.Prompt) != "" {
		req.Prompt = strings.TrimSpace(args.Prompt)
	}
	result, err := g.tools.Execute(ctx, taskID, req, method)
	if err != nil {
		return imageToolFeedback{
			RequirementIndex: args.RequirementIndex,
			Position:         req.Position,
			Method:           method,
			Message:          llmkit.Truncate(err.Error(), 300),
		}
	}
	results[args.RequirementIndex] = result

	g.tools.log.Info("image tool executed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.executed",
		"task_id", taskID,
		"tool", call.Function.Name,
		"requirement_index", args.RequirementIndex,
		"method", method,
	)
	return imageToolFeedback{
		OK:               true,
		Generated:        true,
		RequirementIndex: args.RequirementIndex,
		Position:         result.Position,
		Method:           result.Method,
		Message:          "image generated successfully",
	}
}

func (g *ToolCallingGenerator) buildTools(allowedMethods []port.ImageMethod) ([]*schema.ToolInfo, map[string]port.ImageMethod) {
	allowed := make(map[port.ImageMethod]bool)
	if len(allowedMethods) == 0 {
		for _, method := range g.tools.RegisteredMethods() {
			allowed[method.Normalize()] = true
		}
	} else {
		for _, method := range allowedMethods {
			allowed[method.Normalize()] = true
		}
	}

	tools := make([]*schema.ToolInfo, 0, len(allowed))
	toolMethods := make(map[string]port.ImageMethod, len(allowed))
	for _, method := range g.tools.RegisteredMethods() {
		method = method.Normalize()
		if !allowed[method] {
			continue
		}
		name, description := method.MethodMeta()
		if name == "" {
			continue
		}
		tools = append(tools, &schema.ToolInfo{
			Name: name,
			Desc: description,
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"requirementIndex": {
						Type:     schema.Integer,
						Desc:     "Zero-based requirement index.",
						Required: true,
					},
					"keywords": {
						Type: schema.String,
						Desc: "English search keywords.",
					},
					"prompt": {
						Type: schema.String,
						Desc: "Generation prompt or complete diagram source.",
					},
				},
			),
		})
		toolMethods[name] = method
	}
	return tools, toolMethods
}

func sortedImageResults(results map[int]port.ImageResult) []port.ImageResult {
	out := make([]port.ImageResult, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})
	return out
}

const imageToolSystemPrompt = `你是文章配图执行 Agent。你的唯一任务是为用户给出的每一项配图需求调用工具。

规则：
1. 每项至少成功调用一个工具，requirementIndex 必须使用输入数组的真实下标。
2. 优先使用需求中的 imageSource；工具失败时，分析错误并修改 keywords/prompt 重试，或选择其他可用工具。
3. 已成功的 requirementIndex 不要重复生成。
4. 不得虚构图片 URL，也不要在正文里输出图片；图片结果完全以工具返回为准。
5. 所有项目成功后，用一句简短文字确认完成，不要继续调用工具。`
