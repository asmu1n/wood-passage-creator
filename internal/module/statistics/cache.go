package statistics

import (
	"context"
	"time"

	"wood-passage-creator/internal/port"
)

// 概览缓存约定：全站一份，与查询用户无关。
// 其它 module 若需失效，依赖 OverviewInvalidator 接口（由 Service 实现），勿复制 key 字符串。
const (
	// OverviewCacheKey Redis/L1 缓存键。
	OverviewCacheKey = "statistics:overview"
	// OverviewCacheTTL 概览缓存有效期（infra 层仍可能对 TTL 做抖动）。
	OverviewCacheTTL = time.Hour
)

// OverviewInvalidator 供 article/user 等在写路径上主动失效概览缓存。
// 由 statistics.Service 实现；调用方只依赖本接口，避免循环依赖细节。
type OverviewInvalidator interface {
	InvalidateOverview(ctx context.Context)
}

// InvalidateOverviewCache 在仅有 port.Cache 时失效概览（无 Service 实例时可用）。
func InvalidateOverviewCache(ctx context.Context, c port.Cache) {
	if c == nil {
		return
	}
	_ = c.Delete(ctx, OverviewCacheKey)
}
