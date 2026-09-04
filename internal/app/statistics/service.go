package statistics

import (
	"context"
	"log/slog"
	"time"

	module "wood-passage-creator/internal/module/statistics"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/port"
)

// Service 管理端统计概览。
type Service struct {
	repo  module.Repository
	cache port.Cache
	log   *slog.Logger
	loc   *time.Location
}

func NewService(repo module.Repository, cache port.Cache) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return &Service{
		repo:  repo,
		cache: cache,
		log:   logger.Module("app.statistics"),
		loc:   loc,
	}
}

// InvalidateOverview 删除概览缓存，使下次 GetOverview 重新聚合。
// 失败只记日志，不影响主业务流程。
func (s *Service) InvalidateOverview(ctx context.Context) {
	if s == nil || s.cache == nil {
		return
	}
	if err := s.cache.Delete(ctx, module.OverviewCacheKey); err != nil {
		s.log.Warn("invalidate overview cache failed",
			logger.FieldPurpose, logger.PurposeCache,
			logger.FieldEvent, "statistics.overview.invalidate_failed",
			logger.FieldErr, err,
			"key", module.OverviewCacheKey,
		)
		return
	}
	s.log.Info("overview cache invalidated",
		logger.FieldPurpose, logger.PurposeCache,
		logger.FieldEvent, "statistics.overview.invalidated",
		"key", module.OverviewCacheKey,
	)
}

// GetOverview 聚合系统概览；需管理员。命中缓存则跳过 DB 聚合。
func (s *Service) GetOverview(ctx context.Context, actor moduser.Actor) (*module.Overview, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, err
	}

	return port.TryFetch(ctx, s.cache, module.OverviewCacheKey, module.OverviewCacheTTL, func() (*module.Overview, error) {
		return s.computeOverview(ctx, actor.ID)
	})
}

func (s *Service) computeOverview(ctx context.Context, actorID int64) (*module.Overview, error) {
	now := time.Now().In(s.loc)
	todayStart := startOfDay(now)
	weekStart := startOfWeek(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc)

	today, err := s.repo.CountArticlesBetween(ctx, todayStart, now)
	if err != nil {
		return nil, err
	}
	week, err := s.repo.CountArticlesBetween(ctx, weekStart, now)
	if err != nil {
		return nil, err
	}
	month, err := s.repo.CountArticlesBetween(ctx, monthStart, now)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountArticlesTotal(ctx)
	if err != nil {
		return nil, err
	}
	completed, err := s.repo.CountArticlesByStatus(ctx, "COMPLETED")
	if err != nil {
		return nil, err
	}
	avgMs, err := s.repo.AvgArticleDurationMs(ctx)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.CountActiveAuthorsSince(ctx, weekStart)
	if err != nil {
		return nil, err
	}
	usersTotal, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	vipCount, err := s.repo.CountUsersByRole(ctx, moduser.RoleVIP)
	if err != nil {
		return nil, err
	}
	remaining, normalCount, err := s.repo.SumQuotaByRole(ctx, moduser.RoleUser)
	if err != nil {
		return nil, err
	}

	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}
	quotaUsed := normalCount*module.DefaultUserQuota - remaining
	if quotaUsed < 0 {
		quotaUsed = 0
	}

	out := &module.Overview{
		TodayCount:      today,
		WeekCount:       week,
		MonthCount:      month,
		TotalCount:      total,
		SuccessRate:     successRate,
		AvgDurationMs:   avgMs,
		ActiveUserCount: active,
		TotalUserCount:  usersTotal,
		VipUserCount:    vipCount,
		QuotaUsed:       quotaUsed,
	}

	s.log.Info("statistics overview computed",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "statistics.overview.computed",
		"actor_id", actorID,
		"total_articles", total,
	)
	return out, nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	sod := startOfDay(t)
	wd := int(sod.Weekday())
	if wd == 0 {
		wd = 7
	}
	return sod.AddDate(0, 0, -(wd - 1))
}
