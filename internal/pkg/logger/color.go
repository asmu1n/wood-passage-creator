package logger

import (
	"bytes"
	"io"
	"os"
)

const (
	ansiReset       = "\x1b[0m"
	ansiDim         = "\x1b[2;37m"
	ansiBrightWhite = "\x1b[1;37m"
	ansiRed         = "\x1b[31m"
	ansiBrightRed   = "\x1b[1;31m"
	ansiGreen       = "\x1b[32m"
	ansiYellow      = "\x1b[33m"
	ansiBlue        = "\x1b[34m"
	ansiMagenta     = "\x1b[35m"
	ansiCyan        = "\x1b[36m"
)

// colorEnabled 仅在交互终端启用颜色，避免 ANSI 控制字符污染文件、
// 容器日志和日志采集系统。NO_COLOR 遵循 https://no-color.org/ 约定。
func colorEnabled(output *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := output.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// colorWriter 对 TextHandler 已完成编码的一整行日志着色。TextHandler 自身负责
// 串行写入，因此这里不会改变 slog 原有的并发与原子输出行为。
type colorWriter struct {
	output io.Writer
}

func (w colorWriter) Write(p []byte) (int, error) {
	colored := colorizeText(p)
	n, err := w.output.Write(colored)
	if err != nil {
		return 0, err
	}
	if n != len(colored) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

func colorizeText(input []byte) []byte {
	var output bytes.Buffer
	// ANSI 序列会使输出略长，预留一些空间以减少扩容。
	output.Grow(len(input) + 64)

	for len(input) > 0 {
		lineEnd := bytes.IndexByte(input, '\n')
		if lineEnd < 0 {
			colorizeLine(&output, input)
			break
		}
		colorizeLine(&output, input[:lineEnd])
		output.WriteByte('\n')
		input = input[lineEnd+1:]
	}

	return output.Bytes()
}

func colorizeLine(output *bytes.Buffer, line []byte) {
	for len(line) > 0 {
		fieldEnd := nextFieldEnd(line)
		field := line[:fieldEnd]
		color := fieldColor(field)
		if color != "" {
			output.WriteString(color)
		}
		output.Write(field)
		if color != "" {
			output.WriteString(ansiReset)
		}

		line = line[fieldEnd:]
		for len(line) > 0 && line[0] == ' ' {
			output.WriteByte(' ')
			line = line[1:]
		}
	}
}

// nextFieldEnd 按 TextHandler 的 key=value 格式切分字段，同时保留引号内的空格。
func nextFieldEnd(line []byte) int {
	inQuotes := false
	escaped := false
	for i, b := range line {
		if inQuotes {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inQuotes = false
			}
			continue
		}
		if b == '"' {
			inQuotes = true
			continue
		}
		if b == ' ' {
			return i
		}
	}
	return len(line)
}

func fieldColor(field []byte) string {
	separator := bytes.IndexByte(field, '=')
	if separator < 0 {
		return ""
	}
	key, value := string(field[:separator]), field[separator+1:]
	switch key {
	case "time", "service", "source":
		return ansiDim
	case "level":
		switch string(value) {
		case "DEBUG":
			return ansiCyan
		case "INFO":
			return ansiGreen
		case "WARN":
			return ansiYellow
		case "ERROR":
			return ansiBrightRed
		}
	case "msg":
		return ansiBrightWhite
	case FieldModule:
		return ansiMagenta
	case FieldPurpose:
		return ansiCyan
	case FieldEvent:
		return ansiBlue
	case FieldErr:
		return ansiRed
	}
	return ""
}
