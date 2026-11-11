package articleworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/core/tracing"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

func runImageTest[Out any](ctx context.Context, execute func(context.Context) (Out, error)) (Out, error) {
	flow := core.NewFlow("imageExecutorTest", func(ctx context.Context, _ struct{}) (Out, error) {
		return execute(ctx)
	})
	return flow.Run(ctx, struct{}{})
}

func executeTestImage(ctx context.Context, executor *imageExecutor, input imageExecutionInput) (ImageOutcome, error) {
	return runImageTest(ctx, func(ctx context.Context) (ImageOutcome, error) {
		return executor.execute(ctx, input)
	})
}

func TestImageExecutorValidatesInputBeforeCallingModel(t *testing.T) {
	_, _, g := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		t.Fatal("invalid image input reached model")
		return nil, nil
	})
	executor, err := newImageExecutor(g, newArticleExecutor())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"task", "budget"} {
		t.Run(field, func(t *testing.T) {
			input := imageInput()
			if field == "task" {
				input.Task.ID = ""
			} else {
				input.Budget.MaxModelTurns = 0
			}
			if _, err := executeTestImage(t.Context(), executor, input); !errors.Is(err, status.ErrInvalidInput) {
				t.Fatalf("invalid input error = %v", err)
			}
		})
	}
}

func TestArticleFlowKeepsImageExecutionAsNestedStep(t *testing.T) {
	var modelPath, providerPath string
	provider := newArticleExecutor()
	provider.execute = func(ctx context.Context, _ port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
		providerPath = tracing.SpanPath(ctx)
		return port.ImageResult{URL: "https://example.com/cover.jpg", Method: method}, nil
	}
	flow, _, g := newArticleTestFlows(t, func(ctx context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		switch system := req.Messages[0].Text(); {
		case strings.Contains(system, "按照已经确认的大纲"):
			return completedJSON("Original article."), nil
		case strings.Contains(system, "文章配图编辑"):
			return completedJSON(`{"images":[{"slotId":"cover","kind":"cover","brief":"Cover","preferredMethod":"PEXELS"}]}`), nil
		default:
			modelPath = tracing.SpanPath(ctx)
			return toolRequest("search_photo", map[string]any{"keywords": "wood"}), nil
		}
	}, provider)
	registered := genkit.ListFlows(g)
	if len(registered) != 4 {
		t.Fatalf("registered %d flows, want only four business entry points", len(registered))
	}
	for _, flow := range registered {
		if flow.Name() == "generateImage" {
			t.Fatal("internal image executor registered as independent flow")
		}
	}
	input := GenerateArticleInput{
		Run: imageInput().Run, Editorial: planningEditorial(),
		Outline: []article.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}},
	}
	out, err := flow.GenerateArticle(t.Context(), input, nil)
	if err != nil || len(out.Images) != 1 {
		t.Fatalf("article result = %+v, err = %v", out, err)
	}
	for _, path := range []string{modelPath, providerPath} {
		articleIndex := strings.Index(path, "generateArticle")
		batchIndex := strings.Index(path, "image_generator")
		imageIndex := strings.Index(path, "generateImage")
		if articleIndex < 0 || batchIndex <= articleIndex || imageIndex <= batchIndex {
			t.Fatalf("image call not nested beneath article and batch steps: %q", path)
		}
	}
}

func coverTask() ImageTask {
	return ImageTask{
		ID:       "image-1",
		Position: 1,
		PlannedImage: PlannedImage{
			SlotID:          CoverSlotID,
			Kind:            ImageKindCover,
			Brief:           "A woodworking bench",
			PreferredMethod: port.MethodPexels,
		},
	}
}

