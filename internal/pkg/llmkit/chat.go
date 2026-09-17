package llmkit

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// CollectTextStream 单次消费 Eino 消息流，同时转发文本增量并拼接完整内容。
func CollectTextStream(sr *schema.StreamReader[*schema.Message], onDelta func(string)) (string, error) {
	if sr == nil {
		return "", fmt.Errorf("message stream is nil")
	}
	defer sr.Close()

	var result strings.Builder
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return result.String(), nil
		}
		if err != nil {
			return result.String(), err
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}

		result.WriteString(chunk.Content)
		if onDelta != nil {
			onDelta(chunk.Content)
		}
	}
}
