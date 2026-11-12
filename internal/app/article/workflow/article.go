package articleworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/core/tracing"
	"github.com/firebase/genkit/go/genkit"
	"github.com/samber/lo"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

type articleUpdate struct {
	Delta   *ContentDelta     `json:"delta,omitempty"`
	Content *ContentGenerated `json:"content,omitempty"`
	Planned *ImagesPlanned    `json:"planned,omitempty"`
	Image   *ImageFinished    `json:"image,omitempty"`
	Images  *ImagesDone       `json:"images,omitempty"`
}

type ArticleFlows struct {
	flow   *core.Flow[GenerateArticleInput, ArticleResult, articleUpdate]
	images *imageExecutor
	stepRecorder
}

// 生成 article 的 flow 初始化（正文生成、配图分析、配图生成、合并完整内容）
func NewArticleFlows(g *genkit.Genkit, executor port.ProviderExecutor, logs article.AgentLogRecorder) (*ArticleFlows, error) {
	if g == nil || lo.IsNil(executor) {
		return nil, fmt.Errorf("Genkit runtime and image executor are required")
	}
	// Shared prompt schemas are registered by NewPlanningFlows during startup.
	content := genkit.LookupDataPrompt[ContentPromptInput, string](g, PromptContent)
	plan := genkit.LookupDataPrompt[PlanImagesInput, *ImagePlan](g, PromptImagePlan)
	if content == nil || plan == nil {
		return nil, fmt.Errorf("article prompts are missing")
	}
	// 初始化图片生成执行器（可复用于配图、头像等）
	images, err := newImageExecutor(g, executor)
	if err != nil {
		return nil, err
	}
	// 装配控制器实例
	f := &ArticleFlows{
		images:       images,
		stepRecorder: stepRecorder{logs: logs, log: logger.Module("article.workflow")},
	}
	// 初始化流式 flow (存在流式输出的步骤：正文生成、配图计划、配图生成)（流式输出内容 包括 关键节点信息回传以及增量文本）
	f.flow = genkit.DefineStreamingFlow(g, string(FlowGenerateArticle),
		func(ctx context.Context, input GenerateArticleInput, send core.StreamCallback[articleUpdate]) (ArticleResult, error) {
			// 追踪数据注入以及基础输入校验
			tracing.SetSpanMetadata(ctx, map[string]string{"taskId": input.Run.TaskID, "runId": input.Run.RunID})
			if err := validateEditorial(input.Editorial); err != nil {
				return ArticleResult{}, status.Errorf(status.ErrInvalidInput, "%w", err)
			}
			if err := validateOutline(OutlineResult{Sections: input.Outline}); err != nil {
				return ArticleResult{}, status.Errorf(status.ErrInvalidInput, "%w", err)
			}
			// 第一步：生成正文内容（流式输出 --透传增量文本并包装流式输出结果）
			body, err := runWorkflowStep(f.stepRecorder, ctx, input.Run, "content_generator",
				map[string]any{"outlineSections": len(input.Outline)},
				func(text string) any { return map[string]any{"contentLength": len(text)} },
				func(ctx context.Context) (string, error) {
					text, resp, err := content.Execute(ctx, ContentPromptInput{
						Editorial: promptEditorial(input.Editorial), Outline: input.Outline,
					}, ai.WithStreaming(func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
						if delta := chunk.Text(); delta != "" {
							return send(ctx, articleUpdate{Delta: &ContentDelta{Delta: delta}})
						}
						return nil
					}))
					if err != nil {
						return "", err
					}
					if err := validateCompletedResponse(resp); err != nil {
						return "", err
					}
					if strings.TrimSpace(text) == "" {
						return "", status.Errorf(status.ErrInvalidOutput, "empty article content")
					}
					return text, nil
				})
			if err != nil {
				return ArticleResult{}, err
			}
			// 流式回传正文生成完整结果（用于客户端实时渲染）
			if err := send(ctx, articleUpdate{Content: &ContentGenerated{Content: body}}); err != nil {
				return ArticleResult{}, err
			}
			// 解析正文内容为结构数据（用于后续配图计划和生成）
			doc, err := BuildArticleDocument(body)
			if err != nil {
				return ArticleResult{}, err
			}
			// 第二步：生成配图方案并且执行配图生成（按照关键节点回传执行结果）
			outcomes, err := runWorkflowStep(f.stepRecorder, ctx, input.Run, "image_generator",
				map[string]any{"contentLength": len(body)},
				func(out []ImageOutcome) any {
					return map[string]any{"requirements": len(out), "images": lo.CountBy(out, func(o ImageOutcome) bool { return o.Status == ImageSucceeded })}
				},
				func(ctx context.Context) ([]ImageOutcome, error) {
					providers := f.images.permittedProviders(input.Run.AllowedImageMethods)
					tasks := []ImageTask{}
					if len(providers) > 0 {
						result, resp, err := plan.Execute(ctx, PlanImagesInput{
							Editorial: input.Editorial, Document: doc,
							Providers: lo.Map(providers, func(p port.ImageProviderMetadata, _ int) ImageProviderGuide {
								return ImageProviderGuide{Method: p.Method, Description: p.PlannerDescription, UsageGuide: p.PlannerUsageGuide}
							}),
						})
						if err != nil {
							return nil, err
						}
						if err := validateCompletedResponse(resp); err != nil {
							return nil, err
						}
						if result == nil {
							return nil, status.Errorf(status.ErrInvalidOutput, "image plan returned no data")
						}
						tasks, err = AssignImageTasks(*result, doc, lo.Map(providers, func(p port.ImageProviderMetadata, _ int) port.ImageMethod { return p.Method }))
						if err != nil {
							return nil, status.Errorf(status.ErrInvalidOutput, "invalid image plan: %w", err)
						}
					}
					if err := send(ctx, articleUpdate{Planned: &ImagesPlanned{Tasks: tasks}}); err != nil {
						return nil, err
					}
					outcomes, err := f.images.executeBatch(ctx, input.Run, tasks,
						func(ctx context.Context, outcome ImageOutcome, summary ImageSummary) error {
							return send(ctx, articleUpdate{Image: &ImageFinished{Outcome: outcome, Summary: summary}})
						})
					if err != nil {
						return nil, err
					}
					succeeded := lo.CountBy(outcomes, func(o ImageOutcome) bool { return o.Status == ImageSucceeded })
					summary := ImageSummary{
						Total: len(tasks), Settled: len(outcomes),
						Succeeded: succeeded, Skipped: len(outcomes) - succeeded,
					}
					if err := send(ctx, articleUpdate{Images: &ImagesDone{Outcomes: outcomes, Summary: summary}}); err != nil {
						return nil, err
					}
					return outcomes, nil
				})
			if err != nil {
				return ArticleResult{}, err
			}
			images := lo.FilterMap(outcomes, func(o ImageOutcome, _ int) (GeneratedImage, bool) {
				if o.Status != ImageSucceeded {
					return GeneratedImage{}, false
				}
				return GeneratedImage{Task: o.Task, Asset: *o.Asset}, true
			})
			warnings := lo.FilterMap(outcomes, func(o ImageOutcome, _ int) (article.GenerationWarning, bool) {
				if o.Warning == nil {
					return article.GenerationWarning{}, false
				}
				return *o.Warning, true
			})
			// 第三步：合并正文内容和配图结果为完整文章
			full, err := runWorkflowStep(f.stepRecorder, ctx, input.Run, "content_merger",
				map[string]any{"images": len(images)},
				func(text string) any { return map[string]any{"fullContentLength": len(text)} },
				func(context.Context) (string, error) { return ComposeArticle(doc, images) })
			if err != nil {
				return ArticleResult{}, err
			}
			return ArticleResult{Content: body, FullContent: full, Images: images, Warnings: warnings}, nil
		})
	return f, nil
}

// 对外接口所使用的函数
func (f *ArticleFlows) GenerateArticle(ctx context.Context, input GenerateArticleInput, emit EmitProgress) (ArticleResult, error) {
	return f.flow.Action.Run(ctx, input, func(ctx context.Context, update articleUpdate) error {
		if emit == nil {
			return nil
		}
		switch {
		case update.Delta != nil:
			return emit(ctx, *update.Delta)
		case update.Content != nil:
			return emit(ctx, *update.Content)
		case update.Planned != nil:
			return emit(ctx, *update.Planned)
		case update.Image != nil:
			return emit(ctx, *update.Image)
		case update.Images != nil:
			return emit(ctx, *update.Images)
		default:
			return fmt.Errorf("article flow emitted an empty update")
		}
	})
}
