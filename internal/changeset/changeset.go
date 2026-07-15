// SPDX-License-Identifier: MIT

// Package changeset models an agent's multi-step plan as ONE reviewable unit —
// a "pull request for your money" (AG1). A Changeset is an ordered list of
// proposed operations; each operation is pure data (a dispatch Kind, a
// human-readable Line, and opaque JSON Args) plus a per-item Enabled flag, so
// the whole proposal is inspectable and unit-testable with no syscall/js and no
// dependency on appstate.
//
// The APPLY side — mapping a Kind to a real mutation — deliberately lives
// elsewhere (internal/appstate/changeset_apply.go) so this package stays a pure
// value type. Applying an enabled subset produces a Receipt: what ran, and the
// first failure if one occurred (apply stops on the first error — never a silent
// partial state). The Receipt is the input to AG1's one-tap "Undo all" and to
// AG20's cumulative session receipt.
package changeset

import "encoding/json"

// Op is one proposed operation in a changeset. It is pure data: Kind selects the
// apply dispatcher, Line is the plain-English description shown in the review
// card, and Args carries the operation's parameters as opaque JSON handed
// verbatim to the dispatcher. Enabled lets the user toggle the single item off
// before applying the rest.
type Op struct {
	// Kind is the dispatch key (e.g. "add_transaction", "create_category").
	Kind string
	// Line is a one-line, plain-English description of what the op will do.
	Line string
	// Args are the operation's parameters, opaque to this package.
	Args json.RawMessage
	// Enabled reports whether the op is included in an apply-all. New ops are
	// enabled by default (use New/Add, which set it).
	Enabled bool
}

// Changeset is an ordered, reviewable list of proposed operations with a short
// title. The zero value is a valid empty changeset.
type Changeset struct {
	// Title is a short heading for the whole proposal (e.g. "Set up a vacation
	// fund"). Optional.
	Title string
	// Ops are the proposed operations, applied in slice order.
	Ops []Op
}

// New returns an empty changeset with the given title.
func New(title string) *Changeset { return &Changeset{Title: title} }

// Add appends an enabled op and returns the changeset for chaining.
func (c *Changeset) Add(kind, line string, args json.RawMessage) *Changeset {
	c.Ops = append(c.Ops, Op{Kind: kind, Line: line, Args: args, Enabled: true})
	return c
}

// Len reports the number of ops (enabled or not).
func (c *Changeset) Len() int { return len(c.Ops) }

// IsEmpty reports whether the changeset has no ops.
func (c *Changeset) IsEmpty() bool { return len(c.Ops) == 0 }

// SetEnabled toggles the op at index i. Out-of-range indexes are ignored so
// callers wired to UI events never panic on a stale index.
func (c *Changeset) SetEnabled(i int, enabled bool) {
	if i < 0 || i >= len(c.Ops) {
		return
	}
	c.Ops[i].Enabled = enabled
}

// EnabledCount reports how many ops are currently enabled — the N in
// "Apply all (N)".
func (c *Changeset) EnabledCount() int {
	n := 0
	for _, op := range c.Ops {
		if op.Enabled {
			n++
		}
	}
	return n
}

// AppliedOp records one op that ran successfully, with the dispatcher's
// human-readable result. The slice of these on a Receipt drives the receipt card
// and the per-op "Undo all".
type AppliedOp struct {
	// Index is the op's position in the source changeset.
	Index int
	// Kind and Line are copied from the source op for display without holding a
	// reference to it.
	Kind string
	Line string
	// Result is the dispatcher's short, plain-English confirmation.
	Result string
}

// FailedOp records the single op whose apply returned an error. Because apply
// stops on the first failure, at most one FailedOp exists per Receipt.
type FailedOp struct {
	Index int
	Kind  string
	Line  string
	// Err is the failure message (the dispatcher's error, or "unknown
	// operation" when no dispatcher is registered for the Kind).
	Err string
}

// Receipt is the outcome of applying a changeset's enabled ops in order. Applied
// lists every op that ran (in run order); Failed is non-nil when an op errored,
// in which case no later op ran (no silent partial state) — the ops in Applied
// are exactly the durable changes to offer "Undo all" over.
type Receipt struct {
	Applied []AppliedOp
	Failed  *FailedOp
}

// OK reports whether every enabled op applied with no failure.
func (r Receipt) OK() bool { return r.Failed == nil }

// AppliedCount reports how many ops applied successfully.
func (r Receipt) AppliedCount() int { return len(r.Applied) }

// Kinds returns the Kind of each applied op, in run order — the aggregation
// input for AG20's cumulative session receipt.
func (r Receipt) Kinds() []string {
	out := make([]string, len(r.Applied))
	for i, a := range r.Applied {
		out[i] = a.Kind
	}
	return out
}
