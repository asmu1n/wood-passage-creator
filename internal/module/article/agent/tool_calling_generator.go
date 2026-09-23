package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"sort"
	"strings"
	"sync"

	"wood-passage-creator/internal/pkg/llmkit"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/pool"
	"wood-passage-creator/internal/port"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
)

const (
	imageFetchWorkerCap  = 6
	imageReactMaxStepCap = 16
)

type imageSlot int

const (
	imageSlotFree imageSlot = iota
	imageSlotRunning
	imageSlotDone
)

// ToolCallingGenerator 用 Eino ReAct 选择并执行图片 Provider。
// ProviderExecutor 仍是确定性执行层；本类型只负责有界工具循环、结果收集与 fallback。
type ToolCallingGenerator struct {
	model model.ToolCallingChatModel
	tools port.ProviderExecutor
	log   *slog.Logger
}

type imageToolArgs struct {
	RequirementIndex int    `json:"requirementIndex"`
	Keywords         string `json:"keywords"`
	Prompt           string `json:"prompt"`
}

type imageToolOutput struct {
	OK       bool   `json:"ok"`
	Position int    `json:"position"`
	Method   string `json:"method,omitempty"`
	Error    string `json:"error,omitempty"`
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

// 查看目标方案是否有对应的 provider
func (g *ToolCallingGenerator) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	if g == nil || g.tools == nil {
		return port.ImageProviderMetadata{}, false
	}
	return g.tools.LookupProvider(method)
}

// 获取可执行方案的 provider 信息
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
	run := &imageReactRun{
		results:    make(map[int]port.ImageResult, len(reqs)),
		slots:      make(map[int]imageSlot, len(reqs)),
		total:      len(reqs),
		onProgress: onProgress,
	}
	einoTools, toolNames := g.buildTools(taskID, reqs, allowedMethods, run)
	if len(einoTools) == 0 {
		return nil, fmt.Errorf("no permitted image tools are available")
	}

	requirementsJSON, err := json.Marshal(reqs)
	if err != nil {
		return nil, fmt.Errorf("marshal image requirements: %w", err)
	}
	messages := []*schema.Message{
		schema.SystemMessage(imageToolSystemPrompt),
		schema.UserMessage(fmt.Sprintf(
			"请为下面 JSON 数组中的每一项生成一张图片。数组下标就是 requirementIndex。优先使用每项的 imageSource。工具返回 ok=false 时请改参数或换用其他可用工具；ok=true 后不要重复调用。不要输出图片 URL。\n%s",
			string(requirementsJSON),
		)),
	}

	// 初始化 ReAct Agent，设置最大步数和工具配置
	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: g.model,
		MaxStep:          imageReactMaxStep(len(reqs)),
		GraphName:        "ImageToolCalling",
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: einoTools,
			UnknownToolsHandler: func(_ context.Context, name, _ string) (string, error) {
				return fmt.Sprintf("unknown tool %q. available tools: %s", name, strings.Join(toolNames, ", ")), nil
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build image react agent: %w", err)
	}

	if _, callErr := reactAgent.Generate(ctx, messages); callErr != nil {
		g.log.Warn("image react agent stopped; falling back unfinished requirements",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.model_failed",
			logger.FieldErr, callErr,
			"task_id", taskID,
		)
		if ctx.Err() != nil {
			return sortedImageResults(run.snapshot()), ctx.Err()
		}
	}

	// 统一补全缺失的 requirements。
	g.runFallback(ctx, taskID, run.missing(reqs), run)

	results := sortedImageResults(run.snapshot())
	g.log.Info("image tool calling done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.done",
		"task_id", taskID,
		"tool_calls", run.executedCount(),
		"requested", len(reqs),
		"generated", len(results),
	)
	return results, nil
}

// 单次 ReAct 执行状态，管理 requirement 的执行结果和状态。
type imageReactRun struct {
	mu         sync.Mutex
	results    map[int]port.ImageResult
	slots      map[int]imageSlot
	total      int
	done       int
	executed   int
	onProgress port.ImageProgressFunc
}

