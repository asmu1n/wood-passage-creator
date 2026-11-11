package articleworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"go.yaml.in/yaml/v3"

	"wood-passage-creator/internal/config"
)

func TestImageSourcesUseApprovedDiagramPrompts(t *testing.T) {
	_, _, g := planningTestFlows(t, func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		text := req.Messages[0].Text()
		if !strings.Contains(req.Messages[1].Text(), "Source description") {
			t.Error("description missing")
		}
		if strings.Contains(text, "Mermaid") {
			return completedJSON("flowchart TD\nA-->B"), nil
		}
		return completedJSON(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), nil
	})
	sources, err := NewImageSources(g, config.NanoBananaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if sources.NanoBanana != nil {
		t.Fatal("AI image provider enabled without credentials")
	}
	if out, err := sources.Mermaid(t.Context(), "Source description"); err != nil || !strings.HasPrefix(out, "flowchart") {
		t.Fatalf("%q %v", out, err)
	}
	if out, err := sources.SVG(t.Context(), "Source description"); err != nil || !strings.HasPrefix(out, "<svg") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestNanoBananaUsesGenkitMediaAndConfiguredAspectRatio(t *testing.T) {
	_, _, g := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		t.Fatal("image generation called the text model")
		return nil, nil
	})
	genkit.DefineModelAction(g, "googleai/test-image", &ai.ModelOptions{Supports: &ai.ModelSupports{Multiturn: true, Media: true}},
		func(_ context.Context, req *ai.ModelRequest, config map[string]any, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			if !strings.Contains(req.Messages[len(req.Messages)-1].Text(), "Wood bench") {
				t.Error("missing visual prompt")
			}
			if config["imageConfig"].(map[string]any)["aspectRatio"] != "4:3" {
				t.Fatalf("image config = %+v", config)
			}
			return &ai.ModelResponse{FinishReason: ai.FinishReasonStop, Message: ai.NewMessage(ai.RoleModel, nil,
				ai.NewTextPart("Generated"), ai.NewMediaPart("image/png", "data:image/png;base64,AAAA"))}, nil
		})
	sources, err := NewImageSources(g, config.NanoBananaConfig{APIKey: "test-key", Model: "test-image", AspectRatio: "4:3"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := sources.NanoBanana(t.Context(), "Wood bench")
	if err != nil || out != "data:image/png;base64,AAAA" {
		t.Fatalf("generated image = %q %v", out, err)
	}
}

func TestImageSourcesRejectAbnormalFinishes(t *testing.T) {
	_, _, g := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return &ai.ModelResponse{FinishReason: ai.FinishReasonAborted, Message: ai.NewModelTextMessage("Partial")}, nil
	})
	sources, err := NewImageSources(g, config.NanoBananaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sources.Mermaid(t.Context(), "Description"); err == nil {
		t.Fatal("partial source accepted")
	}
}

