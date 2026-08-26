package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Article 文章实体。
// 字段对齐 ai-passage-creator GORM model.Article / SQL article 表，命名按本项目 Ent 惯例使用 snake_case。
type Article struct {
	ent.Schema
}

func (Article) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "articles"},
	}
}

// Fields of the Article.
func (Article) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("主键"),
		field.String("task_id").
			NotEmpty().
			Unique().
			Comment("任务 ID，唯一"),
		field.Int64("user_id").
			Comment("所属用户 ID"),
		field.String("topic").
			NotEmpty().
			Comment("文章主题"),
		field.Text("user_description").
			Optional().
			Nillable().
			Comment("用户补充描述"),
		field.String("main_title").
			Optional().
			Nillable().
			Comment("主标题"),
		field.String("sub_title").
			Optional().
			Nillable().
			Comment("副标题"),
		field.JSON("title_options", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}).
			Comment("标题方案列表（JSON）"),
		field.JSON("outline", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}).
			Comment("大纲（JSON）"),
		field.Text("content").
			Optional().
			Nillable().
			Comment("正文内容"),
		field.Text("full_content").
			Optional().
			Nillable().
			Comment("完整内容（含配图等）"),
		field.JSON("images", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}).
			Comment("配图结果列表（JSON）"),
		field.Enum("status").
			Values("PENDING", "PROCESSING", "COMPLETED", "FAILED").
			Default("PENDING").
			Comment("文章状态：PENDING/PROCESSING/COMPLETED/FAILED"),
		field.Enum("phase").
			Values(
				"PENDING",
				"TITLE_GENERATING",
				"TITLE_SELECTING",
				"OUTLINE_GENERATING",
				"OUTLINE_EDITING",
				"CONTENT_GENERATING",
				"COMPLETED",
			).
			Default("PENDING").
			Comment("当前阶段：PENDING/TITLE_GENERATING/TITLE_SELECTING/OUTLINE_GENERATING/OUTLINE_EDITING/CONTENT_GENERATING/COMPLETED"),
		field.Text("error_message").
			Optional().
			Nillable().
			Comment("失败错误信息"),
		field.Enum("style").
			Values("tech", "emotional", "educational", "humorous").
			Optional().
			Comment("文章风格：tech/emotional/educational/humorous"),
		field.JSON("enabled_image_methods", []string{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}).
			Comment("允许的配图方式列表（JSON）"),
		field.Time("create_time").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
		field.Time("completed_time").
			Optional().
			Nillable().
			Comment("完成时间"),
		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
		field.Bool("is_delete").
			Default(false).
			Comment("是否删除（软删除）"),
	}
}

// Edges of the Article.
// - 多对一：文章属于一个用户（FK user_id）
// - 一对多：一篇文章有多条智能体执行日志
func (Article) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("articles").
			Field("user_id").
			Unique().
			Required(),
		edge.To("agent_logs", AgentLog.Type),
	}
}

func (Article) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("create_time"),
		index.Fields("is_delete"),
	}
}
