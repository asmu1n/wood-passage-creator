package agent

import (
	"context"
	"fmt"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

// Name 阶段/任务标识（日志、可观测）。
type Name string

const (
	NameTitleGenerator   Name = "title_generator"
	NameOutlineGenerator Name = "outline_generator"
	NameContentGenerator Name = "content_generator"
	NameImageGenerator   Name = "image_generator"
	NameContentMerger    Name = "content_merger"
)

// agent 单个任务智能体：读写共享 ArticleState。
type agent interface {
	Name() Name
	Execute(ctx context.Context, state *article.ArticleState) error
}

type streamingAgent interface {
	Name() Name
	Execute(ctx context.Context, state *article.ArticleState, onDelta func(string)) error
}

type agentWithModify interface {
	streamingAgent
	ExecuteWithModify(ctx context.Context, state *article.ArticleState, modifySuggestion string) error
}

type imageAgent interface {
	Name() Name

	Execute(
		ctx context.Context,
		state *article.ArticleState,
		onPlanned func([]port.ImageRequirement),
		onImage port.ImageProgressFunc,
	) error
}

func requireTitle(state *article.ArticleState) error {
	if state == nil || state.MainTitle == nil || *state.MainTitle == "" {
		return fmt.Errorf("state.mainTitle is required")
	}
	if state.SubTitle == nil || *state.SubTitle == "" {
		return fmt.Errorf("state.subTitle is required")
	}
	return nil
}
