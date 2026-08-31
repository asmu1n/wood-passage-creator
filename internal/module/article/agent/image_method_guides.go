package agent

import "wood-passage-creator/internal/port"

// DefaultImageMethodGuides 全部配图方式说明（Analyzer 再按 enabledImageMethods 过滤）。
// 实际能否出图取决于 infra 是否注册了对应 Provider（New 非 nil：key/CLI 等）。
func DefaultImageMethodGuides() []ImageMethodGuide {
	return []ImageMethodGuide{
		{
			Code:        port.MethodPexels,
			Description: "Pexels 免费图库，适合真实照片",
			UsageGuide:  "imageSource=PEXELS；keywords 填英文检索词；无需 prompt。",
		},
		{
			Code:        port.MethodIconify,
			Description: "Iconify 开源图标库，适合简洁图标",
			UsageGuide:  "imageSource=ICONIFY；keywords 填图标语义（如 rocket、chart）；返回 SVG URL。",
		},
		{
			Code:        port.MethodEmojiPack,
			Description: "网络表情包检索，适合轻松风格配图",
			UsageGuide:  "imageSource=EMOJI_PACK；keywords 填中文主题词，系统会自动拼接检索后缀。",
		},
		{
			Code:        port.MethodMermaid,
			Description: "Mermaid 流程图/时序图（需服务器安装 mmdc）",
			UsageGuide:  "imageSource=MERMAID；prompt 填完整 mermaid 源码（flowchart/sequenceDiagram 等）。",
		},
		{
			Code:        port.MethodSVGDiagram,
			Description: "LLM 生成 SVG 概念示意图",
			UsageGuide:  "imageSource=SVG_DIAGRAM；prompt 描述示意图内容与结构；keywords 可作补充。",
		},
		{
			Code:        port.MethodNanoBanana,
			Description: "Gemini 等模型 AI 生图（需 API Key）",
			UsageGuide:  "imageSource=NANO_BANANA；prompt 填英文或中文画面描述。",
		},
	}
}
