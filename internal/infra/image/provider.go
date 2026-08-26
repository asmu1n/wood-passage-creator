package image

import (
	"context"

	"wood-passage-creator/internal/port"
)

// Provider 单一配图来源（Pexels / Picsum / 后续 AI 生图等）。
type Provider interface {
	Method() string
	Available() bool
	// Fetch 返回可公开访问的图片 URL（当前阶段不做 COS 转存）。
	Fetch(ctx context.Context, req port.ImageRequirement) (url string, err error)
}
