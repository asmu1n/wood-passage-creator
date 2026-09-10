package userapp

import (
	moduser "wood-passage-creator/internal/module/user"
)

// ---------- Swagger 响应辅助（仅文档；运行时仍用 page.PageResponse）----------

// UserListData 用户分页 data 形态，供 swag 展示。
type UserListData struct {
	Records  []*moduser.User `json:"records"`
	Total    int             `json:"total"`
	PageSize int             `json:"pageSize"`
	PageNum  int             `json:"pageNum"`
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

// AdminAddRequest 管理端创建用户（对齐旧 POST /user/add）。
// 默认密码 12345678；角色仅允许 user / admin（不含 vip，升 VIP 走 upgrade-vip）。
type AdminAddRequest struct {
	UserAccount string  `json:"userAccount" validate:"required,min=3,max=20,regexp=^[a-zA-Z][a-zA-Z0-9_]*$"`
	UserName    *string `json:"userName" validate:"omitempty,min=1,max=256"`
	UserAvatar  *string `json:"userAvatar" validate:"omitempty,url,max=1024"`
	UserProfile *string `json:"userProfile" validate:"omitempty,max=512"`
	UserRole    string  `json:"userRole" validate:"omitempty,oneof=user admin"`
}
