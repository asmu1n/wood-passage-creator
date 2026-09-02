package repo

import (
	"context"
	"fmt"
	"time"

	"wood-passage-creator/ent"
	entarticle "wood-passage-creator/ent/article"
	entuser "wood-passage-creator/ent/user"
	"wood-passage-creator/internal/module/statistics"
	"wood-passage-creator/internal/module/user"
)

// StatisticsRepo 只读聚合，实现 statistics.Repository。
type StatisticsRepo struct {
	client *ent.Client
}

func New(client *ent.Client) statistics.Repository {
	return &StatisticsRepo{client: client}
}

func (r *StatisticsRepo) CountArticlesBetween(ctx context.Context, from, to time.Time) (int64, error) {
	n, err := r.client.Article.Query().
		Where(
			entarticle.IsDeleteEQ(false),
			entarticle.CreateTimeGTE(from),
			entarticle.CreateTimeLTE(to),
		).
		Count(ctx)
	return int64(n), err
}

func (r *StatisticsRepo) CountArticlesTotal(ctx context.Context) (int64, error) {
	n, err := r.client.Article.Query().
		Where(entarticle.IsDeleteEQ(false)).
		Count(ctx)
	return int64(n), err
}

func (r *StatisticsRepo) CountArticlesByStatus(ctx context.Context, status string) (int64, error) {
	st := entarticle.Status(status)
	if err := entarticle.StatusValidator(st); err != nil {
		return 0, fmt.Errorf("invalid article status %q: %w", status, err)
	}
	n, err := r.client.Article.Query().
		Where(
			entarticle.IsDeleteEQ(false),
			entarticle.StatusEQ(st),
		).
		Count(ctx)
	return int64(n), err
}

func (r *StatisticsRepo) AvgArticleDurationMs(ctx context.Context) (int, error) {
	rows, err := r.client.Article.Query().
		Where(
			entarticle.IsDeleteEQ(false),
			entarticle.StatusEQ(entarticle.StatusCOMPLETED),
			entarticle.CompletedTimeNotNil(),
		).
		Select(entarticle.FieldCreateTime, entarticle.FieldCompletedTime).
		All(ctx)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	var total int64
	var valid int
	for _, row := range rows {
		if row.CompletedTime == nil {
			continue
		}
		ms := row.CompletedTime.Sub(row.CreateTime).Milliseconds()
		if ms < 0 {
			continue
		}
		total += ms
		valid++
	}
	if valid == 0 {
		return 0, nil
	}
	return int(total / int64(valid)), nil
}

func (r *StatisticsRepo) CountActiveAuthorsSince(ctx context.Context, since time.Time) (int64, error) {
	var groups []struct {
		UserID int64 `json:"user_id"`
		Count  int   `json:"count"`
	}
	err := r.client.Article.Query().
		Where(
			entarticle.IsDeleteEQ(false),
			entarticle.CreateTimeGTE(since),
		).
		GroupBy(entarticle.FieldUserID).
		Aggregate(ent.Count()).
		Scan(ctx, &groups)
	if err != nil {
		return 0, err
	}
	return int64(len(groups)), nil
}

func (r *StatisticsRepo) CountUsers(ctx context.Context) (int64, error) {
	n, err := r.client.User.Query().
		Where(entuser.IsDeleteEQ(false)).
		Count(ctx)
	return int64(n), err
}

func (r *StatisticsRepo) CountUsersByRole(ctx context.Context, role user.UserRole) (int64, error) {
	ur := entuser.UserRole(role)
	if err := entuser.UserRoleValidator(ur); err != nil {
		return 0, fmt.Errorf("invalid user role %q: %w", role, err)
	}
	n, err := r.client.User.Query().
		Where(
			entuser.IsDeleteEQ(false),
			entuser.UserRoleEQ(ur),
		).
		Count(ctx)
	return int64(n), err
}

func (r *StatisticsRepo) SumQuotaByRole(ctx context.Context, role user.UserRole) (remaining int64, userCount int64, err error) {
	ur := entuser.UserRole(role)
	if err := entuser.UserRoleValidator(ur); err != nil {
		return 0, 0, fmt.Errorf("invalid user role %q: %w", role, err)
	}
	rows, err := r.client.User.Query().
		Where(
			entuser.IsDeleteEQ(false),
			entuser.UserRoleEQ(ur),
		).
		Select(entuser.FieldQuota).
		All(ctx)
	if err != nil {
		return 0, 0, err
	}
	userCount = int64(len(rows))
	for _, row := range rows {
		remaining += int64(row.Quota)
	}
	return remaining, userCount, nil
}
