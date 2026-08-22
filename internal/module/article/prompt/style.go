package prompt

import "wood-passage-creator/internal/module/article"

const (
	styleTechPrompt = `

**重要：请使用科技风格进行创作**
- 语言专业、严谨，多使用专业术语和行业词汇
- 逻辑清晰，重视数据和事实支撑
- 叙述客观理性，避免主观情感表达
- 突出技术创新、发展趋势、解决方案
- 可适当引用权威资料或专家观点`

	styleEmotionalPrompt = `

**重要：请使用情感风格进行创作**
- 语言温暖细腻，富有感染力和共鸣
- 善用比喻、排比等修辞手法增强表现力
- 注重情感表达，讲述真实故事和感悟
- 引发读者情感共鸣，传递正能量
- 适当使用抒情语句，增加文章温度`

	styleEducationalPrompt = `

**重要：请使用教育风格进行创作**
- 语言通俗易懂，深入浅出地讲解概念
- 结构清晰，循序渐进，便于学习理解
- 多用案例、类比帮助读者理解复杂内容
- 总结重点知识点，提供实用的学习建议
- 鼓励思考，启发读者自主学习和探索`

	styleHumorousPrompt = `

**重要：请使用轻松幽默风格进行创作**
- 语言轻松活泼，幽默风趣
- 善用网络流行语、俏皮话和有趣的比喻
- 适当自嘲或调侃，增加趣味性
- 内容轻松易读，让读者在愉快中获取信息
- 可加入一些有趣的段子或梗，但不失专业性`
)

// StyleSuffix 返回风格附加说明；未知或空返回空串。
func StyleSuffix(style article.ArticleStyle) string {
	switch style {
	case article.StyleTech:
		return styleTechPrompt
	case article.StyleEmotional:
		return styleEmotionalPrompt
	case article.StyleEducational:
		return styleEducationalPrompt
	case article.StyleHumorous:
		return styleHumorousPrompt
	default:
		return ""
	}
}

// ValidStyles 全部合法风格。
func ValidStyles() []article.ArticleStyle {
	return []article.ArticleStyle{article.StyleTech, article.StyleEmotional, article.StyleEducational, article.StyleHumorous}
}

// IsValidStyle 空串视为合法（未指定风格）。
func IsValidStyle(style article.ArticleStyle) bool {
	if style == "" {
		return true
	}
	for _, s := range ValidStyles() {
		if s == style {
			return true
		}
	}
	return false
}
