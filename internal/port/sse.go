package port

type SSEEvent struct {
	Topic string // 路由键（如 taskId），不是 SSE 帧 id:
	Name  string // SSE event:
	Data  []byte // SSE data:（通常为 JSON）
}

type SSEHub interface {
	// Subscribe 在 topic 下新增订阅者（fan-out，互不踢除）。
	// 返回该订阅者专属 ch，以及 cancel（只退订自己；可安全多次调用）。
	Subscribe(topic string) (<-chan SSEEvent, func())
	// Publish 向 topic 上所有订阅者投递；无订阅者则 no-op。
	Publish(event SSEEvent)
}