// 尝试标记某个 requirement 的 position 为正在执行状态，返回是否成功。
func (run *imageReactRun) claim(position int) (imageToolOutput, bool) {
	run.mu.Lock()
	defer run.mu.Unlock()
	switch run.slots[position] {
	case imageSlotDone:
		return imageToolOutput{OK: false, Position: position, Error: "requirement already completed"}, false
	case imageSlotRunning:
		return imageToolOutput{OK: false, Position: position, Error: "requirement is already in progress"}, false
	default:
		run.slots[position] = imageSlotRunning
		return imageToolOutput{}, true
	}
}

// 尝试释放某个 requirement 的 position，将其状态从正在执行变为可用。
func (run *imageReactRun) release(position int) {
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.slots[position] == imageSlotRunning {
		run.slots[position] = imageSlotFree
	}
}

// 标记完成某个 requirement 的 position，并记录结果。返回当前完成数量和是否新增完成。
func (run *imageReactRun) add(ctx context.Context, result port.ImageResult, fromTool bool) (int, bool) {
	run.mu.Lock()
	if _, exists := run.results[result.Position]; exists {
		run.slots[result.Position] = imageSlotDone
		run.slots[result.Position] = imageSlotDone
		done := run.done
		run.mu.Unlock()
		return done, false
	}
	run.results[result.Position] = result
	run.slots[result.Position] = imageSlotDone
	run.done++
	if fromTool {
		run.executed++
	}
	done := run.done
	run.mu.Unlock()
	if run.onProgress != nil {
		run.onProgress(ctx, done, run.total, result)
	}
	return done, true
}

// 返回缺失的 requirements 列表
func (run *imageReactRun) missing(reqs []port.ImageRequirement) []port.ImageRequirement {
	run.mu.Lock()
	defer run.mu.Unlock()
	out := make([]port.ImageRequirement, 0)
	for _, req := range reqs {
		if _, ok := run.results[req.Position]; !ok {
			out = append(out, req)
		}
	}
	return out
}

func (run *imageReactRun) snapshot() map[int]port.ImageResult {
	run.mu.Lock()
	defer run.mu.Unlock()
	out := make(map[int]port.ImageResult, len(run.results))
	maps.Copy(out, run.results)
	return out
}

func (run *imageReactRun) executedCount() int {
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.executed
}

// fallback 只补 ReAct 结束后仍缺的 position，不把失败送回模型。
func (g *ToolCallingGenerator) runFallback(
	ctx context.Context,
	taskID string,
	reqs []port.ImageRequirement,
	run *imageReactRun,
) {
	if len(reqs) == 0 {
		return
	}

	p := pool.NewPool[port.ImageResult](ctx, imageFetchWorkers(len(reqs)), len(reqs))
	for reqIndex, req := range reqs {
		_ = p.AddTask(reqIndex, func(taskCtx context.Context) (port.ImageResult, error) {
			res, err := g.tools.ExecuteWithFallback(taskCtx, taskID, req)
			if err != nil {
				return port.ImageResult{}, err
			}
			run.add(taskCtx, res, false)
			return res, nil
		})
	}

	for _, pr := range p.CollectResults() {
		if pr.Error == nil {
			continue
		}
		g.log.Warn("image tool fallback skipped",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.fallback_failed",
			logger.FieldErr, pr.Error,
			"task_id", taskID,
			"position", reqs[pr.ID].Position,
		)
	}
}

// 根据当前可用的 provider 构建 tool
func (g *ToolCallingGenerator) buildTools(
	taskID string,
	reqs []port.ImageRequirement,
	allowedMethods []port.ImageMethod,
	run *imageReactRun,
) ([]tool.BaseTool, []string) {
	providers := g.AvailableProviders(allowedMethods)
	tools := make([]tool.BaseTool, 0, len(providers))
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider.ToolName == "" {
			continue
		}
		bound := imageProviderTool{
			exector:  g.tools.Execute,
			log:      g.log,
			taskID:   taskID,
			reqs:     reqs,
			state:    run,
			method:   provider.Method,
			toolName: provider.ToolName,
		}
		names = append(names, bound.toolName)
		tools = append(tools, utils.NewTool(
			&schema.ToolInfo{
				Name: provider.ToolName,
				Desc: provider.ToolDescription,
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
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
				}),
			}, bound.invoke))
	}
	return tools, names
}

