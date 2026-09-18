package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

type imageAgentModelStub struct {
	response *schema.Message
	messages []*schema.Message
}

func (m *imageAgentModelStub) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = append([]*schema.Message(nil), messages...)
	return m.response, nil
}

func (m *imageAgentModelStub) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream is not used by image agent tests")
}

type imageGeneratorStub struct {
	providers []port.ImageProviderMetadata
	allowed   []port.ImageMethod
	reqs      []port.ImageRequirement
	result    []port.ImageResult
}

func (g *imageGeneratorStub) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	method = method.Normalize()
	for _, provider := range g.providers {
		if provider.Method.Normalize() == method {
			return provider, true
		}
	}
	return port.ImageProviderMetadata{}, false
}

func (g *imageGeneratorStub) AvailableProviders(allowedMethods []port.ImageMethod) []port.ImageProviderMetadata {
	g.allowed = append([]port.ImageMethod(nil), allowedMethods...)
	return append([]port.ImageProviderMetadata(nil), g.providers...)
}

func (g *imageGeneratorStub) Generate(
	ctx context.Context,
	_ string,
	reqs []port.ImageRequirement,
	_ []port.ImageMethod,
	onProgress port.ImageProgressFunc,
) ([]port.ImageResult, error) {
	g.reqs = append([]port.ImageRequirement(nil), reqs...)
	for i, result := range g.result {
		if onProgress != nil {
			onProgress(ctx, i+1, len(g.result), result)
		}
	}
	return append([]port.ImageResult(nil), g.result...), nil
}

func TestImageAgentUsesAvailableProviderMetadata(t *testing.T) {
	modelStub := &imageAgentModelStub{response: schema.AssistantMessage(`{
		"contentWithPlaceholders":"{{IMAGE_PLACEHOLDER_1}}\n正文",
		"imageRequirements":[{
			"position":1,
			"type":"cover",
			"imageSource":"MERMAID",
			"prompt":"flowchart LR; A-->B",
			"placeholderId":"{{IMAGE_PLACEHOLDER_1}}"
		}]
	}`, nil)}
	generator := &imageGeneratorStub{
		providers: []port.ImageProviderMetadata{{
			Method:             port.MethodMermaid,
			Access:             port.ImageAccessFree,
			PlannerDescription: "当前可用的 Mermaid Provider",
			PlannerUsageGuide:  "只生成合法 Mermaid 源码",
			ToolName:           "render_mermaid_diagram",
			ToolDescription:    "Render a Mermaid diagram.",
		}},
		result: []port.ImageResult{{
			Position:      1,
			Method:        port.MethodMermaid,
			URL:           "https://example.com/diagram.png",
			PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
		}},
	}
	mainTitle, subTitle := "主标题", "副标题"
	state := &article.ArticleState{
		TaskID:              "task-1",
		MainTitle:           &mainTitle,
		SubTitle:            &subTitle,
		Content:             "正文",
		EnabledImageMethods: []port.ImageMethod{port.MethodMermaid},
	}

	planned, generated := 0, 0
	err := NewImageAgent(modelStub, generator).Execute(
		context.Background(),
		state,
		func(reqs []port.ImageRequirement) { planned = len(reqs) },
		func(context.Context, int, int, port.ImageResult) { generated++ },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(modelStub.messages) != 1 {
		t.Fatalf("model messages=%d, want 1", len(modelStub.messages))
	}
	promptText := modelStub.messages[0].Content
	if !strings.Contains(promptText, "当前可用的 Mermaid Provider") ||
		!strings.Contains(promptText, "只生成合法 Mermaid 源码") {
		t.Fatalf("provider metadata missing from prompt: %s", promptText)
	}
	if strings.Contains(promptText, "NANO_BANANA") {
		t.Fatalf("prompt exposes unavailable provider: %s", promptText)
	}
	if len(generator.allowed) != 1 || generator.allowed[0] != port.MethodMermaid {
		t.Fatalf("unexpected allowed methods: %+v", generator.allowed)
	}
	if len(generator.reqs) != 1 || len(state.Images) != 1 || planned != 1 || generated != 1 {
		t.Fatalf("unexpected image flow: reqs=%d images=%d planned=%d generated=%d", len(generator.reqs), len(state.Images), planned, generated)
	}
}

func TestImageAgentSkipsWhenNoProviderIsAvailable(t *testing.T) {
	mainTitle, subTitle := "主标题", "副标题"
	state := &article.ArticleState{
		MainTitle: &mainTitle,
		SubTitle:  &subTitle,
		Content:   "正文",
	}

	if err := NewImageAgent(nil, nil).Execute(context.Background(), state, nil, nil); err != nil {
		t.Fatal(err)
	}
	if state.ContentWithPlaceholders != state.Content || len(state.ImageRequirements) != 0 || len(state.Images) != 0 {
		t.Fatalf("unexpected skipped state: %+v", state)
	}
}
