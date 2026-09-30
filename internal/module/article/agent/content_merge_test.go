package agent

import (
	"strings"
	"testing"

	"wood-passage-creator/internal/port"
)

func TestMergeImagesKeepsMermaidSource(t *testing.T) {
	content := "正文\n{{IMAGE_PLACEHOLDER_1}}\n结束"
	got := mergeImages(content, []port.ImageResult{{
		PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
		Method:        port.MethodMermaid,
		URL:           "mermaid:flowchart TD\n  A-->B",
	}})
	if strings.Contains(got, "![") || !strings.Contains(got, "```mermaid\nflowchart TD\n  A-->B\n```") {
		t.Fatalf("got %q", got)
	}
}

func TestMergeImagesKeepsPhotoURL(t *testing.T) {
	got := mergeImages("{{IMAGE_PLACEHOLDER_1}}", []port.ImageResult{{
		PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
		Method:        port.MethodPexels,
		URL:           "https://example.com/a.jpg",
		SectionTitle:  "封面",
	}})
	if got != "![封面](https://example.com/a.jpg)" {
		t.Fatalf("got %q", got)
	}
}
