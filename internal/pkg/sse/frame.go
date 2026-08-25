package sse

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// WriteEvent 将一条业务 SSE 帧写入 w。
// 格式：
//
//	event: <name>\n
//	data: <line>\n   （Data 按行拆分，可多行）
//	\n
//
// name 为空时不写 event 行；data 为空时写一行空 data，保证帧结构完整。
func WriteEvent(w io.Writer, name string, data []byte) error {
	if name != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", name); err != nil {
			return err
		}
	}

	// 规范化换行，避免 \r\n 残留 \r
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))

	if len(data) == 0 {
		if _, err := io.WriteString(w, "data: \n"); err != nil {
			return err
		}
	} else {
		lines := bytes.Split(data, []byte("\n"))
		for i := range lines {
			line := lines[i]
			if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
				return err
			}
		}
	}

	_, err := io.WriteString(w, "\n")
	return err
}

// WriteComment 写入 SSE 注释帧（客户端不可见，常用于心跳保活）。
// comment 不应包含换行；若包含会截断到首行。
func WriteComment(w io.Writer, comment string) error {
	if i := strings.IndexAny(comment, "\r\n"); i >= 0 {
		comment = comment[:i]
	}
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	return nil
}

// Flush 将已写入内容立即推给客户端。失败通常表示连接已断开。
func Flush(rc *http.ResponseController) error {
	if rc == nil {
		return nil
	}
	return rc.Flush()
}
