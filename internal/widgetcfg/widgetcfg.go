// SPDX-License-Identifier: MIT

// Package widgetcfg is the per-widget settings API that connects a dashboard
// widget's flip-panel settings to its content. Each widget registers a Schema
// (a list of typed Fields with defaults); the settings panel renders the schema
// generically, and the widget reads its values from a Config (the persisted
// key→value map). Pure Go, no platform dependencies, unit-tested on native Go;
// the wasm layer only persists Configs and renders the fields.
package widgetcfg

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/contrast"
)

// FieldType is the kind of control a Field renders as.
type FieldType string

// The supported field types.
const (
	Toggle FieldType = "toggle" // boolean ("true"/"false")
	Number FieldType = "number" // integer, optionally clamped to [Min, Max]
	Select FieldType = "select" // one of Options
)

// Option is one choice for a Select field.
type Option struct {
	Value string
	Label string
}

// Field describes a single setting: how it renders, its default, and (for
// numbers) its bounds. Values are stored as strings in a Config and parsed
// through the typed accessors below, so persistence stays simple and tolerant.
type Field struct {
	Key     string
	Label   string
	Type    FieldType
	Default string
	Unit    string   // optional display suffix, e.g. "%"
	Min     int      // Number only; ignored when Max <= Min (unbounded)
	Max     int      // Number only
	Options []Option // Select only
}

// Schema is the ordered set of settings a widget exposes.
type Schema struct {
	WidgetID string
	Title    string
	Fields   []Field
}

// Field returns the field with the given key.
func (s Schema) FieldByKey(key string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Config holds a widget's setting values, keyed by field key, as strings.
type Config map[string]string

// AccentKey is the reserved Config key holding a widget's optional per-widget
// accent color (a hex string). It is not a schema field — every widget can be
// tinted — so it uses a leading underscore to stay clear of real field keys.
const AccentKey = "_accent"

// Accent returns the widget's per-widget accent color if one is set and is a
// valid hex color, else "". The UI uses it to tint just this tile.
func (c Config) Accent() string {
	v := strings.TrimSpace(c[AccentKey])
	if _, _, _, err := contrast.ParseHex(v); err != nil {
		return ""
	}
	return v
}

// Str returns the field's value from c, falling back to the default when missing
// or empty. For a Select, an unknown value also falls back to the default.
func (f Field) Str(c Config) string {
	v := c[f.Key]
	if v == "" {
		v = f.Default
	}
	if f.Type == Select && !f.validOption(v) {
		v = f.Default
	}
	return v
}

// Bool returns the field's value as a boolean ("true" is true; anything else,
// including the default, is false).
func (f Field) Bool(c Config) bool {
	return f.Str(c) == "true"
}

// Int returns the field's value parsed as an integer, falling back to the
// default on a parse error and clamping to [Min, Max] when bounded (Max > Min).
func (f Field) Int(c Config) int {
	n, err := strconv.Atoi(f.Str(c))
	if err != nil {
		n, _ = strconv.Atoi(f.Default)
	}
	if f.Max > f.Min {
		if n < f.Min {
			n = f.Min
		}
		if n > f.Max {
			n = f.Max
		}
	}
	return n
}

// validOption reports whether v is one of the field's Select options.
func (f Field) validOption(v string) bool {
	for _, o := range f.Options {
		if o.Value == v {
			return true
		}
	}
	return false
}

// registry holds the schema for every widget that exposes settings.
var registry = map[string]Schema{}

// register adds (or replaces) a widget's schema. Called from builtins.go init.
func register(s Schema) { registry[s.WidgetID] = s }

// SchemaFor returns the settings schema for a widget id, if it has one.
func SchemaFor(id string) (Schema, bool) {
	s, ok := registry[id]
	return s, ok
}

// Has reports whether a widget exposes any settings.
func Has(id string) bool {
	_, ok := registry[id]
	return ok
}

// IDs returns the widget ids that have settings, sorted.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
