// SPDX-License-Identifier: MIT

// Package importdraft owns the import wizard's editable, session-local draft.
// It has no browser or transport dependencies. Requests are fenced by identity
// and draft revision, never by a wall clock or a saved slice position.
package importdraft

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/extract"
)

// Request identifies one asynchronous operation against one draft revision.
type Request struct{ ID, Revision uint64 }

// Model is the single controller for draft identity, stage and in-flight work.
// Rows are replaced with fresh slices on every mutation, including same-length
// edits, so browser memo dependencies can safely compare the slice base pointer.
type Model struct {
	Sources              map[string]string
	usedIDs              map[string]bool
	Rows                 []extract.Row
	Source               string
	Stage                string
	Errors               map[string]string
	Revision             uint64
	nextRow, nextRequest uint64
	active               Request
	closed               bool
}

// New restores a draft, assigning identities to any legacy rows.
func New(rows []extract.Row) *Model {
	m := &Model{Stage: "input", Errors: map[string]string{}, Sources: map[string]string{}, usedIDs: map[string]bool{}}
	m.Append("", rows)
	return m
}

// Clean removes control and directional formatting characters from imported text.
func Clean(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		if unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s))
}

// Append adds a source without replacing earlier edits or equal purchases.
func (m *Model) Append(source string, rows []extract.Row) {
	if len(rows) == 0 {
		return
	}
	next := append([]extract.Row(nil), m.Rows...)
	ids := m.usedIDs
	for _, r := range next {
		ids[r.ID] = true
	}
	for _, r := range rows {
		for r.ID == "" || ids[r.ID] {
			m.nextRow++
			r.ID = fmt.Sprintf("draft-%d", m.nextRow)
		}
		ids[r.ID] = true
		m.Sources[r.ID] = source
		r.Description, r.Category = Clean(r.Description), Clean(r.Category)
		next = append(next, r)
	}
	m.Rows, m.Source, m.Stage = next, source, "review"
	m.changed()
}

// Replace edits an existing identity; a missing row is never recreated.
func (m *Model) Replace(rowID string, row extract.Row) bool {
	for i, r := range m.Rows {
		if r.ID != rowID {
			continue
		}
		next := append([]extract.Row(nil), m.Rows...)
		row.ID, row.Description, row.Category = rowID, Clean(row.Description), Clean(row.Category)
		next[i] = row
		m.Rows = next
		delete(m.Errors, rowID)
		m.changed()
		return true
	}
	return false
}

// Remove drops one identity, without moving any other row's editor identity.
func (m *Model) Remove(rowID string) bool {
	for i, r := range m.Rows {
		if r.ID != rowID {
			continue
		}
		m.Rows = append(append([]extract.Row(nil), m.Rows[:i]...), m.Rows[i+1:]...)
		delete(m.Errors, rowID)
		delete(m.Sources, rowID)
		if len(m.Rows) == 0 {
			m.Stage = "input"
		}
		m.changed()
		return true
	}
	return false
}

// Reset clears the draft after an explicit user confirmation.
func (m *Model) Reset() {
	m.Rows, m.Errors, m.Stage = nil, map[string]string{}, "input"
	m.Sources = map[string]string{}
	m.changed()
}

// Begin starts one request and makes every earlier reply stale.
func (m *Model) Begin() Request {
	m.nextRequest++
	m.active = Request{m.nextRequest, m.Revision}
	return m.active
}

// Accept consumes a current reply exactly once. Removed, edited, reset and
// closed drafts reject old replies, including replies from cancelled transports.
func (m *Model) Accept(r Request) bool {
	if m.closed || r.ID == 0 || r != m.active || r.Revision != m.Revision {
		return false
	}
	m.active = Request{}
	return true
}

// Cancel invalidates the current operation before its transport is aborted.
func (m *Model) Cancel() { m.active = Request{} }

// Close prevents all callbacks from touching an unmounted panel.
func (m *Model) Close() { m.closed = true; m.Cancel() }

func (m *Model) changed() { m.Revision++; m.Cancel() }

// ApplyCategories updates only the returned stable identities and keeps user edits.
func (m *Model) ApplyCategories(categories map[string]string) {
	next := append([]extract.Row(nil), m.Rows...)
	for i := range next {
		if next[i].Category == "" {
			next[i].Category = Clean(categories[next[i].ID])
		}
	}
	m.Rows = next
	m.changed()
}

// Outcome is the identity-based result of a staged commit.
type Outcome struct{ ID, Status, Reason string }

// RetainFailures removes only explicitly imported or accepted duplicate rows.
// Unknown outcomes and failed rows stay editable with their error attached.
func (m *Model) RetainFailures(outcomes []Outcome) {
	byID := map[string]Outcome{}
	for _, o := range outcomes {
		byID[o.ID] = o
	}
	next := make([]extract.Row, 0, len(m.Rows))
	for _, row := range m.Rows {
		o := byID[row.ID]
		if o.Status == "imported" || o.Status == "duplicate" {
			delete(m.Errors, row.ID)
			delete(m.Sources, row.ID)
			continue
		}
		if o.Reason != "" {
			m.Errors[row.ID] = o.Reason
		}
		next = append(next, row)
	}
	m.Rows = next
	if len(next) == 0 {
		m.Stage = "input"
	}
	m.changed()
}

// Kind identifies the source history label from the retained row provenance.
func (m *Model) Kind() string {
	source := ""
	for _, row := range m.Rows {
		next := m.Sources[row.ID]
		if source != "" && next != source {
			return "mixed"
		}
		source = next
	}
	switch source {
	case "csv":
		return "csv"
	case "stmt":
		return "statement"
	case "pdf":
		return "pdf"
	default:
		return "image"
	}
}
