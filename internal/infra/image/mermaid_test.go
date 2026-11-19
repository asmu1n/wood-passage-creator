package image

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

func TestNewMermaidRequiresGenerator(t *testing.T) {
	if NewMermaid(nil) != nil {
		t.Fatal("missing generator must disable Mermaid")
	}
}

func TestMermaidFetchReturnsSource(t *testing.T) {
	provider := NewMermaid(func(_ context.Context, description string) (string, error) {
		if description != "Three steps" {
			t.Fatalf("description = %q", description)
		}
		return "```mermaid\nflowchart TD\n  A-->B\n```", nil
	})
	got, err := provider.Fetch(t.Context(), port.ImageRequirement{Prompt: "Three steps"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mermaid:flowchart TD\n  A-->B" {
		t.Fatalf("source = %q", got)
	}
}

func TestMermaidRejectsInvalidSource(t *testing.T) {
	for _, source := range []string{"", "An explanation", "flowchart TD\n```evil", "%%{init: {}}%%\nflowchart TD"} {
		provider := NewMermaid(func(context.Context, string) (string, error) { return source, nil })
		if _, err := provider.Fetch(t.Context(), port.ImageRequirement{Prompt: "Diagram"}); err == nil {
			t.Errorf("invalid source accepted: %q", source)
		}
	}
	modelErr := errors.New("model unavailable")
	provider := NewMermaid(func(context.Context, string) (string, error) { return "", modelErr })
	if _, err := provider.Fetch(t.Context(), port.ImageRequirement{Prompt: "Diagram"}); !errors.Is(err, modelErr) {
		t.Fatalf("lost model error: %v", err)
	}
}

func TestProviderExecutorGeneratesMermaidSource(t *testing.T) {
	if _, ok := NewProviderExecutor(&config.Config{}, port.ImageSources{}, nil).LookupProvider(port.MethodMermaid); ok {
		t.Fatal("Mermaid must be disabled without a generator")
	}
	executor := NewProviderExecutor(&config.Config{}, port.ImageSources{
		Mermaid: func(context.Context, string) (string, error) { return "flowchart LR\nA-->B", nil },
	}, nil)
	result, err := executor.Execute(t.Context(), "task", port.ImageRequirement{Prompt: "Diagram", Position: 1}, port.MethodMermaid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.URL, "mermaid:") || result.Method != port.MethodMermaid {
		t.Fatalf("result = %+v", result)
	}
}
