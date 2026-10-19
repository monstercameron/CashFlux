// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// durableNoticeApp identifies the page whose durable writer has been installed.
// Before boot installs it there is no financial UI whose success can be promised.
var durableNoticeApp *appstate.App

var (
	staleDatasetBlocked     bool
	staleRecoveryReady      bool
	staleDatasetRecoveryKey string
	staleDatasetNotice      state.Atom[int]
	staleDatasetNoticeRev   int
	staleDatasetNoticeLive  bool
)

func resetStaleDatasetState() {
	staleDatasetBlocked, staleRecoveryReady = false, false
	staleDatasetRecoveryKey = ""
	staleDatasetNoticeRev++
	if staleDatasetNoticeLive {
		staleDatasetNotice.Set(staleDatasetNoticeRev)
	}
}

// preserveStaleDatasetEdits stores the stale tab's live data under a unique,
// encrypted recovery key before presenting Reload. It never replaces the
// generation-owned dataset or another recovery copy.
func preserveStaleDatasetEdits(app *appstate.App, workspace string, owners ...string) {
	if staleDatasetBlocked {
		return
	}
	staleDatasetBlocked = true
	suspendAutosave = true
	makeStaleDatasetReadOnly()
	staleDatasetNoticeRev++
	if staleDatasetNoticeLive {
		staleDatasetNotice.Set(staleDatasetNoticeRev)
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		app.Log().Error("stale tab edits could not be prepared for recovery", "err", err)
		return
	}
	owner := browserstore.GetString(datasetOwnerKey)
	if len(owners) > 0 {
		owner = owners[0]
	}
	staleDatasetRecoveryKey = fmt.Sprintf("cashflux:stale-recovery:%s:%s:%s", checkpointHash(workspace), checkpointHash(owner), id.New())
	browserstore.RetainRecovery(staleDatasetRecoveryKey, string(data), "", describeRecoveryPayload, liveRecoveryVersions(), func(key string, err error) {
		if err != nil {
			app.Log().Error("stale tab recovery copy could not be saved", "err", err)
			return
		}
		staleDatasetRecoveryKey = key
		staleRecoveryReady = true
		staleDatasetNoticeRev++
		if staleDatasetNoticeLive {
			staleDatasetNotice.Set(staleDatasetNoticeRev)
		}
	})
}

func makeStaleDatasetReadOnly() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	controls := doc.Call("querySelectorAll", "button, input, select, textarea")
	for i := 0; i < controls.Get("length").Int(); i++ {
		control := controls.Index(i)
		if control.Call("closest", ".stale-dataset-banner").Truthy() {
			continue
		}
		control.Set("disabled", true)
	}
}

// SaveFailureBanner stays mounted with the global feedback surface. Individual
// writers clear their own failure only after strict durable success.
func SaveFailureBanner() ui.Node {
	failed := uistate.UsePersistFailures()
	notice := state.UseAtom("app:stale-dataset", staleDatasetNoticeRev)
	staleDatasetNotice, staleDatasetNoticeLive = notice, true
	if staleDatasetBlocked {
		return ui.CreateElement(staleDatasetBanner)
	}
	if !failed {
		return nil
	}
	return ui.CreateElement(retrySaveBanner)
}

func staleDatasetBanner() ui.Node {
	state.UseAtom("app:stale-dataset", staleDatasetNoticeRev)
	download := ui.UseEvent(func() { downloadStaleRecovery() })
	reload := ui.UseEvent(func() { js.Global().Get("location").Call("reload") })
	message := "tc4n9.staleSaving"
	if staleRecoveryReady {
		message = "tc4n9.staleReady"
	}
	return Div(
		Div(Attr("aria-hidden", "true"), Attr("style", "position:fixed;inset:0;z-index:9998;background:transparent;cursor:not-allowed")),
		Div(css.Class("toast toast-err"), Attr("data-testid", "stale-dataset-banner"), Attr("role", "alert"),
			Attr("style", "position:fixed;top:1rem;bottom:auto;z-index:10000;max-width:calc(100vw - 2rem)"),
			Span(uistate.T(message)),
			Button(Type("button"), Disabled(!staleRecoveryReady), css.Class("btn"), Attr("data-testid", "download-stale-recovery"), OnClick(download), uistate.T("tc4n9.download")),
			Button(Type("button"), Disabled(!staleRecoveryReady), css.Class("btn"), Attr("data-testid", "reload-stale-dataset"), OnClick(reload), uistate.T("tc4n9.reload")),
		),
	)
}

func retrySaveBanner() ui.Node {
	retry := ui.UseEvent(func() { uistate.RetryPersistFailures() })
	return Div(css.Class("toast toast-err"), Attr("data-testid", "unsaved-changes"),
		Attr("role", "alert"), Attr("aria-live", "assertive"),
		Attr("style", "position:fixed;top:1rem;bottom:auto;z-index:10000;max-width:calc(100vw - 2rem)"),
		Span(uistate.T("bv6d.unsaved")),
		Button(Type("button"), css.Class("btn"), Attr("data-testid", "retry-unsaved-changes"),
			OnClick(retry), uistate.T("bv6d.retry")))
}

func downloadStaleRecovery() { go downloadStaleRecoveryAsync() }

func downloadStaleRecoveryAsync() {
	if staleDatasetRecoveryKey == "" {
		return
	}
	raw, exists, err := browserstore.GetSecure(staleDatasetRecoveryKey)
	if err != nil || !exists || raw == "" {
		return
	}
	blob := js.Global().Get("Blob").New([]any{raw}, map[string]any{"type": "application/json"})
	url := js.Global().Get("URL").Call("createObjectURL", blob).String()
	anchor := js.Global().Get("document").Call("createElement", "a")
	anchor.Set("href", url)
	anchor.Set("download", "cashflux-unsaved-recovery.json")
	anchor.Call("click")
	js.Global().Get("URL").Call("revokeObjectURL", url)
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
