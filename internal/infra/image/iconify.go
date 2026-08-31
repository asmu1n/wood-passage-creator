package image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type Iconify struct {
	baseURL string
	client  *http.Client
}

func NewIconify(cfg config.IconifyConfig) *Iconify {
	cfg = cfg.Normalized()
	return &Iconify{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		client:  &http.Client{Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond},
	}
}

func (p *Iconify) Method() string {
	return MethodIconify
}

func (p *Iconify) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	q := reqText(req, false)
	if q == "" {
		return "", fmt.Errorf("iconify: keywords required")
	}
	searchURL := fmt.Sprintf("%s/search?query=%s&limit=5", p.baseURL, url.QueryEscape(q))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("iconify: status %d", resp.StatusCode)
	}
	var parsed struct {
		Icons []string `json:"icons"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Icons) == 0 {
		return "", fmt.Errorf("iconify: no icon for %q", q)
	}
	return fmt.Sprintf("%s/%s.svg", p.baseURL, parsed.Icons[0]), nil
}
