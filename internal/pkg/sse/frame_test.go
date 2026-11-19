package sse

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteEvent_SingleLineJSON(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEvent(&buf, "outline_delta", []byte(`{"delta":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := "event: outline_delta\ndata: {\"delta\":\"x\"}\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteEvent_MultiLineData(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEvent(&buf, "outline_delta", []byte("a\nb"))
	if err != nil {
		t.Fatal(err)
	}
	want := "event: outline_delta\ndata: a\ndata: b\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteEvent_CRLFNormalized(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEvent(&buf, "x", []byte("a\r\nb\rc"))
	if err != nil {
		t.Fatal(err)
	}
	want := "event: x\ndata: a\ndata: b\ndata: c\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteEvent_EmptyData(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEvent(&buf, "connected", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "event: connected\ndata: \n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteEvent_EmptyName(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEvent(&buf, "", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	want := "data: {\"ok\":true}\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteComment(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteComment(&buf, "ping"); err != nil {
		t.Fatal(err)
	}
	want := ": ping\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteComment_StripsNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteComment(&buf, "ping\nmore"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != ": ping\n\n" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(buf.String(), "more") {
		t.Fatal("should not contain rest after newline")
	}
}
