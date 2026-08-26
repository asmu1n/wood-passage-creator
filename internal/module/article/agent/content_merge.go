package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// ContentMerger 阶段 3d：用真实图片 URL 替换占位符（无 LLM）。
type ContentMerger struct {
	log *slog.Logger
}

func NewContentMerger() agent {
	return &ContentMerger{log: logger.Module("article.agent")}
}

func (a *ContentMerger) Name() Name { return NameContentMerger }

func (a *ContentMerger) Execute(ctx context.Context, state *article.ArticleState) error {
	_ = ctx
	if state == nil {
		return fmt.Errorf("state is nil")
	}
	body := state.ContentWithPlaceholders
	if body == "" {
		body = state.Content
	}

	merged := mergeImages(body, state.Images)
	state.FullContent = merged

	a.log.Info("agent done",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "agent.content_merge.done",
		"task_id", state.TaskID,
		"images", len(state.Images),
		"chars", len(merged),
	)
	return nil
}

func mergeImages(content string, images []port.ImageResult) string {
	if len(images) == 0 {
		return content
	}
	out := content
	for _, img := range images {
		if img.PlaceholderID == "" || img.URL == "" {
			continue
		}
		md := fmt.Sprintf("![%s](%s)", altText(img), img.URL)
		out = strings.ReplaceAll(out, img.PlaceholderID, md)
	}
	return out
}

func altText(img port.ImageResult) string {
	if img.SectionTitle != "" {
		return img.SectionTitle
	}
	if img.Description != "" {
		return img.Description
	}
	return "image"
}
