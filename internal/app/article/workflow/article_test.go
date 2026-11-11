package articleworkflow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

func TestArticleFlowSkipsPlanningWithoutAuthorizedProviders(t *testing.T) {
	calls := 0
	flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls++
		return completedJSON("An article with no pictures."), nil
	}, newArticleExecutor())
	var progress []ProgressPayload
	out, err := flow.GenerateArticle(t.Context(), GenerateArticleInput{
		Run: planningRun(), Editorial: planningEditorial(),
		Outline: []article.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}},
	}, func(_ context.Context, payload ProgressPayload) error {
		progress = append(progress, payload)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Content != out.FullContent || out.Images == nil || out.Warnings == nil || len(out.Images) != 0 {
		t.Fatalf("no-provider flow = %+v, calls=%d", out, calls)
	}
	if len(progress) != 3 {
		t.Fatalf("empty image batch progress = %+v", progress)
	}
	planned, ok := progress[1].(ImagesPlanned)
	if !ok || planned.Tasks == nil || len(planned.Tasks) != 0 {
		t.Fatalf("empty image plan = %+v", progress[1])
	}
	done, ok := progress[2].(ImagesDone)
	if !ok || done.Outcomes == nil || len(done.Outcomes) != 0 || done.Summary != (ImageSummary{}) {
		t.Fatalf("empty image completion = %+v", progress[2])
	}
}

func TestArticleFlowRejectsInvalidPlanWithoutExecutingTools(t *testing.T) {
	executor := newArticleExecutor()
	calls := 0
	flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls++
		if calls == 1 {
			return completedJSON("Original body"), nil
		}
		return completedJSON(`{"images":[{"slotId":"made-up-slot","kind":"illustration","brief":"Photo","preferredMethod":"PEXELS"}]}`), nil
	}, executor)
	run := planningRun()
	run.AllowedImageMethods = []port.ImageMethod{port.MethodPexels}
	_, err := flow.GenerateArticle(t.Context(), GenerateArticleInput{
		Run: run, Editorial: planningEditorial(),
		Outline: []article.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}},
	}, nil)
	if err == nil || executor.calls.Load() != 0 {
		t.Fatalf("invalid plan executed: calls=%d err=%v", executor.calls.Load(), err)
	}
}

func TestArticleProgressFailureCancelsOtherImages(t *testing.T) {
	executor := newArticleExecutor()
	executor.execute = func(ctx context.Context, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
		if req.Position == 1 {
			return port.ImageResult{URL: "https://example.com/cover.jpg", Method: method}, nil
		}
		<-ctx.Done()
		return port.ImageResult{}, ctx.Err()
	}
	flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return toolRequest("search_photo", map[string]any{"keywords": "wood"}), nil
	}, executor)
	tasks := []ImageTask{coverTask(), {ID: "image-2", Position: 2, PlannedImage: PlannedImage{
		SlotID: "after:block-1", Kind: ImageKindIllustration, Brief: "Body", PreferredMethod: port.MethodPexels,
	}}}
	want := errors.New("subscriber failed")
	_, err := runImageTest(t.Context(), func(ctx context.Context) ([]ImageOutcome, error) {
		return flow.images.executeBatch(ctx, imageInput().Run, tasks, func(context.Context, ImageOutcome, ImageSummary) error { return want })
	})
	if !errors.Is(err, want) {
		t.Fatalf("callback error was lost: %v", err)
	}
}

type articleExecutorStub struct {
	providers []port.ImageProviderMetadata
	execute   func(context.Context, port.ImageRequirement, port.ImageMethod) (port.ImageResult, error)
	calls     atomic.Int32
}

