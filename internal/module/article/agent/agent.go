package agent

import (
	"context"
	"fmt"

	"wood-passage-creator/internal/module/article"
)

// Name 阶段/任务标识（日志、可观测）。
type Name string

const (
	NameTitleGenerator   Name = "title_generator"
	NameOutlineGenerator Name = "outline_generator"
	NameContentGenerator Name = "content_generator"
	NameImageAnalyzer    Name = "image_analyzer"
	NameContentMerger    Name = "content_merger"
)

// agent 单个任务智能体：读写共享 ArticleState。
// 阶段 1：只依赖 port.ChatModel，不引入 Eino Graph/ADK。
type agent interface {
	Name() Name
	Execute(ctx context.Context, state *article.ArticleState) error
}

type agentWithModify interface {
	agent
	ExecuteWithModify(ctx context.Context, state *article.ArticleState, modifySuggestion string) error
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
