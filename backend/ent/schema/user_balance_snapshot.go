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

type UserBalanceSnapshot struct {
	ent.Schema
}

func (UserBalanceSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_balance_snapshots"}}
}

func (UserBalanceSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("period").MaxLen(7).Validate(func(value string) error {
			_, err := time.Parse("2006-01", value)
			return err
		}),
		field.Float("balance").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Time("created_at").Default(time.Now),
	}
}

func (UserBalanceSnapshot) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "period").Unique()}
}