func newArticleExecutor() *articleExecutorStub {
	return &articleExecutorStub{providers: []port.ImageProviderMetadata{
		{Method: port.MethodPexels, Access: port.ImageAccessFree, ToolName: "search_photo", ToolDescription: "Search a photo"},
		{Method: port.MethodMermaid, Access: port.ImageAccessFree, ToolName: "make_diagram", ToolDescription: "Generate a diagram"},
	}}
}
func (s *articleExecutorStub) AvailableProviders(allowed []port.ImageMethod) []port.ImageProviderMetadata {
	return slices.Clone(s.providers)
}
func (s *articleExecutorStub) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	for _, provider := range s.providers {
		if provider.Method == method {
			return provider, true
		}
	}
	return port.ImageProviderMetadata{}, false
}
func (s *articleExecutorStub) Execute(ctx context.Context, _ string, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
	s.calls.Add(1)
	if s.execute != nil {
		return s.execute(ctx, req, method)
	}
	if method == port.MethodMermaid {
		return port.ImageResult{URL: "mermaid:flowchart TD\nA-->B", Method: method}, nil
	}
	return port.ImageResult{URL: "https://example.com/photo.png", Method: method}, nil
}
func newArticleTestFlows(t *testing.T, model planningModelFunc, executor *articleExecutorStub) (*ArticleFlows, *planningLogRecorder, *genkit.Genkit) {
	t.Helper()
	_, logs, g := planningTestFlows(t, model)
	flow, err := NewArticleFlows(g, executor, logs)
	if err != nil {
		t.Fatal(err)
	}
	return flow, logs, g
}
func toolRequest(name string, input map[string]any) *ai.ModelResponse {
	return &ai.ModelResponse{FinishReason: ai.FinishReasonStop, Message: ai.NewMessage(ai.RoleModel, nil,
		ai.NewToolRequestPart(&ai.ToolRequest{Name: name, Ref: "call-1", Input: input}))}
}
func imageInput() imageExecutionInput {
	return imageExecutionInput{
		Run:  RunContext{TaskID: "task", RunID: "run", AllowedImageMethods: []port.ImageMethod{port.MethodPexels, port.MethodMermaid}},
		Task: coverTask(), Budget: ImageExecutionBudget{MaxModelTurns: 3, MaxToolAttempts: 3},
	}
}

func TestSingleImageStopsImmediatelyAfterFirstSuccess(t *testing.T) {
	var calls atomic.Int32
	executor := newArticleExecutor()
	flow, _, g := newArticleTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls.Add(1)
		text := req.Messages[len(req.Messages)-1].Text()
		if strings.Contains(text, "requirementIndex\":") || strings.Contains(text, "articleId\":") {
			t.Error("identity leaked into tool prompt")
		}
		return toolRequest("search_photo", map[string]any{"keywords": "wood bench"}), nil
	}, executor)
	for range 2 {
		result, err := executeTestImage(t.Context(), flow.images, imageInput())
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != ImageSucceeded || result.Asset == nil {
			t.Fatalf("outcome = %+v", result)
		}
	}
	if calls.Load() != 2 || executor.calls.Load() != 2 {
		t.Fatal("success triggered another model call")
	}
	// Each call uses local tool handles; repeated same-name tools never pollute the shared registry.
	if genkit.LookupTool(g, "search_photo") != nil {
		t.Fatal("request-scoped tool leaked into shared registry")
	}
}

func TestSingleImageRepairsWithAnotherProviderAndKeepsURLsOutOfHistory(t *testing.T) {
	executor := newArticleExecutor()
	executor.execute = func(_ context.Context, _ port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
		if method == port.MethodPexels {
			return port.ImageResult{}, errors.New("private upstream error")
		}
		return port.ImageResult{URL: "mermaid:flowchart TD\nA-->B", Method: method}, nil
	}
	calls := 0
	flow, _, _ := newArticleTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls++
		if calls == 1 {
			return toolRequest("search_photo", map[string]any{"keywords": "wood"}), nil
		}
		if calls != 2 {
			t.Fatal("model called after success")
		}
		history := req.Messages[len(req.Messages)-1]
		if history.Role != ai.RoleTool || len(history.Content) != 1 {
			t.Fatalf("missing error feedback: %+v", history)
		}
		return toolRequest("make_diagram", map[string]any{"prompt": "A to B"}), nil
	}, executor)
	out, err := executeTestImage(t.Context(), flow.images, imageInput())
	if err != nil || out.Status != ImageSucceeded || out.Asset.Method != port.MethodMermaid {
		t.Fatalf("%+v %v", out, err)
	}
	if calls != 2 || executor.calls.Load() != 2 {
		t.Fatal("unexpected attempts")
	}
}

