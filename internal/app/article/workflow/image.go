package articleworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/core/tracing"
	"github.com/firebase/genkit/go/genkit"
	"github.com/samber/lo"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

const (
	imageConcurrency = 3
	imageTimeout     = 3 * time.Minute
	providerTimeout  = 90 * time.Second
)

type ImageKind string

const (
	ImageKindCover        ImageKind = "cover"
	ImageKindIllustration ImageKind = "illustration"
)

type PlannedImage struct {
	SlotID          string           `json:"slotId"`
	Kind            ImageKind        `json:"kind"`
	Brief           string           `json:"brief"`
	PreferredMethod port.ImageMethod `json:"preferredMethod"`
}

// ImagePlan contains placement decisions only, never rewritten article content.
type ImagePlan struct {
	Images []PlannedImage `json:"images"`
}

func (p ImagePlan) Validate(slots []ImageSlot, allowedMethods []port.ImageMethod) error {
	slotIDs := make(map[string]bool, len(slots))
	for _, slot := range slots {
		if strings.TrimSpace(slot.ID) == "" || slotIDs[slot.ID] {
			return fmt.Errorf("empty or duplicate image slot %q", slot.ID)
		}
		slotIDs[slot.ID] = true
	}
	allowed := make(map[port.ImageMethod]bool, len(allowedMethods))
	for _, method := range allowedMethods {
		if method.Normalize() != port.MethodPicsum {
			allowed[method.Normalize()] = true
		}
	}
	used := make(map[string]bool, len(p.Images))
	for i, image := range p.Images {
		if err := image.validate(); err != nil {
			return fmt.Errorf("image plan item %d: %w", i, err)
		}
		if !slotIDs[image.SlotID] || used[image.SlotID] {
			return fmt.Errorf("image plan item %d: unknown or repeated slot %q", i, image.SlotID)
		}
		if !allowed[image.PreferredMethod.Normalize()] {
			return fmt.Errorf("image plan item %d: provider %q is not allowed", i, image.PreferredMethod)
		}
		used[image.SlotID] = true
	}
	return nil
}

func (p PlannedImage) validate() error {
	if strings.TrimSpace(p.SlotID) == "" || strings.TrimSpace(p.Brief) == "" {
		return fmt.Errorf("image slot and brief are required")
	}
	switch p.Kind {
	case ImageKindCover:
		if p.SlotID != CoverSlotID {
			return fmt.Errorf("cover image must use the cover slot")
		}
	case ImageKindIllustration:
		if p.SlotID == CoverSlotID {
			return fmt.Errorf("illustration cannot use the cover slot")
		}
	default:
		return fmt.Errorf("unknown image kind %q", p.Kind)
	}
	return validateImageMethod(p.PreferredMethod)
}

func validateImageMethod(method port.ImageMethod) error {
	switch method {
	case port.MethodPexels, port.MethodIconify, port.MethodEmojiPack,
		port.MethodMermaid, port.MethodSVGDiagram, port.MethodNanoBanana:
		return nil
	default:
		return fmt.Errorf("unsupported workflow image provider %q", method)
	}
}

// ID and Position are assigned by code after plan validation, not by the model.
// Position 1 is reserved for the cover and is never reassigned after a skip.
type ImageTask struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	PlannedImage
}

func (t ImageTask) Validate() error {
	if strings.TrimSpace(t.ID) == "" || t.Position <= 0 {
		return fmt.Errorf("image task id and positive position are required")
	}
	if err := t.PlannedImage.validate(); err != nil {
		return err
	}
	if (t.Kind == ImageKindCover) != (t.Position == 1) {
		return fmt.Errorf("image position 1 is reserved for the cover")
	}
	return nil
}

// URL retains the existing image/data URL or mermaid: source convention.
type ImageAsset struct {
	URL    string           `json:"url"`
	Method port.ImageMethod `json:"method"`
}

func (a ImageAsset) Validate() error {
	if strings.TrimSpace(a.URL) == "" {
		return fmt.Errorf("image asset URL is required")
	}
	return validateImageMethod(a.Method)
}

type GeneratedImage struct {
	Task  ImageTask  `json:"task"`
	Asset ImageAsset `json:"asset"`
}

type ImageOutcomeStatus string

const (
	ImageSucceeded ImageOutcomeStatus = "succeeded"
	ImageSkipped   ImageOutcomeStatus = "skipped"
)

// A succeeded outcome has only Asset; a skipped outcome has only Warning.
type ImageOutcome struct {
	Task    ImageTask                  `json:"task"`
	Status  ImageOutcomeStatus         `json:"status"`
	Asset   *ImageAsset                `json:"asset,omitempty"`
	Warning *article.GenerationWarning `json:"warning,omitempty"`
}

