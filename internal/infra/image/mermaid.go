package image

import (
	"context"
	"fmt"
	"strings"

	"wood-passage-creator/internal/port"
)

const mermaidSourcePrefix = "mermaid:"

type Mermaid struct {
	generate func(context.Context, string) (string, error)
}

func NewMermaid(generate func(context.Context, string) (string, error)) *Mermaid {
	if generate == nil {
		return nil
	}
	return &Mermaid{generate: generate}
}

func (p *Mermaid) Metadata() port.ImageProviderMetadata {
	return port.ImageProviderMetadata{
		Method:             port.MethodMermaid,
		Access:             port.ImageAccessFree,
		PlannerDescription: "LLM 生成 Mermaid 流程图/时序图，由前端渲染",
		PlannerUsageGuide:  "适合流程图或时序图；执行工具时 prompt 描述元素及其关系，不要填写 Mermaid 源码。",
		ToolName:           "render_mermaid_diagram",
		ToolDescription:    "Generate a Mermaid diagram from a description of the elements and their relationships. Do not supply Mermaid source code.",
	}
}

func (p *Mermaid) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	desc := reqText(req, true)
	if desc == "" {
		return "", fmt.Errorf("mermaid: prompt required")
	}
	source, err := p.generate(ctx, desc)
	if err != nil {
		return "", err
	}
	code := strings.TrimSpace(source)
	code = strings.TrimPrefix(code, "```mermaid")
	code = strings.TrimPrefix(code, "```")
	code = strings.TrimSuffix(code, "```")
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("mermaid: no source in model output")
	}
	if strings.Contains(code, "```") || strings.Contains(code, "%%{") {
		return "", fmt.Errorf("mermaid: embedded fences or directives are not supported")
	}
	for _, line := range strings.Split(code, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "click ") {
			return "", fmt.Errorf("mermaid: click actions are not supported")
		}
	}
	firstLine, _, _ := strings.Cut(code, "\n")
	fields := strings.Fields(firstLine)
	if len(fields) == 0 || (fields[0] != "flowchart" && fields[0] != "graph" && fields[0] != "sequenceDiagram") {
		return "", fmt.Errorf("mermaid: model output must start with flowchart, graph or sequenceDiagram")
	}
	return mermaidSourcePrefix + code, nil
}
