package entschema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
)

type Issuance struct{ mixin.Schema }

func (Issuance) Fields() []ent.Field {
	return []ent.Field{field.String("custom_subscription_policy").MaxLen(32).Default(subscriptionextensions.Independent).Immutable().Comment("Durable subscription issuance ownership; never derived from live toggle during fulfillment")}
}
