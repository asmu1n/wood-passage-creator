package image

import (
	"encoding/base64"
	"fmt"
	"strings"

	"wood-passage-creator/internal/port"
)

const (
	MethodPexels     = "PEXELS"
	MethodPicsum     = "PICSUM"
	MethodIconify    = "ICONIFY"
	MethodEmojiPack  = "EMOJI_PACK"
	MethodMermaid    = "MERMAID"
	MethodSVGDiagram = "SVG_DIAGRAM"
	MethodNanoBanana = "NANO_BANANA"
)

func dataURL(mime string, raw []byte) string {
	mime = strings.TrimSpace(mime)
	if mime == "" {
		mime = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(raw))
}

func reqText(req port.ImageRequirement, preferPrompt bool) string {
	k := strings.TrimSpace(req.Keywords)
	p := strings.TrimSpace(req.Prompt)
	if preferPrompt {
		if p != "" {
			return p
		}
		return k
	}
	if k != "" {
		return k
	}
	return p
}
