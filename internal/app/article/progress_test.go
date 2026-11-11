package articleapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"wood-passage-creator/internal/app/article/workflow"
	module "wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/sse"
	"wood-passage-creator/internal/port"
)

func progressArticle() *module.Article {
	return &module.Article{ID: 1, TaskID: "task", Status: module.StatusProcessing, Phase: module.PhaseContentGenerating}
}

func snapshotEvent(t *testing.T, ch <-chan sse.SSEEvent) articleworkflow.Event[articleworkflow.TaskSnapshot] {
	t.Helper()
	event := nextPlanningEvent(t, ch, string(articleworkflow.EventTaskSnapshot))
	var out articleworkflow.Event[articleworkflow.TaskSnapshot]
	if err := json.Unmarshal(event.Data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestProgressSubscriptionIsPrivateAndStartsAtCurrentRevision(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	run, err := p.begin(a, a.Phase)
	if err != nil {
		t.Fatal(err)
	}
	load := func(context.Context) (*module.Article, error) { return a, nil }
	first, cancel1, err := p.subscribe(t.Context(), a.TaskID, load)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel1()
	if snapshot := snapshotEvent(t, first); snapshot.Seq != 0 || snapshot.RunID != run.RunID {
		t.Fatalf("first snapshot = %+v", snapshot)
	}
	if err := p.emit(a.TaskID, a.Phase, articleworkflow.ContentDelta{Delta: "One"}); err != nil {
		t.Fatal(err)
	}
	event := nextPlanningEvent(t, first, string(articleworkflow.EventContentDelta))
	var delta articleworkflow.Event[articleworkflow.ContentDelta]
	if err := json.Unmarshal(event.Data, &delta); err != nil {
		t.Fatal(err)
	}
	if delta.Seq != 1 || delta.Data.Delta != "One" {
		t.Fatalf("delta = %+v", delta)
	}
	second, cancel2, err := p.subscribe(t.Context(), a.TaskID, load)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel2()
	if snapshot := snapshotEvent(t, second); snapshot.Seq != 1 || snapshot.Data.Content != "One" {
		t.Fatalf("late snapshot = %+v", snapshot)
	}
	select {
	case event := <-first:
		t.Fatalf("another subscription broadcast its snapshot: %+v", event)
	default:
	}
}

func TestProgressSnapshotCopiesPartialOutline(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	if _, err := p.begin(a, module.PhaseOutlineGenerating); err != nil {
		t.Fatal(err)
	}
	section := articleworkflow.OutlinePreviewSection{Section: new(1), Title: new("First"), Points: []string{"Point"}}
	if err := p.emit(a.TaskID, module.PhaseOutlineGenerating, articleworkflow.OutlineSnapshot{Sections: []articleworkflow.OutlinePreviewSection{section}}); err != nil {
		t.Fatal(err)
	}
	*section.Title = "Modified"
	section.Points[0] = "Modified"
	ch, cancel, err := p.subscribe(t.Context(), a.TaskID, func(context.Context) (*module.Article, error) { return a, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	got := snapshotEvent(t, ch)
	if *got.Data.Outline[0].Title != "First" || got.Data.Outline[0].Points[0] != "Point" {
		t.Fatalf("mutable snapshot = %+v", got)
	}
}

func TestProgressSlowConsumerReconnectsToFullContent(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	if _, err := p.begin(a, a.Phase); err != nil {
		t.Fatal(err)
	}
	load := func(context.Context) (*module.Article, error) { return a, nil }
	ch, cancel, err := p.subscribe(t.Context(), a.TaskID, load)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	for range 100 {
		if err := p.emit(a.TaskID, a.Phase, articleworkflow.ContentDelta{Delta: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	for range ch {
	}
	reconnected, closeAgain, err := p.subscribe(t.Context(), a.TaskID, load)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAgain()
	got := snapshotEvent(t, reconnected)
	if got.Seq != 100 || got.Data.Content != strings.Repeat("x", 100) {
		t.Fatalf("lost current projection after overflow: %+v", got)
	}
}

func TestProgressTerminalSnapshotComesFromCommittedArticle(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	run, err := p.begin(a, a.Phase)
	if err != nil {
		t.Fatal(err)
	}
	body := "Saved body"
	result := articleworkflow.ContentDone{Content: body, FullContent: body, Images: []articleworkflow.GeneratedImage{}, Warnings: []module.GenerationWarning{}}
	if err := p.complete(a.TaskID, run.RunID, module.PhaseCompleted, result, func() error {
		a.Status = module.StatusCompleted
		a.Phase = module.PhaseCompleted
		a.Content = &body
		a.FullContent = &body
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(p.active) != 0 {
		t.Fatal("terminal runs must not retain full article content indefinitely")
	}
	ch, cancel, err := p.subscribe(t.Context(), a.TaskID, func(context.Context) (*module.Article, error) { return a, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	event := nextPlanningEvent(t, ch, string(articleworkflow.EventTaskSnapshot))
	if !event.Terminal {
		t.Fatal("late terminal snapshot did not end subscription")
	}
	var got articleworkflow.Event[articleworkflow.TaskSnapshot]
	if err := json.Unmarshal(event.Data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.FullContent != body || !got.Data.Terminal || got.Data.Status != module.StatusCompleted {
		t.Fatalf("terminal snapshot = %+v", got)
	}
}

func TestProgressDoesNotPublishSuccessWhenPersistenceFails(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	run, _ := p.begin(a, a.Phase)
	ch, cancel, _ := p.subscribe(t.Context(), a.TaskID, func(context.Context) (*module.Article, error) { return a, nil })
	defer cancel()
	snapshotEvent(t, ch)
	want := errors.New("database unavailable")
	err := p.complete(a.TaskID, run.RunID, module.PhaseCompleted, articleworkflow.ContentDone{}, func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if p.active[a.TaskID].seq != 0 {
		t.Fatal("failed persistence advanced the progress revision")
	}
	select {
	case event := <-ch:
		t.Fatalf("uncommitted completion published: %+v", event)
	default:
	}
}

func TestProgressSubscribeAndPublishAreAtomic(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	p.begin(a, a.Phase)
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			if err := p.emit(a.TaskID, a.Phase, articleworkflow.ContentDelta{Delta: "x"}); err != nil {
				t.Error(err)
			}
		}
	})
	for range 20 {
		ch, cancel, err := p.subscribe(t.Context(), a.TaskID, func(context.Context) (*module.Article, error) { return a, nil })
		if err != nil {
			t.Fatal(err)
		}
		first := snapshotEvent(t, ch)
		select {
		case event, ok := <-ch:
			if ok {
				var next articleworkflow.Event[articleworkflow.ContentDelta]
				json.Unmarshal(event.Data, &next)
				if next.Seq != first.Seq+1 {
					t.Errorf("subscription gap: %d -> %d", first.Seq, next.Seq)
				}
			}
		default:
		}
		cancel()
	}
	wg.Wait()
}

func TestPersistedSnapshotKeepsHistoricalProvidersAndSafeErrors(t *testing.T) {
	a := progressArticle()
	a.Status = module.StatusFailed
	a.Images = []port.ImageResult{{Position: 1, URL: "https://picsum.photos/seed/1/800/600", Method: port.MethodPicsum}}
	a.ErrorMessage = new("private upstream credentials")
	got := persistedSnapshot(a)
	if len(got.SavedImages) != 1 || got.SavedImages[0].Method != port.MethodPicsum {
		t.Fatal("historical image lost")
	}
	if !got.Terminal || got.Error == nil || strings.Contains(got.Error.Message, "private") {
		t.Fatalf("unsafe error snapshot: %+v", got)
	}
}

func TestProgressRejectsOutOfOrderImageSummary(t *testing.T) {
	p := newProgressTracker(sse.NewHub())
	a := progressArticle()
	p.begin(a, a.Phase)
	task := articleworkflow.ImageTask{ID: "image-1", Position: 1, PlannedImage: articleworkflow.PlannedImage{
		SlotID: "cover", Kind: articleworkflow.ImageKindCover, Brief: "Cover", PreferredMethod: port.MethodPexels,
	}}
	if err := p.emit(a.TaskID, a.Phase, articleworkflow.ImagesPlanned{Tasks: []articleworkflow.ImageTask{task}}); err != nil {
		t.Fatal(err)
	}
	err := p.emit(a.TaskID, a.Phase, articleworkflow.ImagesDone{Summary: articleworkflow.ImageSummary{Total: 1}})
	if err == nil {
		t.Fatal("unfinished image batch accepted")
	}
}

func Example_publicGenerationError() {
	fmt.Println(publicGenerationError(context.Background(), errors.New("private error")).Code)
	// Output: GENERATION_FAILED
}
