package articleworkflow

import (
	"strings"
	"testing"

	"wood-passage-creator/internal/port"
)

func TestDocumentPreservesSourceAndUsesOnlyBlockBoundaries(t *testing.T) {
	for _, source := range []string{
		"# 标题\r\n\r\n第一段。\r\n\r\n第二段。\r\n",
		"Intro\n\n```go\n\nx := 1\n\n```\n\nAfter\n",
		"Intro\n\n- first\n\n  continuation\n- second\n\nAfter\n",
		"Intro\n\n> quoted\n>\n> second\n\nAfter\n",
		"Title\n=====\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\nAfter\n",
		"Intro\n\n```go\nunclosed\n",
		"Intro\n\n<div>\n\nunclosed html",
	} {
		t.Run(source, func(t *testing.T) {
			doc, err := BuildArticleDocument(source)
			if err != nil {
				t.Fatal(err)
			}
			var rebuilt strings.Builder
			for _, block := range doc.Blocks {
				rebuilt.WriteString(block.Markdown)
			}
			if rebuilt.String() != source {
				t.Fatalf("source changed: %q", rebuilt.String())
			}
			if doc.Slots[0].ID != CoverSlotID || doc.Slots[0].ByteOffset != 0 {
				t.Fatal("missing cover slot")
			}
			for _, slot := range doc.Slots[1:] {
				if slot.ByteOffset <= 0 || slot.ByteOffset > len(source) {
					t.Fatalf("bad offset: %+v", slot)
				}
				if strings.HasPrefix(source[slot.ByteOffset:], "x :=") ||
					strings.HasPrefix(source[slot.ByteOffset:], "  continuation") ||
					strings.HasPrefix(source[slot.ByteOffset:], "| 1") ||
					strings.HasPrefix(source[slot.ByteOffset:], "> second") {
					t.Fatalf("slot lies inside a compound block: %+v", slot)
				}
			}
			merged, err := ComposeArticle(doc, nil)
			if err != nil || merged != source {
				t.Fatalf("no-image composition rewrote content: %q %v", merged, err)
			}
		})
	}
}

func TestUnclosedCodeHasNoEndSlot(t *testing.T) {
	source := "Intro\n\n```go\ninside\n"
	doc, err := BuildArticleDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range doc.Slots {
		if slot.ByteOffset > strings.Index(source, "```") {
			t.Fatalf("slot inside unclosed code: %+v", slot)
		}
	}
}

func TestCompositionInsertsOnlySuccessfulImagesWithoutRenumbering(t *testing.T) {
	source := "第一段。\r\n\r\n## 第二节\r\n\r\n正文中有 {{IMAGE_PLACEHOLDER_1}} 字面量。\r\n"
	doc, err := BuildArticleDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	plan := ImagePlan{Images: []PlannedImage{
		{SlotID: doc.Slots[1].ID, Kind: ImageKindIllustration, Brief: "正文示意图", PreferredMethod: port.MethodMermaid},
		coverTask().PlannedImage,
	}}
	tasks, err := AssignImageTasks(plan, doc, []port.ImageMethod{port.MethodPexels, port.MethodMermaid})
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].Position != 1 || tasks[1].Position != 2 {
		t.Fatalf("task ordering = %+v", tasks)
	}
	code := "flowchart TD\n  A-->B"
	merged, err := ComposeArticle(doc, []GeneratedImage{{Task: tasks[1], Asset: ImageAsset{URL: "mermaid:" + code, Method: port.MethodMermaid}}})
	if err != nil {
		t.Fatal(err)
	}
	inserted := "\n\n```mermaid\n" + code + "\n```\n\n"
	if strings.Replace(merged, inserted, "", 1) != source {
		t.Fatalf("original Markdown changed: %q", merged)
	}
	if !strings.Contains(merged, "{{IMAGE_PLACEHOLDER_1}}") {
		t.Fatal("literal source placeholder was interpreted")
	}
	if tasks[1].Position != 2 {
		t.Fatal("skipped cover must not promote the body image")
	}
}

func TestCompositionRejectsDuplicateSlotsAndUnsafeAssets(t *testing.T) {
	doc, _ := BuildArticleDocument("Body")
	image := GeneratedImage{Task: coverTask(), Asset: ImageAsset{URL: "https://example.com/image(a).png", Method: port.MethodPexels}}
	if text, err := ComposeArticle(doc, []GeneratedImage{image}); err != nil || !strings.Contains(text, "(<https://") {
		t.Fatalf("valid parenthesized URL was not protected: %q %v", text, err)
	}
	if _, err := ComposeArticle(doc, []GeneratedImage{image, image}); err == nil {
		t.Fatal("duplicate placement accepted")
	}
	for _, source := range []string{"javascript:alert(1)", "https://example.com/a\ninjected", "https://example.com/<tag>"} {
		image.Asset.URL = source
		if _, err := ComposeArticle(doc, []GeneratedImage{image}); err == nil {
			t.Errorf("unsafe source accepted: %q", source)
		}
	}
}
