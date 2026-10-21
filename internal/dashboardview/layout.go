// SPDX-License-Identifier: MIT

package dashboardview

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
)

// LayoutInput is the complete configurable surface. Financial revisions are
// intentionally absent: edits to transactions do not change placement schemas.
// The host supplies current defaults so registry/spec changes are explicit too.
type LayoutInput struct {
	Instance uint64
	Ready    bool
	Items    []dashlayout.Item
	Configs  map[string]widgetcfg.Config
	Hidden   widgetvis.Set
	Stored   []domain.Placement
	Defaults map[string]domain.WidgetSpec
}

// LayoutEntry is either a validated placement or a graph-published card.
type LayoutEntry struct {
	Placement *domain.Placement
	Published dashlayout.Item
}

// PreparedLayout is shared and read-only. Entries retain tile order, including
// graph-published cards which are rendered but never persisted as placements.
type PreparedLayout struct {
	Entries    []LayoutEntry
	Hero       *domain.Placement
	Placements []domain.Placement
	Signature  string
	Err        error
}

type layoutEntry struct {
	input  LayoutInput
	layout *PreparedLayout
}

// LayoutCache retains at most two immutable configurations. Equality checks
// typed inputs without JSON encoding or schema validation on a warm render.
type LayoutCache struct {
	mu      sync.Mutex
	entries []layoutEntry
	builds  atomic.Uint64
}

// Prepare returns a shared immutable layout and whether its configuration was cached.
func (c *LayoutCache) Prepare(input LayoutInput) (*PreparedLayout, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, entry := range c.entries {
		if reflect.DeepEqual(entry.input, input) {
			copy(c.entries[i:], c.entries[i+1:])
			c.entries[len(c.entries)-1] = entry
			return entry.layout, true
		}
	}
	frozen := cloneLayoutInput(input)
	c.builds.Add(1)
	result := buildLayout(frozen)
	if len(c.entries) == 2 {
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, layoutEntry{input: frozen, layout: result})
	return result, false
}

// Builds counts configurations actually validated and serialized.
func (c *LayoutCache) Builds() uint64 { return c.builds.Load() }

func buildLayout(in LayoutInput) *PreparedLayout {
	result := &PreparedLayout{}
	stored := make(map[string]domain.WidgetSpec, len(in.Stored))
	for _, placement := range in.Stored {
		stored[placement.ID] = placement.Spec
	}
	if in.Ready {
		for _, item := range in.Items {
			spec, builtin := in.Defaults[item.ID]
			if builtin {
				spec.Settings = in.Configs[item.ID]
				if spec.Settings == nil {
					spec.Settings = widgetcfg.Config{}
				}
				if saved, ok := stored[item.ID]; ok {
					spec = OverlayStoredSpec(spec, saved)
				}
			} else if strings.HasPrefix(item.ID, "us:") {
				var found bool
				spec, found = stored[item.ID]
				if !found {
					continue
				}
			} else if strings.HasPrefix(item.ID, "wb:") && !in.Hidden.IsHidden(item.ID) {
				result.Entries = append(result.Entries, LayoutEntry{Published: item})
				continue
			} else {
				continue
			}
			placement := domain.Placement{SchemaVersion: domain.WidgetSpecVersion, ID: item.ID, Surface: "dashboard", Spec: spec, Layout: item, Hidden: in.Hidden.IsHidden(item.ID)}
			if placement.Validate() != nil {
				continue
			}
			result.Placements = append(result.Placements, placement)
			if !placement.Hidden {
				result.Entries = append(result.Entries, LayoutEntry{Placement: &placement})
			}
		}
	}
	if spec, ok := in.Defaults["hero"]; ok {
		hero := domain.Placement{SchemaVersion: domain.WidgetSpecVersion, ID: "hero", Surface: "dashboard", Spec: spec, Layout: dashlayout.Item{ID: "hero", ColSpan: 4, RowSpan: 2}}
		if hero.Validate() == nil {
			result.Hero = &hero
			result.Placements = append(result.Placements, hero)
		}
	}
	encoded, err := json.Marshal(result.Placements)
	result.Signature, result.Err = string(encoded), err
	return result
}

// OverlayStoredSpec restores only configurable presentation; structural built-in
// bindings continue to follow the current registry definition.
func OverlayStoredSpec(def, saved domain.WidgetSpec) domain.WidgetSpec {
	if saved.Content.Mode == domain.LayoutCustom && len(saved.Content.Blocks) > 0 {
		def.Content = saved.Content
	}
	if !saved.Style.Empty() {
		def.Style = saved.Style
	}
	if saved.Title != "" {
		def.Title = saved.Title
	}
	return def
}
func cloneLayoutInput(in LayoutInput) LayoutInput {
	in.Items = slices.Clone(in.Items)
	in.Hidden = maps.Clone(in.Hidden)
	configs := make(map[string]widgetcfg.Config, len(in.Configs))
	for id, config := range in.Configs {
		configs[id] = maps.Clone(config)
	}
	if in.Configs != nil {
		in.Configs = configs
	}
	if in.Stored != nil {
		stored := slices.Clone(in.Stored)
		for i, placement := range stored {
			stored[i].Spec = cloneSpec(placement.Spec)
			stored[i].Access.ViewRoles = slices.Clone(placement.Access.ViewRoles)
			stored[i].Access.EditRoles = slices.Clone(placement.Access.EditRoles)
			stored[i].Access.OnlyMembers = slices.Clone(placement.Access.OnlyMembers)
		}
		in.Stored = stored
	}
	if in.Defaults != nil {
		defaults := make(map[string]domain.WidgetSpec, len(in.Defaults))
		for id, spec := range in.Defaults {
			defaults[id] = cloneSpec(spec)
		}
		in.Defaults = defaults
	}
	return in
}
func cloneSpec(spec domain.WidgetSpec) domain.WidgetSpec {
	if spec.Scalar != nil {
		scalar := *spec.Scalar
		spec.Scalar = &scalar
	}
	if spec.Pipeline != nil {
		pipeline := *spec.Pipeline
		pipeline.Transform = slices.Clone(pipeline.Transform)
		spec.Pipeline = &pipeline
	}
	spec.Settings = maps.Clone(spec.Settings)
	spec.Graph = slices.Clone(spec.Graph)
	spec.Content.Blocks = slices.Clone(spec.Content.Blocks)
	return spec
}
