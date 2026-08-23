package llmkit

import (
	"context"
	"wood-passage-creator/internal/port"
)

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
