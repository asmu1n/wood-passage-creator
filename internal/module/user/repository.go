package user

import (
	"context"
	"errors"
	"wood-passage-creator/internal/pkg/page"
)

// ErrAccountConflict 账号唯一约束冲突（并发注册等）。
var ErrAccountConflict = errors.New("account conflict")

// Repository 用户持久化端口。
type Repository interface {
	Create(ctx context.Context, in CreateRepoParams) (*User, error)
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByAccount(ctx context.Context, account string) (*UserWithSecret, error)
	QueryList(ctx context.Context, params page.PageRequest) ([]*User, int, error)
	Update(ctx context.Context, id int64, in UpdateRepoParams) (*User, error)
	Delete(ctx context.Context, id int64) error
	ExistsAccount(ctx context.Context, account string) (bool, error)
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
}

// UserWithSecret 含密码哈希，仅限 Service 校验登录使用，禁止直接作为 API 响应。
type UserWithSecret struct {
	User
	UserPassword string
}
