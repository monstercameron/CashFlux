// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/reportsnap"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// reportSnapshotsKV is the KV slot holding report snapshots (JSON list).
const reportSnapshotsKV = "cashflux:report-snapshots"

func loadReportSnaps() []reportsnap.Snapshot {
	raw := uistate.KVGet(reportSnapshotsKV)
	if raw == "" {
		return nil
	}
	var out []reportsnap.Snapshot
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func storeReportSnaps(list []reportsnap.Snapshot) {
	if b, err := json.Marshal(list); err == nil {
		uistate.KVSet(reportSnapshotsKV, string(b))
		uistate.RequestPersist()
	}
}

// reportSnapshotProps carries the live aggregates the Snapshot button freezes.
type reportSnapshotProps struct {
	Rows        []reports.CategorySpend // expense categories, rank-ordered
	IncomeRows  []reports.CategorySpend
	Payees      []reports.PayeeTotal
	NameOf      func(string) string
	Base        string
	PeriodLabel string
}

// reportSnapshotControl is the /reports snapshot cluster: "Snapshot" freezes
// the current report's headline aggregates (income, spending, top categories
// and payees) under the period label; the picker reopens a frozen state as a
// read-only panel — late edits can't rewrite what month-end looked like.
func reportSnapshotControl(props reportSnapshotProps) ui.Node {
	selS := ui.UseState("")
	rev := ui.UseState(0)
	_ = rev.Get()
	list := loadReportSnaps()

	takeSnap := ui.UseEvent(Prevent(func() {
		var inc, exp int64
		var cats, pays []reportsnap.LabelAmount
		for _, r := range props.IncomeRows {
			inc += r.Amount
		}
		for _, r := range props.Rows {
			exp += r.Amount
			cats = append(cats, reportsnap.LabelAmount{Label: props.NameOf(r.CategoryID), Amount: r.Amount})
		}
		for _, p := range props.Payees {
			pays = append(pays, reportsnap.LabelAmount{Label: p.Name, Amount: p.Amount})
		}
		s := reportsnap.Snapshot{
			ID: id.New(), TakenAt: time.Now(), PeriodLabel: props.PeriodLabel, Base: props.Base,
			Income: inc, Expense: exp,
			Categories: reportsnap.TopN(cats, 6), Payees: reportsnap.TopN(pays, 6),
		}
		storeReportSnaps(reportsnap.Add(loadReportSnaps(), s))
		selS.Set(s.ID)
		rev.Set(rev.Get() + 1)
		uistate.PostNotice(uistate.T("reports.snapTaken", s.PeriodLabel), false)
	}))
	deleteSel := ui.UseEvent(Prevent(func() {
		if idv := selS.Get(); idv != "" {
			storeReportSnaps(reportsnap.Remove(loadReportSnaps(), idv))
			selS.Set("")
			rev.Set(rev.Get() + 1)
			uistate.PostNotice(uistate.T("reports.snapDeleted"), false)
		}
	}))

	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("reports.snapPlaceholder")}}
	for i := len(list) - 1; i >= 0; i-- { // newest first in the picker
		s := list[i]
		opts = append(opts, uiw.SelectOption{Value: s.ID,
			Label: uistate.T("reports.snapOption", s.PeriodLabel, s.TakenAt.Format("Jan 2"))})
	}

	// The read-only frozen panel for the selected snapshot.
	var panel ui.Node = Fragment()
	if s, ok := reportsnap.ByID(list, selS.Get()); ok {
		m := func(v int64) string { return fmtMoney(money.New(v, s.Base)) }
		lines := []any{css.Class("t-caption"), Attr("data-testid", "report-snap-panel"),
			Style(map[string]string{"margin-top": "0.5rem", "padding": "0.6rem 0.75rem",
				"border": "1px dashed var(--border)", "border-radius": "8px", "display": "grid", "gap": "0.25rem"}),
			Div(css.Class(tw.TextDim), uistate.T("reports.snapFrozen", s.PeriodLabel, s.TakenAt.Format("Jan 2, 2006"))),
			Div(uistate.T("reports.snapTotals", m(s.Income), m(s.Expense), m(s.Net()))),
		}
		for _, c := range s.Categories {
			lines = append(lines, Div(Style(map[string]string{"display": "flex", "gap": "1rem"}),
				Span(Style(map[string]string{"flex": "1 1 auto"}), c.Label), Span(m(c.Amount))))
		}
		panel = Div(lines...)
	}

	return Div(Attr("data-testid", "reports-snapshots"),
		Div(Style(map[string]string{"display": "inline-flex", "gap": "0.4rem", "align-items": "center", "flex-wrap": "wrap"}),
			Button(css.Class("strip-toggle"), Type("button"), Attr("data-testid", "reports-snap-take"),
				Title(uistate.T("reports.snapTakeTitle")), OnClick(takeSnap), uistate.T("reports.snapTake")),
			If(len(list) > 0, uiw.SelectInput(uiw.SelectInputProps{
				Options: opts, Selected: selS.Get(), TestID: "reports-snap-select",
				OnChange: func(v string) { selS.Set(v) }, AriaLabel: uistate.T("reports.snapLabel"),
			})),
			If(selS.Get() != "", Button(css.Class("btn", "btn-sm"), Type("button"),
				Attr("data-testid", "reports-snap-delete"), Title(uistate.T("reports.snapDeleteTitle")),
				OnClick(deleteSel), "✕")),
		),
		panel,
	)
}
