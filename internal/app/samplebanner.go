// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// SampleDataBanner renders a compact dismissible status chip when the app is
// currently showing the seeded sample dataset (L6). Per the 2026-07-17 visual
// audit (P0 "reduce persistent vertical chrome") it lives INSIDE the top bar's
// context zone — "Sample data · Start fresh" — instead of consuming its own
// banner row above every page's content.
//
// Two actions are offered:
//   - "Start fresh" — wipes the sample and leaves an intentionally empty slate.
//   - ✕ — hides the chip for this session (clears the localStorage flag)
//     without touching the data.
//
// The component is its own element so its two click hooks occupy stable
// positions in the hook chain regardless of whether the chip is mounted.
func SampleDataBanner() uic.Node {
	active := uistate.UseSampleActive()
	uistate.CaptureSampleActive(active)

	if !active.Get() {
		return Fragment()
	}

	dismiss := func() {
		uistate.SetSampleActive(false)
	}

	startFresh := func() {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.Wipe(); err != nil {
			return
		}
		uistate.SetSampleActive(false)
		// Authoritative wipe: clear non-settings keys + persist empty, then reload
		// clean once the IndexedDB write commits.
		suspendAutosave = true
		wipeFinancialLocalState(reloadPage)
	}

	onDismiss := uic.UseEvent(dismiss)
	onStartFresh := uic.UseEvent(startFresh)

	return Div(
		// C4: upgraded to a visually prominent amber-tinted notice strip with an icon
		// so users clearly understand they are viewing demo data (not their own). Still
		// a compact chip (R41) — not a full-width banner — role="status" (persistent,
		// non-urgent); the full explanation is the title tooltip.
		css.Class("sample-banner"),
		Attr("role", "status"),
		Attr("data-testid", "sample-data-banner"),
		Attr("title", uistate.T("sample.chipTitle")),
		ui.Icon(icon.AlertCircle, css.Class("sample-banner-icon")),
		Span(css.Class("sample-banner-text"), uistate.T("sample.chipLabel")),
		Div(css.Class("sample-banner-actions"),
			Button(
				css.Class("sample-banner-btn"),
				Type("button"),
				Attr("title", uistate.T("sample.startFreshTitle")),
				Attr("data-testid", "sample-start-fresh"),
				OnClick(onStartFresh),
				uistate.T("sample.startFresh"),
			),
			// Session dismiss is a quiet ✕ (audit: the chip carries one labeled
			// action; dismiss is secondary chrome, not a second verb to read).
			Button(
				css.Class("sample-banner-x sample-banner-dismiss"),
				Type("button"),
				Attr("title", uistate.T("sample.dismissTitle")),
				Attr("aria-label", uistate.T("sample.dismiss")),
				Attr("data-testid", "sample-dismiss"),
				OnClick(onDismiss),
				ui.Icon(icon.Close, css.Class(tw.W3, tw.H3)),
			),
		),
	)
}
