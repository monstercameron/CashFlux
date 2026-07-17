// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — report-local scope atom.
//
// UseReportScope / SetReportScope hold the scope chosen through the /reports
// Scope control. It is deliberately SEPARATE from the app-wide active scope
// (activescope.go, the top-bar "Viewing as" lens): a filter chosen while
// reading a report must never rewrite what the dashboard, accounts, insights,
// or assistant show — the commercial-parity scan flagged exactly that leak.
// Report views combine the two with scope.Merge(lens, reportScope).
//
// The report scope persists under its own KV key so a chosen report view
// survives reloads without touching the lens.
package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	reportScopeAtomID = "reports:scope"
	reportScopeStore  = "cashflux:report-scope"
)

// UseReportScope returns the shared atom tracking the /reports-local scope.
// An IsAll() scope means the report follows the app-wide lens unfiltered.
func UseReportScope() state.Atom[scope.ReportScope] {
	a := state.UseAtom(reportScopeAtomID, loadReportScope())
	capturedReportScope = a
	reportScopeCaptured = true
	return a
}

var (
	capturedReportScope state.Atom[scope.ReportScope]
	reportScopeCaptured bool
)

// SetReportScope changes the report-local scope from outside a component
// render (the scope-selector callbacks) and persists the choice. No-op until
// UseReportScope has been called at least once by a mounted component.
func SetReportScope(s scope.ReportScope) {
	if !reportScopeCaptured {
		return
	}
	capturedReportScope.Set(s)
	if b, err := json.Marshal(s); err == nil {
		kvSet(reportScopeStore, string(b))
		RequestPersist() // KV writes only reach IndexedDB on the autosave ticker
	}
}

// loadReportScope reads the persisted report scope from the KV store.
func loadReportScope() scope.ReportScope {
	raw := kvGet(reportScopeStore)
	if raw == "" {
		return scope.ReportScope{}
	}
	var s scope.ReportScope
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return scope.ReportScope{}
	}
	return s
}
