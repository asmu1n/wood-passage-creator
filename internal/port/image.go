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

// FreeImageMethods 普通用户默认启用列表。
var FreeImageMethods = []ImageMethod{
	MethodPexels,
	MethodMermaid,
	MethodIconify,
	MethodEmojiPack,
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

// IsUserMethod 是否允许出现在创建请求 / enabled 列表。
func (m ImageMethod) IsUserMethod() bool {
	switch m.Normalize() {
	case MethodPexels, MethodIconify, MethodEmojiPack,
		MethodMermaid, MethodSVGDiagram, MethodNanoBanana:
		return true
	default:
		return false
	}
}

// IsVIPMethod 是否 VIP 专属。
func (m ImageMethod) IsVIPMethod() bool {
	switch m.Normalize() {
	case MethodNanoBanana, MethodSVGDiagram:
		return true
	default:
		return false
	}
}

func (m ImageMethod) MethodMeta() (name, description string) {
	switch m.Normalize() {
	case MethodPexels:
		return "search_pexels_image", "Search Pexels for a realistic stock photo. Use concise English keywords."
	case MethodIconify:
		return "search_iconify_icon", "Search Iconify for a clean vector icon. Use a short icon concept as keywords."
	case MethodEmojiPack:
		return "search_emoji_image", "Search an emoji/sticker image for a light, expressive illustration."
	case MethodMermaid:
		return "render_mermaid_diagram", "Render a Mermaid diagram. The prompt must contain complete valid Mermaid source code."
	case MethodSVGDiagram:
		return "generate_svg_diagram", "Generate an SVG information diagram from a precise visual description."
	case MethodNanoBanana:
		return "generate_ai_image", "Generate an original AI image from a detailed visual prompt."
	default:
		return "", ""
	}
}

// Allow 判断 method 是否在 enabled 内；enabled 为空表示不限制。
func Allow(enabled []ImageMethod, m ImageMethod) bool {
	if len(enabled) == 0 {
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

// ImageGenerator 配图生成。
type ImageGenerator interface {
	Generate(ctx context.Context, taskID string, reqs []ImageRequirement, allowedMethods []ImageMethod, onProgress ImageProgressFunc) ([]ImageResult, error)
}

// Provider 单一配图来源。
// 不要再维护 Available()==false 的“空壳”实现。
type Provider interface {
	Method() ImageMethod
	// Fetch 返回可公开访问的图片 URL，或 data: URL（生成类）。
	Fetch(ctx context.Context, req ImageRequirement) (url string, err error)
}
