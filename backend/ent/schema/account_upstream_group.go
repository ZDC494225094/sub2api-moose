package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountUpstreamGroup is the persistent directory for administrator-defined
// upstream providers. Accounts retain a display-name cache for compatibility,
// while this entity owns identity and order.
type AccountUpstreamGroup struct {
	ent.Schema
}

func (AccountUpstreamGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_upstream_groups"},
	}
}

func (AccountUpstreamGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (AccountUpstreamGroup) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("normalized_name").MaxLen(100).NotEmpty().Unique(),
		field.Int64("sort_order").Default(0),
	}
}

func (AccountUpstreamGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("accounts", Account.Type),
	}
}

func (AccountUpstreamGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sort_order"),
	}
}
