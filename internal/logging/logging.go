// Package logging provides CashFlux's structured logging on top of log/slog.
// A single Handler writes human-readable lines to an io.Writer (the browser
// console in the wasm app, stderr natively) and also records entries in a
// bounded in-app ring buffer for a debug log viewer.
//
// The package is pure Go and unit-tested on native Go; the wasm app supplies a
// console-backed writer.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// Entry is one captured log record for the in-app viewer.
type Entry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// Ring is a bounded, concurrency-safe ring buffer of log entries. When full, the
// oldest entry is dropped.
type Ring struct {
	mu    sync.Mutex
	buf   []Entry
	start int
	size  int
}

// NewRing returns a ring buffer that retains up to capacity entries.
func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{buf: make([]Entry, capacity)}
}

// Add appends an entry, evicting the oldest when full.
func (r *Ring) Add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size < len(r.buf) {
		r.buf[(r.start+r.size)%len(r.buf)] = e
		r.size++
		return
	}
	r.buf[r.start] = e
	r.start = (r.start + 1) % len(r.buf)
}

// Entries returns the buffered entries, oldest first.
func (r *Ring) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, r.size)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%len(r.buf)]
	}
	return out
}

// Len returns the number of buffered entries.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}

// Handler is a slog.Handler that writes lines and records entries in a Ring.
type Handler struct {
	w      io.Writer
	ring   *Ring
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

// NewHandler builds a Handler. w and ring may be nil. level defaults to Info.
func NewHandler(w io.Writer, ring *Ring, level slog.Leveler) *Handler {
	if level == nil {
		level = slog.LevelInfo
	}
	return &Handler{w: w, ring: ring, level: level}
}

// New returns a *slog.Logger and its Ring, wired to writer w.
func New(w io.Writer, capacity int, level slog.Leveler) (*slog.Logger, *Ring) {
	ring := NewRing(capacity)
	return slog.New(NewHandler(w, ring, level)), ring
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

// Handle implements slog.Handler.
func (h *Handler) Handle(_ context.Context, rec slog.Record) error {
	attrs := make(map[string]any)
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", rec.Level.String(), rec.Message)

	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.Any()
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
	}
	prefix := groupPrefix(h.groups)
	rec.Attrs(func(a slog.Attr) bool {
		key := prefix + a.Key
		attrs[key] = a.Value.Any()
		fmt.Fprintf(&b, " %s=%v", key, a.Value.Any())
		return true
	})
	b.WriteByte('\n')

	if h.w != nil {
		if _, err := io.WriteString(h.w, b.String()); err != nil {
			return err
		}
	}
	if h.ring != nil {
		h.ring.Add(Entry{Level: rec.Level, Message: rec.Message, Attrs: attrs})
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	prefix := groupPrefix(h.groups)
	clone := h.clone()
	for _, a := range attrs {
		clone.attrs = append(clone.attrs, slog.Any(prefix+a.Key, a.Value.Any()))
	}
	return clone
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *Handler) clone() *Handler {
	c := *h
	c.attrs = append([]slog.Attr(nil), h.attrs...)
	c.groups = append([]string(nil), h.groups...)
	return &c
}

func groupPrefix(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	return strings.Join(groups, ".") + "."
}
