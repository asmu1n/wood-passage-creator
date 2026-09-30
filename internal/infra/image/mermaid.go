package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/port"
)

const mermaidSourcePrefix = "mermaid:"

type Mermaid struct {
	llm model.BaseChatModel
}

func NewMermaid(llm model.BaseChatModel) *Mermaid {
	if llm == nil {
		return nil
	}
	return &Mermaid{llm: llm}
}

func (p *Mermaid) Metadata() port.ImageProviderMetadata {
	return port.ImageProviderMetadata{
		Method:             port.MethodMermaid,
		Access:             port.ImageAccessFree,
		PlannerDescription: "LLM 生成 Mermaid 流程图/时序图，由前端渲染",
		PlannerUsageGuide:  "imageSource=MERMAID；prompt 描述图中元素及其关系，不要填写 Mermaid 源码。",
		ToolName:           "render_mermaid_diagram",
		ToolDescription:    "Generate a Mermaid diagram from a description of the elements and their relationships. Do not supply Mermaid source code.",
	}
}

func (p *Mermaid) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	desc := reqText(req, true)
	if desc == "" {
		return "", fmt.Errorf("mermaid: prompt required")
	}
	prompt := fmt.Sprintf(`你是流程图设计师。请根据需求生成完整的 Mermaid 流程图或时序图源码。
要求：仅输出 Mermaid 源码，不要 markdown 代码围栏，不要解释文字；使用 Mermaid 支持的语法，中文节点文字用引号括起来。
需求：%s`, desc)
	response, err := p.llm.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return "", err
	}
	if response == nil {
		return "", fmt.Errorf("mermaid: empty model response")
	}
	code := strings.TrimSpace(response.Content)
	code = strings.TrimPrefix(code, "```mermaid")
	code = strings.TrimPrefix(code, "```")
	code = strings.TrimSuffix(code, "```")
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("mermaid: no source in model output")
	}
	firstLine, _, _ := strings.Cut(code, "\n")
	fields := strings.Fields(firstLine)
	if len(fields) == 0 || (fields[0] != "flowchart" && fields[0] != "graph" && fields[0] != "sequenceDiagram") {
		return "", fmt.Errorf("mermaid: model output must start with flowchart, graph or sequenceDiagram")
	}
	return mermaidSourcePrefix + code, nil
}
