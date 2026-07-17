// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/savedreports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// savedReportsKV is the KV slot holding the saved report views (JSON list).
const savedReportsKV = "cashflux:saved-reports"

func loadSavedReports() []savedreports.Saved {
	raw := uistate.KVGet(savedReportsKV)
	if raw == "" {
		return nil
	}
	var out []savedreports.Saved
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func storeSavedReports(list []savedreports.Saved) {
	if b, err := json.Marshal(list); err == nil {
		uistate.KVSet(savedReportsKV, string(b))
		uistate.RequestPersist()
	}
}

// savedReportsControl is the /reports "Saved views" cluster: a picker that
// re-applies a named configuration (period window + report-local scope), a
// save-current flow (inline name field), and delete for the selected view.
func savedReportsControl(props struct{}) ui.Node {
	periodAtom := uistate.UsePeriod()
	scopeAtom := uistate.UseReportScope() // also captures the atom so SetReportScope applies
	selS := ui.UseState("")
	nameOpen := ui.UseState(false)
	nameS := ui.UseState("")
	rev := ui.UseState(0) // bumps after save/delete so the picker re-reads KV

	_ = rev.Get()
	list := loadSavedReports()

	apply := func(idv string) {
		selS.Set(idv)
		s, ok := savedreports.ByID(loadSavedReports(), idv)
		if !ok {
			return
		}
		w := period.Window{Res: period.Resolution(s.Res), From: s.From, To: s.To,
			WeekStart: uistate.LoadPrefs().WeekStartWeekday()}
		periodAtom.Set(w)
		uistate.PersistPeriodWindow(w)
		uistate.SetReportScope(s.Scope)
		uistate.PostNotice(uistate.T("reports.savedApplied", s.Name), false)
	}
	toggleName := ui.UseEvent(Prevent(func() { nameOpen.Set(!nameOpen.Get()) }))
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	saveCurrent := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			return
		}
		w := periodAtom.Get()
		sc := scopeAtom.Get()
		entry := savedreports.Saved{ID: id.New(), Name: name,
			Res: string(w.Res), From: w.From, To: w.To, Scope: sc}
		storeSavedReports(savedreports.Add(loadSavedReports(), entry))
		nameS.Set("")
		nameOpen.Set(false)
		selS.Set(entry.ID)
		rev.Set(rev.Get() + 1)
		uistate.PostNotice(uistate.T("reports.savedStored", name), false)
	}))
	deleteSel := ui.UseEvent(Prevent(func() {
		idv := selS.Get()
		if idv == "" {
			return
		}
		storeSavedReports(savedreports.Remove(loadSavedReports(), idv))
		selS.Set("")
		rev.Set(rev.Get() + 1)
		uistate.PostNotice(uistate.T("reports.savedDeleted"), false)
	}))

	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("reports.savedPlaceholder")}}
	for _, s := range list {
		opts = append(opts, uiw.SelectOption{Value: s.ID, Label: s.Name})
	}

	// #46: the name-current-view form opens in the app-standard flip modal
	// (standard Save/Cancel footer via FormID) instead of expanding inline in
	// the toolbar. Constructed unconditionally (FlipPanel carries a hook),
	// rendered only while open.
	saveModal := uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        uistate.T("reports.savedSave"),
		Width:        uiw.FlipSmallW,
		Height:       "min(60vh, 300px)",
		FormID:       "reports-saved-form",
		SaveTestID:   "reports-saved-confirm",
		CancelTestID: "reports-saved-cancel",
		OnClose:      func() { nameOpen.Set(false); nameS.Set("") },
		Back: Form(Attr("id", "reports-saved-form"), OnSubmit(saveCurrent),
			uiw.FormField(uistate.T("reports.savedNameLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "reports-saved-name"),
					Attr("aria-label", uistate.T("reports.savedNameLabel")), Attr("autofocus", "true"),
					Placeholder(uistate.T("reports.savedNamePh")), Value(nameS.Get()), OnInput(onName))),
		),
	})

	return Div(Attr("data-testid", "reports-saved"),
		Style(map[string]string{"display": "inline-flex", "gap": "0.4rem", "align-items": "center", "flex-wrap": "wrap"}),
		If(len(list) > 0, uiw.SelectInput(uiw.SelectInputProps{
			Options: opts, Selected: selS.Get(), TestID: "reports-saved-select",
			OnChange: apply, AriaLabel: uistate.T("reports.savedLabel"),
		})),
		If(selS.Get() != "", Button(css.Class("btn", "btn-sm"), Type("button"),
			Attr("data-testid", "reports-saved-delete"), Title(uistate.T("reports.savedDeleteTitle")),
			OnClick(deleteSel), "✕")),
		Button(css.Class("strip-toggle"), Type("button"), Attr("data-testid", "reports-saved-open"),
			Title(uistate.T("reports.savedSaveTitle")), OnClick(toggleName), uistate.T("reports.savedSave")),
		If(nameOpen.Get(), saveModal),
	)
}
