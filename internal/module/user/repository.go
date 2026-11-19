package user

import (
	"context"
	"errors"
	"time"

	"wood-passage-creator/internal/pkg/page"
)

// ErrAccountConflict 账号唯一约束冲突（并发注册等）。
var ErrAccountConflict = errors.New("account conflict")

// Repository 用户持久化端口。
type Repository interface {
	Create(ctx context.Context, in CreateRepoParams) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByAccount(ctx context.Context, account string) (*UserWithSecret, error)
	List(ctx context.Context, params page.PageRequest) ([]*User, int, error)
	Update(ctx context.Context, id int64, in UpdateRepoParams) (*User, error)
	Delete(ctx context.Context, id int64) error
	ExistsAccount(ctx context.Context, account string) (bool, error)
	// DecrementQuota 原子：quota = quota - 1 WHERE id=? AND quota > 0 AND 未删除。
	// 返回 affected rows（0=不足或用户不存在）。
	DecrementQuota(ctx context.Context, id int64) (int, error)

	// IncrementQuota 原子：quota = quota + 1 WHERE id=? AND 未删除。
	// 回滚用；一般 affected 应为 1。
	IncrementQuota(ctx context.Context, id int64) (int, error)
}

// CreateRepoParams 仓储创建参数（已哈希密码）。
type CreateRepoParams struct {
	UserAccount  string
	UserPassword string
	UserName     *string
	UserAvatar   *string
	UserProfile  *string
	UserRole     UserRole
	Quota        int
}

// UpdateRepoParams 仓储更新参数。
type UpdateRepoParams struct {
	UserPassword *string
	UserName     *string
	UserAvatar   *string
	UserProfile  *string
	UserRole     *UserRole
	Quota        *int
	VipTime      *time.Time
	ClearVipTime bool // true 时清空 vip_time（降级）
}

// UserWithSecret 含密码哈希，仅限 Service 校验登录使用，禁止直接作为 API 响应。
type UserWithSecret struct {
	User
	UserPassword string
}
