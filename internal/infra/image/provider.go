package image

import (
	"context"

	"wood-passage-creator/internal/port"
)

// Provider 单一配图来源。
// 约定：NewXxx 在不可用时返回 nil，Generator.Register 会跳过；
// 不要再维护 Available()==false 的“空壳”实现。
type Provider interface {
	Method() port.ImageMethod
	// Fetch 返回可公开访问的图片 URL，或 data: URL（生成类）。
	Fetch(ctx context.Context, req port.ImageRequirement) (url string, err error)
}
