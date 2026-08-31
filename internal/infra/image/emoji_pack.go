package image

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

const bingImageSearchURL = "https://cn.bing.com/images/async"

var bingMimgSrc = regexp.MustCompile(`(?i)<img[^>]+class="[^"]*mimg[^"]*"[^>]+src="([^"]+)"`)

type EmojiPack struct {
	suffix  string
	timeout time.Duration
}

func NewEmojiPack(cfg config.EmojiPackConfig) *EmojiPack {
	cfg = cfg.Normalized()
	return &EmojiPack{
		suffix:  cfg.Suffix,
		timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
	}
}

func (p *EmojiPack) Method() port.ImageMethod { return port.MethodEmojiPack }

func (p *EmojiPack) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	kw := reqText(req, false)
	if kw == "" {
		return "", fmt.Errorf("emoji_pack: keywords required")
	}
	searchText := kw + p.suffix
	fetchURL := fmt.Sprintf("%s?q=%s&mmasync=1", bingImageSearchURL, url.QueryEscape(searchText))

	client := &http.Client{Timeout: p.timeout}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; wood-passage-creator/1.0)")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emoji_pack: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	m := bingMimgSrc.FindSubmatch(body)
	if len(m) < 2 {
		// fallback: any mimg src=
		alt := regexp.MustCompile(`(?i)src="(https?://[^"]+)"`)
		all := alt.FindAllSubmatch(body, 20)
		for _, sm := range all {
			u := string(sm[1])
			if strings.Contains(u, "th?id=") || strings.Contains(u, "images") {
				if i := strings.Index(u, "?"); i > 0 {
					// keep bing image ids often need query; only strip w/h style
					if strings.Contains(u, "w=") {
						u = u[:i]
					}
				}
				return u, nil
			}
		}
		return "", fmt.Errorf("emoji_pack: no image for %q", searchText)
	}
	src := string(m[1])
	if i := strings.Index(src, "?"); i > 0 && strings.Contains(src, "w=") {
		src = src[:i]
	}
	return src, nil
}
