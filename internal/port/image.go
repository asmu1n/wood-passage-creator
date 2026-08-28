package port

import "context"

// ImageRequirement 配图需求（由配图分析产出，供 ImageGenerator 消费）。
type ImageRequirement struct {
	Position      int    `json:"position"`
	Type          string `json:"type"`
	SectionTitle  string `json:"sectionTitle"`
	ImageSource   string `json:"imageSource"` // PEXELS/NANO_BANANA/MERMAID/...
	Keywords      string `json:"keywords"`
	Prompt        string `json:"prompt"`
	PlaceholderID string `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ImageResult 配图结果（写入文章 state / 持久化）。
type ImageResult struct {
	Position      int    `json:"position"`
	URL           string `json:"url"`
	Method        string `json:"method"`
	Keywords      string `json:"keywords"`
	SectionTitle  string `json:"sectionTitle"`
	Description   string `json:"description"`
	PlaceholderID string `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ImageProgressFunc 单张图片成功时回调；done 为已成功张数，total 为需求总数。
// 实现不得长时间阻塞；可 nil。
type ImageProgressFunc func(ctx context.Context, done, total int, img ImageResult)

// ImageGenerator 根据需求列表生成图片结果。
// 实现位于 infra；编排层负责写回业务 state 与 SSE，infra 不依赖 article。
type ImageGenerator interface {
	Generate(ctx context.Context, taskID string, reqs []ImageRequirement, onProgress ImageProgressFunc) ([]ImageResult, error)
}
