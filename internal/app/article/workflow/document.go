package articleworkflow

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"wood-passage-creator/internal/port"
)

const CoverSlotID = "cover"

// ByteOffset refers to the immutable source Markdown and is never model input.
type ImageSlot struct {
	ID         string `json:"id"`
	ByteOffset int    `json:"-"`
}

type ContentBlock struct {
	ID          string `json:"id"`
	Markdown    string `json:"markdown"`
	AfterSlotID string `json:"afterSlotId"`
}

type ArticleDocument struct {
	Content string         `json:"-"`
	Blocks  []ContentBlock `json:"blocks"`
	Slots   []ImageSlot    `json:"slots"`
}

// Capture top-level block starts during parsing; AST line spans alone omit
// opening fences and container markers. No renderer rewrites the source.
type positionedBlockParser struct {
	parser.BlockParser
	starts *[]int
}

func (p positionedBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	_, position := reader.Position()
	start := position.Start - reader.LineOffset()
	node, state := p.BlockParser.Open(parent, reader, pc)
	if node != nil && parent.Kind() == ast.KindDocument && state&parser.RequireParagraph == 0 {
		*p.starts = append(*p.starts, start)
	}
	return node, state
}

func BuildArticleDocument(content string) (ArticleDocument, error) {
	if strings.TrimSpace(content) == "" {
		return ArticleDocument{}, fmt.Errorf("article content is required")
	}
	starts := []int{0}
	blockParsers := parser.DefaultBlockParsers()
	for i := range blockParsers {
		blockParsers[i].Value = positionedBlockParser{
			BlockParser: blockParsers[i].Value.(parser.BlockParser), starts: &starts,
		}
	}
	p := parser.NewParser(parser.WithBlockParsers(blockParsers...))
	root := p.Parse(text.NewReader([]byte(content)))
	slices.Sort(starts)
	starts = slices.Compact(starts)
	doc := ArticleDocument{Content: content, Slots: []ImageSlot{{ID: CoverSlotID}}}
	for i, start := range starts {
		end := len(content)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if start >= end {
			continue
		}
		block := ContentBlock{ID: fmt.Sprintf("block-%d", len(doc.Blocks)+1), Markdown: content[start:end]}
		// A later top-level block proves this boundary is outside containers.
		// EOF is unsafe for an unclosed fence, HTML block or nested container.
		last := root.LastChild()
		safeEOF := last != nil && (last.Kind() == ast.KindParagraph || last.Kind() == ast.KindHeading ||
			last.Kind() == ast.KindThematicBreak)
		if end < len(content) || safeEOF {
			block.AfterSlotID = "after:" + block.ID
			doc.Slots = append(doc.Slots, ImageSlot{ID: block.AfterSlotID, ByteOffset: end})
		}
		doc.Blocks = append(doc.Blocks, block)
	}
	return doc, nil
}

func AssignImageTasks(plan ImagePlan, doc ArticleDocument, allowed []port.ImageMethod) ([]ImageTask, error) {
	if err := plan.Validate(doc.Slots, allowed); err != nil {
		return nil, err
	}
	offsets := lo.SliceToMap(doc.Slots, func(slot ImageSlot) (string, int) { return slot.ID, slot.ByteOffset })
	images := slices.Clone(plan.Images)
	slices.SortStableFunc(images, func(a, b PlannedImage) int { return offsets[a.SlotID] - offsets[b.SlotID] })
	position := 2
	tasks := make([]ImageTask, 0, len(images))
	for _, image := range images {
		current := 1
		if image.Kind != ImageKindCover {
			current = position
			position++
		}
		tasks = append(tasks, ImageTask{ID: fmt.Sprintf("image-%d", current), Position: current, PlannedImage: image})
	}
	return tasks, nil
}

func ComposeArticle(doc ArticleDocument, images []GeneratedImage) (string, error) {
	slots := make(map[string]int, len(doc.Slots))
	for _, slot := range doc.Slots {
		if slot.ByteOffset < 0 || slot.ByteOffset > len(doc.Content) {
			return "", fmt.Errorf("image slot %q is outside the source document", slot.ID)
		}
		slots[slot.ID] = slot.ByteOffset
	}
	type insertion struct {
		offset int
		text   string
	}
	insertions := make([]insertion, 0, len(images))
	seen := map[string]bool{}
	for _, image := range images {
		if err := image.Task.Validate(); err != nil {
			return "", err
		}
		if err := image.Asset.Validate(); err != nil {
			return "", err
		}
		offset, ok := slots[image.Task.SlotID]
		if !ok || seen[image.Task.SlotID] {
			return "", fmt.Errorf("unknown or repeated image slot %q", image.Task.SlotID)
		}
		seen[image.Task.SlotID] = true
		markdown, err := assetMarkdown(image)
		if err != nil {
			return "", err
		}
		insertions = append(insertions, insertion{offset, "\n\n" + markdown + "\n\n"})
	}
	slices.SortFunc(insertions, func(a, b insertion) int { return a.offset - b.offset })
	var out strings.Builder
	cursor := 0
	for _, item := range insertions {
		out.WriteString(doc.Content[cursor:item.offset])
		out.WriteString(item.text)
		cursor = item.offset
	}
	out.WriteString(doc.Content[cursor:])
	return out.String(), nil
}

func assetMarkdown(image GeneratedImage) (string, error) {
	if image.Asset.Method == "MERMAID" {
		code, ok := strings.CutPrefix(image.Asset.URL, "mermaid:")
		if !ok || strings.TrimSpace(code) == "" {
			return "", fmt.Errorf("Mermaid asset has no source")
		}
		fence := "```"
		for strings.Contains(code, fence) {
			fence += "`"
		}
		return fence + "mermaid\n" + code + "\n" + fence, nil
	}
	u, err := url.Parse(image.Asset.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "data") ||
		strings.ContainsAny(image.Asset.URL, "\r\n<>") {
		return "", fmt.Errorf("image asset must be an HTTP, HTTPS or data URL")
	}
	if (u.Scheme == "http" || u.Scheme == "https") && u.Host == "" ||
		u.Scheme == "data" && !strings.HasPrefix(image.Asset.URL, "data:image/") {
		return "", fmt.Errorf("image asset has no HTTP host or image media type")
	}
	alt := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ", "\r", " ").Replace(image.Task.Brief)
	return "![" + alt + "](<" + image.Asset.URL + ">)", nil
}
