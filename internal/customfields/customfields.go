// Package customfields models user-defined custom fields and validates entity
// custom{} value maps against their definitions. It is the typed-extensibility
// layer: core entities stay strongly typed, while users add validated fields.
// Pure Go, no platform dependencies; unit-tested on native Go.
package customfields

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// FieldType is the data type of a custom field.
type FieldType string

// The supported custom-field types.
const (
	TypeText   FieldType = "text"
	TypeNumber FieldType = "number"
	TypeDate   FieldType = "date" // YYYY-MM-DD string
	TypeBool   FieldType = "bool"
	TypeSelect FieldType = "select"
)

// Def defines one custom field for an entity type.
type Def struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entityType"` // e.g. "account", "transaction"
	Key        string    `json:"key"`        // map key in the entity's custom{} map
	Label      string    `json:"label"`      // human label for forms and errors
	Type       FieldType `json:"type"`
	Options    []string  `json:"options,omitempty"` // allowed values for TypeSelect
	Required   bool      `json:"required,omitempty"`
}

// Validate reports problems with the definition itself (not its values): a Def
// needs an id, entity type, key, label, and a known type; a select field needs
// at least one option. Returns plain-English issues, empty when the Def is sound.
func (d Def) Validate() []string {
	var issues []string
	if d.ID == "" {
		issues = append(issues, "Field id is required.")
	}
	if d.EntityType == "" {
		issues = append(issues, "Entity type is required.")
	}
	if d.Key == "" {
		issues = append(issues, "Field key is required.")
	} else if !ValidKey(d.Key) {
		issues = append(issues, "Field key can use only letters, numbers, and underscores.")
	} else if ReservedKey(d.Key) {
		issues = append(issues, "Field key is reserved.")
	}
	if d.Label == "" {
		issues = append(issues, "Field label is required.")
	}
	if !d.Type.Valid() {
		issues = append(issues, "Field type is not recognized.")
	}
	if d.Type == TypeSelect && len(d.Options) == 0 {
		issues = append(issues, "A choice field needs at least one option.")
	}
	return issues
}

// Valid reports whether the field type is recognized.
func (t FieldType) Valid() bool {
	switch t {
	case TypeText, TypeNumber, TypeDate, TypeBool, TypeSelect:
		return true
	default:
		return false
	}
}

// ValidKey reports whether key is safe for the custom{} map: ASCII letters,
// digits, and underscores only. It intentionally stays stricter than JSON keys so
// custom fields are easy to reference from formulas, imports, and future sync.
func ValidKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

// ReservedKey reports keys that would be confusing or unsafe as user-defined
// custom fields because they shadow core metadata names.
func ReservedKey(key string) bool {
	switch strings.ToLower(key) {
	case "id", "entitytype", "type", "key", "label", "custom", "options", "required":
		return true
	default:
		return false
	}
}

// Validate returns human-readable issues for a custom-field value map against the
// given definitions (all collected, not just the first). Missing required fields,
// type mismatches, invalid dates, and out-of-list select values are reported.
// Unknown keys in values are ignored, so old data stays forward-compatible.
func Validate(defs []Def, values map[string]any) []string {
	var issues []string
	for _, d := range defs {
		v, present := values[d.Key]
		if !present || v == nil {
			if d.Required {
				issues = append(issues, d.Label+" is required.")
			}
			continue
		}
		switch d.Type {
		case TypeText:
			if _, ok := v.(string); !ok {
				issues = append(issues, d.Label+" must be text.")
			}
		case TypeNumber:
			if !isNumber(v) {
				issues = append(issues, d.Label+" must be a number.")
			}
		case TypeBool:
			if _, ok := v.(bool); !ok {
				issues = append(issues, d.Label+" must be true or false.")
			}
		case TypeSelect:
			s, ok := v.(string)
			if !ok || !contains(d.Options, s) {
				issues = append(issues, d.Label+" must be one of the allowed options.")
			}
		case TypeDate:
			s, ok := v.(string)
			if !ok {
				issues = append(issues, d.Label+" must be a date (YYYY-MM-DD).")
			} else if _, err := dateutil.ParseDate(s); err != nil {
				issues = append(issues, d.Label+" must be a valid date (YYYY-MM-DD).")
			}
		default:
			issues = append(issues, d.Label+" has an unknown field type.")
		}
	}
	return issues
}

func isNumber(v any) bool {
	switch v.(type) {
	case float64, float32, int, int64:
		return true
	default:
		return false
	}
}

func contains(opts []string, s string) bool {
	for _, o := range opts {
		if o == s {
			return true
		}
	}
	return false
}
