package articleapp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"wood-passage-creator/internal/app/article/workflow"
	module "wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/sse"
)

type taskProgress struct {
	runID    string
	seq      uint64
	phase    module.ArticlePhase
	snapshot articleworkflow.TaskSnapshot
	tasks    map[string]articleworkflow.ImageTask
	settled  map[string]bool
}

type progressTracker struct {
	mu     sync.Mutex
	active map[string]*taskProgress
	hub    sse.SSEHub
}

func newProgressTracker(hub sse.SSEHub) *progressTracker {
	return &progressTracker{active: map[string]*taskProgress{}, hub: hub}
}

func persistedSnapshot(a *module.Article) articleworkflow.TaskSnapshot {
	snapshot := articleworkflow.TaskSnapshot{
		Topic:  a.Topic,
		Status: a.Status, Editorial: new(articleEditorial(a)), SavedImages: slices.Clone(a.Images),
		TitleOptions: slices.Clone(a.TitleOptions), Warnings: slices.Clone(a.GenerationWarnings),
		Terminal: a.Status == module.StatusFailed || a.Phase == module.PhaseTitleSelecting ||
			a.Phase == module.PhaseOutlineEditing || a.Phase == module.PhaseCompleted,
	}
	for _, section := range a.Outline {
		snapshot.Outline = append(snapshot.Outline, articleworkflow.OutlinePreviewSection{
			Section: new(section.Section), Title: new(section.Title), Points: slices.Clone(section.Points),
		})
	}
	if a.Content != nil {
		snapshot.Content = *a.Content
	}
	if a.FullContent != nil {
		snapshot.FullContent = *a.FullContent
	}
	snapshot.ImageSummary = articleworkflow.ImageSummary{
		Total: len(a.Images) + len(a.GenerationWarnings), Settled: len(a.Images) + len(a.GenerationWarnings),
		Succeeded: len(a.Images), Skipped: len(a.GenerationWarnings),
	}
	switch a.Phase {
	case module.PhasePending, module.PhaseTitleGenerating, module.PhaseTitleSelecting:
		snapshot.Step = "titles"
	case module.PhaseOutlineGenerating, module.PhaseOutlineEditing:
		snapshot.Step = "outline"
	case module.PhaseContentGenerating:
		snapshot.Step = "content"
	case module.PhaseCompleted:
		snapshot.Step = "completed"
	}
	if a.Status == module.StatusFailed {
		snapshot.Error = &articleworkflow.TaskError{Code: "GENERATION_FAILED", Message: "文章生成失败，请稍后重新创建"}
	}
	return snapshot
}

func (p *progressTracker) begin(a *module.Article, phase module.ArticlePhase) (articleworkflow.RunContext, error) {
	run := newPlanningRun(a)
	state := &taskProgress{runID: run.RunID, phase: phase, snapshot: persistedSnapshot(a)}
	state.snapshot.Status = module.StatusProcessing
	state.snapshot.Terminal = false
	state.snapshot.Error = nil
	switch phase {
	case module.PhaseTitleGenerating:
		state.snapshot.Step = "titles"
	case module.PhaseOutlineGenerating:
		state.snapshot.Step = "outline"
	case module.PhaseContentGenerating:
		state.snapshot.Step = "content"
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active[a.TaskID] = state
	event, err := progressEvent(a.TaskID, state, state.snapshot)
	if err != nil {
		return run, err
	}
	if p.hub != nil {
		p.hub.Publish(event)
	}
	return run, nil
}

func progressEvent(taskID string, state *taskProgress, payload articleworkflow.ProgressPayload) (sse.SSEEvent, error) {
	event := articleworkflow.Event[articleworkflow.ProgressPayload]{
		RunID: state.runID, Seq: state.seq, Phase: state.phase, Data: payload,
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return sse.SSEEvent{}, err
	}
	return sse.SSEEvent{Topic: taskID, Name: string(event.Name()), Data: raw, Terminal: event.IsTerminal()}, nil
}

func (p *progressTracker) emit(taskID string, phase module.ArticlePhase, payload articleworkflow.ProgressPayload) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.emitLocked(taskID, phase, payload)
}

// Persisting the confirmation boundary and publishing its terminal event share
// the subscription lock, so a newly started phase cannot overtake completion.
func (p *progressTracker) complete(taskID, runID string, phase module.ArticlePhase, payload articleworkflow.ProgressPayload, persist func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.active[taskID]
	if state == nil || state.runID != runID {
		return fmt.Errorf("completion does not match the active progress run")
	}
	if err := persist(); err != nil {
		return err
	}
	return p.emitLocked(taskID, phase, payload)
}

func (p *progressTracker) emitLocked(taskID string, phase module.ArticlePhase, payload articleworkflow.ProgressPayload) error {
	state := p.active[taskID]
	if state == nil {
		if _, ok := payload.(articleworkflow.TaskError); !ok {
			return fmt.Errorf("progress run for task %q has not started", taskID)
		}
		state = &taskProgress{runID: uuid.NewString(), phase: phase}
		p.active[taskID] = state
	}
	if state.seq >= 1<<53-1 {
		return fmt.Errorf("progress sequence exceeds JavaScript safe integer range")
	}
	if err := state.apply(payload); err != nil {
		return err
	}
	state.phase = phase
	state.seq++
	event, err := progressEvent(taskID, state, payload)
	if err != nil {
		return err
	}
	if p.hub != nil {
		p.hub.Publish(event)
	}
	if event.Terminal {
		delete(p.active, taskID)
	}
	return nil
}

