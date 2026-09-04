package auth

// RegisterRequest 注册入参。
type RegisterRequest struct {
	UserAccount  string  `json:"userAccount" validate:"required,min=3,max=20,regexp=^[a-zA-Z][a-zA-Z0-9_]*$"`
	UserPassword string  `json:"userPassword" validate:"required,min=6,max=20,hasalpha,hasdigit"`
	UserName     *string `json:"userName" validate:"omitempty,min=1,max=256"`
	UserAvatar   *string `json:"userAvatar" validate:"omitempty,url,max=1024"`
	UserProfile  *string `json:"userProfile" validate:"omitempty,max=512"`
}

// LoginRequest 登录入参。
type LoginRequest struct {
	UserAccount  string `json:"userAccount" validate:"required,min=3,max=20,regexp=^[a-zA-Z][a-zA-Z0-9_]*$"`
	UserPassword string `json:"userPassword" validate:"required,min=6,max=20"`
}
