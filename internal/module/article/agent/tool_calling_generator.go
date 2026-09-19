package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/pool"
	"wood-passage-creator/internal/port"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	maxImageToolRounds  = 6
	imageFetchWorkerCap = 6
)

// ToolCallingGenerator 让模型通过原生 tool calling 选择并执行图片 Provider。
// ProviderExecutor 是确定性的工具执行层；本类型只负责对话循环、权限边界与结果收敛。
type ToolCallingGenerator struct {
	model model.ToolCallingChatModel
	tools port.ProviderExecutor
	log   *slog.Logger
}

func NewToolCallingGenerator(
	toolModel model.ToolCallingChatModel,
	tools port.ProviderExecutor,
) port.ImageGenerator {
	return &ToolCallingGenerator{
		model: toolModel,
		tools: tools,
		log:   logger.Module("article.agent"),
	}
}

type imageToolArgs struct {
	RequirementIndex int    `json:"requirementIndex"`
	Keywords         string `json:"keywords"`
	Prompt           string `json:"prompt"`
}

type imageToolFeedback struct {
	OK               bool              `json:"ok"`
	Generated        bool              `json:"generated,omitempty"`
	RequirementIndex int               `json:"requirementIndex"`
	Position         int               `json:"position,omitempty"`
	Method           port.ImageMethod  `json:"method,omitempty"`
	Message          string            `json:"message"`
	Result           *port.ImageResult `json:"-"`
}

func (g *ToolCallingGenerator) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	if g == nil || g.tools == nil {
		return port.ImageProviderMetadata{}, false
	}
	return g.tools.LookupProvider(method)
}

func (g *ToolCallingGenerator) AvailableProviders(allowedMethods []port.ImageMethod) []port.ImageProviderMetadata {
	if g == nil || g.tools == nil {
		return nil
	}
	return g.tools.AvailableProviders(allowedMethods)
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

	doneCount := 0
	progress := func(ctx context.Context, result port.ImageResult) {
		doneCount++
		if onProgress != nil {
			onProgress(ctx, doneCount, len(reqs), result)
		}
	}
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
			g.log.Warn("image tool model call failed; falling back",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.model_failed",
				logger.FieldErr, callErr,
				"task_id", taskID,
			)
			break
		}
		if response == nil {
			g.log.Warn("image tool model returned nil response; falling back",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.empty_response",
				"task_id", taskID,
			)
			break
		}
		if len(response.ToolCalls) == 0 {
			messages = append(messages, response)
			// 获取到所有图片，直接返回结果
			if len(results) == len(reqs) {
				return sortedImageResults(results), nil
			} else {
				// 如果存在未完成的图片，则补充仍缺少图片的 prompt
				messages = append(messages, schema.UserMessage(
					fmt.Sprintf("还有 %d 项没有生成。请继续调用工具，不要只返回文字。", len(reqs)-len(results)),
				))
				continue
			}
		}

		messages = append(messages, response)

		callsToRun := make([]schema.ToolCall, 0, len(response.ToolCalls))
		for _, call := range response.ToolCalls {
			if callCount >= maxCalls {
				break
			}
			callCount++
			callsToRun = append(callsToRun, call)
		}

		feedbacks := g.runToolCalls(ctx, taskID, reqs, results, toolMethods, callsToRun, progress)
		for i, fb := range feedbacks {
			payload, marshalErr := json.Marshal(fb)
			if marshalErr != nil {
				payload = []byte(`{"ok":false,"message":"encode tool result failed"}`)
			}
			call := callsToRun[i]
			messages = append(messages, schema.ToolMessage(
				string(payload),
				call.ID,
				schema.WithToolName(call.Function.Name),
			))

			results[fb.RequirementIndex] = *fb.Result
		}

		if callCount >= maxCalls {
			break
		}
	}

	g.runFallback(ctx, taskID, reqs, results, progress)

	g.log.Info("image tool calling done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.done",
		"task_id", taskID,
		"tool_calls", callCount,
		"requested", len(reqs),
		"generated", len(results),
	)
	return sortedImageResults(results), nil
}

func imageFetchWorkers(n int) int {
	if n <= 1 {
		return 1
	}
	if n > imageFetchWorkerCap {
		return imageFetchWorkerCap
	}
	return n
}

func (g *ToolCallingGenerator) runToolCalls(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	results map[int]port.ImageResult,
	toolMethods map[string]port.ImageMethod,
	calls []schema.ToolCall,
	onProgress func(ctx context.Context, result port.ImageResult),
) []imageToolFeedback {
	if len(calls) == 0 {
		return nil
	}

	p := pool.NewPool[imageToolFeedback](ctx, imageFetchWorkers(len(calls)), len(calls))
	count := len(results)
	for i, call := range calls {
		_ = p.AddTask(i, func(taskCtx context.Context) (imageToolFeedback, error) {

			fb := g.executeToolCall(taskCtx, taskID, reqs, results, toolMethods, call)

			if onProgress != nil && fb.Result != nil && fb.Generated {
				count++
				onProgress(ctx, *fb.Result)
			}
			return fb, nil
		})
	}

	feedbacks := make([]imageToolFeedback, len(calls))
	for i, pr := range p.CollectResults() {
		if pr.Value != nil {
			feedbacks[i] = *pr.Value
		} else if pr.Error != nil {
			feedbacks[i] = imageToolFeedback{Message: llmkit.Truncate(pr.Error.Error(), 300)}
		}
	}
	return feedbacks
}

func (g *ToolCallingGenerator) runFallback(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	results map[int]port.ImageResult,
	onProgress func(ctx context.Context, result port.ImageResult),
) {
	type missing struct {
		index int
		req   port.ImageRequirement
	}
	pending := make([]missing, 0, len(reqs))
	for i, req := range reqs {
		if _, ok := results[i]; ok {
			continue
		}
		pending = append(pending, missing{index: i, req: req})
	}
	if len(pending) == 0 {
		return
	}

	p := pool.NewPool[port.ImageResult](ctx, imageFetchWorkers(len(pending)), len(pending))
	for _, item := range pending {
		_ = p.AddTask(item.index, func(taskCtx context.Context) (port.ImageResult, error) {
			return g.tools.ExecuteWithFallback(taskCtx, taskID, item.req)
		})
	}

	for _, pr := range p.CollectResults() {
		if pr.Error != nil {
			g.log.Warn("image tool fallback skipped",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.fallback_failed",
				logger.FieldErr, pr.Error,
				"task_id", taskID,
				"requirement_index", pr.ID,
			)
			continue
		}
		if pr.Value == nil {
			continue
		}
		if _, exists := results[pr.ID]; exists {
			continue
		}
		results[pr.ID] = *pr.Value
		if onProgress != nil {
			onProgress(ctx, *pr.Value)
		}
	}
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

	g.log.Info("image tool executed",
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
		Result:           &result,
	}
}

func (g *ToolCallingGenerator) buildTools(allowedMethods []port.ImageMethod) ([]*schema.ToolInfo, map[string]port.ImageMethod) {
	providers := g.AvailableProviders(allowedMethods)
	tools := make([]*schema.ToolInfo, 0, len(providers))
	toolMethods := make(map[string]port.ImageMethod, len(providers))
	for _, provider := range providers {
		if provider.ToolName == "" {
			continue
		}
		tools = append(tools, &schema.ToolInfo{
			Name: provider.ToolName,
			Desc: provider.ToolDescription,
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
		toolMethods[provider.ToolName] = provider.Method
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
