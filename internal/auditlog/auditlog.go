// SPDX-License-Identifier: MIT

// Package auditlog is the pure, in-memory audit-log model for CashFlux (C78 phase 4).
// It stores human-readable entries derived from the diff-based change history and
// provides convenient query methods. The package has no syscall/js dependency and
// no imports from appstate, store, or history — callers translate those types into
// Entry values and feed them in. This keeps the model unit-testable on native Go.
//
// Phase 3 note: the current implementation is in-memory (session-scoped). Persisting
// the log to an audit_log SQLite table (C78 phase 3) will be additive: the same Entry
// type and Log API are the contract; the store layer will populate the Log on hydration.
package auditlog

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one event in the audit trail. Every write-path mutation that the app
// records produces exactly one Entry. Callers must populate all fields; the Log
// never synthesises display strings.
//
// Fields:
//   - ID         — unique entry identifier (e.g. a random uuid or sequential int as string).
//   - At         — wall-clock time the change was applied (UTC).
//   - Actor      — who made the change: a member ID/name, or "system" for automated actions.
//   - Action     — short verb phrase describing the operation ("added", "updated", "deleted",
//     "undone", "redone", "imported").
//   - EntityType — the collection or domain type affected ("transaction", "account",
//     "budget", "goal", "task", "category", "member", "rule", …).
//   - EntityID   — the stable ID of the affected row (empty when a bulk/import op touches many).
//   - Summary    — a one-line, redacted, plain-English description for display. Must never
//     contain an API key value or other secret (see Redact).
type Entry struct {
	ID         string
	At         time.Time
	Actor      string
	Action     string
	EntityType string
	EntityID   string
	Summary    string
	// Details are the field-level before → after changes behind an update (or
	// the notable fields of a deleted row), display-ready and redacted. Empty
	// for adds and for entries recorded before details existed.
	Details []FieldChange
}

// FieldChange is one field's before → after in a recorded change. All three
// strings are display-ready (formatted and redacted by the recorder).
type FieldChange struct {
	Field  string
	Before string
	After  string
}

// Log is a bounded, append-only in-memory audit log. It is safe for concurrent use.
// The zero value is not usable; construct with New.
type Log struct {
	mu      sync.Mutex
	entries []Entry
	cap     int // maximum number of entries to keep (0 = unbounded)
}

// New returns an empty Log bounded to maxEntries (0 = unbounded). The cap prevents
// unbounded growth during a long session; oldest entries are dropped when it is hit.
func New(maxEntries int) *Log {
	return &Log{cap: maxEntries}
}

// Append adds e to the log. If the log is at capacity the oldest entry is evicted
// first (drop-oldest). Entries with an empty ID or zero At are accepted — callers
// are responsible for populating them.
func (l *Log) Append(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cap > 0 && len(l.entries) >= l.cap {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, e)
}

// Clear removes every entry, resetting the log to empty. A data wipe uses this to
// reset the session-scoped activity feed so cleared activity can't linger in
// memory (the persisted audit_log table is cleared separately by the store wipe).
func (l *Log) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// Recent returns at most n entries in reverse-chronological order (newest first).
// If n ≤ 0 all entries are returned.
func (l *Log) Recent(n int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) == 0 {
		return nil
	}
	src := l.entries
	if n > 0 && n < len(src) {
		src = src[len(src)-n:]
	}
	// Copy and reverse so callers get newest-first order.
	out := make([]Entry, len(src))
	for i, e := range src {
		out[len(src)-1-i] = e
	}
	return out
}

// ByEntity returns all entries for a given (entityType, entityID) pair in
// reverse-chronological order. Both arguments must be non-empty.
func (l *Log) ByEntity(entityType, entityID string) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Entry
	for i := len(l.entries) - 1; i >= 0; i-- {
		e := l.entries[i]
		if e.EntityType == entityType && e.EntityID == entityID {
			out = append(out, e)
		}
	}
	return out
}

// FilterByEntityType returns the entries whose EntityType matches the given type,
// preserving order. An empty entityType returns the input unchanged (no filter).
// This is a pure helper extracted from the activity screen so the filtering logic
// is unit-tested on native Go rather than leaking into view code (§1.9).
func FilterByEntityType(entries []Entry, entityType string) []Entry {
	if entityType == "" {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.EntityType == entityType {
			out = append(out, e)
		}
	}
	return out
}

