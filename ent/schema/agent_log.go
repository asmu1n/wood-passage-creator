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

// AgentLog 智能体执行日志。
// 字段对齐 ai-passage-creator GORM model.AgentLog；另增 article_id 外键以便 Ent edge
//（原表仅有 task_id 业务键，无法直接 edge 到 Article 非主键字段）。
type AgentLog struct {
	ent.Schema
}

func (AgentLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "agent_logs"},
	}
}

func (AgentLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("主键"),
		field.Int64("article_id").
			Positive().
			Comment("所属文章 ID（FK → articles.id）"),
		field.String("task_id").
			MaxLen(64).
			NotEmpty().
			Comment("任务 ID（与 articles.task_id 对齐的业务键，便于按任务检索）"),
		field.String("agent_name").
			MaxLen(64).
			NotEmpty().
			Comment("智能体名称"),
		field.Time("start_time").
			Comment("开始时间"),
		field.Time("end_time").
			Optional().
			Nillable().
			Comment("结束时间"),
		field.Int("duration_ms").
			Optional().
			Nillable().
			NonNegative().
			Comment("耗时（毫秒）"),
		field.Enum("status").
			Values("RUNNING", "SUCCESS", "FAILED").
			Comment("状态：RUNNING/SUCCESS/FAILED"),
		field.Text("error_message").
			Optional().
			Nillable().
			Comment("错误信息"),
		field.Text("prompt").
			Optional().
			Nillable().
			Comment("提示词"),
		field.Text("input_data").
			Optional().
			Nillable().
			Comment("输入数据"),
		field.Text("output_data").
			Optional().
			Nillable().
			Comment("输出数据"),
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

// Edges of the AgentLog.
// 多对一：多条日志属于一篇文章。
func (AgentLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("article", Article.Type).
			Ref("agent_logs").
			Field("article_id").
			Unique().
			Required(),
	}
}

func (AgentLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("article_id"),
		index.Fields("task_id"),
		index.Fields("status"),
		index.Fields("create_time"),
		index.Fields("is_delete"),
	}
}
