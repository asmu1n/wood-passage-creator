package articleapp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/app/article/workflow"
	module "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/pkg/sse"
	"wood-passage-creator/internal/port"
)

type planningArticleRepo struct {
	module.Repository
	stored        *module.Article
	titleWrites   int
	outlineWrites int
}

func (r *planningArticleRepo) GetByTaskID(context.Context, string) (*module.Article, error) {
	copy := *r.stored
	return &copy, nil
}

func (r *planningArticleRepo) UpdateStatus(_ context.Context, _ string, status module.ArticleStatus) error {
	r.stored.Status = status
	return nil
}

func (r *planningArticleRepo) UpdatePhase(_ context.Context, _ string, phase module.ArticlePhase) error {
	r.stored.Phase = phase
	return nil
}

func (r *planningArticleRepo) UpdateTitleOptions(_ context.Context, _ string, options []module.TitleOption) error {
	r.titleWrites++
	r.stored.TitleOptions = options
	return nil
}

func (r *planningArticleRepo) UpdateOutline(_ context.Context, _ string, outline []module.OutlineSection) error {
	r.outlineWrites++
	r.stored.Outline = outline
	return nil
}

func (r *planningArticleRepo) Update(ctx context.Context, _ int64, params module.UpdateArticleParams) (*module.Article, error) {
	if params.TitleOptions != nil {
		if err := r.UpdateTitleOptions(ctx, r.stored.TaskID, params.TitleOptions); err != nil {
			return nil, err
		}
	}
	if params.Outline != nil {
		if err := r.UpdateOutline(ctx, r.stored.TaskID, params.Outline); err != nil {
			return nil, err
		}
	}
	if params.Status != nil {
		r.stored.Status = *params.Status
	}
	if params.Phase != nil {
		r.stored.Phase = *params.Phase
	}
	if params.Content != nil {
		r.stored.Content = params.Content
	}
	if params.FullContent != nil {
		r.stored.FullContent = params.FullContent
	}
	if params.Images != nil {
		r.stored.Images = params.Images
	}
	if params.GenerationWarnings != nil {
		r.stored.GenerationWarnings = params.GenerationWarnings
	}
	r.stored.CompletedTime = params.CompletedTime
	if params.ErrorMessage != nil {
		r.stored.ErrorMessage = params.ErrorMessage
	}
	return r.GetByTaskID(ctx, r.stored.TaskID)
}

func (r *planningArticleRepo) UpdateByTaskID(ctx context.Context, _ string, params module.UpdateArticleParams) (*module.Article, error) {
	return r.Update(ctx, r.stored.ID, params)
}

type unusedArticleWorkflow struct{}

func (unusedArticleWorkflow) GenerateArticle(context.Context, articleworkflow.GenerateArticleInput, articleworkflow.EmitProgress) (articleworkflow.ArticleResult, error) {
	panic("planning must not execute Phase3")
}

func planningService(t *testing.T, model func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error)) (*Service, *planningArticleRepo, <-chan sse.SSEEvent) {
	t.Helper()
	t.Setenv("GENKIT_REFLECTION_ENABLED", "false")
	g := genkit.Init(t.Context(), genkit.WithPromptFS(articleworkflow.PromptFS), genkit.WithDefaultModel("test/planning"))
	genkit.DefineModelAction(g, "test/planning", &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, SystemRole: true},
	}, func(ctx context.Context, req *ai.ModelRequest, _ struct{}, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 10*time.Minute {
			t.Error("planning request is missing the phase deadline")
		}
		return model(ctx, req, cb)
	})
	flows, err := articleworkflow.NewPlanningFlows(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &planningArticleRepo{stored: &module.Article{
		ID: 7, TaskID: "task-1", UserID: 11, Topic: "Woodworking",
		MainTitle: new("A practical guide"), SubTitle: new("For beginners"),
		Style: new(module.StyleEducational), UserDescription: new("Keep local examples"),
		Phase: module.PhaseTitleSelecting,
	}}
	hub := sse.NewHub()
	ch, cancel := hub.Subscribe(repo.stored.TaskID)
	t.Cleanup(cancel)
	service := NewService(repo, nil, nil, flows, unusedArticleWorkflow{}, nil, hub, nil)
	return service, repo, ch
}

