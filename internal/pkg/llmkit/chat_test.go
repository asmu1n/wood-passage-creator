package llmkit

import (
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestCollectTextStream(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("hello", nil),
		nil,
		schema.AssistantMessage("", nil),
		schema.AssistantMessage(" world", nil),
	})

	var deltas []string
	text, err := CollectTextStream(stream, func(delta string) {
		deltas = append(deltas, delta)
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Fatalf("text=%q, want %q", text, "hello world")
	}
	if want := []string{"hello", " world"}; !reflect.DeepEqual(deltas, want) {
		t.Fatalf("deltas=%q, want %q", deltas, want)
	}
}

func TestCollectTextStreamRejectsNilStream(t *testing.T) {
	if _, err := CollectTextStream(nil, nil); err == nil {
		t.Fatal("expected nil stream error")
	}
}
