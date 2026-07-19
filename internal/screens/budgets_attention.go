// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgets_attention.go is the "Needs attention" strip that opens the /budgets
// surface: the three most over/at-risk budgets, each with its spend-of-limit,
// a status pill, and one action (drill to the transactions behind it). It reuses
// the shared budget evaluation (computeBudgetView) and the pure budgeting.TopProblems
// selector, and self-gates to nothing when every budget is healthy — so the page
// opens on real work instead of settings and methodology.

func init() {
	widgetrender.Register("budget-attention", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetAttentionWidget, budgetListProps{App: c.App})
	})
}

// budgetAttentionWidget renders the top-three problem budgets as a compact triage
// strip. It shares computeBudgetView with the summary/list tiles (so the figures
// never disagree) and hides itself entirely when nothing needs attention.
func budgetAttentionWidget(props budgetListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	showLastMonth := uistate.UseBudgetsLastMonth().Get()

	v := computeBudgetView(app, activeMemberID, vw, pr, showLastMonth)
	smartSettings := uistate.LoadSmartSettings()

	// A pace-flagged budget is trending over though not yet at the limit — surface it
	// alongside the over/near budgets so the strip catches problems before they land.
	paceRisk := make(map[string]bool, len(v.PaceOver))
	for id := range v.PaceOver {
		paceRisk[id] = true
	}
	problems := budgeting.TopProblems(v.Statuses, paceRisk, 3)
	if len(problems) == 0 {
		return Fragment() // everything healthy → no strip, no clutter
	}

	// Drill from a problem budget to its spending — the same filter the list rows
	// use, PLUS the budget's own period as a date range: the figures on this
	// strip describe one period, and landing on the category's full multi-year
	// history read as a mismatch (UI/UX task #14). The dates arrive as normal,
	// clearable From/To filter chips, so widening back out is one click.
	viewTransactions := func(categoryIDs []string, from, to string) {
		var f uistate.TxFilter
		switch len(categoryIDs) {
		case 0:
		case 1:
			f.Category = categoryIDs[0]
		default:
			f.Categories = strings.Join(categoryIDs, ",")
		}
		f.From = from
		f.To = to
		f = f.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	// This strip is now the ONE authoritative review queue for the page (the summary's
	// duplicate over/near disclosure was demoted), so the subtitle NAMES what the
	// flagged budgets are rather than the vaguer "N need a look" (July-19 review #4).
	sub := uistate.T("budgetrefine.queueNamedOne")
	if len(problems) != 1 {
		sub = uistate.T("budgetrefine.queueNamed", len(problems))
	}
	rows := MapKeyed(problems,
		func(p budgeting.Problem) any { return p.Status.Budget.ID },
		func(p budgeting.Problem) ui.Node {
			return ui.CreateElement(budgetAttentionRow, budgetAttentionRowProps{
				Status: p.Status, PaceRisk: p.PaceRisk,
				Category:   v.CatName[p.Status.Budget.CategoryID],
				PeriodFrom: v.PeriodFrom[p.Status.Budget.ID],
				PeriodTo:   v.PeriodTo[p.Status.Budget.ID],
				OnView:     viewTransactions,
			})
		})

	body := Div(css.Class("bgattn"), Attr("data-testid", "budgets-attention"),
		Div(css.Class("bgattn-head"),
			uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("bgattn-title"), uistate.T("bgpolish.attnTitle")),
			Span(css.Class("bgattn-sub"), sub),
			// SMART-B14: cover every overage in one pass. It used to ride the summary's
			// (now-demoted) over-banner; it lives on the queue head so the capability
			// survives the consolidation and stays beside the budgets it acts on.
			If(v.OverCount > 0 && smartSettings.IsEnabled(coverAllFeatureCode),
				Div(css.Class("bgattn-head-action"),
					ui.CreateElement(coverAllBannerButton, coverAllButtonProps{}))),
		),
		rows,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-attention", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

type budgetAttentionRowProps struct {
	Status   budgeting.Status
	PaceRisk bool
	Category string
	// PeriodFrom/PeriodTo are the budget's current evaluation window (inclusive
	// ISO dates) so the View-spending drill scopes the ledger to the period the
	// strip's figures describe (task #14).
	PeriodFrom string
	PeriodTo   string
	OnView     func(categoryIDs []string, from, to string)
}

// budgetAttentionRow is one problem budget in the strip: name + spend-of-limit,
// a tone-keyed status pill, and one "View spending" action. Its own component so
// the click hook sits at a stable position (never registered inside a loop).
func budgetAttentionRow(props budgetAttentionRowProps) ui.Node {
	s := props.Status
	onView := ui.UseEvent(Prevent(func() {
		ids := s.Budget.CategoryIDs
		if len(ids) == 0 && s.Budget.CategoryID != "" {
			ids = []string{s.Budget.CategoryID}
		}
		props.OnView(ids, props.PeriodFrom, props.PeriodTo)
	}))

	var tone, pillTxt string
	switch {
	case s.State == budgeting.StateOver:
		tone, pillTxt = "is-over", uistate.T("bgpolish.attnOver")
	case s.State == budgeting.StateNear:
		tone, pillTxt = "is-near", uistate.T("bgpolish.attnNear")
	default:
		tone, pillTxt = "is-pace", uistate.T("bgpolish.attnPace")
	}

	title := budgetTitle(s.Budget.Name, props.Category)
	limit := money.New(s.Spent.Amount+s.Remaining.Amount, s.Spent.Currency)
	nums := uistate.T("bgpolish.attnSpentOf", fmtMoney(s.Spent), fmtMoney(limit))
	// An over budget also states how far over — the number that makes it urgent.
	var overNote ui.Node = Fragment()
	if s.Remaining.IsNegative() {
		overNote = Span(css.Class("bgattn-over"), " · ", uistate.T("bgpolish.attnOverBy", fmtMoney(s.Remaining.Abs())))
	}

	return Div(css.Class("bgattn-row "+tone), Attr("data-testid", "budgets-attn-row-"+s.Budget.ID),
		Div(css.Class("bgattn-main"),
			Span(css.Class("bgattn-cat"), title),
			Span(css.Class("bgattn-nums fig"), nums, overNote),
		),
		Span(css.Class("bgattn-pill "+tone), pillTxt),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-attn-view-"+s.Budget.ID),
			Title(uistate.T("bgpolish.attnViewTitle", title)), OnClick(onView),
			uistate.T("bgpolish.attnView")),
	)
}
