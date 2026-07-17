// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// AppState is the shared five-state visual vocabulary (task #68): every surface
// that tones a status uses these states and the matching .state-chip classes,
// so "needs attention" looks identical on budgets, accounts, subscriptions, and
// detections — and green/red stay reserved for financial meaning elsewhere.
type AppState string

const (
	// StateHealthy: on track, nothing to do.
	StateHealthy AppState = "healthy"
	// StateWatch: fine today, trending toward a problem.
	StateWatch AppState = "watch"
	// StateAction: needs the user to act now.
	StateAction AppState = "action"
	// StateBlocked: cannot proceed until something outside this item changes.
	StateBlocked AppState = "blocked"
	// StateUnconfirmed: derived/detected, not yet user-verified.
	StateUnconfirmed AppState = "unconfirmed"
)

// StateLabel is the state's standard visible wording (localized).
func StateLabel(s AppState) string {
	switch s {
	case StateHealthy:
		return uistate.T("state.healthy")
	case StateWatch:
		return uistate.T("state.watch")
	case StateAction:
		return uistate.T("state.action")
	case StateBlocked:
		return uistate.T("state.blocked")
	default:
		return uistate.T("state.unconfirmed")
	}
}

// StateChipProps configures one StateChip. Label defaults to StateLabel(State);
// Title (optional) carries the WHY as a tooltip; TestID (optional) is emitted
// as data-testid.
type StateChipProps struct {
	State  AppState
	Label  string
	Title  string
	TestID string
}

// StateChip renders the shared state token: a small labeled chip toned by
// state. Word + tone together (never color alone — WCAG 1.4.1). Pure node
// builder, no hooks — safe to call anywhere, including inside MapKeyed rows.
func StateChip(p StateChipProps) uic.Node {
	label := p.Label
	if label == "" {
		label = StateLabel(p.State)
	}
	args := []any{ClassStr("state-chip state-" + string(p.State))}
	if p.TestID != "" {
		args = append(args, Attr("data-testid", p.TestID))
	}
	if p.Title != "" {
		args = append(args, Title(p.Title), Attr("aria-label", label+": "+p.Title))
	}
	args = append(args, label)
	return Span(args...)
}