// Len returns the total number of stored entries.
func (l *Log) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// ─── Redaction ────────────────────────────────────────────────────────────────

// redactPatterns are substrings that, when found inside a summary candidate,
// signal that the value must be replaced with the placeholder. Extend this list
// as new secret-shaped fields are introduced.
var redactPatterns = []string{
	"sk-",     // OpenAI / Anthropic secret keys begin with "sk-"
	"Bearer ", // HTTP Bearer tokens
}

// redactPlaceholder replaces a matched secret segment.
const redactPlaceholder = "[REDACTED]"

// Redact sanitises a candidate summary string, replacing any segment that looks
// like an API key or bearer token with [REDACTED]. The check is conservative:
// any occurrence of a known-prefix pattern causes the whole value-bearing
// token in the string to be replaced (up to the next whitespace or end-of-string).
//
// Callers MUST pass user-supplied string fields through Redact before storing them
// in Entry.Summary.
func Redact(s string) string {
	for _, pat := range redactPatterns {
		for {
			idx := strings.Index(s, pat)
			if idx == -1 {
				break
			}
			// Find the extent of the token: everything after the pattern until
			// the next whitespace character (or end of string).
			end := idx + len(pat)
			for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '\n' {
				end++
			}
			s = s[:idx] + redactPlaceholder + s[end:]
		}
	}
	return s
}

// ─── Field-level diffs ─────────────────────────────────────────────────────────

// ValueFormatter renders one raw JSON value (as decoded by encoding/json: string,
// float64, bool, nil, map[string]any, []any) for display. key is the JSON field
// name, so a caller can resolve IDs to names or format money shapes.
type ValueFormatter func(key string, v any) string

// DiffJSON compares two JSON object payloads (an update's before/after rows) and
// returns the changed top-level fields as display-ready FieldChanges, sorted by
// field name. fmtVal renders values (nil falls back to a compact default);
// skip lists JSON keys to ignore (ids, blob payloads, timestamps — noise).
// Formatted values pass through Redact so a secret-bearing field can never land
// in the activity feed. Unparseable payloads yield no details (never an error —
// the summary still describes the change).
func DiffJSON(before, after []byte, fmtVal ValueFormatter, skip map[string]bool) []FieldChange {
	var b, a map[string]any
	if err := json.Unmarshal(before, &b); err != nil {
		return nil
	}
	if err := json.Unmarshal(after, &a); err != nil {
		return nil
	}
	if fmtVal == nil {
		fmtVal = DefaultValueFormatter
	}
	keys := map[string]bool{}
	for k := range b {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	var out []FieldChange
	for k := range keys {
		if skip[k] {
			continue
		}
		bv, bok := b[k]
		av, aok := a[k]
		if !bok && !aok {
			continue
		}
		if jsonEqual(bv, av) {
			continue
		}
		bs, as := "", ""
		if bok {
			bs = Redact(fmtVal(k, bv))
		}
		if aok {
			as = Redact(fmtVal(k, av))
		}
		if bs == as {
			continue // formatting collapsed the difference (e.g. float noise)
		}
		out = append(out, FieldChange{Field: k, Before: bs, After: as})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

// jsonEqual reports deep equality of two decoded-JSON values by re-marshalling
// (cheap at row scale, and immune to map ordering).
func jsonEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// DefaultValueFormatter renders a decoded JSON value compactly: strings as-is,
// bools as yes/no, nil/missing as an em dash, numbers trimmed, and composite
// values as compact JSON truncated to a readable length.
func DefaultValueFormatter(_ string, v any) string {
	switch t := v.(type) {
	case nil:
		return "—"
	case string:
		if t == "" {
			return "—"
		}
		return truncateVal(t)
	case bool:
		if t {
			return "yes"
		}
		return "no"
	case float64:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		bs, err := json.Marshal(v)
		if err != nil {
			return "…"
		}
		return truncateVal(string(bs))
	}
}

// truncateVal caps a display value at a readable length.
func truncateVal(s string) string {
	const max = 60
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
