package image

import (
	"context"
	"fmt"
	"strings"

	"wood-passage-creator/internal/port"
)

type NanoBanana struct {
	generate func(context.Context, string) (string, error)
}

func NewNanoBanana(generate func(context.Context, string) (string, error)) *NanoBanana {
	if generate == nil {
		return nil
	}
	return &NanoBanana{generate: generate}
}

func (p *NanoBanana) Metadata() port.ImageProviderMetadata {
	return port.ImageProviderMetadata{
		Method: port.MethodNanoBanana, Access: port.ImageAccessVIP,
		PlannerDescription: "AI 生图（VIP）",
		PlannerUsageGuide:  "适合原创 AI 图片；执行工具时 prompt 填完整画面描述。",
		ToolName:           "generate_ai_image", ToolDescription: "Generate an original AI image from a detailed visual prompt.",
	}
}

func (p *NanoBanana) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	description := reqText(req, true)
	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("nano_banana: prompt required")
	}
	return p.generate(ctx, description)
}
