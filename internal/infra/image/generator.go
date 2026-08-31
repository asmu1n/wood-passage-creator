package image

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/objectstore"
	"wood-passage-creator/internal/port"

	"golang.org/x/sync/errgroup"
)

const defaultConcurrency = 4

// Generator 实现 port.ImageGenerator。
type Generator struct {
	log         *slog.Logger
	providers   map[port.ImageMethod]Provider
	fallback    Provider
	store       objectstore.Store
	concurrency int
}

// NewGenerator 按完整配置注册可用 Provider。
// llm 用于 SVG_DIAGRAM；可 nil（则不注册 SVG）。
// store 可选：传入已构造的 objectstore.Store（与头像等共用同一实例）；
// 未传时按 cfg.R2 自行 New；未配置则为 nil，fetch 时跳过转存。
func NewGenerator(cfg *config.Config, llm port.ChatModel, store ...objectstore.Store) port.ImageGenerator {
	if cfg == nil {
		cfg = &config.Config{}
	}
	var st objectstore.Store
	if len(store) > 0 && store[0] != nil {
		st = store[0]
	} else {
		st = newObjectStore(cfg)
	}
	g := &Generator{
		log:         logger.Module("infra.image"),
		providers:   make(map[port.ImageMethod]Provider),
		fallback:    NewPicsum(),
		store:       st,
		concurrency: defaultConcurrency,
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
func (g *Generator) Register(p Provider) {
	if g == nil || p == nil {
		return
	}
	if g.providers == nil {
		g.providers = make(map[port.ImageMethod]Provider)
	}
	g.providers[p.Method().Normalize()] = p
}

// RegisteredMethods 返回已注册（New 非 nil）的 method。
func (g *Generator) RegisteredMethods() []port.ImageMethod {
	if g == nil {
		return nil
	}
	out := make([]port.ImageMethod, 0, len(g.providers))
	for k, p := range g.providers {
		if p != nil {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Generate 按需求并行拉图；onProgress 在每张成功时回调。
func (g *Generator) Generate(ctx context.Context, taskID string, reqs []port.ImageRequirement, onProgress port.ImageProgressFunc) ([]port.ImageResult, error) {
	if g == nil {
		return nil, fmt.Errorf("image generator is nil")
	}
	if g.log == nil {
		g.log = slog.Default()
	}
	if len(reqs) == 0 {
		return nil, nil
	}

	total := len(reqs)
	g.log.Info("image generate start",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.generate.start",
		"task_id", taskID,
		"count", total,
		"providers", port.ImageMethodsToStrings(g.RegisteredMethods()),
	)

	limit := g.concurrency
	if limit <= 0 {
		limit = defaultConcurrency
	}

	type slot struct {
		res port.ImageResult
		ok  bool
	}
	slots := make([]slot, total)
	var doneCount atomic.Int32

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(limit)

	for i := range reqs {
		i := i
		req := reqs[i]
		eg.Go(func() error {
			res, err := g.fetchOne(ctx, taskID, req)
			if err != nil {
				g.log.Warn("image fetch skipped",
					logger.FieldPurpose, logger.PurposeJob,
					logger.FieldEvent, "image.generate.item_failed",
					logger.FieldErr, err,
					"task_id", taskID,
					"source", req.ImageSource,
					"position", req.Position,
				)
				return nil
			}
			slots[i] = slot{res: res, ok: true}
			n := int(doneCount.Add(1))
			if onProgress != nil {
				onProgress(ctx, n, total, res)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	out := make([]port.ImageResult, 0, total)
	for i := range slots {
		if slots[i].ok {
			out = append(out, slots[i].res)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})

	g.log.Info("image generate done",
		logger.FieldPurpose, logger.PurposeJob,
		logger.FieldEvent, "image.generate.done",
		"task_id", taskID,
		"requested", total,
		"generated", len(out),
	)
	return out, nil
}

func (g *Generator) fetchOne(ctx context.Context, taskID string, req port.ImageRequirement) (port.ImageResult, error) {
	src := req.ImageSource.Normalize()
	if src == "" {
		src = port.MethodPexels
	}

	url, method, err := g.tryProvider(ctx, src, req, false)
	if err != nil && g.fallback != nil {
		g.log.Info("image fallback",
			logger.FieldPurpose, logger.PurposeJob,
			logger.FieldEvent, "image.generate.fallback",
			"task_id", taskID,
			"from", src,
			"to", g.fallback.Method(),
			"position", req.Position,
		)
		url, method, err = g.tryProvider(ctx, g.fallback.Method(), req, true)
	}
	if err != nil {
		return port.ImageResult{}, err
	}

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
	}, nil
}

func (g *Generator) tryProvider(ctx context.Context, method port.ImageMethod, req port.ImageRequirement, isFallback bool) (url string, usedMethod port.ImageMethod, err error) {
	method = method.Normalize()
	var p Provider
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
	return u, p.Method(), nil
}

func newObjectStore(cfg *config.Config) objectstore.Store {
	if cfg == nil {
		return nil
	}
	r := cfg.R2
	return objectstore.New(objectstore.Options{
		Provider:        "r2",
		AccountID:       r.AccountID,
		AccessKeyID:     r.AccessKeyID,
		SecretAccessKey: r.SecretAccessKey,
		Bucket:          r.Bucket,
		Endpoint:        r.Endpoint,
		PublicBaseURL:   r.PublicBaseURL,
		KeyPrefix:       r.KeyPrefix,
	})
}
