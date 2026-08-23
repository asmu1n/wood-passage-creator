package llmkit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractJSON 尽量从模型输出中截取 JSON（去掉 markdown fence / 前后废话）。
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// ```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	// 数组或对象
	if i := strings.IndexAny(s, "[{"); i >= 0 {
		j := strings.LastIndexAny(s, "]}")
		if j > i {
			return strings.TrimSpace(s[i : j+1])
		}
	}
	return s
}

func UnmarshalJSON(raw string, dst any) error {
	s := ExtractJSON(raw)
	if err := json.Unmarshal([]byte(s), dst); err != nil {
		return fmt.Errorf("parse json: %w; raw=%s", err, Truncate(raw, 500))
	}
	return nil
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
