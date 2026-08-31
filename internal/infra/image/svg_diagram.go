package image

import (
	"context"
	"fmt"
	"strings"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type SVGDiagram struct {
	llm port.ChatModel
}

// NewSVGDiagram 未启用或无 llm 时返回 nil。
func NewSVGDiagram(cfg config.SVGDiagramConfig, llm port.ChatModel) *SVGDiagram {
	if !cfg.Enabled || llm == nil {
		return nil
	}
	return &SVGDiagram{llm: llm}
}

func (p *SVGDiagram) Method() string {
	return MethodSVGDiagram
}

func (p *SVGDiagram) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	desc := reqText(req, true)
	if desc == "" {
		return "", fmt.Errorf("svg_diagram: prompt required")
	}
	prompt := fmt.Sprintf(`你是信息图设计师。请根据需求生成完整、合法的 SVG 代码（单个 <svg>...</svg>）。
要求：白底、清晰、中文可用、不要 markdown 代码围栏、不要解释文字。
需求：%s`, desc)

	raw, err := p.llm.Generate(ctx, []port.Message{{Role: port.RoleUser, Content: prompt}}, nil)
	if err != nil {
		return "", err
	}
	svg := extractSVG(raw)
	if svg == "" {
		return "", fmt.Errorf("svg_diagram: no svg in model output")
	}
	return dataURL("image/svg+xml;charset=utf-8", []byte(svg)), nil
}

func extractSVG(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```svg")
	s = strings.TrimPrefix(s, "```xml")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(strings.ToLower(s), "<svg")
	end := strings.LastIndex(strings.ToLower(s), "</svg>")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return s[start : end+len("</svg>")]
}
