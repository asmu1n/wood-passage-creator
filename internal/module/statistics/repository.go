package statistics

import (
	"context"
	"time"

	"wood-passage-creator/internal/module/user"
)

type Repository interface {
	CountArticlesBetween(ctx context.Context, from, to time.Time) (int64, error)
	CountArticlesTotal(ctx context.Context) (int64, error)
	CountArticlesByStatus(ctx context.Context, status string) (int64, error)
	AvgArticleDurationMs(ctx context.Context) (int, error)
	CountActiveAuthorsSince(ctx context.Context, since time.Time) (int64, error)
	CountUsers(ctx context.Context) (int64, error)
	CountUsersByRole(ctx context.Context, role user.UserRole) (int64, error)
	SumQuotaByRole(ctx context.Context, role user.UserRole) (remaining int64, userCount int64, err error)
}
