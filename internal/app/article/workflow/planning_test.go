package articleworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"wood-passage-creator/internal/module/article"
)

const validTitlesJSON = `{"options":[{"mainTitle":"First title","subTitle":"First subtitle"},{"mainTitle":"Second title","subTitle":"Second subtitle"},{"mainTitle":"Third title","subTitle":"Third subtitle"}]}`
const validOutlineJSON = `{"sections":[{"section":1,"title":"Getting started","points":["Choose tools"]}]}`

type planningModelFunc func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error)

type planningLogRecorder struct {
	mu   sync.Mutex
	rows []article.CreateAgentLogParams
}

func (r *planningLogRecorder) SaveAsync(row article.CreateAgentLogParams) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, row)
}

func (r *planningLogRecorder) entries() []article.CreateAgentLogParams {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.rows)
}

func planningTestFlows(t *testing.T, model planningModelFunc) (*PlanningFlows, *planningLogRecorder, *genkit.Genkit) {
	t.Helper()
	t.Setenv("GENKIT_REFLECTION_ENABLED", "false")
	g := genkit.Init(t.Context(), genkit.WithPromptFS(PromptFS), genkit.WithDefaultModel("test/planning"))
	genkit.DefineModelAction(g, "test/planning", &ai.ModelOptions{
		Supports: &ai.ModelSupports{
			Multiturn: true, SystemRole: true, Tools: true, Constrained: ai.ConstrainedSupportAll,
			Output: []string{ai.OutputFormatJSON},
		},
	}, func(ctx context.Context, req *ai.ModelRequest, _ struct{}, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return model(ctx, req, cb)
	})
	logs := &planningLogRecorder{}
	flows, err := NewPlanningFlows(g, logs)
	if err != nil {
		t.Fatal(err)
	}
	return flows, logs, g
}

func completedJSON(raw string) *ai.ModelResponse {
	return &ai.ModelResponse{FinishReason: ai.FinishReasonStop, Message: ai.NewModelTextMessage(raw)}
}

func planningRun() RunContext {
	return RunContext{ArticleID: 7, TaskID: "task-private-sentinel", RunID: "run-private-sentinel"}
}

func planningEditorial() EditorialContext {
	return EditorialContext{
		MainTitle: "A practical guide", SubTitle: "For beginners",
		Style: new(article.StyleEducational), UserDescription: "Keep local examples",
	}
}

func TestPlanningRejectsMissingRuntimeAndAssets(t *testing.T) {
	if _, err := NewPlanningFlows(nil, nil); err == nil {
		t.Fatal("missing runtime accepted")
	}
	t.Setenv("GENKIT_REFLECTION_ENABLED", "false")
	g := genkit.Init(t.Context(), genkit.WithPromptFS(fstest.MapFS{
		"prompts/_unused.prompt": &fstest.MapFile{Data: []byte("Partial only")},
	}))
	if _, err := NewPlanningFlows(g, nil); err == nil {
		t.Fatal("missing planning prompts accepted")
	}
}

func TestPlanningTitlesUseApprovedPromptAndRecordOneStep(t *testing.T) {
	var request *ai.ModelRequest
	flows, logs, g := planningTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		request = req
		return completedJSON(validTitlesJSON), nil
	})
	out, err := flows.GenerateTitles(t.Context(), GenerateTitlesInput{
		Run: planningRun(), Topic: "Woodworking {{literal input}}", Style: new(article.StyleTech),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Options) != 3 || out.Options[0].MainTitle != "First title" {
		t.Fatalf("unexpected titles: %+v", out)
	}
	assertModelPrompt(t, request, "使用科技风格", "{{literal input}}")
	if request.Output == nil || request.Output.Schema == nil || !request.Output.Constrained {
		t.Fatalf("structured output is not constrained: %+v", request.Output)
	}
	rows := logs.entries()
	if len(rows) != 1 || rows[0].AgentName != "title_generator" || rows[0].Status != article.AgentLogSuccess ||
		rows[0].TaskID != planningRun().TaskID || rows[0].OutputData != `{"options":3}` {
		t.Fatalf("unexpected business logs: %+v", rows)
	}
	var names []string
	for _, flow := range genkit.ListFlows(g) {
		names = append(names, flow.Name())
	}
	slices.Sort(names)
	want := []string{string(FlowGenerateOutline), string(FlowGenerateTitles), string(FlowReviseOutline)}
	slices.Sort(want)
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("registered flows = %v, want %v", names, want)
	}
}

func streamedOutline(ctx context.Context, _ *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
	parts := []string{`{"sections":[{"section":1,"title":"Getting`, ` started","points":["Choose`, ` tools"]}]}`}
	for _, text := range parts {
		if err := cb(ctx, &ai.ModelResponseChunk{
			Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart(text)},
		}); err != nil {
			return nil, err
		}
	}
	return completedJSON(validOutlineJSON), nil
}

