package user

import "time"

// UserRole 与 ent schema / API 对齐。
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
	RoleVIP   UserRole = "vip"
)

// IsValid 判断角色是否有效。
func (r UserRole) IsValid() bool {
	return r == RoleUser || r == RoleAdmin || r == RoleVIP
}

// IsVipOrAdmin VIP 或管理员（高级配图、AI 改大纲等）。
func (r UserRole) IsVipOrAdmin() bool {
	return r == RoleVIP || r == RoleAdmin
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

// ---------- Swagger 响应辅助类型（仅文档；运行时仍用 page.PageResponse）----------

// UserListData 用户分页 data 形态，供 swag 展示（泛型 PageResponse 无法直接被 swag 解析）。
type UserListData struct {
	Records  []*User `json:"records"`
	Total    int     `json:"total"`
	PageSize int     `json:"pageSize"`
	PageNum  int     `json:"pageNum"`
}

// ---------- API 入参（Handler 直接 BindAndValidate 到这些类型）----------
//
// 说明：go-playground 的 tag 以英文逗号分段，regexp 模式里不能写 {2,19} 这类逗号。
// 长度用 min/max；字符形态用 regexp（或 hasalpha/hasdigit）。

// RegisterRequest 注册入参。
type RegisterRequest struct {
	// 字母开头，后仅字母数字下划线；长度 3–20
	UserAccount  string  `json:"userAccount" validate:"required,min=3,max=20,regexp=^[a-zA-Z][a-zA-Z0-9_]*$"`
	UserPassword string  `json:"userPassword" validate:"required,min=6,max=20,hasalpha,hasdigit"`
	UserName     *string `json:"userName" validate:"omitempty,min=1,max=256"`
	UserAvatar   *string `json:"userAvatar" validate:"omitempty,url,max=1024"`
	UserProfile  *string `json:"userProfile" validate:"omitempty,max=512"`
}

// LoginRequest 登录入参（密码不做复杂度校验，避免策略变更导致无法登录）。
type LoginRequest struct {
	UserAccount  string `json:"userAccount" validate:"required,min=3,max=20,regexp=^[a-zA-Z][a-zA-Z0-9_]*$"`
	UserPassword string `json:"userPassword" validate:"required,min=6,max=20"`
}

// UpdateRequest 部分更新；指针 nil 表示不修改该字段。
type UpdateRequest struct {
	UserPassword *string `json:"userPassword" validate:"omitempty,min=6,max=20,hasalpha,hasdigit"`
	UserName     *string `json:"userName" validate:"omitempty,min=1,max=256"`
	UserAvatar   *string `json:"userAvatar" validate:"omitempty,url,max=1024"`
	UserProfile  *string `json:"userProfile" validate:"omitempty,max=512"`
}

// HasUpdates 是否至少带了一个可更新字段。
func (in UpdateRequest) HasUpdates() bool {
	return in.UserPassword != nil || in.UserName != nil ||
		in.UserAvatar != nil || in.UserProfile != nil
}
