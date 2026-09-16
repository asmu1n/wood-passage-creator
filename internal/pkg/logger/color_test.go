package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorizeText(t *testing.T) {
	input := []byte(`time=2026-09-16T10:00:00.000+08:00 level=INFO msg="article created" service=wood-passage-creator module=article purpose=biz event=article.created err="example error" count=1` + "\n")
	got := string(colorizeText(input))

	expected := []string{
		ansiDim + "time=2026-09-16T10:00:00.000+08:00" + ansiReset,
		ansiGreen + "level=INFO" + ansiReset,
		ansiBrightWhite + `msg="article created"` + ansiReset,
		ansiMagenta + "module=article" + ansiReset,
		ansiCyan + "purpose=biz" + ansiReset,
		ansiBlue + "event=article.created" + ansiReset,
		ansiRed + `err="example error"` + ansiReset,
	}
	for _, part := range expected {
		if !strings.Contains(got, part) {
			t.Errorf("colorized log does not contain %q; got %q", part, got)
		}
	}
	if !strings.Contains(got, " count=1\n") {
		t.Errorf("ordinary attribute or trailing newline changed; got %q", got)
	}
}

func TestColorizeTextKeepsQuotedEscapesTogether(t *testing.T) {
	input := []byte(`level=ERROR msg="say \"hello world\"" err="bad value"`)
	got := colorizeText(input)

	if count := bytes.Count(got, []byte(ansiBrightWhite)); count != 1 {
		t.Fatalf("message was split into %d colored sections; got %q", count, got)
	}
	if count := bytes.Count(got, []byte(ansiRed)); count != 1 {
		t.Fatalf("error was split into %d colored sections; got %q", count, got)
	}
}

func TestColorWriterReportsOriginalLength(t *testing.T) {
	var output bytes.Buffer
	writer := colorWriter{output: &output}
	input := []byte("level=WARN msg=test\n")

	n, err := writer.Write(input)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("Write() = %d, want %d", n, len(input))
	}
	if !bytes.Contains(output.Bytes(), []byte(ansiYellow)) {
		t.Fatalf("warning color missing from %q", output.String())
	}
}