func TestImagePlanContainsPlacementDecisionsOnly(t *testing.T) {
	raw, err := json.Marshal(ImagePlan{Images: []PlannedImage{coverTask().PlannedImage}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"images":[{"slotId":"cover","kind":"cover","brief":"A woodworking bench","preferredMethod":"PEXELS"}]}`
	if string(raw) != want {
		t.Fatalf("plan JSON = %s, want %s", raw, want)
	}
}

func TestDocumentKeepsSourceAndOffsetsOutOfModelInput(t *testing.T) {
	document := ArticleDocument{
		Content: "Original Markdown",
		Blocks: []ContentBlock{{
			ID: "block-1", Markdown: "Original Markdown", AfterSlotID: "after:block-1",
		}},
		Slots: []ImageSlot{{ID: CoverSlotID}, {ID: "after:block-1", ByteOffset: 17}},
	}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"blocks":[{"id":"block-1","markdown":"Original Markdown","afterSlotId":"after:block-1"}],"slots":[{"id":"cover"},{"id":"after:block-1"}]}`
	if string(raw) != want {
		t.Fatalf("document JSON = %s, want %s", raw, want)
	}
}

func TestImagePlanValidation(t *testing.T) {
	cover := coverTask().PlannedImage
	body := PlannedImage{
		SlotID: "after:block-1", Kind: ImageKindIllustration,
		Brief: "Three processing steps", PreferredMethod: port.MethodMermaid,
	}
	invalidCover := cover
	invalidCover.SlotID = body.SlotID
	invalidBody := body
	invalidBody.SlotID = CoverSlotID
	blankBrief := cover
	blankBrief.Brief = " "
	unknownKind := cover
	unknownKind.Kind = "unknown"
	fallback := cover
	fallback.PreferredMethod = port.MethodPicsum
	noncanonical := cover
	noncanonical.PreferredMethod = "pexels"
	unknownMethod := cover
	unknownMethod.PreferredMethod = "UNKNOWN"
	slots := []ImageSlot{{ID: CoverSlotID}, {ID: body.SlotID}}
	methods := []port.ImageMethod{port.MethodPexels, port.MethodMermaid}

	tests := []struct {
		name    string
		images  []PlannedImage
		slots   []ImageSlot
		methods []port.ImageMethod
		wantErr bool
	}{
		{"cover and illustration", []PlannedImage{cover, body}, slots, methods, false},
		{"no image requirements", []PlannedImage{}, slots, nil, false},
		{"unknown slot", []PlannedImage{cover}, slots[1:], methods, true},
		{"duplicate slot definitions", nil, append(slots, slots[0]), methods, true},
		{"empty slot definition", nil, []ImageSlot{{}}, methods, true},
		{"repeated placement", []PlannedImage{cover, cover}, slots, methods, true},
		{"nil grants deny tools", []PlannedImage{cover}, slots, nil, true},
		{"empty grants deny tools", []PlannedImage{cover}, slots, []port.ImageMethod{}, true},
		{"unauthorized provider", []PlannedImage{body}, slots, methods[:1], true},
		{"cover in body", []PlannedImage{invalidCover}, slots, methods, true},
		{"illustration in cover", []PlannedImage{invalidBody}, slots, methods, true},
		{"blank brief", []PlannedImage{blankBrief}, slots, methods, true},
		{"unknown image kind", []PlannedImage{unknownKind}, slots, methods, true},
		{"noncanonical provider", []PlannedImage{noncanonical}, slots, methods, true},
		{"unknown provider", []PlannedImage{unknownMethod}, slots, []port.ImageMethod{"UNKNOWN"}, true},
		{"fallback cannot be granted", []PlannedImage{fallback}, slots, []port.ImageMethod{port.MethodPicsum}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (ImagePlan{Images: tt.images}).Validate(tt.slots, tt.methods)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImageTaskReservesCoverPosition(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*ImageTask)
		wantErr bool
	}{
		{"cover", func(*ImageTask) {}, false},
		{"missing id", func(task *ImageTask) { task.ID = "" }, true},
		{"nonpositive position", func(task *ImageTask) { task.Position = 0 }, true},
		{"cover cannot move", func(task *ImageTask) { task.Position = 2 }, true},
		{"body cannot become cover", func(task *ImageTask) {
			task.Kind, task.SlotID = ImageKindIllustration, "after:block-1"
		}, true},
		{"body after skipped cover", func(task *ImageTask) {
			task.Kind, task.SlotID, task.Position = ImageKindIllustration, "after:block-1", 2
		}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			task := coverTask()
			tt.mutate(&task)
			if err := task.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func skippedOutcome() ImageOutcome {
	task := coverTask()
	return ImageOutcome{
		Task: task, Status: ImageSkipped,
		Warning: &article.GenerationWarning{
			Code: article.WarningImageSkipped, ImageID: task.ID,
			Position: task.Position, Message: "No usable image after bounded attempts",
		},
	}
}

func TestImageOutcomeValidation(t *testing.T) {
	asset := ImageAsset{URL: "https://example.com/bench.jpg", Method: port.MethodPexels}
	tests := []struct {
		name    string
		mutate  func(*ImageOutcome)
		wantErr bool
	}{
		{"skipped", func(*ImageOutcome) {}, false},
		{"succeeded", func(out *ImageOutcome) {
			out.Status, out.Asset, out.Warning = ImageSucceeded, &asset, nil
		}, false},
		{"skipped with asset", func(out *ImageOutcome) { out.Asset = &asset }, true},
		{"skipped without warning", func(out *ImageOutcome) { out.Warning = nil }, true},
		{"succeeded without asset", func(out *ImageOutcome) {
			out.Status, out.Warning = ImageSucceeded, nil
		}, true},
		{"succeeded with warning", func(out *ImageOutcome) {
			out.Status, out.Asset = ImageSucceeded, &asset
		}, true},
		{"empty asset", func(out *ImageOutcome) {
			out.Status, out.Asset, out.Warning = ImageSucceeded, &ImageAsset{}, nil
		}, true},
		{"fallback asset", func(out *ImageOutcome) {
			out.Status, out.Warning = ImageSucceeded, nil
			out.Asset = &ImageAsset{URL: "https://picsum.photos/800", Method: port.MethodPicsum}
		}, true},
		{"noncanonical asset provider", func(out *ImageOutcome) {
			out.Status, out.Warning = ImageSucceeded, nil
			out.Asset = &ImageAsset{URL: asset.URL, Method: "pexels"}
		}, true},
		{"unknown asset provider", func(out *ImageOutcome) {
			out.Status, out.Warning = ImageSucceeded, nil
			out.Asset = &ImageAsset{URL: asset.URL, Method: "UNKNOWN"}
		}, true},
		{"wrong warning task", func(out *ImageOutcome) { out.Warning.ImageID = "another-image" }, true},
		{"wrong warning position", func(out *ImageOutcome) { out.Warning.Position = 2 }, true},
		{"wrong warning code", func(out *ImageOutcome) { out.Warning.Code = "UNKNOWN" }, true},
		{"missing reason", func(out *ImageOutcome) { out.Warning.Message = " " }, true},
		{"unknown status", func(out *ImageOutcome) { out.Status = "failed" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := skippedOutcome()
			tt.mutate(&out)
			if err := out.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImageSummaryCountsSkippedTasksAsSettled(t *testing.T) {
	tests := []struct {
		summary ImageSummary
		wantErr bool
	}{
		{ImageSummary{}, false},
		{ImageSummary{Total: 4, Settled: 4, Succeeded: 3, Skipped: 1}, false},
		{ImageSummary{Total: 4, Settled: 2, Succeeded: 1, Skipped: 1}, false},
		{ImageSummary{Total: 1, Settled: 2, Succeeded: 2}, true},
		{ImageSummary{Total: 4, Settled: 3, Succeeded: 3, Skipped: 1}, true},
		{ImageSummary{Total: -1}, true},
	}
	for _, tt := range tests {
		if err := tt.summary.Validate(); (err != nil) != tt.wantErr {
			t.Errorf("%+v: Validate() = %v, wantErr %v", tt.summary, err, tt.wantErr)
		}
	}
}

func TestImageExecutionBudgetRequiresBothBounds(t *testing.T) {
	for _, budget := range []ImageExecutionBudget{
		{}, {MaxModelTurns: 3}, {MaxToolAttempts: 3},
		{MaxModelTurns: -1, MaxToolAttempts: 3},
	} {
		if err := budget.Validate(); err == nil {
			t.Errorf("expected invalid budget: %+v", budget)
		}
	}
	if err := (ImageExecutionBudget{MaxModelTurns: 3, MaxToolAttempts: 3}).Validate(); err != nil {
		t.Fatal(err)
	}
}
