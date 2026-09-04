package user

import "wood-passage-creator/internal/module/user"

// ---------- Swagger 响应辅助（仅文档；运行时仍用 page.PageResponse）----------

// UserListData 用户分页 data 形态，供 swag 展示。
type UserListData struct {
	Records  []*user.User `json:"records"`
	Total    int          `json:"total"`
	PageSize int          `json:"pageSize"`
	PageNum  int          `json:"pageNum"`
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