func fakePlanningResponse(text string) *ai.ModelResponse {
	return &ai.ModelResponse{FinishReason: ai.FinishReasonStop, Message: ai.NewModelTextMessage(text)}
}

func nextPlanningEvent(t *testing.T, ch <-chan sse.SSEEvent, name string) sse.SSEEvent {
	t.Helper()
	select {
	case event := <-ch:
		if event.Name != name {
			t.Fatalf("event=%s, want %s", event.Name, name)
		}
		return event
	default:
		t.Fatalf("missing event %s", name)
		return sse.SSEEvent{}
	}
}

func TestPhase1UsesGenkitAndPreservesTitleSelection(t *testing.T) {
	service, repo, events := planningService(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		if !strings.Contains(req.Messages[1].Text(), "Woodworking") {
			t.Error("title flow lost the requested topic")
		}
		return fakePlanningResponse(`{"options":[{"mainTitle":"One","subTitle":"First"},{"mainTitle":"Two","subTitle":"Second"},{"mainTitle":"Three","subTitle":"Third"}]}`), nil
	})
	service.runPhase1Async(repo.stored.TaskID, repo.stored.Topic, repo.stored.Style)
	nextPlanningEvent(t, events, string(articleworkflow.EventTaskSnapshot))
	if repo.titleWrites != 1 || repo.stored.Phase != module.PhaseTitleSelecting ||
		repo.stored.Status != module.StatusProcessing || len(repo.stored.TitleOptions) != 3 {
		t.Fatalf("title selection state not persisted: %+v", repo)
	}
	event := nextPlanningEvent(t, events, string(articleworkflow.EventTitlesDone))
	var payload articleworkflow.Event[articleworkflow.TitlesDone]
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Phase != module.PhaseTitleSelecting || !reflect.DeepEqual(payload.Data.Options, repo.stored.TitleOptions) || !event.Terminal {
		t.Fatalf("title SSE contract changed: %+v", payload)
	}
}

func TestPhase2UsesGenkitAndPublishesOutlineSnapshots(t *testing.T) {
	parts := []string{`{"sections":[{"section":1,"title":"Intro`, `duction","points":["Start here"]}]}`}
	service, repo, events := planningService(t, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		if !strings.Contains(req.Messages[0].Text(), "使用教育风格") ||
			!strings.Contains(req.Messages[1].Text(), "Keep local examples") {
			t.Error("outline flow lost editorial context")
		}
		for _, part := range parts {
			if err := cb(ctx, &ai.ModelResponseChunk{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart(part)}}); err != nil {
				return nil, err
			}
		}
		return fakePlanningResponse(strings.Join(parts, "")), nil
	})
	service.runPhase2Async(repo.stored)
	nextPlanningEvent(t, events, string(articleworkflow.EventTaskSnapshot))
	if repo.outlineWrites != 1 || repo.stored.Phase != module.PhaseOutlineEditing || len(repo.stored.Outline) != 1 {
		t.Fatalf("outline editing state not persisted: %+v", repo)
	}
	for range parts {
		event := nextPlanningEvent(t, events, string(articleworkflow.EventOutlineSnapshot))
		var payload articleworkflow.Event[articleworkflow.OutlineSnapshot]
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Data.Sections) == 0 {
			t.Fatalf("missing structured preview: %+v", payload)
		}
	}
	event := nextPlanningEvent(t, events, string(articleworkflow.EventOutlineDone))
	var payload articleworkflow.Event[articleworkflow.OutlineDone]
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Phase != module.PhaseOutlineEditing || !reflect.DeepEqual(payload.Data.Sections, repo.stored.Outline) || !event.Terminal {
		t.Fatalf("outline SSE contract changed: %+v", payload)
	}
}

