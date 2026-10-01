// Package entschema owns extension-only Ent schema declarations.
package entschema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
)

type Routing struct{ mixin.Schema }

func (Routing) Fields() []ent.Field {
	return []ent.Field{
		field.String("custom_routing_policy").MaxLen(32).
			Default(multigroupbilling.LegacyRouting).
			Immutable().
			Comment("Extension routing ownership; existing keys keep multigroup-v1 across toggle changes"),
	}
}
