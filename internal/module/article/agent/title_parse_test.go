package agent

import (
	"strings"
	"testing"
)

func TestParseTitleOptionsEnvelope(t *testing.T) {
	raw := `{"options":[{"mainTitle":"主标题1","subTitle":"副标题1"},{"mainTitle":"主标题2","subTitle":"副标题2"}]}`
	options, err := parseTitleOptions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 {
		t.Fatalf("options=%d, want 2", len(options))
	}
	if options[0].MainTitle != "主标题1" || options[0].SubTitle != "副标题1" {
		t.Fatalf("first option=%+v", options[0])
	}
}

func TestParseTitleOptionsEnvelopeWithFence(t *testing.T) {
	raw := "```json\n{\"options\":[{\"mainTitle\":\"A\",\"subTitle\":\"B\"}]}\n```"
	options, err := parseTitleOptions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 || options[0].MainTitle != "A" {
		t.Fatalf("got %+v", options)
	}
}

func TestParseTitleOptionsRejectsTopLevelArray(t *testing.T) {
	raw := `[{"mainTitle":"A","subTitle":"B"}]`
	_, err := parseTitleOptions(raw)
	if err == nil {
		t.Fatal("expected error when root is array instead of {\"options\":[...]}")
	}
}

func TestParseTitleOptionsEmptyOptions(t *testing.T) {
	raw := `{"options":[]}`
	options, err := parseTitleOptions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 0 {
		t.Fatalf("got %+v", options)
	}
}

func TestParseTitleOptionsInvalidJSON(t *testing.T) {
	raw := `{"options":[{"mainTitle":"a` + "\n" + `b","subTitle":"c"}]}`
	_, err := parseTitleOptions(raw)
	if err == nil {
		t.Fatal("expected parse error for bare newline in string")
	}
	if !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("err=%v", err)
	}
}
