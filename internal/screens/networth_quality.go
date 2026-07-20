// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/ledger"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// What the headline figure rests on.
//
// The page says "as of Jul 20, 2026" and, until now, nothing else about where
// the number came from. That is a real gap here specifically: a property
// holding is most of the asset side and is a figure somebody typed in once, so
// one forgotten valuation can move net worth more than a year of saving, and
// the reader had no way to tell. The FX table has the same property for a
// foreign-currency account.
//
// It is a DISCLOSURE, not a nag: it sits inline with the as-of line, collapsed,
// and says plainly whether anything needs attention. When nothing does it says
// so in a few words and takes up one line. It never restates a figure — it only
// describes the figure's footing — so it cannot disagree with the headline.

// nwsQualityProps carries the assessed disclosure into the component.
type nwsQualityProps struct {
	Q    balancesheet.Quality
	Base string
	// AsOfLine is the "your balance sheet as of X" sentence the trigger sits
	// beside, so the two read as one line rather than two competing labels.
	AsOfLine string
}

// nwsQuality renders the as-of line with an expandable disclosure of what the
// figure rests on. Own component so its popover hooks sit at a stable
// call-site. It reuses the app's popover convention (add-wrap / add-menu,
// DismissPopover + AnchorPopover, role=dialog) exactly as the "?" explainers do.
func nwsQuality(p nwsQualityProps) ui.Node {
	const id = "nws-quality"
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	uiw.DismissPopover(open.Get(), id, func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), id)

	q := p.Q
	// The trigger states the headline of the disclosure, so the common case
	// ("everything is current") needs no click at all.
	summary := uistate.T("nws.dqClean", q.AccountsIncluded)
	trigCls := "nws-dq-btn"
	if n := len(q.Stale); n > 0 {
		summary = uistate.T("nws.dqStaleSummary", q.AccountsIncluded, n)
		trigCls += " is-attention"
	}

	menuCls := "add-menu nws-explain-pop"
	if !open.Get() {
		menuCls += " hidden-menu"
	}

	lines := []ui.Node{
		P(css.Class("nws-explain-line"), uistate.T("nws.dqIncluded", q.AccountsIncluded)),
	}
	// Every overdue account is named. A count alone would tell the reader
	// something is wrong without telling them what to go and fix.
	for _, a := range q.Stale {
		if a.DaysSince < 0 {
			lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-stale"),
				uistate.T("nws.dqStaleNever", a.Name)))
			continue
		}
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-stale"),
			uistate.T("nws.dqStaleAccount", a.Name, plural(a.DaysSince, "day"))))
	}
	// The oldest hand-entered valuation, and — when it is also most of what the
	// household owns — why that matters for this particular figure.
	if q.HasOldestManual {
		m := q.OldestManual
		when := uistate.T("nws.dqNever")
		if !m.AsOf.IsZero() {
			when = uistate.LoadPrefs().FormatDate(m.AsOf)
		}
		if q.HasDominant && q.Dominant.ID == m.ID {
			lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-manual"),
				uistate.T("nws.dqManualDominant", m.Name, when, m.ShareOfSideBips/100)))
		} else {
			lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-manual"),
				uistate.T("nws.dqManual", m.Name, when)))
		}
	} else if q.HasDominant {
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-dominant"),
			uistate.T("nws.dqDominant", q.Dominant.Name, q.Dominant.ShareOfSideBips/100)))
	}
	// Currency. The app stores exchange rates but not when they were captured,
	// so this says where they come from and does NOT claim a freshness it
	// cannot substantiate.
	if len(q.Converted) > 0 {
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-fx"),
			uistate.T("nws.dqFx", joinAnd(q.Converted), q.BaseCurrency)))
	}
	if q.ExcludedByChoice > 0 {
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-excluded"),
			uistate.T("nws.dqExcludedChoice", plural(q.ExcludedByChoice, "account"))))
	}
	if q.ExcludedNoRate > 0 {
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-norate"),
			uistate.T("nws.dqExcludedNoRate", plural(q.ExcludedNoRate, "account"))))
	}
	if !q.NeedsAttention() {
		lines = append(lines, P(css.Class("nws-explain-line"), Attr("data-testid", "nws-dq-ok"),
			uistate.T("nws.dqAllCurrent")))
	}

	return P(css.Class("nws-hero-eyebrow"),
		p.AsOfLine,
		Span(css.Class("nws-dq", "add-wrap"), Attr("id", id),
			Button(ClassStr(trigCls), Type("button"),
				Attr("data-testid", "nws-dq-btn"),
				Attr("aria-haspopup", "dialog"), Attr("aria-expanded", boolStr(open.Get())),
				Attr("aria-label", uistate.T("nws.dqAria")), Title(uistate.T("nws.dqAria")),
				OnClick(toggle), summary),
			Div(ClassStr(menuCls), Attr("role", "dialog"),
				Attr("aria-label", uistate.T("nws.dqTitle")), Attr("data-testid", "nws-dq-pop"),
				Div(css.Class("nws-explain-title"), uistate.T("nws.dqTitle")),
				lines,
				A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
					Attr("data-testid", "nws-dq-update"), uistate.T("nws.dqUpdate")),
			),
		),
	)
}

// nwsAssessQuality binds the pure assessment to the app's OWN notions of stale
// and of a hand-set balance, rather than inventing second opinions about
// either: freshness.IsStale with the household's windows, and
// ledger.BalanceProvenance's adjusted/opening kinds.
func nwsAssessQuality(v nwsView) balancesheet.Quality {
	app := appstate.Default
	if app == nil {
		return balancesheet.Quality{}
	}
	windows := app.FreshnessWindows()
	adjDesc := uistate.T("accounts.balanceAdjustment")
	txns := app.Transactions()
	isAdj := func(t domain.Transaction) bool { return t.Desc == adjDesc }
	return balancesheet.AssessQuality(balancesheet.QualityInput{
		Accounts: app.Accounts(),
		Txns:     txns,
		Rates:    v.Rates,
		Now:      time.Now(),
		IsStale: func(a domain.Account) bool {
			return freshness.IsStale(a, windows, time.Now())
		},
		IsManual: func(a domain.Account) bool {
			kind, _ := ledger.BalanceProvenance(a.ID, txns, isAdj)
			return kind == ledger.ProvenanceAdjusted || kind == ledger.ProvenanceOpening
		},
		ExcludedByChoice: len(v.Snapshot.ExcludedByChoice),
		ExcludedNoRate:   len(v.Snapshot.ExcludedAccounts),
	})
}

// joinAnd renders a short list as "EUR and GBP" / "EUR, GBP and JPY".
func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + uistate.T("nws.and") + " " + items[1]
	}
	out := ""
	for i, it := range items[:len(items)-1] {
		if i > 0 {
			out += ", "
		}
		out += it
	}
	return out + " " + uistate.T("nws.and") + " " + items[len(items)-1]
}
