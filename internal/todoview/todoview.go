// SPDX-License-Identifier: MIT

// Package todoview stores named to-do view presets — a filter, sort, lens and
// display mode saved together under a name the user chose (C404).
//
// The to-do screen already had every control it needed; what it lacked was
// memory. Someone who works "everything overdue, assigned to me, oldest first"
// every Monday morning had to rebuild it from four separate controls every
// Monday morning. A saved view is the whole combination as one click.
//
// The package is pure: a View is data, a Store is a slice with rules, and the
// caller owns persistence. Nothing here knows about localStorage, atoms, or the
// screen.
package todoview

import (
	"encoding/json"
	"errors"
	"strings"
)

// MaxViews caps how many a household can save. The list is a picker, and a
// picker with fifty entries is a worse way to find a view than rebuilding it.
const MaxViews = 12

// MaxNameLen bounds a view name so the picker stays readable.
const MaxNameLen = 40

// View is one saved combination of the to-do screen's controls. Every field is
// the exact string the corresponding atom holds, so applying a view is an
// assignment rather than a translation — a mapping layer between saved state and
// live state is a place for the two to disagree.
type View struct {
	Name string `json:"name"`
	// Display is the view mode: "list", "board" or "calendar".
	Display string `json:"display,omitempty"`
	// Quick is the lens: "all", "today" or "overdue".
	Quick string `json:"quick,omitempty"`
	// Sort is the sort mode key.
	Sort string `json:"sort,omitempty"`
	// Priority filters to one priority; empty is every priority.
	Priority string `json:"priority,omitempty"`
	// Link filters by linked-entity type; empty is every kind.
	Link string `json:"link,omitempty"`
	// BoardGroup is the board's group-by ("status" or "priority").
	BoardGroup string `json:"boardGroup,omitempty"`
	// HideDone hides completed tasks.
	HideDone bool `json:"hideDone,omitempty"`
	// Search is the query text. It is saved because "unpaid, containing
	// 'insurance'" is a real saved view; it is the one field most likely to be
	// deliberately left blank, which the zero value already handles.
	Search string `json:"search,omitempty"`
}

// ErrNameRequired is returned when a view is saved without a name.
var ErrNameRequired = errors.New("a saved view needs a name")

// ErrTooMany is returned when the store is full.
var ErrTooMany = errors.New("no room for another saved view")

// Key is the comparison identity of a view name: trimmed and case-folded, so
// "Overdue" and "overdue " are the same view rather than two entries one letter
// apart in a picker.
func Key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Store is the ordered list of saved views. The zero value is an empty store.
type Store struct {
	Views []View `json:"views"`
}

// Find returns the view saved under name, matched by Key.
func (s Store) Find(name string) (View, bool) {
	k := Key(name)
	for _, v := range s.Views {
		if Key(v.Name) == k {
			return v, true
		}
	}
	return View{}, false
}

// Save adds v, or REPLACES the existing view with the same name. Overwriting is
// the right default: "save this as Monday triage" when Monday triage already
// exists means update it, and refusing would force a delete-then-save dance for
// the most common edit there is.
//
// The stored name keeps the caller's capitalization, trimmed. The order of an
// overwritten view is preserved — a saved view that jumped to the end of the
// picker every time it was tweaked would be maddening.
func (s *Store) Save(v View) error {
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		return ErrNameRequired
	}
	if len([]rune(v.Name)) > MaxNameLen {
		v.Name = string([]rune(v.Name)[:MaxNameLen])
	}
	k := Key(v.Name)
	for i, existing := range s.Views {
		if Key(existing.Name) == k {
			s.Views[i] = v
			return nil
		}
	}
	if len(s.Views) >= MaxViews {
		return ErrTooMany
	}
	s.Views = append(s.Views, v)
	return nil
}

// Delete removes the view saved under name and reports whether it existed.
func (s *Store) Delete(name string) bool {
	k := Key(name)
	for i, v := range s.Views {
		if Key(v.Name) == k {
			s.Views = append(s.Views[:i], s.Views[i+1:]...)
			return true
		}
	}
	return false
}

// Names lists the saved view names in order, for a picker.
func (s Store) Names() []string {
	out := make([]string, 0, len(s.Views))
	for _, v := range s.Views {
		out = append(out, v.Name)
	}
	return out
}

// ActiveName returns the name of the saved view that exactly matches current, if
// any. Highlighting the picker requires knowing whether what is on screen still
// IS the saved view — as soon as one control is changed, it is not, and a picker
// that keeps claiming otherwise lies about what the user is looking at.
//
// The comparison ignores Name, so a view is "active" when every control matches.
func (s Store) ActiveName(current View) string {
	for _, v := range s.Views {
		a, b := v, current
		a.Name, b.Name = "", ""
		if a == b {
			return v.Name
		}
	}
	return ""
}

// Marshal serializes the store for persistence.
func (s Store) Marshal() string {
	b, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Unmarshal parses a persisted store. Malformed or empty input yields an empty
// store rather than an error: a corrupt preference must not stop the screen from
// rendering, and there is nothing a user could do with the error anyway.
//
// Entries beyond MaxViews are dropped, so a store written by a future version
// with a higher cap cannot make this one unbounded.
func Unmarshal(raw string) Store {
	var s Store
	if strings.TrimSpace(raw) == "" {
		return s
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Store{}
	}
	// Drop nameless entries — they cannot be selected, deleted or overwritten,
	// so they would be permanent dead rows in the picker.
	kept := s.Views[:0]
	seen := map[string]bool{}
	for _, v := range s.Views {
		k := Key(v.Name)
		if k == "" || seen[k] || len(kept) >= MaxViews {
			continue
		}
		seen[k] = true
		kept = append(kept, v)
	}
	s.Views = kept
	return s
}
