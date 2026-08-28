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
	"wood-passage-creator/internal/port"

	"golang.org/x/sync/errgroup"
)

const defaultConcurrency = 4

// Generator 实现 port.ImageGenerator。
type Generator struct {
	log         *slog.Logger
	providers   map[string]Provider
	fallback    Provider
	concurrency int
}

// NewGenerator 根据配置装配默认 providers（Pexels + Picsum 降级）。
func NewGenerator(pexelsCfg config.PexelsConfig) port.ImageGenerator {
	g := &Generator{
		log:         logger.Module("infra.image"),
		providers:   make(map[string]Provider),
		fallback:    NewPicsum(),
		concurrency: defaultConcurrency,
	}
	g.Register(NewPexels(pexelsCfg.APIKey))
	return g
}

// Register 注册/覆盖一种配图来源。
func (g *Generator) Register(p Provider) {
	if g == nil || p == nil {
		return
	}
	if g.providers == nil {
		g.providers = make(map[string]Provider)
	}
	g.providers[strings.ToUpper(p.Method())] = p
}

// Generate 按需求并行拉图，返回结果列表（按 position 排序）。
// onProgress 在每张成功时回调（done 为成功计数，total 为需求总数）；可为 nil。
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
					"placeholder", req.PlaceholderID,
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
	src := strings.ToUpper(strings.TrimSpace(req.ImageSource))
	if src == "" {
		src = MethodPexels
	}

	url, method, err := g.tryProvider(ctx, src, req, false)
	if err != nil && g.fallback != nil && g.fallback.Available() {
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

func (g *Generator) tryProvider(ctx context.Context, method string, req port.ImageRequirement, isFallback bool) (url, usedMethod string, err error) {
	method = strings.ToUpper(method)
	var p Provider
	if isFallback {
		p = g.fallback
	} else if g.providers != nil {
		p = g.providers[method]
	}
	if p == nil || !p.Available() {
		return "", "", fmt.Errorf("provider unavailable: %s", method)
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
