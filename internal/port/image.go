package port

import (
	"context"
	"strings"
)

// ImageRequirement 配图需求（由配图分析产出，供 ImageGenerator 消费）。
type ImageRequirement struct {
	Position      int         `json:"position"`
	Type          string      `json:"type"`
	SectionTitle  string      `json:"sectionTitle"`
	ImageSource   ImageMethod `json:"imageSource"` // PEXELS/NANO_BANANA/MERMAID/...
	Keywords      string      `json:"keywords"`
	Prompt        string      `json:"prompt"`
	PlaceholderID string      `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ImageResult 配图结果（写入文章 state / 持久化）。
type ImageResult struct {
	Position      int         `json:"position"`
	URL           string      `json:"url"`
	Method        ImageMethod `json:"method"`
	Keywords      string      `json:"keywords"`
	SectionTitle  string      `json:"sectionTitle"`
	Description   string      `json:"description"`
	PlaceholderID string      `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ImageMethod 配图来源枚举（API / state / provider 统一使用）。
type ImageMethod string

const (
	MethodPexels     ImageMethod = "PEXELS"
	MethodPicsum     ImageMethod = "PICSUM"
	MethodIconify    ImageMethod = "ICONIFY"
	MethodEmojiPack  ImageMethod = "EMOJI_PACK"
	MethodMermaid    ImageMethod = "MERMAID"
	MethodSVGDiagram ImageMethod = "SVG_DIAGRAM"
	MethodNanoBanana ImageMethod = "NANO_BANANA"
)

// AllImageMethods 业务可见的全部配图方式（不含仅作降级的 PICSUM）。
var ALL_IMAGE_METHODS = [6]ImageMethod{
	MethodPexels,
	MethodIconify,
	MethodEmojiPack,
	MethodMermaid,
	MethodSVGDiagram,
	MethodNanoBanana,
}

// FreeImageMethods 普通用户默认可选（非 VIP）。
var FREE_IMAGE_METHODS = [4]ImageMethod{
	MethodPexels,
	MethodMermaid,
	MethodIconify,
	MethodEmojiPack,
}

// VIPOnlyImageMethods 仅 VIP/Admin 可选。
var VIP_ONLY_IMAGE_METHODS = [2]ImageMethod{
	MethodNanoBanana,
	MethodSVGDiagram,
}

func (m ImageMethod) String() string {
	return string(m)
}

// Normalize 去空白并转大写。
func (m ImageMethod) Normalize() ImageMethod {
	return ImageMethod(strings.ToUpper(strings.TrimSpace(string(m))))
}

// IsValid 是否为已知配图方式（含 PICSUM 降级）。
func (m ImageMethod) IsValid() bool {
	switch m.Normalize() {
	case MethodPexels, MethodPicsum, MethodIconify, MethodEmojiPack,
		MethodMermaid, MethodSVGDiagram, MethodNanoBanana:
		return true
	default:
		return false
	}
}

// RequiresVIP 是否为 VIP 专属配图方式。
func (m ImageMethod) RequiresVIP() bool {
	switch m.Normalize() {
	case MethodNanoBanana, MethodSVGDiagram:
		return true
	default:
		return false
	}
}

// ParseImageMethod 从字符串解析；非法时返回空串（!IsValid）。
func ParseImageMethod(s string) ImageMethod {
	return ImageMethod(s).Normalize()
}

// ImageMethodsToStrings 便于 JSON/DB []string 字段。
func ImageMethodsToStrings(ms []ImageMethod) []string {
	if len(ms) == 0 {
		return nil
	}
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.String()
	}
	return out
}

// ParseImageMethods 从 []string 解析并规范化；跳过空串。
func ParseImageMethods(ss []string) []ImageMethod {
	if len(ss) == 0 {
		return nil
	}
	out := make([]ImageMethod, 0, len(ss))
	for _, s := range ss {
		m := ParseImageMethod(s)
		if m == "" {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ImageProgressFunc 单张图片成功时回调；done 为已成功张数，total 为需求总数。
// 实现不得长时间阻塞；可 nil。
type ImageProgressFunc func(ctx context.Context, done, total int, img ImageResult)

// ImageGenerator 根据需求列表生成图片结果。
// 实现位于 infra；编排层负责写回业务 state 与 SSE，infra 不依赖 article。
type ImageGenerator interface {
	Generate(ctx context.Context, taskID string, reqs []ImageRequirement, onProgress ImageProgressFunc) ([]ImageResult, error)
}
