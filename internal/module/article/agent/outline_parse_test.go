package agent

import (
	"testing"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/llmkit"
)

func TestUnmarshalOutlineTopLevelArray(t *testing.T) {
	raw := "```json\n[{\"section\":1,\"title\":\"引入\",\"points\":[\"a\",\"b\"]}]\n```"
	var sections []article.OutlineSection
	if err := llmkit.UnmarshalJSON(raw, &sections); err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Title != "引入" {
		t.Fatalf("got %+v", sections)
	}
}