// 绑定当前执行 tool 和 ReAct 状态，统一处理 tool 回调以及进度通知。
type imageProviderTool struct {
	exector func(ctx context.Context, taskID string, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error)
	log     *slog.Logger
	state   *imageReactRun

	taskID   string
	reqs     []port.ImageRequirement
	method   port.ImageMethod
	toolName string
}

func (t imageProviderTool) invoke(ctx context.Context, args imageToolArgs) (imageToolOutput, error) {
	if err := ctx.Err(); err != nil {
		return imageToolOutput{}, err
	}
	if args.RequirementIndex < 0 || args.RequirementIndex >= len(t.reqs) {
		t.log.Warn("image tool call skipped",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.invalid",
			"task_id", t.taskID,
			"tool", t.toolName,
			"requirement_index", args.RequirementIndex,
		)
		return imageToolOutput{
			OK:    false,
			Error: fmt.Sprintf("requirementIndex %d out of range", args.RequirementIndex),
		}, nil
	}

	req := t.reqs[args.RequirementIndex]
	if keywords := strings.TrimSpace(args.Keywords); keywords != "" {
		req.Keywords = keywords
	}
	if prompt := strings.TrimSpace(args.Prompt); prompt != "" {
		req.Prompt = prompt
	}

	if rejected, ok := t.state.claim(req.Position); !ok {
		return rejected, nil
	}

	result, err := t.exector(ctx, t.taskID, req, t.method)
	if err != nil {
		t.state.release(req.Position)
		if ctx.Err() != nil {
			return imageToolOutput{}, ctx.Err()
		}
		t.log.Warn("image tool call failed; model may retry",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.tool_call.execute_failed",
			logger.FieldErr, err,
			"task_id", t.taskID,
			"tool", t.toolName,
			"position", req.Position,
		)
		return imageToolOutput{
			OK:       false,
			Position: req.Position,
			Error:    llmkit.Truncate(err.Error(), 200),
		}, nil
	}

	t.log.Info("image tool executed",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.tool_call.executed",
		"task_id", t.taskID,
		"tool", t.toolName,
		"position", req.Position,
		"method", t.method,
	)
	t.state.add(ctx, result, true)
	return imageToolOutput{OK: true, Position: req.Position, Method: t.method.String()}, nil
}

// 切片转换以及排序
func sortedImageResults(resultsByPosition map[int]port.ImageResult) []port.ImageResult {
	out := lo.Values(resultsByPosition)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})
	return out
}

// 并发工作池参数校验
func imageFetchWorkers(n int) int {
	if n <= 1 {
		return 1
	}
	if n > imageFetchWorkerCap {
		return imageFetchWorkerCap
	}
	return n
}

func imageReactMaxStep(n int) int {
	// 图步数按「模型一步、工具一步」计算：每项一轮、一次失败重试，再加最终停止调用。
	steps := 2*(n+1) + 2
	if steps > imageReactMaxStepCap {
		return imageReactMaxStepCap
	}
	return steps
}

const imageToolSystemPrompt = `你是文章配图执行 Agent。你的唯一任务是为用户给出的每一项配图需求调用工具。

规则：
1. requirementIndex 必须使用输入数组的真实下标。
2. 优先使用需求中的 imageSource，并按需完善 keywords 或 prompt。
3. 工具返回 ok=true 表示该项已完成，不要再次调用。
4. 工具返回 ok=false 时，修改 keywords 或 prompt，或改用其他已授权工具重试。不要重复完全相同的调用。
5. 不要对同一个 requirementIndex 并发重复调用。
6. 全部需求都已 ok=true，或没有可再试的工具时，停止调用工具。
7. 不得虚构图片 URL，也不要在回复里输出图片地址。图片由工具产生，最终回答保持简短。`
