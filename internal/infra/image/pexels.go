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

	"wood-passage-creator/internal/port"
)

// Pexels 通过关键词检索 Pexels 图库。
type Pexels struct {
	apiKey string
	client *http.Client
}

func NewPexels(apiKey string) *Pexels {
	return &Pexels{
		apiKey: strings.TrimSpace(apiKey),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Pexels) Method() string {
	return MethodPexels
}

func (p *Pexels) Available() bool {
	return p != nil && p.apiKey != ""
}

func (p *Pexels) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	if !p.Available() {
		return "", fmt.Errorf("pexels: api key not configured")
	}
	q := strings.TrimSpace(req.Keywords)
	if q == "" {
		q = strings.TrimSpace(req.Prompt)
	}
	if q == "" {
		return "", fmt.Errorf("pexels: keywords/prompt required")
	}

	apiURL := fmt.Sprintf(
		"https://api.pexels.com/v1/search?query=%s&per_page=1",
		url.QueryEscape(q),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", p.apiKey)

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
		return "", fmt.Errorf("pexels: status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		Photos []struct {
			Src struct {
				Large    string `json:"large"`
				Original string `json:"original"`
			} `json:"src"`
		} `json:"photos"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("pexels: decode: %w", err)
	}
	if len(parsed.Photos) == 0 {
		return "", fmt.Errorf("pexels: no photo for %q", q)
	}
	src := parsed.Photos[0].Src
	if src.Large != "" {
		return src.Large, nil
	}
	if src.Original != "" {
		return src.Original, nil
	}
	return "", fmt.Errorf("pexels: empty image url")
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
