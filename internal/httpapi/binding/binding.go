package binding

import (
	"reflect"
	"regexp"
	"unicode"

	"wood-passage-creator/internal/port"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

// Validator 将 go-playground/validator 接到 Echo#Validator。
//
// 通用 tag：hasalpha / hasdigit / hasspecial / image_method
// 业务字段规则写在各 module model 的 validate tag 上。
type Validator struct {
	v *validator.Validate
}

// NewValidator 创建校验器并挂上通用扩展 tag。
func NewValidator() *Validator {
	v := validator.New()

	_ = v.RegisterValidation("hasalpha", validateHasAlpha)
	_ = v.RegisterValidation("hasdigit", validateHasDigit)
	_ = v.RegisterValidation("hasspecial", validateHasSpecial)
	_ = v.RegisterValidation("regexp", validateRegexp)
	_ = v.RegisterValidation("image_method", validateImageMethod)

	// 让 oneof / dive 等对 port.ImageMethod 按底层 string 取值
	v.RegisterCustomTypeFunc(func(field reflect.Value) any {
		if !field.IsValid() {
			return nil
		}
		if m, ok := field.Interface().(port.ImageMethod); ok {
			return string(m)
		}
		if field.CanInterface() {
			if m, ok := field.Interface().(*port.ImageMethod); ok && m != nil {
				return string(*m)
			}
		}
		return nil
	}, port.ImageMethod(""), (*port.ImageMethod)(nil))

	return &Validator{v: v}
}

func (cv *Validator) Validate(i any) error {
	return cv.v.Struct(i)
}

// BindAndValidate 先 Bind（含 ImageMethod.UnmarshalJSON 规范化）再 Validate。
func BindAndValidate(c *echo.Context, dst any) error {
	if err := c.Bind(dst); err != nil {
		return err
	}
	return c.Validate(dst)
}

// validateImageMethod：用户可选配图枚举白名单（已在 Unmarshal 后规范化）。
func validateImageMethod(fl validator.FieldLevel) bool {
	switch v := fl.Field().Interface().(type) {
	case port.ImageMethod:
		return v.IsUserMethod()
	case string:
		return port.ImageMethod(v).IsUserMethod()
	default:
		return false
	}
}

func validateRegexp(fl validator.FieldLevel) bool {
	pat := fl.Param()
	if pat == "" {
		return false
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return false
	}
	return re.MatchString(fl.Field().String())
}

func validateHasAlpha(fl validator.FieldLevel) bool {
	for _, r := range fl.Field().String() {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func validateHasDigit(fl validator.FieldLevel) bool {
	for _, r := range fl.Field().String() {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func validateHasSpecial(fl validator.FieldLevel) bool {
	for _, r := range fl.Field().String() {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return true
		}
	}
	return false
}
