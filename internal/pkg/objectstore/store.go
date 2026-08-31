// Package objectstore 提供与业务无关的对象存储抽象。
// 当前实现 Cloudflare R2（S3 兼容）；配图、头像等均可复用。
package objectstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store 对象存储。未配置时 New 返回 nil，调用方 if store != nil 再上传即可。
type Store interface {
	// Put 上传对象，返回可公开访问的 URL。
	Put(ctx context.Context, in PutInput) (publicURL string, err error)
	// PublicBase 公开访问前缀（无尾斜杠）。
	PublicBase() string
}

// PutInput 一次上传请求。
type PutInput struct {
	// Folder 逻辑目录，如 "pexels"、"avatars"；会拼在 KeyPrefix 之后。
	Folder string
	// Name 对象文件名；空则自动 uuid + 扩展名。
	Name string
	// ContentType MIME，建议填写。
	ContentType string
	// Body 内容流。
	Body io.Reader
	// Size 可选；>0 时部分实现可少一次缓冲。
	Size int64
}

// Options 构造参数（不依赖 internal/config）。
type Options struct {
	// Provider 预留："r2"；空视为 r2。
	Provider string

	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Endpoint        string // 空则按 AccountID 推导 R2 endpoint
	PublicBaseURL   string // 对外 URL 前缀，必需才能启用
	KeyPrefix       string // 全局 key 前缀，如 articles/images
}

// Enabled 判断配置是否齐全能真正上传。
func (o Options) Enabled() bool {
	if strings.TrimSpace(o.AccessKeyID) == "" || strings.TrimSpace(o.SecretAccessKey) == "" {
		return false
	}
	if strings.TrimSpace(o.Bucket) == "" || strings.TrimSpace(o.PublicBaseURL) == "" {
		return false
	}
	if strings.TrimSpace(o.Endpoint) == "" && strings.TrimSpace(o.AccountID) == "" {
		return false
	}
	return true
}

// New 按 Options 创建 Store；未配齐或未知 Provider 时返回 nil。
func New(opt Options) Store {
	if !opt.Enabled() {
		return nil
	}
	provider := strings.ToLower(strings.TrimSpace(opt.Provider))
	if provider == "" || provider == "r2" {
		return newR2(opt)
	}
	return nil
}

// PutBytes 上传字节切片。s 为 nil 时返回错误。
func PutBytes(ctx context.Context, s Store, folder, contentType string, data []byte) (string, error) {
	if s == nil {
		return "", fmt.Errorf("objectstore: not configured")
	}
	return s.Put(ctx, PutInput{
		Folder:      folder,
		ContentType: contentType,
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
	})
}

// PublishSource 将 http(s) 或 data: URL 转存到 Store，返回公开 URL。
// s == nil 时原样返回 source（表示未启用对象存储）。
// 若 source 已位于 PublicBase 下则原样返回。
// maxBytes <=0 时默认 16MiB。
func PublishSource(ctx context.Context, s Store, source, folder string, maxBytes int64) (string, error) {
	if s == nil {
		return source, nil
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("objectstore: empty source")
	}
	if base := s.PublicBase(); base != "" && strings.HasPrefix(source, base+"/") {
		return source, nil
	}

	var (
		raw  []byte
		mime string
		err  error
	)
	switch {
	case strings.HasPrefix(source, "data:"):
		mime, raw, err = ParseDataURL(source)
		if err != nil {
			return "", err
		}
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		mime, raw, err = downloadURL(ctx, source, maxBytes)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("objectstore: unsupported source scheme")
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("objectstore: empty payload")
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return PutBytes(ctx, s, folder, mime, raw)
}

// ParseDataURL 解析 data:[<mime>][;base64],<payload>
func ParseDataURL(s string) (mime string, raw []byte, err error) {
	if !strings.HasPrefix(s, "data:") {
		return "", nil, fmt.Errorf("objectstore: not data url")
	}
	comma := strings.Index(s, ",")
	if comma < 0 {
		return "", nil, fmt.Errorf("objectstore: invalid data url")
	}
	meta := s[5:comma]
	payload := s[comma+1:]
	mime = "application/octet-stream"
	if i := strings.Index(meta, ";"); i >= 0 {
		if i > 0 {
			mime = meta[:i]
		}
	} else if meta != "" {
		mime = meta
	}
	if strings.Contains(meta, "base64") {
		b, decErr := base64.StdEncoding.DecodeString(payload)
		return mime, b, decErr
	}
	return mime, []byte(payload), nil
}

func downloadURL(ctx context.Context, rawURL string, maxBytes int64) (mime string, data []byte, err error) {
	if maxBytes <= 0 {
		maxBytes = 16 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("objectstore: download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("objectstore: download status=%d", resp.StatusCode)
	}
	limited, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("objectstore: read body: %w", err)
	}
	if int64(len(limited)) > maxBytes {
		return "", nil, fmt.Errorf("objectstore: object too large (>%d bytes)", maxBytes)
	}
	mime = resp.Header.Get("Content-Type")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	if mime == "" {
		mime = http.DetectContentType(limited)
	}
	return mime, limited, nil
}

// BuildObjectKey 拼 key：{keyPrefix}/{folder}/{name}
func BuildObjectKey(keyPrefix, folder, name string) string {
	parts := make([]string, 0, 3)
	if p := strings.Trim(strings.ReplaceAll(keyPrefix, "\\", "/"), "/"); p != "" {
		parts = append(parts, p)
	}
	if f := strings.Trim(strings.ReplaceAll(folder, "\\", "/"), "/"); f != "" {
		parts = append(parts, f)
	}
	name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" {
		name = uuid.New().String()
	}
	name = path.Base(name) // 防止 path traversal
	parts = append(parts, name)
	return path.Join(parts...)
}

// ExtFromMIME 常见 MIME → 扩展名。
func ExtFromMIME(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

// AutoName 按 MIME 生成 uuid 文件名。
func AutoName(contentType string) string {
	return uuid.New().String() + ExtFromMIME(contentType)
}
