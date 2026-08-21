package middleware

import (
	"context"

	"projecttemp/internal/module/user"
	"projecttemp/internal/pkg/response"

	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
)

// SessionName 与 cookie 名一致，登录/鉴权共用。
const SessionName = "session"

// SessionKeyUserID session / context 中存放登录用户 ID 的键。
const SessionKeyUserID = "userID"

// ContextKeyLoginUser 鉴权通过后写入 context 的当前用户（*user.User）。
const ContextKeyLoginUser = "loginUser"

// ContextKeyLoginUserRole 鉴权通过后写入 context 的当前用户角色。
const ContextKeyLoginUserRole = "loginUserRole"

// UserLoader 按 ID 加载用户，供需要角色/资料的鉴权中间件使用。
type UserLoader interface {
	GetByID(ctx context.Context, id int64) (*user.User, error)
}

// AuthRequired 要求请求已登录；未登录时 return BizError，由全局 HTTPErrorHandler 写 401 JSON。
func AuthRequired() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if _, err := requireLogin(c); err != nil {
				return err
			}
			return next(c)
		}
	}
}

// AuthWithRoleRequired 要求已登录,并且会查询注入权限
// loader 用于读取最新角色（避免仅信 session 导致提权/降权滞后）。
func AuthWithRoleRequired(loader UserLoader, onlyAdmin bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			uid, err := requireLogin(c)
			if err != nil {
				return err
			}
			if loader == nil {
				return response.NewBizError(response.SystemError)
			}

			u, err := loader.GetByID(c.Request().Context(), uid)
			if err != nil {
				return err
			}
			if u == nil {
				// session 仍在但用户已删/不可见
				return response.NewBizError(response.NotLogin)
			}
			if onlyAdmin && u.UserRole != user.RoleAdmin {
				return response.NewBizError(response.NoAuth)
			}

			c.Set(ContextKeyLoginUser, u)
			c.Set(ContextKeyLoginUserRole, u.UserRole)
			return next(c)
		}
	}
}

// GetLoginUserID 从请求上下文读取当前登录用户 ID（应在 AuthRequired / AuthWithRoleRequired 之后调用）。
func GetLoginUserID(c *echo.Context) (int64, error) {
	uid, ok := asInt64(c.Get(SessionKeyUserID))
	if !ok || uid == 0 {
		return 0, response.NewBizError(response.NotLogin)
	}
	return uid, nil
}

// GetLoginUserRole 从请求上下文读取当前登录用户角色（应在 AuthWithRoleRequired 之后调用）。
func GetLoginUserRole(c *echo.Context) (user.UserRole, error) {
	role, ok := c.Get(ContextKeyLoginUserRole).(user.UserRole)
	if !ok || role == "" {
		return user.RoleUser, response.NewBizError(response.NotLogin)
	}
	return role, nil
}

// GetLoginUser 读取 AdminRequired（或其它已写入的中间件）放入的当前用户。
// 仅登录、未加载用户资料时返回 NotLogin。
func GetLoginUser(c *echo.Context) (*user.User, error) {
	u, ok := c.Get(ContextKeyLoginUser).(*user.User)
	if !ok || u == nil {
		return nil, response.NewBizError(response.NotLogin)
	}
	return u, nil
}

// SaveLoginUserID 写入登录用户 ID 并持久化 session（供登录 Handler 调用）。
func SaveLoginUserID(c *echo.Context, userID int64) error {
	if userID == 0 {
		return response.NewBizError(response.ParamsError)
	}
	sess, err := session.Get(SessionName, c)
	if err != nil {
		return err
	}
	sess.Values[SessionKeyUserID] = userID
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return err
	}
	c.Set(SessionKeyUserID, userID)
	return nil
}

// ClearLoginSession 清除登录态（供登出 Handler 调用）。
func ClearLoginSession(c *echo.Context) error {
	sess, err := session.Get(SessionName, c)
	if err != nil {
		return err
	}
	delete(sess.Values, SessionKeyUserID)
	// MaxAge < 0：指示 store 删除 session 并让浏览器丢弃 cookie
	sess.Options.MaxAge = -1
	return sess.Save(c.Request(), c.Response())
}

// requireLogin 校验 session 并写入 userID 到 context，返回用户 ID。
func requireLogin(c *echo.Context) (int64, error) {
	sess, err := session.Get(SessionName, c)
	if err != nil {
		return 0, response.NewBizError(response.NotLogin)
	}
	uid, ok := asInt64(sess.Values[SessionKeyUserID])
	if !ok || uid == 0 {
		return 0, response.NewBizError(response.NotLogin)
	}
	c.Set(SessionKeyUserID, uid)
	return uid, nil
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint64:
		if n > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}
