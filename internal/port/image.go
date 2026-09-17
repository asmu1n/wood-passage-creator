package port

import (
	"context"
	"encoding/json"
	"strings"
)

// ImageRequirement 配图需求。
type ImageRequirement struct {
	Position      int         `json:"position"`
	Type          string      `json:"type"`
	SectionTitle  string      `json:"sectionTitle"`
	ImageSource   ImageMethod `json:"imageSource"`
	Keywords      string      `json:"keywords"`
	Prompt        string      `json:"prompt"`
	PlaceholderID string      `json:"placeholderId"`
}

// ImageResult 配图结果。
type ImageResult struct {
	Position      int         `json:"position"`
	URL           string      `json:"url"`
	Method        ImageMethod `json:"method"`
	Keywords      string      `json:"keywords"`
	SectionTitle  string      `json:"sectionTitle"`
	Description   string      `json:"description"`
	PlaceholderID string      `json:"placeholderId"`
}

// ImageMethod 配图来源枚举（JSON 即为字符串；反序列化时自动规范化）。
type ImageMethod string

const (
	MethodPexels     ImageMethod = "PEXELS"
	MethodIconify    ImageMethod = "ICONIFY"
	MethodEmojiPack  ImageMethod = "EMOJI_PACK"
	MethodMermaid    ImageMethod = "MERMAID"
	MethodSVGDiagram ImageMethod = "SVG_DIAGRAM"
	MethodNanoBanana ImageMethod = "NANO_BANANA"
	MethodPicsum     ImageMethod = "PICSUM" // 仅系统 fallback，不可出现在请求中
)

// ImageProviderAccess 描述 Provider 对业务用户的开放范围。
// INTERNAL 仅供系统内部流程使用，不可由用户选择，也不会注入 Agent。
type ImageProviderAccess string

const (
	ImageAccessFree     ImageProviderAccess = "FREE"
	ImageAccessVIP      ImageProviderAccess = "VIP"
	ImageAccessInternal ImageProviderAccess = "INTERNAL"
)

// ImageProviderMetadata 是一个图片 Provider 对外暴露的完整 LLM 元信息。
// Planner* 用于配图规划提示词；Tool* 用于构建模型可调用的工具。
// 该结构不依赖具体 LLM 框架，由 infra 层转换为 Eino ToolInfo。
type ImageProviderMetadata struct {
	Method             ImageMethod
	Access             ImageProviderAccess
	PlannerDescription string
	PlannerUsageGuide  string
	ToolName           string
	ToolDescription    string
}

func (m ImageMethod) String() string {
	return string(m)
}

func (m ImageMethod) Normalize() ImageMethod {
	return ImageMethod(strings.ToUpper(strings.TrimSpace(string(m))))
}

// UnmarshalJSON 使 HTTP/JSON 绑定后即为规范大写枚举，减少内部再转换。
func (m *ImageMethod) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*m = ImageMethod(s).Normalize()
	return nil
}

// Allow 判断 method 是否在 enabled 内；nil 表示不限制，非 nil 空列表表示全部禁用。
func Allow(enabled []ImageMethod, m ImageMethod) bool {
	if enabled == nil {
		return true
	}
	m = m.Normalize()
	for _, e := range enabled {
		if e.Normalize() == m {
			return true
		}
	}
	return false
}

// ImageProgressFunc 单张成功回调。
type ImageProgressFunc func(ctx context.Context, done, total int, img ImageResult)

// ImageProviderCatalog 返回当前已注册且通过调用方授权过滤的 Provider 元信息。
// Provider 的配置可用性由实现层在注册阶段决定。
type ImageProviderCatalog interface {
	LookupProvider(method ImageMethod) (ImageProviderMetadata, bool)
	AvailableProviders(allowedMethods []ImageMethod) []ImageProviderMetadata
}

// ImageGenerator 配图生成，同时作为 ImageAgent 的 Provider 元信息来源。
type ImageGenerator interface {
	ImageProviderCatalog
	Generate(ctx context.Context, taskID string, reqs []ImageRequirement, allowedMethods []ImageMethod, onProgress ImageProgressFunc) ([]ImageResult, error)
}

// Provider 单一配图来源。
// 不要再维护 Available()==false 的“空壳”实现。
type Provider interface {
	Metadata() ImageProviderMetadata
	// Fetch 返回可公开访问的图片 URL，或 data: URL（生成类）。
	Fetch(ctx context.Context, req ImageRequirement) (url string, err error)
}
