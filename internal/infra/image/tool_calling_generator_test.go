package image

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/port"
)

type stubEinoToolModel struct {
	responses []*schema.Message
	calls     int
	tools     []*schema.ToolInfo
	messages  [][]*schema.Message
}

func (m *stubEinoToolModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = append(m.messages, append([]*schema.Message(nil), messages...))
	if m.calls >= len(m.responses) {
		return nil, fmt.Errorf("unexpected model call %d", m.calls)
	}
	response := m.responses[m.calls]
	m.calls++
	return response, nil
}

func (m *stubEinoToolModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream is not used by image tool calling tests")
}

func (m *stubEinoToolModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.tools = append([]*schema.ToolInfo(nil), tools...)
	return m, nil
}

func TestToolCallingGenerator_ExecutesToolAndReturnsObservation(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"wood workshop"}`,
			},
		}}),
		schema.AssistantMessage("全部图片已生成。", nil),
	}}
	tools := &ProviderExecutor{providers: map[port.ImageMethod]port.Provider{
		port.MethodPexels: stubProvider{method: port.MethodPexels, url: "https://example.com/wood.jpg"},
	}}
	g := &ToolCallingGenerator{model: chatModel, tools: tools}

	progress := 0
	results, err := g.Generate(context.Background(), "task-1", []port.ImageRequirement{{
		Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}}, []port.ImageMethod{port.MethodPexels}, func(context.Context, int, int, port.ImageResult) {
		progress++
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Method != port.MethodPexels {
		t.Fatalf("unexpected results: %+v", results)
	}
	if chatModel.calls != 2 {
		t.Fatalf("model calls=%d, want 2", chatModel.calls)
	}
	if progress != 1 {
		t.Fatalf("progress calls=%d, want 1", progress)
	}
	last := chatModel.messages[1][len(chatModel.messages[1])-1]
	if last.Role != schema.Tool || last.ToolCallID != "call-1" {
		t.Fatalf("missing tool observation: %+v", last)
	}
	if len(chatModel.tools) != 1 || chatModel.tools[0].Name != "search_pexels_image" {
		t.Fatalf("unexpected tools: %+v", chatModel.tools)
	}
}

func TestToolCallingGenerator_ChangesToolAfterFailure(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"architecture"}`,
			},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-2", Type: "function", Function: schema.FunctionCall{
				Name: "render_mermaid_diagram", Arguments: `{"requirementIndex":0,"prompt":"flowchart LR; A-->B"}`,
			},
		}}),
		schema.AssistantMessage("完成。", nil),
	}}
	tools := &ProviderExecutor{providers: map[port.ImageMethod]port.Provider{
		port.MethodPexels:  stubProvider{method: port.MethodPexels, err: fmt.Errorf("no matching photo")},
		port.MethodMermaid: stubProvider{method: port.MethodMermaid, url: "https://example.com/diagram.svg"},
	}}
	g := &ToolCallingGenerator{model: chatModel, tools: tools}

	results, err := g.Generate(context.Background(), "task-2", []port.ImageRequirement{{
		Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}}, []port.ImageMethod{port.MethodPexels, port.MethodMermaid}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Method != port.MethodMermaid {
		t.Fatalf("expected Mermaid fallback selected by model, got %+v", results)
	}
	if chatModel.calls != 3 {
		t.Fatalf("model calls=%d, want 3", chatModel.calls)
	}
}

var _ model.ToolCallingChatModel = (*stubEinoToolModel)(nil)