func TestPlanningOutlineStreamsTypedSnapshots(t *testing.T) {
	flows, logs, _ := planningTestFlows(t, streamedOutline)
	input := GenerateOutlineInput{Run: planningRun(), Editorial: planningEditorial()}
	var snapshots []OutlineSnapshot
	typedOut, err := flows.GenerateOutline(t.Context(), input, func(_ context.Context, payload ProgressPayload) error {
		snapshot, ok := payload.(OutlineSnapshot)
		if !ok {
			t.Fatalf("unexpected outline progress: %T", payload)
		}
		snapshots = append(snapshots, snapshot)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(typedOut.Sections) != 1 || typedOut.Sections[0].Title != "Getting started" || len(snapshots) < 2 {
		t.Fatalf("unexpected structured stream: result=%+v snapshots=%+v", typedOut, snapshots)
	}
	first := snapshots[0].Sections[0]
	last := snapshots[len(snapshots)-1].Sections[0]
	if first.Title == nil || last.Title == nil || *first.Title == *last.Title ||
		*last.Title != "Getting started" || !reflect.DeepEqual(last.Points, []string{"Choose tools"}) {
		t.Fatalf("snapshots are not immutable cumulative previews: %+v", snapshots)
	}
	if rows := logs.entries(); len(rows) != 1 || rows[0].AgentName != "outline_generator" ||
		rows[0].Status != article.AgentLogSuccess {
		t.Fatalf("unexpected outline logs: %+v", rows)
	}
}

func TestPlanningRevisionKeepsEditorialContext(t *testing.T) {
	flows, logs, _ := planningTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		assertModelPrompt(t, req, "使用教育风格", "Keep local examples", "Original section", "Merge sections")
		return completedJSON(validOutlineJSON), nil
	})
	out, err := flows.ReviseOutline(t.Context(), ReviseOutlineInput{
		Run: planningRun(), Editorial: planningEditorial(), ModifySuggestion: "Merge sections",
		Sections: []article.OutlineSection{{Section: 1, Title: "Original section", Points: []string{"A point"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Sections) != 1 || out.Sections[0].Title != "Getting started" {
		t.Fatalf("unexpected revised outline: %+v", out)
	}
	if rows := logs.entries(); len(rows) != 1 || rows[0].AgentName != "outline_modifier" {
		t.Fatalf("unexpected revision logs: %+v", rows)
	}
}

func assertModelPrompt(t *testing.T, req *ai.ModelRequest, expected ...string) {
	t.Helper()
	if len(req.Messages) != 2 || req.Messages[0].Role != ai.RoleSystem || req.Messages[1].Role != ai.RoleUser {
		t.Fatalf("unexpected prompt roles: %+v", req.Messages)
	}
	text := req.Messages[0].Text() + req.Messages[1].Text()
	for _, marker := range expected {
		if !strings.Contains(text, marker) {
			t.Errorf("prompt is missing %q", marker)
		}
	}
	for _, secret := range []string{planningRun().TaskID, planningRun().RunID} {
		if strings.Contains(text, secret) {
			t.Errorf("execution identity leaked into model input: %q", secret)
		}
	}
}

func TestPlanningRejectsInvalidOrIncompleteOutput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		titles bool
		body   string
		finish ai.FinishReason
	}{
		{"missing title root", true, `{}`, ai.FinishReasonStop},
		{"title array root", true, `[]`, ai.FinishReasonStop},
		{"too few titles", true, `{"options":[{"mainTitle":"A","subTitle":"B"}]}`, ai.FinishReasonStop},
		{"empty titles", true, `{"options":[]}`, ai.FinishReasonStop},
		{"truncated but valid JSON", true, validTitlesJSON, ai.FinishReason("length")},
		{"blocked titles", true, validTitlesJSON, ai.FinishReason("blocked")},
		{"no model text", true, "", ai.FinishReasonStop},
		{"invalid JSON", true, `{`, ai.FinishReasonStop},
		{"empty outline", false, `{"sections":[]}`, ai.FinishReasonStop},
		{"nonsequential outline", false, `{"sections":[{"section":2,"title":"Intro","points":["A"]}]}`, ai.FinishReasonStop},
		{"missing points", false, `{"sections":[{"section":1,"title":"Intro","points":[]}]}`, ai.FinishReasonStop},
		{"multiline point", false, `{"sections":[{"section":1,"title":"Intro","points":["A\nB"]}]}`, ai.FinishReasonStop},
	} {
		t.Run(tt.name, func(t *testing.T) {
			flows, logs, _ := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				resp := completedJSON(tt.body)
				resp.FinishReason = tt.finish
				return resp, nil
			})
			var err error
			if tt.titles {
				_, err = flows.GenerateTitles(t.Context(), GenerateTitlesInput{Run: planningRun(), Topic: "Topic"})
			} else {
				_, err = flows.GenerateOutline(t.Context(), GenerateOutlineInput{Run: planningRun(), Editorial: planningEditorial()}, nil)
			}
			if err == nil {
				t.Fatal("invalid output was accepted as success")
			}
			rows := logs.entries()
			if len(rows) != 1 || rows[0].Status != article.AgentLogFailed || rows[0].ErrorMessage == "" || rows[0].OutputData != "" {
				t.Fatalf("failure was not recorded: %+v", rows)
			}
		})
	}
}

