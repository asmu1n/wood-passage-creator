package prompt

import (
	"encoding/json"
	"strings"
	"wood-passage-creator/internal/module/article"
)

// Render 仅替换 vars 中出现的 {{key}}；未提供的占位符（如 {{IMAGE_PLACEHOLDER_N}}）原样保留。
func Render(tpl string, vars map[string]string, style *article.ArticleStyle) string {
	if len(vars) == 0 {
		return tpl
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	// 冒泡排序，长 key 优先，避免前缀误替换
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	out := tpl
	for _, k := range keys {
		out = strings.ReplaceAll(out, "{{"+k+"}}", vars[k])
	}

	if style != nil {
		out += StyleSuffix(*style)
	}
	return out
}

// TitleOptions 生成标题方案 prompt。
func TitleOptions(topic string, style *article.ArticleStyle) string {
	return Render(titleOptionsTpl, map[string]string{"topic": topic}, style)
}

// Outline 生成大纲 prompt。
func Outline(mainTitle, subTitle, userDescription string, style *article.ArticleStyle) string {
	desc := ""
	if strings.TrimSpace(userDescription) != "" {
		desc = Render(descriptionSectionTpl, map[string]string{
			"userDescription": userDescription,
		}, style)
	}
	return Render(outlineTpl, map[string]string{
		"mainTitle":          mainTitle,
		"subTitle":           subTitle,
		"descriptionSection": desc,
	}, style)
}

// Content 生成正文 prompt；outline 可为 JSON 字符串或格式化文本。
func Content(mainTitle, subTitle, outline string, style *article.ArticleStyle) string {
	return Render(contentTpl, map[string]string{
		"mainTitle": mainTitle,
		"subTitle":  subTitle,
		"outline":   outline,
	}, style)
}

// ContentFromSections 将章节列表序列化后生成正文 prompt。
func ContentFromSections(mainTitle, subTitle string, sections any, style *article.ArticleStyle) (string, error) {
	b, err := json.Marshal(sections)
	if err != nil {
		return "", err
	}
	return Content(mainTitle, subTitle, string(b), style), nil
}

// ImageRequirements 配图需求分析 prompt。
// 注意：模板内字面量 {{IMAGE_PLACEHOLDER_N}} 不会被替换。
func ImageRequirements(mainTitle, content, availableMethods, methodUsageGuide string) string {
	return Render(imageRequirementsTpl, map[string]string{
		"mainTitle":        mainTitle,
		"content":          content,
		"availableMethods": availableMethods,
		"methodUsageGuide": methodUsageGuide,
	}, nil)
}

// ModifyOutline AI 改大纲 prompt。
func ModifyOutline(mainTitle, subTitle, outlineJSON, suggestion string) string {
	return Render(modifyOutlineTpl, map[string]string{
		"mainTitle":        mainTitle,
		"subTitle":         subTitle,
		"outline":          outlineJSON,
		"modifySuggestion": suggestion,
	}, nil)
}
