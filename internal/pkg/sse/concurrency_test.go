package sse

import (
	"sync"
	"testing"
)

func TestPublishCanRaceUnsubscribeWithoutSendingToClosedChannels(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 1000 {
			h.Publish(SSEEvent{Topic: "task", Name: "content_delta"})
		}
	})
	for range 100 {
		_, cancel := h.Subscribe("task")
		cancel()
		cancel()
	}
	wg.Wait()
}

func TestSnapshotIsDeliveredOnlyToItsSubscriber(t *testing.T) {
	h := NewHub()
	first, closeFirst := h.Subscribe("task")
	defer closeFirst()
	second, closeSecond := h.SubscribeSnapshot("task", SSEEvent{Topic: "task", Name: "task_snapshot", Terminal: true})
	defer closeSecond()
	if got := recv(t, second); got.Name != "task_snapshot" || !got.Terminal {
		t.Fatalf("snapshot = %+v", got)
	}
	select {
	case got := <-first:
		t.Fatalf("snapshot was broadcast: %+v", got)
	default:
	}
}
