// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// C590 — two controls named "method", one page.
//
// Budget settings holds the HOUSEHOLD's budgeting method; Add/Edit budget holds a
// per-budget one. Neither said which it was or what the other did, so changing
// the global picker while thinking you were editing one card produced a page-wide
// switch with no warning. The helpers here give both controls the vocabulary to
// say what they govern.

// methodDisplayName is a methodology's user-facing name — the same label the
// pickers show, so a confirmation and the control it followed agree word for word.
func methodDisplayName(m string) string {
	switch budgeting.ParseMethodology(m) {
	case budgeting.MethodZeroBased:
		return uistate.T("settings.budgetMethodZero")
	case budgeting.MethodEnvelope:
		return uistate.T("settings.budgetMethodEnvelope")
	case budgeting.MethodFlex:
		return uistate.T("settings.budgetMethodFlex")
	}
	return uistate.T("settings.budgetMethodSimple")
}

// methodScopeText is the sentence under the household method picker: what
// changing it would actually reach.
//
// The three cases are genuinely different situations, and one hedged sentence
// covering all of them would be true and useless: with no budgets there is
// nothing to warn about, with no overrides the change reaches everything, and
// with overrides the honest answer is a pair of numbers. Each case formats its
// OWN arguments here rather than through a shared call — a key taking one
// placeholder handed two renders "%!(EXTRA …)" straight onto the page.
func methodScopeText(i budgeting.MethodImpact) string {
	switch {
	case i.Total() == 0:
		return uistate.T("budgets.methodScopeNone")
	case i.Overriding == 0:
		return uistate.T("budgets.methodScopeAll", plural(i.Following, "budget"))
	}
	return uistate.T("budgets.methodScopeMixed", plural(i.Following, "budget"), plural(i.Overriding, "budget"))
}

// budgetOwnMethodHint is the line under the PER-BUDGET method picker: which way
// this budget currently resolves, and the fact that the choice stops at this
// budget. globalMethod is the household setting; own is the budget's override
// ("" = following the household).
func budgetOwnMethodHint(globalMethod, own string) string {
	if own == "" {
		return uistate.T("budgets.methodOwnFollowing", methodDisplayName(globalMethod))
	}
	return uistate.T("budgets.methodOwnOverriding", methodDisplayName(own), methodDisplayName(globalMethod))
}
