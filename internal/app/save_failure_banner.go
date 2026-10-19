// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// durableNoticeApp identifies the page whose durable writer has been installed.
// Before boot installs it there is no financial UI whose success can be promised.
var durableNoticeApp *appstate.App

// SaveFailureBanner stays mounted with the global feedback surface. Individual
// writers clear their own failure only after strict durable success.
func SaveFailureBanner() ui.Node {
	failed := uistate.UsePersistFailures()
	retry := ui.UseEvent(func() { uistate.RetryPersistFailures() })
	if !failed {
		return nil
	}
	return Div(css.Class("toast toast-err"), Attr("data-testid", "unsaved-changes"),
		Attr("role", "alert"), Attr("aria-live", "assertive"),
		Attr("style", "position:fixed;top:1rem;bottom:auto;z-index:10000;max-width:calc(100vw - 2rem)"),
		Span(uistate.T("bv6d.unsaved")),
		Button(Type("button"), css.Class("btn"), Attr("data-testid", "retry-unsaved-changes"),
			OnClick(retry), uistate.T("bv6d.retry")))
}

func withSaveFailureBanner(notice ui.Node) ui.Node {
	return Div(ui.CreateElement(SaveFailureBanner), notice)
}

func requestNoticeDurable(app *appstate.App, done func(error)) {
	uistate.RequestPreferenceDurable(func(err error) {
		if err != nil {
			done(err)
			return
		}
		app.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
	})
}
