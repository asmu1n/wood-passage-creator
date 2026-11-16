package image

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type SVGDiagram struct {
	generate func(context.Context, string) (string, error)
}

// NewSVGDiagram 未启用或无生成能力时返回 nil。
func NewSVGDiagram(cfg config.SVGDiagramConfig, generate func(context.Context, string) (string, error)) *SVGDiagram {
	if !cfg.Enabled || generate == nil {
		return nil
	}
	return &SVGDiagram{generate: generate}
}

func (p *SVGDiagram) Metadata() port.ImageProviderMetadata {
	return port.ImageProviderMetadata{
		Method:             port.MethodSVGDiagram,
		Access:             port.ImageAccessVIP,
		PlannerDescription: "LLM 生成 SVG 示意图（VIP）",
		PlannerUsageGuide:  "适合 SVG 示意图；执行工具时 prompt 描述示意图内容，不要填写源码。",
		ToolName:           "generate_svg_diagram",
		ToolDescription:    "Generate an SVG information diagram from a precise visual description.",
	}
}
func (p *SVGDiagram) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	desc := reqText(req, true)
	if desc == "" {
		return "", fmt.Errorf("svg_diagram: prompt required")
	}
	raw, err := p.generate(ctx, desc)
	if err != nil {
		return "", err
	}
	svg := extractSVG(raw)
	if svg == "" {
		return "", fmt.Errorf("svg_diagram: no svg in model output")
	}
	if err := validateSVG(svg); err != nil {
		return "", err
	}
	return dataURL("image/svg+xml;charset=utf-8", []byte(svg)), nil
}

func validateSVG(source string) error {
	decoder := xml.NewDecoder(strings.NewReader(source))
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("svg_diagram: invalid XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(token.Name.Local)
			if depth == 0 {
				roots++
				if name != "svg" {
					return fmt.Errorf("svg_diagram: root must be svg")
				}
			}
			switch name {
			case "script", "foreignobject", "image", "a", "animate", "animatetransform", "set", "style":
				return fmt.Errorf("svg_diagram: unsupported element %s", name)
			}
			for _, attr := range token.Attr {
				key, value := strings.ToLower(attr.Name.Local), strings.ToLower(strings.TrimSpace(attr.Value))
				if strings.HasPrefix(key, "on") || key == "href" && !strings.HasPrefix(value, "#") ||
					strings.Contains(value, "url(") && !strings.Contains(value, "url(#") {
					return fmt.Errorf("svg_diagram: external or active attributes are not supported")
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.Directive, xml.ProcInst:
			return fmt.Errorf("svg_diagram: XML directives are not supported")
		}
	}
	if roots != 1 || depth != 0 {
		return fmt.Errorf("svg_diagram: expected one complete svg")
	}
	return nil
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
