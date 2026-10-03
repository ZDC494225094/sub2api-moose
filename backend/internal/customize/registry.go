// Package customize hosts built-in business extension switches. It is deliberately
// separate from upstream's executable OAuth transport plugin system.
package customize

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingPrefix = "custom_extensions."
const RechargeCampaigns = "recharge-campaigns"

//go:embed catalog.json
var catalogJSON []byte

type Manifest struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Managed         bool     `json:"managed"`
	DisableBehavior string   `json:"disable_behavior"`
	Paths           []string `json:"paths"`
	Slots           []string `json:"slots,omitempty"`
	// Gates name backend admission points for switches without a page or slot.
	Gates []string `json:"gates,omitempty"`
}

type State struct {
	Manifest
	// nil means not migrated, not disabled. Pending modules never get fake switches.
	Enabled *bool `json:"enabled"`
}

type Store interface {
	GetMultiple(context.Context, []string) (map[string]string, error)
	Set(context.Context, string, string) error
}

type Manager struct{ store Store }

func NewManager(store Store) *Manager { return &Manager{store: store} }

var parsedCatalog = func() []Manifest {
	var items []Manifest
	if err := json.Unmarshal(catalogJSON, &items); err != nil {
		panic(err)
	}
	return items
}()

func Catalog() []Manifest {
	items := make([]Manifest, len(parsedCatalog))
	for i, item := range parsedCatalog {
		items[i] = item
		items[i].Paths = append([]string(nil), item.Paths...)
		items[i].Slots = append([]string(nil), item.Slots...)
		items[i].Gates = append([]string(nil), item.Gates...)
	}
	return items
}

func Lookup(id string) (Manifest, bool) {
	for _, item := range Catalog() {
		if item.ID == id {
			return item, true
		}
	}
	return Manifest{}, false
}

func Key(id string) string { return SettingPrefix + id + ".enabled" }

// Snapshot intentionally reads the shared store: switches must not remain enabled
// in another server process after an administrator disables them. Missing keys
// are disabled; startup migrations explicitly adopt existing installations once.
func (m *Manager) Snapshot(ctx context.Context) (map[string]bool, error) {
	keys := []string{}
	for _, item := range Catalog() {
		if item.Managed {
			keys = append(keys, Key(item.ID))
		}
	}
	if m == nil || m.store == nil {
		return nil, unavailable()
	}
	values, err := m.store.GetMultiple(ctx, keys)
	if err != nil {
		return nil, unavailable()
	}
	result := make(map[string]bool, len(keys))
	for _, item := range Catalog() {
		if !item.Managed {
			continue
		}
		value, exists := values[Key(item.ID)]
		result[item.ID] = exists && value == "true"
	}
	return result, nil
}

func (m *Manager) Enabled(ctx context.Context, id string) (bool, error) {
	item, ok := Lookup(id)
	if !ok || !item.Managed {
		return false, infraerrors.BadRequest("CUSTOM_EXTENSION_NOT_MANAGED", "extension is unknown or has not been isolated")
	}
	// Read only this module so a malformed unrelated flag cannot interrupt billing.
	if m == nil || m.store == nil {
		return false, unavailable()
	}
	values, err := m.store.GetMultiple(ctx, []string{Key(id)})
	if err != nil {
		return false, unavailable()
	}
	value, exists := values[Key(id)]
	if exists && value != "true" && value != "false" {
		return false, unavailable()
	}
	return exists && value == "true", nil
}

func (m *Manager) List(ctx context.Context) ([]State, error) {
	values, err := m.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	items := []State{}
	for _, item := range Catalog() {
		state := State{Manifest: item}
		if item.Managed {
			enabled := values[item.ID]
			state.Enabled = &enabled
		}
		items = append(items, state)
	}
	return items, nil
}

func (m *Manager) SetEnabled(ctx context.Context, id string, enabled bool) error {
	item, ok := Lookup(id)
	if !ok {
		return infraerrors.NotFound("CUSTOM_EXTENSION_NOT_FOUND", "unknown extension")
	}
	if !item.Managed {
		return infraerrors.Conflict("CUSTOM_EXTENSION_NOT_MANAGED", "this extension must be isolated before it can be switched off")
	}
	if m == nil || m.store == nil {
		return unavailable()
	}
	// One key per extension prevents concurrent changes to different switches from
	// overwriting each other. A switch never deletes feature data or migrations.
	if err := m.store.Set(ctx, Key(id), strconv.FormatBool(enabled)); err != nil {
		return unavailable()
	}
	return nil
}

func unavailable() error {
	return infraerrors.ServiceUnavailable("CUSTOM_EXTENSION_STATE_UNAVAILABLE", "extension state is unavailable; please retry")
}
func Disabled(id string) error {
	return infraerrors.Forbidden("CUSTOM_EXTENSION_DISABLED", fmt.Sprintf("extension %s is disabled", id))
}

func pathUnder(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// RequestExtension owns only independent entry points, never an upstream API.
// Running task reads/cancellation are deliberately drained after disablement.
func RequestExtension(method, path string) string {
	switch {
	case (method == "GET" || method == "HEAD") && (path == "/" || pathUnder(path, "/docs")):
		return "premium-home"
	case pathUnder(path, "/canvas"):
		return "infinite-canvas"
	case method == "POST" && path == "/api/v1/playground/runs":
		return "playground"
	case path == "/api/v1/canvas/config" || (method == "POST" && path == "/api/v1/canvas/runs"):
		return "infinite-canvas"
	case strings.HasPrefix(path, "/api/v1/admin/dashboard/operations-"):
		return "operations-analytics"
	case pathUnder(path, "/api/v1/admin/payment/campaigns") || path == "/api/v1/payment/campaigns" || path == "/api/v1/payment/public/campaigns":
		return RechargeCampaigns
	default:
		return ""
	}
}
