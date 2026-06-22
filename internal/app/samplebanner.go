//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// SampleDataBanner renders a one-line dismissible info banner when the app is
// currently showing the seeded sample dataset (L6). The banner sits above the
// top-bar inside the main pane so it is visible on every screen without
// intruding on the shell chrome.
//
// Two actions are offered:
//   - "Start fresh" — wipes the sample and leaves an intentionally empty slate.
//   - "Dismiss" — hides the banner for this session (clears the localStorage
//     flag) without touching the data.
//
// The component is its own element so its two click hooks occupy stable
// positions in the hook chain regardless of whether the banner is mounted.
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
		uistate.BumpDataRevision()
	}

	onDismiss := uic.UseEvent(dismiss)
	onStartFresh := uic.UseEvent(startFresh)

	return Div(
		css.Class("sample-banner"),
		Attr("role", "alert"),
		Attr("data-testid", "sample-data-banner"),
		Span(css.Class("sample-banner-text"), uistate.T("sample.bannerText")),
		Div(css.Class("sample-banner-actions"),
			Button(
				css.Class("sample-banner-btn"),
				Type("button"),
				Attr("title", uistate.T("sample.startFreshTitle")),
				Attr("data-testid", "sample-start-fresh"),
				OnClick(onStartFresh),
				uistate.T("sample.startFresh"),
			),
			Button(
				css.Class("sample-banner-btn sample-banner-dismiss"),
				Type("button"),
				Attr("title", uistate.T("sample.dismissTitle")),
				Attr("aria-label", uistate.T("sample.dismiss")),
				Attr("data-testid", "sample-dismiss"),
				OnClick(onDismiss),
				uistate.T("sample.dismiss"),
			),
		),
	)
}
