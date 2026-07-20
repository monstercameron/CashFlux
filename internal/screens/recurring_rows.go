// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// recurAmountTone returns the semantic tone class for a signed money amount:
// income green, outflow red-toned. Shared by the agenda + roster rows.
func recurAmountTone(m money.Money) string {
	if m.IsNegative() {
		return tw.ColorClass("text-down")
	}
	return tw.ColorClass("text-up")
}

// recurVarianceText renders the plain-English variance chip for a paid
// occurrence: "ran $2 over" or "$1 under". variance is signed minor units.
func recurVarianceText(variance int64, cur string) string {
	mag := variance
	if mag < 0 {
		mag = -mag
	}
	amt := fmtMoney(money.New(mag, cur))
	if variance > 0 {
		return uistate.T("billmatch.ranOver", amt)
	}
	return uistate.T("billmatch.ranUnder", amt)
}

// ─── Overdue strip row ───────────────────────────────────────────────────────

// rhyOverdueRowProps drives one overdue-strip row.
type rhyOverdueRowProps struct {
	Occ        recurOccurrence
	Base       string
	OnMarkPaid func(recurOccurrence)
}

// rhyOverdueRow renders one overdue occurrence with the inline pay verb. Its own
// component so the Mark-paid hook stays at a stable position.
func rhyOverdueRow(props rhyOverdueRowProps) ui.Node {
	r := props.Occ.R
	markPaid := ui.UseEvent(Prevent(func() {
		if props.OnMarkPaid != nil {
			props.OnMarkPaid(props.Occ)
		}
	}))
	var verb ui.Node = Fragment()
	if r.Amount.IsNegative() {
		verb = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "rhy-mark-paid-"+r.ID), Title(uistate.T("bills.markPaidTitle")),
			OnClick(markPaid), uistate.T("bills.markPaid"))
	}
	return Div(css.Class("rhy-row"), Attr("role", "listitem"), Attr("data-testid", "rhy-overdue-row-"+r.ID),
		Div(css.Class("rhy-row-main"),
			Span(css.Class("rhy-row-name"), r.Label),
			Span(css.Class("rhy-row-meta"),
				uistate.T("recurring.overdue"), Span(css.Class("rec-sep"), " · "),
				uistate.LoadPrefs().FormatDate(props.Occ.Date)),
		),
		rhyModeBadge(r),
		Span(ClassStr("rhy-row-amt "+recurAmountTone(r.Amount)), fmtMoney(r.Amount)),
		Div(css.Class("rhy-row-actions"), verb),
	)
}

// ─── Findings strip rows ─────────────────────────────────────────────────────

// findingKind distinguishes the findings-row verbs.
type findingKind int

const (
	findingCharged findingKind = iota // charged after cancellation → dispute to-do
	findingStopped                    // seems to have stopped → pause it
)

// rhyFindingRowProps drives one findings-strip row.
type rhyFindingRowProps struct {
	Kind          findingKind
	Name          string
	Text          string
	Base          string
	Late          subscriptions.LateCharge
	CommitmentID  string
	Flows         []domain.Recurring
	OnPauseToggle func(domain.Recurring)
}

// rhyFindingRow renders one inline finding with a one-click verb. Its own
// component (both verb hooks sit at stable positions).
func rhyFindingRow(props rhyFindingRowProps) ui.Node {
	addTodo := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		t := domain.Task{
			ID:       id.New(),
			Title:    uistate.T("rhythm.disputeTask", props.Late.SubName, fmtMoney(money.New(props.Late.Amount, props.Base))),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Source:   domain.SourceNudge,
		}
		if err := app.PutTask(t); err == nil {
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("rhythm.findDispute"), false)
		}
	}))
	pauseIt := ui.UseEvent(Prevent(func() {
		for _, r := range props.Flows {
			if r.ID == props.CommitmentID {
				if props.OnPauseToggle != nil {
					props.OnPauseToggle(r)
				}
				return
			}
		}
	}))

	iconCls := "rhy-finding-ic"
	ic := icon.Clock
	var verb ui.Node
	switch props.Kind {
	case findingCharged:
		iconCls += " is-alarm"
		ic = icon.AlertTriangle
		verb = Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-finding-dispute"),
			OnClick(addTodo), uistate.T("rhythm.findDispute"))
	default: // findingStopped
		verb = Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-finding-pause"),
			OnClick(pauseIt), uistate.T("rhythm.findPause"))
	}
	return Div(css.Class("rhy-finding"), Attr("role", "listitem"),
		uiw.Icon(ic, css.Class(iconCls, tw.ShrinkO, tw.W4, tw.H4)),
		Span(css.Class("rhy-finding-text"), props.Text),
		verb,
	)
}
