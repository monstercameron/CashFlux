// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/scope"

// UseMemberLens returns the household members the top bar's "View as" control is
// currently scoped to — nil for "Everyone".
//
// It exists because reading the perspective was a trap. The app moved its scope
// to the multi-dimensional ActiveScope atom (activescope.go) and left the older
// UseActiveMember atom in place so concurrent work would not break; nothing
// writes that older atom any more. Screens that kept reading it therefore
// applied a member filter that was permanently empty — the switcher looked like
// it scoped the view and silently did nothing (C574).
//
// This is a HOOK: it subscribes to the live atom, so a caller re-renders when the
// perspective changes. ActiveMemberFromScope does not — it reads the persisted
// value once — which is why it is not the right tool inside a render.
func UseMemberLens() []string {
	s := UseActiveScope().Get()
	if len(s.Owners) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.Owners))
	for _, o := range s.Owners {
		if o != "" {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ClearMemberLens returns the app to the "Everyone" perspective while preserving
// every other scope dimension (institutions, account types, specific accounts) —
// the same contract the top-bar switcher honours. It is what a "Viewing as …"
// chip's ✕ calls, so that control removes exactly what it names and nothing more.
func ClearMemberLens() {
	if !activeScopeCaptured {
		return
	}
	cur := capturedActiveScope.Get()
	SetActiveScope(scope.ReportScope{
		Institutions: cur.Institutions,
		Owners:       nil,
		Types:        cur.Types,
		AccountIDs:   cur.AccountIDs,
	})
}
