package agent

import "wood-passage-creator/internal/port"

// DefaultImageMethodGuides 配图方式说明（Analyzer 再按 enabled 过滤）。
func DefaultImageMethodGuides() []ImageMethodGuide {
	return []ImageMethodGuide{
		{Code: port.MethodPexels, Description: "Pexels 免费图库，适合真实照片", UsageGuide: "imageSource=PEXELS；keywords 填英文检索词；无需 prompt。"},
		{Code: port.MethodIconify, Description: "Iconify 开源图标库，适合简洁图标", UsageGuide: "imageSource=ICONIFY；keywords 填图标语义（如 rocket、chart）。"},
		{Code: port.MethodEmojiPack, Description: "网络表情包检索", UsageGuide: "imageSource=EMOJI_PACK；keywords 填中文主题词。"},
		{Code: port.MethodMermaid, Description: "Mermaid 流程图/时序图", UsageGuide: "imageSource=MERMAID；prompt 填完整 mermaid 源码。"},
		{Code: port.MethodSVGDiagram, Description: "LLM 生成 SVG 示意图（VIP）", UsageGuide: "imageSource=SVG_DIAGRAM；prompt 描述示意图内容。"},
		{Code: port.MethodNanoBanana, Description: "AI 生图（VIP）", UsageGuide: "imageSource=NANO_BANANA；prompt 填画面描述。"},
	}
}