func TestPlanningFailurePersistsTaskErrorWithoutPartialResults(t *testing.T) {
	for _, phase := range []int{1, 2} {
		t.Run(string(rune('0'+phase)), func(t *testing.T) {
			service, repo, events := planningService(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				return nil, errors.New("model unavailable")
			})
			if phase == 1 {
				service.runPhase1Async(repo.stored.TaskID, repo.stored.Topic, repo.stored.Style)
			} else {
				service.runPhase2Async(repo.stored)
			}
			if repo.stored.Status != module.StatusFailed || repo.stored.ErrorMessage == nil ||
				repo.titleWrites != 0 || repo.outlineWrites != 0 {
				t.Fatalf("failure state not preserved: %+v", repo)
			}
			nextPlanningEvent(t, events, string(articleworkflow.EventTaskSnapshot))
			nextPlanningEvent(t, events, string(articleworkflow.EventTaskError))
		})
	}
}

func TestModifyOutlineUsesGenkitAndRetainsPermissions(t *testing.T) {
	calls := 0
	service, repo, _ := planningService(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls++
		if !strings.Contains(req.Messages[0].Text(), "使用教育风格") ||
			!strings.Contains(req.Messages[1].Text(), "Keep local examples") ||
			!strings.Contains(req.Messages[1].Text(), "Expand the introduction") {
			t.Error("revision lost style, original requirements or new feedback")
		}
		return fakePlanningResponse(`{"sections":[{"section":1,"title":"Revised","points":["Expanded point"]}]}`), nil
	})
	repo.stored.Phase = module.PhaseOutlineEditing
	repo.stored.Outline = []module.OutlineSection{{Section: 1, Title: "Original", Points: []string{"Point"}}}
	request := AiModifyOutlineRequest{TaskID: repo.stored.TaskID, ModifySuggestion: "Expand the introduction"}
	_, err := service.ModifyOutline(t.Context(), moduser.Actor{ID: repo.stored.UserID, Role: moduser.RoleUser}, request)
	assertBizCode(t, err, response.Forbidden.Biz)
	if calls != 0 {
		t.Fatal("non-VIP caller reached the model")
	}
	out, err := service.ModifyOutline(t.Context(), moduser.Actor{ID: repo.stored.UserID, Role: moduser.RoleVIP}, request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || repo.outlineWrites != 1 || !reflect.DeepEqual(out, repo.stored.Outline) || out[0].Title != "Revised" {
		t.Fatalf("revision was not saved: result=%+v repo=%+v", out, repo)
	}
}

func TestPlanningRunSnapshotsExplicitGrants(t *testing.T) {
	source := &module.Article{ID: 7, TaskID: "task", EnabledImageMethods: []port.ImageMethod{port.MethodPexels}}
	run := newPlanningRun(source)
	source.EnabledImageMethods[0] = port.MethodNanoBanana
	if run.RunID == "" || run.ArticleID != source.ID || run.TaskID != source.TaskID ||
		!reflect.DeepEqual(run.AllowedImageMethods, []port.ImageMethod{port.MethodPexels}) {
		t.Fatalf("run identity or grants were not snapshotted: %+v", run)
	}

}

func TestFailedOutlineRevisionDoesNotOverwriteSavedOutline(t *testing.T) {
	service, repo, _ := planningService(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return nil, errors.New("model unavailable")
	})
	repo.stored.Phase = module.PhaseOutlineEditing
	original := []module.OutlineSection{{Section: 1, Title: "Original", Points: []string{"Point"}}}
	repo.stored.Outline = original
	_, err := service.ModifyOutline(t.Context(), moduser.Actor{ID: repo.stored.UserID, Role: moduser.RoleVIP},
		AiModifyOutlineRequest{TaskID: repo.stored.TaskID, ModifySuggestion: "Expand"})
	if err == nil || repo.outlineWrites != 0 || !reflect.DeepEqual(repo.stored.Outline, original) ||
		repo.stored.Phase != module.PhaseOutlineEditing {
		t.Fatalf("failed revision changed the saved article: error=%v article=%+v", err, repo.stored)
	}
}
