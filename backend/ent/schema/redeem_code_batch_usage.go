package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RedeemCodeBatchUsage records one successful marketing-code redemption per user and batch.
type RedeemCodeBatchUsage struct {
	ent.Schema
}

func (RedeemCodeBatchUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_code_batch_usages"},
	}
}

func (RedeemCodeBatchUsage) Fields() []ent.Field {
	return []ent.Field{
		field.String("batch_id").
			MaxLen(64).
			NotEmpty(),
		field.Int64("user_id"),
		field.Int64("redeem_code_id"),
		field.Time("used_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RedeemCodeBatchUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("batch_id"),
		index.Fields("user_id"),
		index.Fields("batch_id", "user_id").Unique(),
	}
}
