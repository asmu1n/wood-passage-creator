package image

import (
	"context"
	"strings"
	"testing"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

func TestSVGSourceValidation(t *testing.T) {
	for _, source := range []string{
		`<svg><text>broken</svg>`,
		`<svg><script>alert(1)</script></svg>`,
		`<svg><rect onload="bad()"/></svg>`,
		`<svg><image href="https://example.com/image"/></svg>`,
		`<svg><foreignObject>HTML</foreignObject></svg>`,
	} {
		if err := validateSVG(source); err == nil {
			t.Errorf("invalid SVG accepted: %s", source)
		}
	}
	source := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><text x="5" y="20">示意图</text></svg>`
	provider := NewSVGDiagram(config.SVGDiagramConfig{Enabled: true}, func(context.Context, string) (string, error) { return source, nil })
	got, err := provider.Fetch(t.Context(), port.ImageRequirement{Prompt: "Description"})
	if err != nil || !strings.HasPrefix(got, "data:image/svg+xml") {
		t.Fatalf("SVG = %q %v", got, err)
	}
}
