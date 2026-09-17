package image

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/infra/objectstore"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// ProviderExecutor 是图片工具的确定性执行层，不负责 LLM 编排。
type ProviderExecutor struct {
	log       *slog.Logger
	providers map[port.ImageMethod]port.Provider
	fallback  port.Provider
	store     port.ObjectStore
}

// NewProviderExecutor 按完整配置注册可用 Provider。
// llm 用于 SVG_DIAGRAM；可 nil（则不注册 SVG）。
// store 为 port.ObjectStore（cmd 装配；未配置可为 nil，fetch 时跳过转存）。
func NewProviderExecutor(cfg *config.Config, llm model.BaseChatModel, store port.ObjectStore) *ProviderExecutor {
	if cfg == nil {
		cfg = &config.Config{}
	}
	g := &ProviderExecutor{
		log:       logger.Module("infra.image"),
		providers: make(map[port.ImageMethod]port.Provider),
		fallback:  NewPicsum(),
		store:     store,
	}

	g.Register(NewPexels(cfg.Pexels.APIKey))
	g.Register(NewIconify(cfg.Iconify))
	g.Register(NewEmojiPack(cfg.EmojiPack))
	g.Register(NewMermaid(cfg.Mermaid))
	g.Register(NewNanoBanana(cfg.NanoBanana))
	if llm != nil {
		g.Register(NewSVGDiagram(cfg.SVGDiagram, llm))
	}
	return g
}

// Register 注册/覆盖一种配图来源（nil 跳过）。
func (g *ProviderExecutor) Register(p port.Provider) {
	if g == nil || p == nil {
		return
	}
	metadata := p.Metadata()
	method := metadata.Method.Normalize()
	if method == "" || strings.TrimSpace(metadata.ToolName) == "" || !validProviderAccess(metadata.Access) {
		return
	}
	if g.providers == nil {
		g.providers = make(map[port.ImageMethod]port.Provider)
	}
	g.providers[method] = p
}

// RegisteredMethods 返回已注册（New 非 nil）的 method。
func (g *ProviderExecutor) RegisteredMethods() []port.ImageMethod {
	if g == nil {
		return nil
	}
	out := make([]port.ImageMethod, 0, len(g.providers))
	for k, p := range g.providers {
		if p != nil {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

// LookupProvider 返回一个实际已注册 Provider 的规范化元信息。
func (g *ProviderExecutor) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	if g == nil || g.providers == nil {
		return port.ImageProviderMetadata{}, false
	}
	method = method.Normalize()
	provider := g.providers[method]
	if provider == nil {
		return port.ImageProviderMetadata{}, false
	}
	metadata := provider.Metadata()
	metadata.Method = method
	return metadata, true
}

// AvailableProviders 返回实际已注册且在 allowedMethods 范围内的 Provider 元信息。
// 注册表是 Prompt 规划与 Tool Calling 共用的唯一可用性来源。
func (g *ProviderExecutor) AvailableProviders(allowedMethods []port.ImageMethod) []port.ImageProviderMetadata {
	methods := g.RegisteredMethods()
	out := make([]port.ImageProviderMetadata, 0, len(methods))
	for _, method := range methods {
		if !port.Allow(allowedMethods, method) {
			continue
		}
		metadata, ok := g.LookupProvider(method)
		if !ok || metadata.Access == port.ImageAccessInternal {
			continue
		}
		out = append(out, metadata)
	}
	return out
}

func validProviderAccess(access port.ImageProviderAccess) bool {
	switch access {
	case port.ImageAccessFree, port.ImageAccessVIP, port.ImageAccessInternal:
		return true
	default:
		return false
	}
}

// Execute 只执行指定 provider，不做 fallback，供 tool calling 循环把失败结果反馈给模型。
func (g *ProviderExecutor) Execute(ctx context.Context, taskID string, req port.ImageRequirement, method port.ImageMethod) (port.ImageResult, error) {
	if g == nil {
		return port.ImageResult{}, fmt.Errorf("image provider executor is nil")
	}
	if g.log == nil {
		g.log = slog.Default()
	}
	url, usedMethod, err := g.tryProvider(ctx, method, req, false)
	if err != nil {
		return port.ImageResult{}, err
	}
	return g.publishResult(ctx, taskID, req, url, usedMethod), nil
}

// ExecuteWithFallback 执行首选 provider，失败时使用系统 fallback。
func (g *ProviderExecutor) ExecuteWithFallback(ctx context.Context, taskID string, req port.ImageRequirement) (port.ImageResult, error) {
	if g == nil {
		return port.ImageResult{}, fmt.Errorf("image provider executor is nil")
	}
	if g.log == nil {
		g.log = slog.Default()
	}
	src := req.ImageSource.Normalize()
	if src == "" {
		src = port.MethodPexels
	}

	url, method, err := g.tryProvider(ctx, src, req, false)
	if err != nil && g.fallback != nil {
		fallbackMethod := g.fallback.Metadata().Method.Normalize()
		g.log.Info("image fallback",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.generate.fallback",
			"task_id", taskID,
			"from", src,
			"to", fallbackMethod,
			"position", req.Position,
		)
		url, method, err = g.tryProvider(ctx, fallbackMethod, req, true)
	}
	if err != nil {
		return port.ImageResult{}, err
	}
	return g.publishResult(ctx, taskID, req, url, method), nil
}

func (g *ProviderExecutor) publishResult(ctx context.Context, taskID string, req port.ImageRequirement, url string, method port.ImageMethod) port.ImageResult {
	// 可选对象存储转存：未配置 store==nil，直接保留原 URL
	if g.store != nil {
		folder := strings.ToLower(method.String())
		if published, uerr := objectstore.PublishSource(ctx, g.store, url, folder, 0); uerr != nil {
			g.log.Warn("objectstore publish failed, keep original url",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldEvent, "image.objectstore.failed",
				logger.FieldErr, uerr,
				"task_id", taskID,
				"method", method,
			)
		} else if published != "" {
			url = published
		}
	}

	return port.ImageResult{
		Position:      req.Position,
		URL:           url,
		Method:        method,
		Keywords:      req.Keywords,
		SectionTitle:  req.SectionTitle,
		Description:   req.Type,
		PlaceholderID: req.PlaceholderID,
	}
}

func (g *ProviderExecutor) tryProvider(ctx context.Context, method port.ImageMethod, req port.ImageRequirement, isFallback bool) (url string, usedMethod port.ImageMethod, err error) {
	method = method.Normalize()
	var p port.Provider
	if isFallback {
		p = g.fallback
	} else if g.providers != nil {
		p = g.providers[method]
	}
	if p == nil {
		return "", "", fmt.Errorf("provider not registered: %s", method)
	}
	u, err := p.Fetch(ctx, req)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(u) == "" {
		return "", "", fmt.Errorf("provider %s returned empty url", method)
	}
	return u, p.Metadata().Method.Normalize(), nil
}
