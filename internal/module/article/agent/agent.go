package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
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

// Agent 单个任务智能体：读写共享 ArticleState。
// 阶段 1：只依赖 port.ChatModel，不引入 Eino Graph/ADK。
type Agent interface {
	Name() Name
	Execute(ctx context.Context, state *article.ArticleState) error
}

// StreamFromContext 从 ctx 取流式回调（由 Orchestrator/Service 注入）。
// 流式回调放入 context 为的是简化调用链和函数签名，避免在每个 agent 中都传递回调函数。同时能够便捷的复用取出。
type streamCtxKey struct{}

func WithStreamHandler(ctx context.Context, h port.StreamHandler) context.Context {
	if h == nil {
		return ctx
	}
	return context.WithValue(ctx, streamCtxKey{}, h)
}

func streamHandlerFrom(ctx context.Context) port.StreamHandler {
	h, _ := ctx.Value(streamCtxKey{}).(port.StreamHandler)
	return h
}

// base 公共依赖。
type base struct {
	llm port.ChatModel
	log *slog.Logger
}

func newBase(llm port.ChatModel) base {
	return base{
		llm: llm,
		log: logger.Module("article.agent"),
	}
}

func (b base) generate(ctx context.Context, userPrompt string, opt *port.ChatOptions) (string, error) {
	if b.llm == nil {
		return "", fmt.Errorf("chat model is nil")
	}
	return b.llm.Generate(ctx, []port.Message{
		{Role: port.RoleUser, Content: userPrompt},
	}, opt)
}

func (b base) stream(ctx context.Context, userPrompt string, opt *port.ChatOptions) (string, error) {
	if b.llm == nil {
		return "", fmt.Errorf("chat model is nil")
	}
	h := streamHandlerFrom(ctx)
	return b.llm.Stream(ctx, []port.Message{
		{Role: port.RoleUser, Content: userPrompt},
	}, opt, h)
}

// extractJSON 尽量从模型输出中截取 JSON（去掉 markdown fence / 前后废话）。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// ```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	// 数组或对象
	if i := strings.IndexAny(s, "[{"); i >= 0 {
		j := strings.LastIndexAny(s, "]}")
		if j > i {
			return strings.TrimSpace(s[i : j+1])
		}
	}
	return s
}

func unmarshalJSON(raw string, dst any) error {
	s := extractJSON(raw)
	if err := json.Unmarshal([]byte(s), dst); err != nil {
		return fmt.Errorf("parse json: %w; raw=%s", err, truncate(raw, 500))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func requireTitle(state *article.ArticleState) error {
	if state == nil || state.Title == nil {
		return fmt.Errorf("state.title is required")
	}
	if state.Title.MainTitle == "" {
		return fmt.Errorf("state.title.mainTitle is required")
	}
	return nil
}
