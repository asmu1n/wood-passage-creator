package agent

// DefaultImageMethodGuides 当前已实现配图方式的说明（注入 ImageAnalyzer）。
// 与 infra/image 中 MethodPexels 等常量保持字符串一致。
func DefaultImageMethodGuides() []ImageMethodGuide {
	return []ImageMethodGuide{
		{
			Code:        "PEXELS",
			Description: "Pexels 免费图库，适合真实照片类配图",
			UsageGuide:  "使用英文关键词检索照片。imageSource 必须为 PEXELS；keywords 填写检索词（优先英文）；无需 prompt。",
		},
	}
}