func TestSingleImageBudgetsAndInvalidToolCalls(t *testing.T) {
	for _, scenario := range []string{"unknown", "unauthorized", "repeat", "invalid-input", "multi", "provider-limit"} {
		t.Run(scenario, func(t *testing.T) {
			executor := newArticleExecutor()
			executor.execute = func(context.Context, port.ImageRequirement, port.ImageMethod) (port.ImageResult, error) {
				return port.ImageResult{}, errors.New("no results")
			}
			calls := 0
			flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				calls++
				name := "search_photo"
				args := map[string]any{"keywords": "same"}
				if scenario == "unknown" {
					name = "unknown"
				}
				if scenario == "unauthorized" {
					name = "make_diagram"
				}
				if scenario == "invalid-input" {
					args = map[string]any{"requirementIndex": 0}
				}
				if scenario == "provider-limit" {
					args["keywords"] = fmt.Sprintf("query-%d", calls)
				}
				response := toolRequest(name, args)
				if scenario == "multi" {
					response.Message.Content = append(response.Message.Content, ai.NewToolRequestPart(&ai.ToolRequest{
						Name: "make_diagram", Ref: "call-2", Input: map[string]any{"prompt": "diagram"},
					}))
				}
				return response, nil
			}, executor)
			input := imageInput()
			input.Run.AllowedImageMethods = []port.ImageMethod{port.MethodPexels}
			if scenario == "provider-limit" {
				input.Budget.MaxModelTurns, input.Budget.MaxToolAttempts = 5, 2
			}
			out, err := executeTestImage(t.Context(), flow.images, input)
			if err != nil {
				t.Fatal(err)
			}
			if out.Status != ImageSkipped || out.Warning == nil || out.Asset != nil {
				t.Fatalf("outcome = %+v", out)
			}
			expectedAttempts := int32(0)
			if scenario == "repeat" {
				expectedAttempts = 1
			}
			if scenario == "provider-limit" {
				expectedAttempts = 2
			}
			expectedTurns := 3
			if scenario == "provider-limit" {
				expectedTurns = 2
			}
			if executor.calls.Load() != expectedAttempts || calls != expectedTurns {
				t.Fatalf("calls=%d provider attempts=%d", calls, executor.calls.Load())
			}
		})
	}
}

