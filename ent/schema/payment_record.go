package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PaymentRecord 支付记录实体。
// 字段对齐 ai-passage-creator GORM model.PaymentRecord / SQL payment_record 表。
type PaymentRecord struct {
	ent.Schema
}

func (PaymentRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "payment_records"},
	}
}

func (PaymentRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("主键"),
		field.Int64("user_id").
			Positive().
			Comment("所属用户 ID（FK → users.id）"),
		field.String("stripe_session_id").
			MaxLen(128).
			NotEmpty().
			Comment("Stripe Checkout Session ID"),
		field.String("stripe_payment_intent_id").
			MaxLen(128).
			Optional().
			Nillable().
			Comment("Stripe PaymentIntent ID"),
		field.Float("amount").
			SchemaType(map[string]string{
				dialect.Postgres: "numeric(10,2)",
			}).
			Comment("支付金额"),
		field.String("currency").
			MaxLen(8).
			Default("usd").
			Comment("币种"),
		field.Enum("status").
			Values("PENDING", "SUCCEEDED", "FAILED", "REFUNDED").
			Default("PENDING").
			Comment("支付状态：PENDING/SUCCEEDED/FAILED/REFUNDED"),
		field.String("product_type").
			MaxLen(32).
			NotEmpty().
			Comment("产品类型，如 VIP_PERMANENT"),
		field.String("description").
			MaxLen(256).
			Optional().
			Nillable().
			Comment("描述"),
		field.Time("refund_time").
			Optional().
			Nillable().
			Comment("退款时间"),
		field.String("refund_reason").
			MaxLen(512).
			Optional().
			Nillable().
			Comment("退款原因"),
		field.Time("create_time").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

// Edges of the PaymentRecord.
// 多对一：多条支付记录属于一个用户。
func (PaymentRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("payment_records").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (PaymentRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("stripe_session_id"),
		index.Fields("status"),
		index.Fields("product_type"),
		index.Fields("create_time"),
	}
}