func TestPlanningPropagatesModelAndStreamErrors(t *testing.T) {
	modelErr := errors.New("model unavailable")
	flows, _, _ := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return nil, modelErr
	})
	if _, err := flows.GenerateTitles(t.Context(), GenerateTitlesInput{Topic: "Topic"}); !errors.Is(err, modelErr) {
		t.Fatalf("lost model error: %v", err)
	}
	flows, logs, _ := planningTestFlows(t, streamedOutline)
	streamErr := errors.New("progress callback failed")
	_, err := flows.GenerateOutline(t.Context(), GenerateOutlineInput{Run: planningRun(), Editorial: planningEditorial()},
		func(context.Context, ProgressPayload) error { return streamErr })
	if !errors.Is(err, streamErr) {
		t.Fatalf("lost stream error: %v", err)
	}
	if rows := logs.entries(); len(rows) != 1 || rows[0].Status != article.AgentLogFailed {
		t.Fatalf("callback failure was not recorded: %+v", rows)
	}
}

func TestPlanningValidatesInputBeforeCallingModel(t *testing.T) {
	calls := 0
	flows, _, _ := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		calls++
		return completedJSON(validTitlesJSON), nil
	})
	if _, err := flows.GenerateTitles(t.Context(), GenerateTitlesInput{Topic: " "}); err == nil {
		t.Fatal("empty topic accepted")
	}
	if _, err := flows.GenerateOutline(t.Context(), GenerateOutlineInput{}, nil); err == nil {
		t.Fatal("missing editorial context accepted")
	}
	if _, err := flows.ReviseOutline(t.Context(), ReviseOutlineInput{Editorial: planningEditorial()}); err == nil {
		t.Fatal("empty revision request accepted")
	}
	if calls != 0 {
		t.Fatalf("invalid input reached model %d times", calls)
	}
}

func TestTitlesUseCharacterRatherThanByteLimit(t *testing.T) {
	for _, length := range []int{30, 31} {
		var result TitlesResult
		if err := json.Unmarshal([]byte(validTitlesJSON), &result); err != nil {
			t.Fatal(err)
		}
		result.Options[0].MainTitle = strings.Repeat("木", length)
		if err := validateTitles(result); (err != nil) != (length > 30) {
			t.Fatalf("%d characters: %v", length, err)
		}
	}
}

func TestPlanningStylesUseNativeScalarPromptInputs(t *testing.T) {
	markers := []string{"使用科技风格", "使用情感风格", "使用教育风格", "使用轻松幽默风格"}
	styles := []*article.ArticleStyle{
		new(article.StyleTech), new(article.StyleEmotional),
		new(article.StyleEducational), new(article.StyleHumorous), nil,
	}
	for index, style := range styles {
		t.Run(string(rune('0'+index)), func(t *testing.T) {
			check := func(text string) {
				t.Helper()
				for markerIndex, marker := range markers {
					if strings.Contains(text, marker) != (index == markerIndex) {
						t.Errorf("style %v incorrectly selected fragment %q", style, marker)
					}
				}
			}
			calls := 0
			flows, _, g := planningTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				check(req.Messages[0].Text())
				calls++
				if calls == 1 {
					return completedJSON(validTitlesJSON), nil
				}
				return completedJSON(validOutlineJSON), nil
			})
			if _, err := flows.GenerateTitles(t.Context(), GenerateTitlesInput{Topic: "Topic", Style: style}); err != nil {
				t.Fatal(err)
			}
			editorial := planningEditorial()
			editorial.Style = style
			outline, err := flows.GenerateOutline(t.Context(), GenerateOutlineInput{Editorial: editorial}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := flows.ReviseOutline(t.Context(), ReviseOutlineInput{
				Editorial: editorial, Sections: outline.Sections, ModifySuggestion: "Clarify the point",
			}); err != nil {
				t.Fatal(err)
			}
			rendered, err := genkit.LookupPrompt(g, PromptContent).Render(t.Context(), ContentPromptInput{
				Editorial: promptEditorial(editorial), Outline: outline.Sections,
			})
			if err != nil {
				t.Fatal(err)
			}
			check(rendered.Messages[0].Text())
		})
	}
}

func TestPlanningPropagatesCancellation(t *testing.T) {
	flows, _, _ := planningTestFlows(t, func(ctx context.Context, _ *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := flows.GenerateTitles(ctx, GenerateTitlesInput{Topic: "Topic"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was lost: %v", err)
	}
}

func TestPlanningSharedRuntimeSupportsConcurrentCalls(t *testing.T) {
	flows, logs, g := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return completedJSON(validTitlesJSON), nil
	})
	const count = 8
	var wg sync.WaitGroup
	for i := range count {
		wg.Go(func() {
			_, err := flows.GenerateTitles(t.Context(), GenerateTitlesInput{
				Run: RunContext{ArticleID: int64(i + 1), TaskID: "task", RunID: "run"}, Topic: "Topic",
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if len(logs.entries()) != count || len(genkit.ListFlows(g)) != 3 {
		t.Fatal("concurrent calls duplicated registrations or lost business logs")
	}
}
