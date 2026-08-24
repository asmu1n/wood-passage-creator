package sse

import (
	"sync"

	"wood-passage-creator/internal/port"
)

type hub struct {
	mu    sync.RWMutex
	conns map[string]chan []byte
}

func NewHub() port.SSEHub {
	return &hub{
		conns: make(map[string]chan []byte),
	}
}

func (h *hub) Subscribe(id string) (<-chan []byte, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.conns[id]; ok {
		delete(h.conns, id)
		close(old)
	}
	ch := make(chan []byte, 64)
	h.conns[id] = ch

	var once sync.Once
	unSub := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			cur, ok := h.conns[id]
			if ok && cur == ch {
				delete(h.conns, id)
				close(cur)
			}
		})
	}
	return ch, unSub
}

func (h *hub) Publish(id string, payload []byte) {
	h.mu.RLock()
	ch, ok := h.conns[id]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- payload:
	default:
	}
}