func TestProviderContextCancellationReachesSource(t *testing.T) {
	want := errors.New("source error")
	_, _, g := planningTestFlows(t, func(context.Context, *ai.ModelRequest, ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return nil, want
	})
	sources, err := NewImageSources(g, config.NanoBananaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sources.SVG(t.Context(), "Description"); !errors.Is(err, want) {
		t.Fatalf("source error lost: %v", err)
	}
}

type promptAssetCase struct {
	name   string
	input  any
	output any
	format string
}

func promptAssetCases() []promptAssetCase {
	return []promptAssetCase{
		{PromptTitles, TitlesPromptInput{}, TitlesResult{}, "json"},
		{PromptOutline, OutlinePromptInput{}, OutlineResult{}, "json"},
		{PromptReviseOutline, ReviseOutlinePromptInput{}, OutlineResult{}, "json"},
		{PromptContent, ContentPromptInput{}, nil, "text"},
		{PromptImagePlan, PlanImagesInput{}, ImagePlan{}, "json"},
		{PromptImageExecute, ImageExecutePromptInput{}, nil, ""},
		{PromptMermaid, DiagramPromptInput{}, nil, "text"},
		{PromptSVG, DiagramPromptInput{}, nil, "text"},
	}
}

func TestPromptMetadataReferencesGoContracts(t *testing.T) {
	for _, tt := range promptAssetCases() {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := PromptFS.ReadFile("prompts/" + tt.name + ".prompt")
			if err != nil {
				t.Fatal(err)
			}
			metadata, body, ok := strings.Cut(strings.TrimPrefix(string(raw), "---\n"), "\n---\n")
			if !strings.HasPrefix(string(raw), "---\n") || !ok {
				t.Fatal("prompt must start with YAML frontmatter")
			}
			var header struct {
				Description string `yaml:"description"`
				Input       struct {
					Schema string `yaml:"schema"`
				} `yaml:"input"`
				Output struct {
					Format string `yaml:"format"`
					Schema string `yaml:"schema"`
				} `yaml:"output"`
				ReturnToolRequests bool `yaml:"returnToolRequests"`
			}
			decoder := yaml.NewDecoder(strings.NewReader(metadata))
			decoder.KnownFields(true)
			if err := decoder.Decode(&header); err != nil {
				t.Fatal(err)
			}
			if header.Description == "" || header.Input.Schema != reflect.TypeOf(tt.input).Name() {
				t.Fatalf("unexpected prompt description or input schema: %+v", header)
			}
			wantOutput := ""
			if tt.output != nil {
				wantOutput = reflect.TypeOf(tt.output).Name()
			}
			if header.Output.Schema != wantOutput || header.Output.Format != tt.format {
				t.Fatalf("output = %+v, want format=%s schema=%s", header.Output, tt.format, wantOutput)
			}
			if header.ReturnToolRequests != (tt.name == PromptImageExecute) {
				t.Fatal("only image-execute should return tool requests")
			}
			if !strings.Contains(body, `{{role "system"}}`) || !strings.Contains(body, `{{role "user"}}`) {
				t.Fatal("prompt must separate system instructions and user input")
			}
		})
	}
}

func TestPromptAssetsIncludeReferencedPartials(t *testing.T) {
	partialPattern := regexp.MustCompile(`{{>\s*([^\s}]+)`)
	assets, err := fs.Glob(PromptFS, "prompts/*.prompt")
	if err != nil {
		t.Fatal(err)
	}
	var partials []string
	for _, asset := range assets {
		if strings.HasPrefix(strings.TrimPrefix(asset, "prompts/"), "_") {
			partials = append(partials, strings.TrimSuffix(strings.TrimPrefix(asset, "prompts/_"), ".prompt"))
		}
		raw, err := PromptFS.ReadFile(asset)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range partialPattern.FindAllStringSubmatch(string(raw), -1) {
			if _, err := PromptFS.ReadFile("prompts/_" + match[1] + ".prompt"); err != nil {
				t.Fatalf("%s references unavailable partial %q: %v", asset, match[1], err)
			}
		}
	}
	slices.Sort(partials)
	want := []string{"editorial-context", "input-boundary", "style"}
	if !reflect.DeepEqual(partials, want) || len(assets) != len(promptAssetCases())+len(want) {
		t.Fatalf("unexpected embedded assets: %v", assets)
	}
}

func TestPromptInputsExcludeExecutionState(t *testing.T) {
	for _, tt := range promptAssetCases() {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			var input any
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			assertModelScopedInput(t, input)
		})
	}
}

func assertModelScopedInput(t *testing.T, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "run", "articleId", "taskId", "runId", "allowedImageMethods", "budget",
				"maxModelTurns", "maxToolAttempts", "position", "requirementIndex",
				"contentWithPlaceholders", "byteOffset":
				t.Errorf("execution-owned field %q must not be in prompt input", key)
			}
			assertModelScopedInput(t, child)
		}
	case []any:
		for _, child := range value {
			assertModelScopedInput(t, child)
		}
	}
}
