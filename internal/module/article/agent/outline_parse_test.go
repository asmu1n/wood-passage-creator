package agent

import (
	"strings"
	"testing"
)

func TestParseOutlineSectionsEnvelope(t *testing.T) {
	raw := `{"sections":[{"section":1,"title":"引入","points":["a","b"]}]}`
	sections, err := parseOutlineSections(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Title != "引入" {
		t.Fatalf("got %+v", sections)
	}
}

func TestParseOutlineSectionsEnvelopeWithFence(t *testing.T) {
	raw := "```json\n{\"sections\":[{\"section\":1,\"title\":\"引入\",\"points\":[\"a\",\"b\"]}]}\n```"
	sections, err := parseOutlineSections(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Title != "引入" {
		t.Fatalf("got %+v", sections)
	}
}

func TestParseOutlineSectionsRejectsTopLevelArray(t *testing.T) {
	raw := `[{"section":1,"title":"引入","points":["a"]}]`
	_, err := parseOutlineSections(raw)
	if err == nil {
		t.Fatal("expected error when root is array instead of {\"sections\":[...]}")
	}
	if !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("err=%v", err)
	}
}
