package image

import (
	"bytes"
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

// NanoBanana 基于 Gemini 图像生成 HTTP API（可选；无 key 时 New 返回 nil）。
type NanoBanana struct {
	apiKey string
	model  string
	client *http.Client
}

func NewNanoBanana(cfg config.NanoBananaConfig) *NanoBanana {
	cfg = cfg.Normalized()
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil
	}
	return &NanoBanana{
		apiKey: apiKey,
		model:  cfg.Model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *NanoBanana) Method() port.ImageMethod { return port.MethodNanoBanana }

func (p *NanoBanana) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	prompt := reqText(req, true)
	if prompt == "" {
		return "", fmt.Errorf("nano_banana: prompt required")
	}

	// Gemini generateContent REST（image modality 随模型变化；失败则返回明确错误）
	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		urlPathEscape(p.model), p.apiKey,
	)
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nano_banana: status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	// 解析 inlineData
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	for _, c := range parsed.Candidates {
		for _, part := range c.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				mime := part.InlineData.MimeType
				if mime == "" {
					mime = "image/png"
				}
				// already base64
				return fmt.Sprintf("data:%s;base64,%s", mime, part.InlineData.Data), nil
			}
		}
	}
	return "", fmt.Errorf("nano_banana: no image in response")
}

func urlPathEscape(s string) string {
	return url.PathEscape(s)
}