func (state *taskProgress) apply(payload articleworkflow.ProgressPayload) error {
	s := &state.snapshot
	switch v := payload.(type) {
	case articleworkflow.TitlesDone:
		s.TitleOptions = slices.Clone(v.Options)
		s.Terminal = true
	case articleworkflow.OutlineSnapshot:
		s.Outline = cloneOutline(v.Sections)
	case articleworkflow.OutlineDone:
		s.Outline = nil
		for _, section := range v.Sections {
			s.Outline = append(s.Outline, articleworkflow.OutlinePreviewSection{Section: new(section.Section), Title: new(section.Title), Points: slices.Clone(section.Points)})
		}
		s.Terminal = true
	case articleworkflow.ContentDelta:
		s.Content += v.Delta
	case articleworkflow.ContentGenerated:
		s.Content = v.Content
		s.Step = "image_plan"
	case articleworkflow.ImagesPlanned:
		tasks := make(map[string]articleworkflow.ImageTask, len(v.Tasks))
		for _, task := range v.Tasks {
			if err := task.Validate(); err != nil {
				return err
			}
			if _, ok := tasks[task.ID]; ok {
				return fmt.Errorf("duplicate planned image task %q", task.ID)
			}
			tasks[task.ID] = task
		}
		state.tasks = tasks
		state.settled = make(map[string]bool, len(tasks))
		s.ImageSummary = articleworkflow.ImageSummary{Total: len(v.Tasks)}
		s.ImageOutcomes = nil
		s.Images = nil
		s.Warnings = nil
		s.SavedImages = nil
		s.Step = "images"
	case articleworkflow.ImageFinished:
		if err := v.Outcome.Validate(); err != nil {
			return err
		}
		if err := v.Summary.Validate(); err != nil {
			return err
		}
		if planned, ok := state.tasks[v.Outcome.Task.ID]; !ok || planned != v.Outcome.Task || state.settled[v.Outcome.Task.ID] {
			return fmt.Errorf("unknown, changed or repeated image outcome")
		}
		if v.Summary.Total != s.ImageSummary.Total || v.Summary.Settled != s.ImageSummary.Settled+1 {
			return fmt.Errorf("nonsequential image progress")
		}
		expected := s.ImageSummary
		expected.Settled++
		if v.Outcome.Status == articleworkflow.ImageSucceeded {
			expected.Succeeded++
		} else {
			expected.Skipped++
		}
		if v.Summary != expected {
			return fmt.Errorf("image summary counters do not match outcome status")
		}
		s.ImageSummary = v.Summary
		state.settled[v.Outcome.Task.ID] = true
		s.ImageOutcomes = append(s.ImageOutcomes, v.Outcome)
		if v.Outcome.Asset != nil {
			s.Images = append(s.Images, articleworkflow.GeneratedImage{Task: v.Outcome.Task, Asset: *v.Outcome.Asset})
		}
		if v.Outcome.Warning != nil {
			s.Warnings = append(s.Warnings, *v.Outcome.Warning)
		}
	case articleworkflow.ImagesDone:
		if err := v.Summary.Validate(); err != nil {
			return err
		}
		if v.Summary.Settled != v.Summary.Total {
			return fmt.Errorf("image batch ended before all tasks settled")
		}
		if len(v.Outcomes) != v.Summary.Total || v.Summary != s.ImageSummary {
			return fmt.Errorf("image summary does not match settled outcomes")
		}
		seen := make(map[string]bool, len(v.Outcomes))
		for _, outcome := range v.Outcomes {
			if err := outcome.Validate(); err != nil {
				return err
			}
			if planned, ok := state.tasks[outcome.Task.ID]; !ok || planned != outcome.Task || seen[outcome.Task.ID] {
				return fmt.Errorf("final image outcomes do not match the plan")
			}
			seen[outcome.Task.ID] = true
		}
		s.ImageSummary = v.Summary
		s.ImageOutcomes = slices.Clone(v.Outcomes)
		s.Step = "merge"
	case articleworkflow.ContentDone:
		s.Content = v.Content
		s.FullContent = v.FullContent
		s.Images = slices.Clone(v.Images)
		s.Warnings = slices.Clone(v.Warnings)
		s.SavedImages = nil
		s.Step = "completed"
		s.Status = module.StatusCompleted
		s.Terminal = true
	case articleworkflow.TaskError:
		s.Error = new(v)
		s.Status = module.StatusFailed
		s.Terminal = true
	default:
		return fmt.Errorf("unsupported progress payload %T", payload)
	}
	return nil
}

func cloneOutline(sections []articleworkflow.OutlinePreviewSection) []articleworkflow.OutlinePreviewSection {
	return lo.Map(sections, func(s articleworkflow.OutlinePreviewSection, _ int) articleworkflow.OutlinePreviewSection {
		copied := articleworkflow.OutlinePreviewSection{Points: slices.Clone(s.Points)}
		if s.Section != nil {
			copied.Section = new(*s.Section)
		}
		if s.Title != nil {
			copied.Title = new(*s.Title)
		}
		return copied
	})
}

func (p *progressTracker) subscribe(ctx context.Context, taskID string, load func(context.Context) (*module.Article, error)) (<-chan sse.SSEEvent, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.active[taskID]
	if state == nil {
		// Read again under the same lock as publication: a terminal event may
		// have committed after the caller's authorization read.
		a, err := load(ctx)
		if err != nil {
			return nil, nil, err
		}
		if a == nil {
			return nil, nil, fmt.Errorf("article disappeared before progress subscription")
		}
		state = &taskProgress{runID: uuid.NewString(), phase: a.Phase, snapshot: persistedSnapshot(a)}
	}
	event, err := progressEvent(taskID, state, state.snapshot)
	if err != nil {
		return nil, nil, err
	}
	ch, cancel := p.hub.SubscribeSnapshot(taskID, event)
	return ch, cancel, nil
}
