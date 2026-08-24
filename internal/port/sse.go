package port

type SSEHub interface {
	Subscribe(id string) (<-chan []byte, func()) // 返回 ch + unsubscribe
	Publish(id string, payload []byte)           // 无订阅者则 no-op
}
