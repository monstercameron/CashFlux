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