func TestImageFailurePolicyDistinguishesConfigurationAndCancellation(t *testing.T) {
	for _, modelErr := range []error{errors.New("temporary unavailable"), status.Errorf(status.ErrUnauthenticated, "invalid config")} {
		flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			return nil, modelErr
		}, newArticleExecutor())
		out, err := executeTestImage(t.Context(), flow.images, imageInput())
		if fatalGenerationError(modelErr) {
			if err == nil {
				t.Fatal("configuration failure hidden as missing image")
			}
		} else if err != nil || out.Status != ImageSkipped {
			t.Fatalf("%+v %v", out, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		t.Fatal("canceled run reached model")
		return nil, nil
	}, newArticleExecutor())
	if _, err := executeTestImage(ctx, flow.images, imageInput()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestImageBatchBoundsConcurrencyAndEmitsOrderedProgress(t *testing.T) {
	var inFlight, maxIn atomic.Int32
	executor := newArticleExecutor()
	executor.execute = func(ctx context.Context, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
		active := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			old := maxIn.Load()
			if active <= old || maxIn.CompareAndSwap(old, active) {
				break
			}
		}
		select {
		case <-ctx.Done():
			return port.ImageResult{}, ctx.Err()
		case <-time.After(15 * time.Millisecond):
		}
		if req.Position == 1 {
			return port.ImageResult{}, errors.New("cover unavailable")
		}
		return port.ImageResult{Method: method, URL: "https://example.com/photo.png"}, nil
	}
	flow, _, _ := newArticleTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return toolRequest("search_photo", map[string]any{"keywords": "wood"}), nil
	}, executor)
	tasks := []ImageTask{coverTask()}
	for i := 2; i <= 7; i++ {
		tasks = append(tasks, ImageTask{ID: fmt.Sprintf("image-%d", i), Position: i, PlannedImage: PlannedImage{
			SlotID: fmt.Sprintf("after:block-%d", i), Kind: ImageKindIllustration, Brief: "Wood", PreferredMethod: port.MethodPexels,
		}})
	}
	var summaries []ImageSummary
	var mu sync.Mutex
	out, err := runImageTest(t.Context(), func(ctx context.Context) ([]ImageOutcome, error) {
		return flow.images.executeBatch(ctx, imageInput().Run, tasks, func(_ context.Context, _ ImageOutcome, summary ImageSummary) error {
			mu.Lock()
			defer mu.Unlock()
			summaries = append(summaries, summary)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if maxIn.Load() > imageConcurrency || maxIn.Load() < 2 {
		t.Fatalf("concurrency = %d", maxIn.Load())
	}
	if len(out) != len(tasks) || out[0].Status != ImageSkipped || out[1].Task.Position != 2 {
		t.Fatalf("results = %+v", out)
	}
	for i, summary := range summaries {
		if summary.Settled != i+1 || summary.Validate() != nil {
			t.Fatalf("out-of-order progress: %+v", summaries)
		}
	}
	if summaries[len(summaries)-1].Skipped != 1 {
		t.Fatal("skipped cover not counted as settled")
	}
}

func TestArticleFlowPlansWithoutRewritingAndReturnsWarnings(t *testing.T) {
	body := "## Intro\n\nRead-only body.\n\n## End\n\nOriginal ending."
	executor := newArticleExecutor()
	executor.execute = func(_ context.Context, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
		if req.Position == 1 {
			return port.ImageResult{}, errors.New("cover missing")
		}

		return port.ImageResult{URL: "https://example.com/body.png", Method: method}, nil
	}
	var planCalls atomic.Int32
	flow, logs, _ := newArticleTestFlows(t, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		system := req.Messages[0].Text()
		switch {
		case strings.Contains(system, "按照已经确认的大纲"):
			if cb != nil {
				if err := cb(ctx, &ai.ModelResponseChunk{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart(body)}}); err != nil {
					return nil, err
				}
			}
			return completedJSON(body), nil
		case strings.Contains(system, "文章配图编辑"):
			planCalls.Add(1)
			if strings.Contains(req.Messages[1].Text(), "contentWithPlaceholders") {
				t.Fatal("old rewritten-body contract leaked into planner")
			}
			return completedJSON(`{"images":[{"slotId":"cover","kind":"cover","brief":"Cover","preferredMethod":"PEXELS"},{"slotId":"after:block-1","kind":"illustration","brief":"Body","preferredMethod":"PEXELS"}]}`), nil
		default:
			return toolRequest("search_photo", map[string]any{"keywords": "wood"}), nil
		}
	}, executor)
	var progress []ProgressPayload
	input := GenerateArticleInput{Run: planningRun(), Editorial: planningEditorial(),
		Outline: []article.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}}}
	input.Run.AllowedImageMethods = []port.ImageMethod{port.MethodPexels}
	result, err := flow.GenerateArticle(t.Context(), input, func(_ context.Context, payload ProgressPayload) error {
		progress = append(progress, payload)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != body || len(result.Images) != 1 || result.Images[0].Task.Position != 2 ||
		len(result.Warnings) != 1 || result.Warnings[0].Position != 1 || planCalls.Load() != 1 {
		t.Fatalf("article result = %+v", result)
	}
	if strings.Contains(result.FullContent, "IMAGE_PLACEHOLDER") || !strings.Contains(result.FullContent, "Original ending.") {
		t.Fatalf("invalid merged article: %q", result.FullContent)
	}
	if len(progress) < 6 {
		t.Fatalf("progress = %+v", progress)
	}
	done, ok := progress[len(progress)-1].(ImagesDone)
	if !ok || len(done.Outcomes) != 2 || done.Summary != (ImageSummary{Total: 2, Settled: 2, Succeeded: 1, Skipped: 1}) {
		t.Fatalf("image completion = %+v", progress[len(progress)-1])
	}
	rows := logs.entries()
	if len(rows) != 3 || rows[0].AgentName != "content_generator" || rows[1].AgentName != "image_generator" ||
		rows[2].AgentName != "content_merger" || rows[1].Status != article.AgentLogSuccess {
		t.Fatalf("business logs = %+v", rows)
	}
}
