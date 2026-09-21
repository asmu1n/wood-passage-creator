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
	"github.com/samber/lo"
)

const imageFetchWorkerCap = 6

// ToolCallingGenerator 让模型通过原生 tool calling 选择并执行图片 Provider。
// ProviderExecutor 是确定性的工具执行层；本类型只负责单次工具选择、fallback 与结果收敛。
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

type imageToolTask struct {
	requirement port.ImageRequirement
	method      port.ImageMethod
	toolName    string
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

	doneCount := 0
	progress := func(ctx context.Context, result port.ImageResult) {
		doneCount++
		if onProgress != nil {
			onProgress(ctx, doneCount, len(reqs), result)
		}
	}

	toolModel, err := g.model.WithTools(tools)
	if err != nil {
		return nil, fmt.Errorf("bind image tools: %w", err)
	}

	response, callErr := toolModel.Generate(ctx, messages)
	if callErr != nil {
		g.log.Warn("image tool model call failed; falling back",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.model_failed",
			logger.FieldErr, callErr,
			"task_id", taskID,
		)
	} else if response == nil {
		g.log.Warn("image tool model returned nil response; falling back",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.empty_response",
			"task_id", taskID,
		)
	}

	toolTasks := make([]imageToolTask, 0, len(reqs))
	if callErr == nil && response != nil {
		for _, call := range response.ToolCalls {
			task, prepareErr := prepareImageToolTask(reqs, toolMethods, call)
			if prepareErr != nil {
				g.log.Warn("image tool call skipped",
					logger.FieldPurpose, logger.PurposeJob,
					logger.FieldEvent, "image.tool_call.invalid",
					logger.FieldErr, prepareErr,
					"task_id", taskID,
					"tool", call.Function.Name,
				)
				continue
			}
			toolTasks = append(toolTasks, task)
		}
	}
	toolTasks = lo.UniqBy(toolTasks, func(task imageToolTask) int {
		return task.requirement.Position
	})

	resultsByPosition := make(map[int]port.ImageResult, len(reqs))
	mergeImageResults(resultsByPosition, g.runToolCalls(ctx, taskID, toolTasks, progress))

	fallbackReqs := lo.Filter(reqs, func(req port.ImageRequirement, _ int) bool {
		_, ok := resultsByPosition[req.Position]
		return !ok
	})
	mergeImageResults(resultsByPosition, g.runFallback(ctx, taskID, fallbackReqs, progress))

	g.log.Info("image tool calling done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.done",
		"task_id", taskID,
		"tool_calls", len(toolTasks),
		"requested", len(reqs),
		"generated", len(resultsByPosition),
	)
	return sortedImageResults(resultsByPosition), nil
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
	tasks []imageToolTask,
	progress func(ctx context.Context, result port.ImageResult),
) []port.ImageResult {
	if len(tasks) == 0 {
		return nil
	}

	p := pool.NewPool[port.ImageResult](ctx, imageFetchWorkers(len(tasks)), len(tasks))
	for i, task := range tasks {
		_ = p.AddTask(i, func(taskCtx context.Context) (port.ImageResult, error) {
			res, err := g.executeToolCall(taskCtx, taskID, task.requirement, task.method, task.toolName)
			if progress != nil && err == nil {
				progress(taskCtx, res)
			}
			return res, err
		})
	}

	results := make([]port.ImageResult, 0, len(tasks))
	for _, pr := range p.CollectResults() {
		if pr.Error != nil {
			task := tasks[pr.ID]
			g.log.Warn("image tool call failed; falling back",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.execute_failed",
				logger.FieldErr, pr.Error,
				"task_id", taskID,
				"tool", task.toolName,
				"position", task.requirement.Position,
			)
			continue
		}
		if pr.Value != nil {
			results = append(results, *pr.Value)
		}
	}
	return results
}

func (g *ToolCallingGenerator) runFallback(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	progress func(ctx context.Context, result port.ImageResult),
) []port.ImageResult {
	if len(reqs) == 0 {
		return nil
	}

	p := pool.NewPool[port.ImageResult](ctx, imageFetchWorkers(len(reqs)), len(reqs))
	for reqIndex, req := range reqs {
		_ = p.AddTask(reqIndex, func(taskCtx context.Context) (port.ImageResult, error) {
			res, err := g.tools.ExecuteWithFallback(taskCtx, taskID, req)
			if progress != nil && err == nil {
				progress(taskCtx, res)
			}
			return res, err
		})
	}

	results := make([]port.ImageResult, 0, len(reqs))
	for _, pr := range p.CollectResults() {
		req := reqs[pr.ID]
		if pr.Error != nil {
			g.log.Warn("image tool fallback skipped",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.tool_call.fallback_failed",
				logger.FieldErr, pr.Error,
				"task_id", taskID,
				"position", req.Position,
			)
			continue
		}
		if pr.Value == nil {
			continue
		}
		results = append(results, *pr.Value)
	}
	return results
}

func mergeImageResults(
	resultsByPosition map[int]port.ImageResult,
	generated []port.ImageResult,
) {
	for _, result := range generated {
		if _, exists := resultsByPosition[result.Position]; exists {
			continue
		}
		resultsByPosition[result.Position] = result
	}
}

func prepareImageToolTask(
	reqs []port.ImageRequirement,
	toolMethods map[string]port.ImageMethod,
	call schema.ToolCall,
) (imageToolTask, error) {
	method, ok := toolMethods[call.Function.Name]
	if !ok {
		return imageToolTask{}, fmt.Errorf("unknown or unauthorized tool %q", call.Function.Name)
	}
	var args imageToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return imageToolTask{}, fmt.Errorf("invalid arguments: %s", llmkit.Truncate(err.Error(), 200))
	}
	if args.RequirementIndex < 0 || args.RequirementIndex >= len(reqs) {
		return imageToolTask{}, fmt.Errorf("requirementIndex %d out of range", args.RequirementIndex)
	}

	req := reqs[args.RequirementIndex]
	if keywords := strings.TrimSpace(args.Keywords); keywords != "" {
		req.Keywords = keywords
	}
	if prompt := strings.TrimSpace(args.Prompt); prompt != "" {
		req.Prompt = prompt
	}
	return imageToolTask{
		requirement: req,
		method:      method,
		toolName:    call.Function.Name,
	}, nil
}

func (g *ToolCallingGenerator) executeToolCall(
	ctx context.Context,
	taskID string,
	req port.ImageRequirement,
	method port.ImageMethod,
	toolName string,
) (port.ImageResult, error) {
	result, err := g.tools.Execute(ctx, taskID, req, method)
	if err != nil {
		return port.ImageResult{}, err
	}

	g.log.Info("image tool executed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.executed",
		"task_id", taskID,
		"tool", toolName,
		"position", req.Position,
		"method", method,
	)
	return result, nil
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

func sortedImageResults(resultsByPosition map[int]port.ImageResult) []port.ImageResult {
	out := lo.Values(resultsByPosition)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})
	return out
}

const imageToolSystemPrompt = `你是文章配图执行 Agent。你的唯一任务是为用户给出的每一项配图需求调用工具。

规则：
1. 为每项需求调用一次工具，requirementIndex 必须使用输入数组的真实下标。
2. 优先使用需求中的 imageSource，并根据需求完善 keywords 或 prompt。
3. 不要为同一个 requirementIndex 重复调用工具；失败项将由系统自动 fallback。
4. 不得虚构图片 URL，也不要在正文里输出图片；图片结果完全以工具返回为准。`