func (o ImageOutcome) Validate() error {
	if err := o.Task.Validate(); err != nil {
		return err
	}
	switch o.Status {
	case ImageSucceeded:
		if o.Asset == nil || o.Warning != nil {
			return fmt.Errorf("succeeded image requires an asset and no warning")
		}
		if err := o.Asset.Validate(); err != nil {
			return err
		}
	case ImageSkipped:
		if o.Asset != nil || o.Warning == nil {
			return fmt.Errorf("skipped image requires a warning and no asset")
		}
		w := o.Warning
		if w.Code != article.WarningImageSkipped || w.ImageID != o.Task.ID ||
			w.Position != o.Task.Position || strings.TrimSpace(w.Message) == "" {
			return fmt.Errorf("image warning must identify the skipped task and its reason")
		}
	default:
		return fmt.Errorf("unknown image outcome status %q", o.Status)
	}
	return nil
}

type ImageSummary struct {
	Total     int `json:"total"`
	Settled   int `json:"settled"`
	Succeeded int `json:"succeeded"`
	Skipped   int `json:"skipped"`
}

func (s ImageSummary) Validate() error {
	if s.Total < 0 || s.Settled < 0 || s.Succeeded < 0 || s.Skipped < 0 ||
		s.Settled > s.Total || s.Succeeded+s.Skipped != s.Settled {
		return fmt.Errorf("image summary requires succeeded + skipped = settled <= total")
	}
	return nil
}

type imageExecutionInput struct {
	Run    RunContext           `json:"run"`
	Task   ImageTask            `json:"task"`
	Budget ImageExecutionBudget `json:"budget"`
}

// Each decision and each actual provider attempt consume separate budgets.
// Deadlines and batch concurrency are owned by the caller, not by the model.
type ImageExecutionBudget struct {
	MaxModelTurns   int `json:"maxModelTurns"`
	MaxToolAttempts int `json:"maxToolAttempts"`
}

func (b ImageExecutionBudget) Validate() error {
	if b.MaxModelTurns <= 0 || b.MaxToolAttempts <= 0 {
		return fmt.Errorf("image model-turn and tool-attempt budgets must be positive")
	}
	return nil
}

type imageToolArgs struct {
	Keywords string `json:"keywords,omitempty" jsonschema_description:"Search keywords in the language required by this provider."`
	Prompt   string `json:"prompt,omitempty" jsonschema_description:"A visual description or a description of diagram elements and relationships, never a URL."`
}

type imageToolFeedback struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

var errImageAuthorizationChanged = errors.New("image provider authorization changed")

type imageExecutor struct {
	runtime       *genkit.Genkit
	provider      port.ProviderExecutor
	executePrompt ai.Prompt
	log           *slog.Logger
}

func newImageExecutor(g *genkit.Genkit, provider port.ProviderExecutor) (*imageExecutor, error) {
	if g == nil || lo.IsNil(provider) {
		return nil, fmt.Errorf("Genkit runtime and image provider executor are required")
	}
	prompt := genkit.LookupPrompt(g, PromptImageExecute)
	if prompt == nil {
		return nil, fmt.Errorf("image execution prompt is missing")
	}
	return &imageExecutor{
		runtime:       g,
		provider:      provider,
		executePrompt: prompt,
		log:           logger.Module("article.workflow"),
	}, nil
}

