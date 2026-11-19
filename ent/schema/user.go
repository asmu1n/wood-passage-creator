package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User 用户实体。
// 字段对齐 ai-passage-creator GORM model.User / SQL user 表，命名按本项目 Ent 惯例使用 snake_case。
type User struct {
	ent.Schema
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		// Postgres 中 user 为保留字，显式使用 users。
		entsql.Annotation{Table: "users"},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("主键"),
		field.String("user_account").
			MaxLen(256).
			NotEmpty().
			Unique().
			Comment("登录账号，唯一"),
		field.String("user_password").
			MaxLen(512).
			NotEmpty().
			Sensitive().
			Comment("密码哈希，非明文"),
		field.String("user_name").
			MaxLen(256).
			Optional().
			Nillable().
			Comment("用户昵称"),
		field.String("user_avatar").
			MaxLen(1024).
			Optional().
			Nillable().
			Comment("用户头像 URL"),
		field.String("user_profile").
			MaxLen(512).
			Optional().
			Nillable().
			Comment("用户简介"),
		field.Enum("user_role").
			Values("user", "admin", "vip").
			Default("user").
			Comment("用户角色：user/admin/vip"),
		field.Int("quota").
			NonNegative().
			Default(5).
			Comment("剩余配额"),
		field.Time("vip_time").
			Optional().
			Nillable().
			Comment("成为会员时间"),
		field.Time("edit_time").
			Default(time.Now).
			Comment("编辑时间"),
		field.Time("create_time").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
		field.Bool("is_delete").
			Default(false).
			Comment("是否删除（软删除）"),
	}
}

// Edges of the User.
// 一对多：用户拥有多篇文章、多条支付记录。
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("articles", Article.Type),
		edge.To("payment_records", PaymentRecord.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_name"),
		index.Fields("is_delete"),
	}
}
