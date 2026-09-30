package image

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type mermaidModelStub struct {
	response *schema.Message
	err      error
	input    []*schema.Message
}

func (m *mermaidModelStub) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.input = messages
	return m.response, m.err
}

func (m *mermaidModelStub) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream is not used by Mermaid")
}

func TestNewMermaidRequiresModel(t *testing.T) {
	if NewMermaid(nil) != nil {
		t.Fatal("Mermaid must not be registered without a model")
	}
}

func TestMermaidFetchReturnsSource(t *testing.T) {
	llm := &mermaidModelStub{response: schema.AssistantMessage("```mermaid\nflowchart TD\n  A-->B\n```", nil)}
	got, err := NewMermaid(llm).Fetch(context.Background(), port.ImageRequirement{
		Prompt: "从选题到完成的三步流程",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mermaid:flowchart TD\n  A-->B" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "```") {
		t.Fatalf("fence leaked into source: %q", got)
	}
	if len(llm.input) != 1 || !strings.Contains(llm.input[0].Content, "从选题到完成的三步流程") {
		t.Fatalf("model did not receive diagram description: %+v", llm.input)
	}
}

func TestMermaidFetchRejectsMissingInputOrOutput(t *testing.T) {
	llm := &mermaidModelStub{response: schema.AssistantMessage("  ", nil)}
	provider := NewMermaid(llm)
	if _, err := provider.Fetch(context.Background(), port.ImageRequirement{}); err == nil {
		t.Fatal("missing description should fail")
	}
	if len(llm.input) != 0 {
		t.Fatal("missing description must not call model")
	}
	if _, err := provider.Fetch(context.Background(), port.ImageRequirement{Prompt: "三步流程"}); err == nil {
		t.Fatal("empty model output should fail")
	}
	llm.response = schema.AssistantMessage("这是一个流程图", nil)
	if _, err := provider.Fetch(context.Background(), port.ImageRequirement{Prompt: "三步流程"}); err == nil {
		t.Fatal("explanation is not Mermaid source")
	}
	llm.err = errors.New("model unavailable")
	if _, err := provider.Fetch(context.Background(), port.ImageRequirement{Prompt: "三步流程"}); !errors.Is(err, llm.err) {
		t.Fatalf("model error not propagated: %v", err)
	}
}

func TestProviderExecutorGeneratesMermaidSource(t *testing.T) {
	req := port.ImageRequirement{
		Position:      1,
		ImageSource:   port.MethodMermaid,
		Prompt:        "从需求到交付的流程",
		PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}
	if _, ok := NewProviderExecutor(&config.Config{}, nil, nil).LookupProvider(port.MethodMermaid); ok {
		t.Fatal("Mermaid should not be registered without a model")
	}
	llm := &mermaidModelStub{response: schema.AssistantMessage("flowchart LR\nA-->B", nil)}
	executor := NewProviderExecutor(&config.Config{}, llm, nil)
	result, err := executor.Execute(context.Background(), "task-1", req, port.MethodMermaid)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "mermaid:flowchart LR\nA-->B" || result.Method != port.MethodMermaid || result.PlaceholderID != req.PlaceholderID {
		t.Fatalf("unexpected result: %+v", result)
	}
}