func (e *imageExecutor) executeBatch(ctx context.Context, run RunContext, tasks []ImageTask, finished func(context.Context, ImageOutcome, ImageSummary) error) ([]ImageOutcome, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type job struct {
		index int
		task  ImageTask
	}
	jobs := make(chan job, len(tasks))
	for i, task := range tasks {
		jobs <- job{i, task}
	}
	close(jobs)
	outcomes := make([]ImageOutcome, len(tasks))
	summary := ImageSummary{Total: len(tasks)}
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for range min(imageConcurrency, len(tasks)) {
		wg.Go(func() {
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				outcome, err := e.execute(ctx, imageExecutionInput{
					Run: run, Task: job.task, Budget: ImageExecutionBudget{MaxModelTurns: 3, MaxToolAttempts: 3},
				})
				if err != nil && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
					outcome, err = skippedImage(job.task, "单张配图执行超时"), nil
				}
				mu.Lock()
				if firstErr == nil && err != nil {
					firstErr = err
					cancel()
				}
				if err == nil && firstErr == nil {
					outcomes[job.index] = outcome
					summary.Settled++
					if outcome.Status == ImageSucceeded {
						summary.Succeeded++
					} else {
						summary.Skipped++
					}
					if finished != nil {
						if err := finished(ctx, outcome, summary); err != nil {
							firstErr = err
							cancel()
						}
					}
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return outcomes, nil
}

func (e *imageExecutor) execute(ctx context.Context, input imageExecutionInput) (ImageOutcome, error) {
	return genkit.RunWithContext(ctx, "generateImage", func(ctx context.Context) (ImageOutcome, error) {
		tracing.SetSpanMetadata(ctx, map[string]string{
			"taskId": input.Run.TaskID, "runId": input.Run.RunID, "imageId": input.Task.ID,
		})
		if err := input.Task.Validate(); err != nil {
			return ImageOutcome{}, status.Errorf(status.ErrInvalidInput, "%w", err)
		}
		if err := input.Budget.Validate(); err != nil {
			return ImageOutcome{}, status.Errorf(status.ErrInvalidInput, "%w", err)
		}
		ctx, cancel := context.WithTimeout(ctx, imageTimeout)
		defer cancel()
		outcome, err := e.executeImage(ctx, input)
		if err != nil {
			return ImageOutcome{}, err
		}
		if err := outcome.Validate(); err != nil {
			return ImageOutcome{}, err
		}
		return outcome, nil
	})
}

func (e *imageExecutor) permittedProviders(allowed []port.ImageMethod) []port.ImageProviderMetadata {
	if len(allowed) == 0 {
		return nil
	}
	return lo.Filter(e.provider.AvailableProviders(allowed), func(p port.ImageProviderMetadata, _ int) bool {
		return slices.Contains(allowed, p.Method) && validateImageMethod(p.Method) == nil &&
			(p.Access == port.ImageAccessFree || p.Access == port.ImageAccessVIP) && p.ToolName != ""
	})
}

func skippedImage(task ImageTask, reason string) ImageOutcome {
	return ImageOutcome{
		Task: task, Status: ImageSkipped,
		Warning: &article.GenerationWarning{
			Code: article.WarningImageSkipped, ImageID: task.ID, Position: task.Position,
			Message: fmt.Sprintf("配图 %d 已跳过：%s", task.Position, reason),
		},
	}
}

func (e *imageExecutor) executeImage(ctx context.Context, input imageExecutionInput) (ImageOutcome, error) {
	// 获取当前可用的 image tool providers
	providers := e.permittedProviders(input.Run.AllowedImageMethods)
	if len(providers) == 0 {
		return skippedImage(input.Task, "没有可用且已授权的配图工具"), nil
	}

	// 包装需要使用的 image tool
	guides := lo.Map(providers, func(p port.ImageProviderMetadata, _ int) ImageToolGuide {
		return ImageToolGuide{Method: p.Method, ToolName: p.ToolName, UsageGuide: p.PlannerUsageGuide}
	})
	// 根据当前的 image task 以及可用的 image tool providers，生成一个 prompt 用于模型决策
	rendered, err := e.executePrompt.Render(ctx, ImageExecutePromptInput{
		Kind: input.Task.Kind, Brief: input.Task.Brief, PreferredMethod: input.Task.PreferredMethod, Tools: guides,
	})
	if err != nil {
		return ImageOutcome{}, fmt.Errorf("render image execution prompt: %w", err)
	}
	messages := rendered.Messages
	tools := make([]ai.ToolRef, 0, len(providers))
	bound := make(map[string]ai.Tool, len(providers))
	attempts := 0
	var asset *ImageAsset
	// 遍历生成 tools，绑定 tool name 到实际的 provider 执行逻辑
	for _, provider := range providers {
		if bound[provider.ToolName] != nil {
			return ImageOutcome{}, fmt.Errorf("duplicate provider tool %q", provider.ToolName)
		}
		tool := ai.NewTool(provider.ToolName, provider.ToolDescription,
			func(toolCtx *ai.ToolContext, args imageToolArgs) (imageToolFeedback, error) {
				if err := toolCtx.Err(); err != nil {
					return imageToolFeedback{}, err
				}
				if attempts >= input.Budget.MaxToolAttempts {
					return imageToolFeedback{}, fmt.Errorf("image tool budget exhausted")
				}
				metadata, ok := e.provider.LookupProvider(provider.Method)
				if !ok || metadata.ToolName != provider.ToolName || metadata.Access == port.ImageAccessInternal ||
					!slices.Contains(input.Run.AllowedImageMethods, provider.Method) {
					return imageToolFeedback{}, errImageAuthorizationChanged
				}
				attempts++
				callCtx, cancel := context.WithTimeout(toolCtx, providerTimeout)
				defer cancel()
				result, err := e.provider.Execute(callCtx, input.Run.TaskID, port.ImageRequirement{
					Position: input.Task.Position, Type: string(input.Task.Kind),
					Keywords: strings.TrimSpace(args.Keywords), Prompt: strings.TrimSpace(args.Prompt),
				}, provider.Method)
				if err != nil {
					return imageToolFeedback{}, err
				}
				if err := callCtx.Err(); err != nil {
					return imageToolFeedback{}, err
				}
				value := ImageAsset{URL: result.URL, Method: result.Method}
				if value.Method != provider.Method {
					return imageToolFeedback{}, fmt.Errorf("image provider returned a different method")
				}
				if err := value.Validate(); err != nil {
					return imageToolFeedback{}, err
				}
				if _, err := assetMarkdown(GeneratedImage{Task: input.Task, Asset: value}); err != nil {
					return imageToolFeedback{}, err
				}
				asset = &value
				return imageToolFeedback{OK: true}, nil
			})
		tools = append(tools, tool)
		bound[provider.ToolName] = tool
	}
	seen := map[string]bool{}
	// 维护一个 tool loop ，直到模型返回一个成功的配图结果，或者达到最大模型轮次和最大工具调用次数
	for turn := 0; turn < input.Budget.MaxModelTurns && attempts < input.Budget.MaxToolAttempts; turn++ {
		if err := ctx.Err(); err != nil {
			return ImageOutcome{}, err
		}
		// 获取 llm 返回的 tool 调用决策
		resp, err := genkit.Generate(ctx, e.runtime,
			ai.WithMessages(messages...), ai.WithTools(tools...), ai.WithReturnToolRequests(true),
		)
		if err != nil {
			e.log.WarnContext(ctx, "single image model failed",
				logger.FieldEvent, "image.model_failed", logger.FieldErr, err, "task_id", input.Run.TaskID, "image_id", input.Task.ID)
			if ctx.Err() != nil {
				return ImageOutcome{}, ctx.Err()
			}
			if fatalGenerationError(err) {
				return ImageOutcome{}, err
			}
			return skippedImage(input.Task, "配图决策服务暂时不可用"), nil
		}
		if err := validateCompletedResponse(resp); err != nil {
			e.log.WarnContext(ctx, "single image model stopped abnormally", logger.FieldErr, err, "image_id", input.Task.ID)
			return skippedImage(input.Task, "未能获得有效的配图工具请求"), nil
		}
		requests := resp.ToolRequests()
		if len(requests) == 0 {
			return skippedImage(input.Task, "模型未能找到可行的配图方案"), nil
		}
		messages = resp.History()
		responses := make([]*ai.Part, 0, len(requests))
		// 执行模型返回的 tool 调用决策，直到成功生成配图，或者达到最大模型轮次和最大工具调用次数（往往只有一个 request）
		for _, part := range requests {
			req := part.ToolRequest
			feedback := imageToolFeedback{Error: "Request exactly one tool per turn."}
			if len(requests) == 1 {
				tool := bound[req.Name]
				argsJSON, err := json.Marshal(req.Input)
				if err != nil {
					return ImageOutcome{}, fmt.Errorf("encode image tool input: %w", err)
				}
				key := req.Name + ":" + string(argsJSON)
				switch {
				case tool == nil:
					feedback.Error = "Unknown or unauthorized tool. Use one of the provided tools."
				case seen[key]:
					feedback.Error = "This exact call already failed. Change the parameters or provider."
				default:
					seen[key] = true
					_, err := tool.RunRaw(ctx, req.Input)
					if err != nil {
						if ctx.Err() != nil {
							return ImageOutcome{}, ctx.Err()
						}
						if errors.Is(err, errImageAuthorizationChanged) ||
							errors.Is(err, status.ErrUnauthenticated) || errors.Is(err, status.ErrPermissionDenied) ||
							errors.Is(err, status.ErrNotFound) || errors.Is(err, ai.ErrUnsupportedByModel) {
							return ImageOutcome{}, err
						}
						e.log.WarnContext(ctx, "image tool failed",
							logger.FieldEvent, "image.tool_failed", logger.FieldErr, err,
							"task_id", input.Run.TaskID, "image_id", input.Task.ID, "tool", req.Name)
						feedback.Error = "The call failed or input was invalid. Correct its parameters or try another provided tool."
					} else if asset != nil {
						return ImageOutcome{Task: input.Task, Status: ImageSucceeded, Asset: asset}, nil
					}
				}
			}
			responses = append(responses, ai.NewToolResponsePart(&ai.ToolResponse{Name: req.Name, Ref: req.Ref, Output: feedback}))
		}
		messages = append(messages, ai.NewMessage(ai.RoleTool, nil, responses...))
	}
	return skippedImage(input.Task, "已达到配图重试上限"), nil
}

func fatalGenerationError(err error) bool {
	return errors.Is(err, status.ErrInvalidArgument) || errors.Is(err, status.ErrNotFound) ||
		errors.Is(err, status.ErrUnauthenticated) || errors.Is(err, status.ErrPermissionDenied)
}
