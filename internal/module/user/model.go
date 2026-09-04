package user

import (
	"time"
	"wood-passage-creator/internal/pkg/response"
)

// UserRole 与 ent schema / API 对齐（纯枚举，不含权限方法；鉴权统一走 Actor）。
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
	RoleVIP   UserRole = "vip"
)

// ---------- Actor：用例执行者；仅保留 Require* 做权限断言 ----------

// Actor 由 HTTP 中间件加载后传入 Service，禁止在 Service 内解析 session。
type Actor struct {
	ID   int64
	Role UserRole
}

// RequireAdmin 非管理员 → 403。
func (a Actor) RequireAdmin() error {
	if a.Role != RoleAdmin {
		return response.NewBizErrorWithDetail(response.Forbidden, "需要管理员权限")
	}
	return nil
}

// RequireVipOrAdmin 非 VIP/管理员 → 403（高级配图、AI 改大纲等）。
func (a Actor) RequireVipOrAdmin() error {
	if a.Role != RoleVIP && a.Role != RoleAdmin {
		return response.NewBizErrorWithDetail(response.Forbidden, "需要 VIP 或管理员权限")
	}
	return nil
}

// RequireSelfOrAdmin 非本人且非管理员 → 403。
func (a Actor) RequireSelfOrAdmin(targetID int64) error {
	if a.Role == RoleAdmin || (a.ID != 0 && a.ID == targetID) {
		return nil
	}
	return response.NewBizErrorWithDetail(response.Forbidden, "无权限")
}

// User 对外可见的用户信息（过滤 user_password 等敏感字段）。
type User struct {
	ID          int64      `json:"id"`
	UserAccount string     `json:"userAccount"`
	UserName    *string    `json:"userName,omitempty"`
	UserAvatar  *string    `json:"userAvatar,omitempty"`
	UserProfile *string    `json:"userProfile,omitempty"`
	UserRole    UserRole   `json:"userRole"`
	Quota       int        `json:"quota"`
	VipTime     *time.Time `json:"vipTime,omitempty"`
	EditTime    time.Time  `json:"editTime"`
	CreateTime  time.Time  `json:"createTime"`
	UpdateTime  time.Time  `json:"updateTime"`
}

// Actor 从已加载用户构造执行者（中间件 GetLoginUser 后可用）。
func (u *User) Actor() Actor {
	if u == nil {
		return Actor{}
	}
	return Actor{ID: u.ID, Role: u.UserRole}
}
