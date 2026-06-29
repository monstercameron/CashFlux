// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package widgetrender is the wasm-only render registry that pairs with the
// platform-independent widgetregistry (docs/UNIFIED_WIDGET_API.md §6). It maps a
// widget's NativeID to a Go renderer that takes a RenderCtx (the live data +
// callbacks a body needs) and returns a tile node. Keeping the render funcs here
// — separate from the pure registry — is what lets widgetregistry, domain, and
// the other logic packages stay native-testable while the dashboard still renders
// every tile through the engine.
package widgetrender

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/ui"
)

// RenderCtx carries everything a registered widget body needs for one render of a
// surface. The surface (e.g. the dashboard) computes these once and passes the
// same ctx to every tile, with Spec set to the current placement's spec. Bodies
// read from the ctx instead of closing over surface-local variables, which is
// what makes them registry-dispatchable rather than inline closures.
type RenderCtx struct {
	App      *appstate.App
	Accounts []domain.Account
	Txns     []domain.Transaction
	// ScopedAccounts / ScopedTxns are the active-scope-filtered slices (member /
	// institution scope) the KPI variable surface is built from. The declarative
	// frame resolvers use these so collection/chart widgets show the same scoped
	// data as the KPI tiles rather than the full household.
	ScopedAccounts []domain.Account
	ScopedTxns     []domain.Transaction
	Rates          currency.Rates
	Base           string

	Start, End time.Time

	Net, Assets, Liabilities, Income, Expense money.Money

	PeriodLabel    string
	CashFlowSub    string
	IncCount       int
	ExpCount       int
	ActiveAccounts int

	NWSub, NWTone, HHNWSub string
	AttnCol, AttnRow       int

	// Vars is the engine variable surface (internal/engineenv) for this render —
	// scoped + period-windowed figures derived from fundamental sources. KPI tiles
	// evaluate their configurable formula against it.
	Vars map[string]float64

	Dismissals freshness.Dismissals
	// RemindToUpdate / DismissFreshness are the freshness-nudge callbacks created at
	// stable hook positions by the surface and threaded in here.
	RemindToUpdate   ui.Handler
	DismissFreshness ui.Handler

	// Spec is the current placement's validated widget spec.
	Spec domain.WidgetSpec

	// Preview marks a non-interactive render (the Studio live preview): tiles drop
	// their settings gear, drag grip, and resize handles, since the editor — not the
	// tile — owns configuration there.
	Preview bool
}

// Renderer renders one widget body from a RenderCtx.
type Renderer func(RenderCtx) ui.Node

var registry = map[string]Renderer{}

// Register binds a Native widget id to its renderer. Called from the package that
// owns the bodies (internal/screens) at init.
func Register(nativeID string, r Renderer) { registry[nativeID] = r }

// Registered reports whether a renderer exists for nativeID.
func Registered(nativeID string) bool { _, ok := registry[nativeID]; return ok }

// Render dispatches to the renderer for nativeID, returning false if none is
// registered (the caller renders an "unavailable" fallback or skips).
func Render(nativeID string, ctx RenderCtx) (ui.Node, bool) {
	r, ok := registry[nativeID]
	if !ok {
		return nil, false
	}
	return r(ctx), true
}

// FrameRenderer paints a List/Table/Chart widget body from the Frame the engine
// hydrated for its Pipeline (plus the RenderCtx for callbacks/formatting). The data
// comes entirely from the engine; the FrameRenderer only presents it. This keeps the
// bespoke visualizations (status bars, balance grids, charts) while making the data
// path declarative through the spec's Pipeline.
type FrameRenderer func(domain.Frame, RenderCtx) ui.Node

var frameRegistry = map[string]FrameRenderer{}

// RegisterFrame binds a data-driven widget id to its Frame renderer.
func RegisterFrame(id string, r FrameRenderer) { frameRegistry[id] = r }

// RenderFrame dispatches to the Frame renderer for id, returning false if none is
// registered.
func RenderFrame(id string, frame domain.Frame, ctx RenderCtx) (ui.Node, bool) {
	r, ok := frameRegistry[id]
	if !ok {
		return nil, false
	}
	return r(frame, ctx), true
}
