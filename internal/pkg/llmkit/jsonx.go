package llmkit

import (
	"encoding/json"
	"fmt"
)

func UnmarshalJSON(raw string, dst any) error {
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return fmt.Errorf(
			"parse json: %w; raw=%s",
			err,
			Truncate(raw, 500),
		)
	}
	return nil
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
