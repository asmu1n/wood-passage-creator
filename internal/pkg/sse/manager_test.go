package sse

import (
	"testing"
	"time"
)

func TestFanOut_TwoSubscribersReceiveSameEvents(t *testing.T) {
	h := NewHub()
	ch1, unsub1 := h.Subscribe("task-a")
	ch2, unsub2 := h.Subscribe("task-a")
	defer unsub1()
	defer unsub2()

	ev := SSEEvent{Topic: "task-a", Name: "outline_delta", Data: []byte(`{"delta":"x"}`)}
	h.Publish(ev)

	got1 := recv(t, ch1)
	got2 := recv(t, ch2)
	if got1.Name != "outline_delta" || string(got1.Data) != `{"delta":"x"}` {
		t.Fatalf("sub1 got %+v", got1)
	}
	if got2.Name != "outline_delta" || string(got2.Data) != `{"delta":"x"}` {
		t.Fatalf("sub2 got %+v", got2)
	}
}

func TestFanOut_UnsubscribeDoesNotKickOthers(t *testing.T) {
	h := NewHub()
	ch1, unsub1 := h.Subscribe("task-a")
	ch2, unsub2 := h.Subscribe("task-a")
	defer unsub2()

	unsub1() // 第一个退订，不应影响第二个

	// ch1 应已关闭
	select {
	case _, ok := <-ch1:
		if ok {
			t.Fatal("ch1 should be closed")
		}
	default:
		// 也可能尚未读到零值；再等一下
		select {
		case _, ok := <-ch1:
			if ok {
				t.Fatal("ch1 should be closed")
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("ch1 not closed")
		}
	}

	h.Publish(SSEEvent{Topic: "task-a", Name: "connected", Data: []byte(`{}`)})
	got := recv(t, ch2)
	if got.Name != "connected" {
		t.Fatalf("sub2 should still receive, got %q", got.Name)
	}
}

func TestFanOut_DifferentTopicsIsolated(t *testing.T) {
	h := NewHub()
	chA, unsubA := h.Subscribe("task-a")
	chB, unsubB := h.Subscribe("task-b")
	defer unsubA()
	defer unsubB()

	h.Publish(SSEEvent{Topic: "task-a", Name: "outline_done", Data: []byte(`{"a":1}`)})

	got := recv(t, chA)
	if got.Name != "outline_done" {
		t.Fatalf("task-a got %q", got.Name)
	}
	select {
	case msg := <-chB:
		t.Fatalf("task-b should not receive, got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestFanOut_NoSubscribersNoPanic(t *testing.T) {
	h := NewHub()
	h.Publish(SSEEvent{Topic: "none", Name: "x", Data: []byte(`{}`)})
}

func recv(t *testing.T, ch <-chan SSEEvent) SSEEvent {
	t.Helper()
	select {
	case msg, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting event")
		return SSEEvent{}
	}
}
