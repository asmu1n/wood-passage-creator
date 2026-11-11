package articleapp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/firebase/genkit/go/ai"

	"wood-passage-creator/internal/app/article/workflow"
	module "wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/sse"
	"wood-passage-creator/internal/port"
)

type articleFlowStub struct {
	run func(context.Context, articleworkflow.GenerateArticleInput, articleworkflow.EmitProgress) (articleworkflow.ArticleResult, error)
}

func (s articleFlowStub) GenerateArticle(ctx context.Context, input articleworkflow.GenerateArticleInput, emit articleworkflow.EmitProgress) (articleworkflow.ArticleResult, error) {
	return s.run(ctx, input, emit)
}

func TestPhase3SavesMissingImageWarningsAndKeepsArticleCompleted(t *testing.T) {
	service, repo, events := planningService(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		t.Fatal("Phase3 must not execute a planning flow")
		return nil, nil
	})
	repo.stored.Outline = []module.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}}
	warning := module.GenerationWarning{Code: module.WarningImageSkipped, ImageID: "image-1", Position: 1, Message: "Cover skipped"}
	task := articleworkflow.ImageTask{ID: "image-1", Position: 1, PlannedImage: articleworkflow.PlannedImage{
		SlotID: "cover", Kind: articleworkflow.ImageKindCover, Brief: "Cover", PreferredMethod: port.MethodPexels,
	}}
	outcome := articleworkflow.ImageOutcome{Task: task, Status: articleworkflow.ImageSkipped, Warning: &warning}
	service.articleFlow = articleFlowStub{run: func(ctx context.Context, input articleworkflow.GenerateArticleInput, emit articleworkflow.EmitProgress) (articleworkflow.ArticleResult, error) {
		if input.Run.TaskID != repo.stored.TaskID || input.Editorial.UserDescription != "Keep local examples" {
			t.Fatalf("article input lost task/editorial context: %+v", input)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("article deadline is missing")
		}
		for _, payload := range []articleworkflow.ProgressPayload{
			articleworkflow.ContentDelta{Delta: "Original body"},
			articleworkflow.ContentGenerated{Content: "Original body"},
			articleworkflow.ImagesPlanned{Tasks: []articleworkflow.ImageTask{task}},
			articleworkflow.ImageFinished{Outcome: outcome,
				Summary: articleworkflow.ImageSummary{Total: 1, Settled: 1, Skipped: 1}},
			articleworkflow.ImagesDone{Outcomes: []articleworkflow.ImageOutcome{outcome}, Summary: articleworkflow.ImageSummary{Total: 1, Settled: 1, Skipped: 1}},
		} {
			if err := emit(ctx, payload); err != nil {
				return articleworkflow.ArticleResult{}, err
			}
		}
		return articleworkflow.ArticleResult{Content: "Original body", FullContent: "Original body", Images: []articleworkflow.GeneratedImage{}, Warnings: []module.GenerationWarning{warning}}, nil
	}}
	service.runPhase3Async(repo.stored)
	if repo.stored.Status != module.StatusCompleted || repo.stored.Phase != module.PhaseCompleted ||
		repo.stored.CompletedTime == nil || repo.stored.FullContent == nil || *repo.stored.FullContent != "Original body" ||
		!reflect.DeepEqual(repo.stored.GenerationWarnings, []module.GenerationWarning{warning}) {
		t.Fatalf("completed article not persisted: %+v", repo.stored)
	}
	var last sse.SSEEvent
	for len(events) > 0 {
		last = <-events
	}
	if last.Name != string(articleworkflow.EventContentDone) {
		t.Fatalf("final event = %s", last.Name)
	}
	if repo.stored.ErrorMessage != nil {
		t.Fatal("nonfatal warning stored as fatal error")
	}
}

func TestPhase3PropagatesFatalFailureWithoutSavingCompletion(t *testing.T) {
	service, repo, events := planningService(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return nil, nil
	})
	service.articleFlow = articleFlowStub{run: func(context.Context, articleworkflow.GenerateArticleInput, articleworkflow.EmitProgress) (articleworkflow.ArticleResult, error) {
		return articleworkflow.ArticleResult{}, errors.New("invalid image plan")
	}}
	service.runPhase3Async(repo.stored)
	if repo.stored.Status != module.StatusFailed || repo.stored.CompletedTime != nil || repo.stored.Content != nil {
		t.Fatalf("fatal failure marked completed: %+v", repo.stored)
	}
	nextPlanningEvent(t, events, string(articleworkflow.EventTaskSnapshot))
	nextPlanningEvent(t, events, string(articleworkflow.EventTaskError))
}

func TestHistoricalImageShapePreservesCoverIdentity(t *testing.T) {
	images := storedImages([]articleworkflow.GeneratedImage{{
		Task:  articleworkflow.ImageTask{Position: 2, PlannedImage: articleworkflow.PlannedImage{Brief: "Body picture"}},
		Asset: articleworkflow.ImageAsset{URL: "https://example.com/image.jpg", Method: port.MethodPexels},
	}})
	article := &module.Article{Images: images}
	article.FillCoverImage()
	if article.CoverImage != nil || len(images) != 1 || images[0].Position != 2 {
		t.Fatal("body image promoted after skipped cover")
	}
}
