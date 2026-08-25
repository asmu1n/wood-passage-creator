package sse

import (
	"sync"
	"sync/atomic"

	"wood-passage-creator/internal/port"
)

// hub 按 topic（如 taskId）做 fan-out。
// 每个 Subscribe 分配独立 subID + channel；Publish 向该 topic 下所有订阅者各发一份。
type hub struct {
	mu     sync.RWMutex
	seq    atomic.Uint64 // 进程内单调递增 ID，从 1 起
	topics map[string]map[uint64]chan port.SSEEvent
}

func NewHub() port.SSEHub {
	return &hub{
		topics: make(map[string]map[uint64]chan port.SSEEvent),
	}
}

// Subscribe 在 topic 下新增一个订阅者，返回专属 channel 与退订函数。
// 多次 Subscribe 同一 topic 互不踢除；每个 reader 只读自己的 channel。
func (h *hub) Subscribe(topic string) (<-chan port.SSEEvent, func()) {
	subID := h.seq.Add(1)
	ch := make(chan port.SSEEvent, 64)

	h.mu.Lock()
	subs := h.topics[topic]
	if subs == nil {
		subs = make(map[uint64]chan port.SSEEvent)
		h.topics[topic] = subs
	}
	subs[subID] = ch
	h.mu.Unlock()

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()

			subs, ok := h.topics[topic]
			if !ok {
				return
			}
			cur, ok := subs[subID]
			// 仅移除自己的订阅，避免误伤同 topic 其他连接
			if !ok || cur != ch {
				return
			}
			delete(subs, subID)
			close(ch)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		})
	}
	return ch, unsub
}

// Publish 向 topic 上所有订阅者投递事件；无订阅者则 no-op。
// 单个订阅者 channel 满时丢弃该订阅者的本条消息，不影响其他人。
func (h *hub) Publish(event port.SSEEvent) {
	h.mu.RLock()
	subs := h.topics[event.Topic]
	if len(subs) == 0 {
		h.mu.RUnlock()
		return
	}
	// 拷贝 channel 列表后再发，避免持锁发送
	chs := make([]chan port.SSEEvent, 0, len(subs))
	for i := range subs {
		chs = append(chs, subs[i])
	}
	h.mu.RUnlock()

	for _, ch := range chs {
		select {
		case ch <- event:
		default:
			// 慢消费者：丢弃本条，保持 Publish 非阻塞
		}
	}
}
