package port

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type ChatOptions struct {
	Temperature *float64
}

// StreamHandler 每收到一片增量文本回调一次；返回 error 可中断流。
type StreamHandler func(ctx context.Context, delta string) error

// ChatModel 业务侧 LLM 端口。
type ChatModel interface {
	Generate(ctx context.Context, messages []Message, opt *ChatOptions) (string, error)
	Stream(ctx context.Context, messages []Message, opt *ChatOptions, handler StreamHandler) (string, error)
}
