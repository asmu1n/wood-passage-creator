package statistics

import (
	"context"
	"log/slog"
	"time"

	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
)

// Service 管理端统计概览。
type Service struct {
	repo Repository
	log  *slog.Logger
	loc  *time.Location
}

func NewService(repo Repository) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return &Service{
		repo: repo,
		log:  logger.Module("statistics"),
		loc:  loc,
	}
}

// GetOverview 聚合系统概览；需管理员。
func (s *Service) GetOverview(ctx context.Context, actor user.Actor) (*Overview, error) {
	if err := actor.RequireAdmin(); err != nil {
		return nil, err
	}

	now := time.Now().In(s.loc)
	todayStart := startOfDay(now)
	weekStart := startOfWeek(now) // 周一 00:00
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
	vipCount, err := s.repo.CountUsersByRole(ctx, user.RoleVIP)
	if err != nil {
		return nil, err
	}
	remaining, normalCount, err := s.repo.SumQuotaByRole(ctx, user.RoleUser)
	if err != nil {
		return nil, err
	}

	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}
	quotaUsed := normalCount*DefaultUserQuota - remaining
	if quotaUsed < 0 {
		quotaUsed = 0
	}

	out := &Overview{
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
		logger.FieldEvent, "statistics.overview",
		"actor_id", actor.ID,
		"total_articles", total,
	)
	return out, nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// startOfWeek 返回本周一 00:00（本地 loc）。
func startOfWeek(t time.Time) time.Time {
	sod := startOfDay(t)
	// Go: Sunday=0 ... Saturday=6；转到距离周一的天数
	wd := int(sod.Weekday())
	if wd == 0 {
		wd = 7
	}
	return sod.AddDate(0, 0, -(wd - 1))
}
