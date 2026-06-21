// Package widgetvis tracks which dashboard widget instances are hidden. It is a
// tiny pure value type (no syscall/js) so visibility is unit-testable and shared
// between the dashboard (which skips hidden tiles and reflows around them) and the
// Widget Manager (which toggles them). Keyed by widget instance id.
package widgetvis

// Set is the set of hidden widget instance ids. The empty set hides nothing.
type Set map[string]bool

// IsHidden reports whether the widget with the given id is hidden.
func (s Set) IsHidden(id string) bool { return s[id] }

// Toggle returns a copy with the widget's hidden state flipped.
func (s Set) Toggle(id string) Set {
	out := s.clone()
	if out[id] {
		delete(out, id)
	} else {
		out[id] = true
	}
	return out
}

// With returns a copy with the widget explicitly hidden or shown.
func (s Set) With(id string, hidden bool) Set {
	out := s.clone()
	if hidden {
		out[id] = true
	} else {
		delete(out, id)
	}
	return out
}

// Filter returns the subset of ids that are NOT hidden, preserving order — used
// by the dashboard to drop hidden widgets before packing so the rest reflow.
func (s Set) Filter(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !s[id] {
			out = append(out, id)
		}
	}
	return out
}

// Normalize drops false entries so the persisted form is minimal and stable.
func (s Set) Normalize() Set { return s.clone() }

func (s Set) clone() Set {
	out := make(Set, len(s)+1)
	for k, v := range s {
		if v {
			out[k] = true
		}
	}
	return out
}
