// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// dashboard_whatchanged.go is the E-DB "What changed since your last visit"
// card — the first vertical slice of the E1 attribution engine. It renders the
// top-3 ranked findings for the window since the persisted visit baseline
// (uistate.RollVisitBaseline): each row is finding → amount → why → evidence,
// so the 20-second read the E-series contract asks for happens above the bento.
// "Got it" moves the baseline to now; the card then stays quiet until something
// actually changes. Renders nothing on a first-ever open (no baseline yet) or
// a quiet window.

// wcNavKind selects a row's jump-to behavior.
type wcNavKind string

const (
	wcNavNone    wcNavKind = ""
	wcNavAccount wcNavKind = "account"
	wcNavTxns    wcNavKind = "txns"
)

// wcRowProps is one finding row, fully phrased by the parent (the row only
// owns its navigation hook, per the no-On*-in-loop rule).
type wcRowProps struct {
	Icon     string
	Lead     string
	Amount   money.Money
	Why      string   // "" = no why line
	Evidence []string // pre-formatted evidence lines (≤3)
	Nav      wcNavKind
	AcctID   string
	NavTitle string
	TestID   string
}

// wcRow renders one finding: icon | lead + toned amount | why | evidence, with
// an optional jump button.
func wcRow(p wcRowProps) ui.Node {
	nav := router.UseNavigate()
	kind, acctID := p.Nav, p.AcctID
	onGo := ui.UseEvent(func() {
		switch kind {
		case wcNavAccount:
			uistate.SetDeepLinkFocus(`[data-testid="acct-row-` + acctID + `"]`)
			nav.Navigate(uistate.RoutePath("/accounts"))
		case wcNavTxns:
			nav.Navigate(uistate.RoutePath("/transactions"))
		}
	})
	body := []any{css.Class("wc-row-body")}
	body = append(body, Div(css.Class("wc-lead"),
		Span(p.Lead),
		Strong(css.Class("wc-amt "+figTone(p.Amount)), fmtMoney(p.Amount)),
	))
	if p.Why != "" {
		body = append(body, Div(css.Class("wc-why"), p.Why))
	}
	for _, ev := range p.Evidence {
		body = append(body, Div(css.Class("wc-ev"), ev))
	}
	row := []any{css.Class("wc-row"), Attr("data-testid", p.TestID),
		Span(css.Class("wc-icon"), Attr("aria-hidden", "true"), p.Icon),
		Div(body...),
	}
	if p.Nav != wcNavNone {
		row = append(row, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("aria-label", p.NavTitle),
			Attr("title", p.NavTitle),
			OnClick(onGo),
			uistate.T("dashboard.wcView"),
		))
	}
	return Div(row...)
}

// wcMemo caches the attribution pass per (data revision, baseline) so dashboard
// re-renders don't recompute an O(accounts × transactions) scan.
var wcMemo struct {
	key string
	rep attribution.Report
}

// wcPlaceholder holds the card's DOM slot while it has nothing to say. A
// stable hidden element (not an empty Fragment) so the reconciler keeps the
// card's position above the bento when it appears after the settle window —
// late-mounting from nothing appends to the end of the page instead.
func wcPlaceholder() ui.Node {
	return Div(css.Class("wc-slot"), Attr("data-testid", "dash-whatchanged-slot"),
		Attr("aria-hidden", "true"), Style(map[string]string{"display": "none"}))
}

// dashWhatChangedCard assembles the card. Hooks sit above every early return.
func dashWhatChangedCard() ui.Node {
	app := appstate.Default
	rev := uistate.UseDataRevision().Get()
	baseline := uistate.UseVisitBaseline().Get()
	settled := useAfterSettle("whatchanged")
	// Roll the baseline once per session, after render (never during it).
	ui.UseEffect(func() func() {
		uistate.RollVisitBaseline(time.Now().Unix())
		return nil
	}, "once")

	if app == nil || !settled || baseline <= 0 {
		return wcPlaceholder()
	}

	settings := app.Settings()
	base := settings.BaseCurrency
	rates := currency.Rates{Base: base, Rates: settings.FXRates}
	adjDesc := uistate.T("accounts.balanceAdjustment")

	key := fmt.Sprintf("%d|%d|%s", rev, baseline, base)
	if wcMemo.key != key {
		rep, err := attribution.Compute(attribution.Input{
			Accounts: app.Accounts(),
			Txns:     app.Transactions(),
			Rates:    rates,
			Since:    time.Unix(baseline, 0),
			Until:    time.Now(),
			IsAdjustment: func(t domain.Transaction) bool {
				return t.Desc == adjDesc
			},
		})
		if err != nil {
			slog.Warn("what-changed: attribution failed", "err", err)
			return wcPlaceholder()
		}
		wcMemo.key, wcMemo.rep = key, rep
	}
	rep := wcMemo.rep
	if len(rep.Items) == 0 {
		return wcPlaceholder()
	}

	txnByID := map[string]domain.Transaction{}
	for _, t := range app.Transactions() {
		txnByID[t.ID] = t
	}
	cats := app.Categories()

	rows := make([]ui.Node, 0, len(rep.Items))
	for i, it := range rep.Items {
		rows = append(rows, ui.CreateElement(wcRow, wcRowProp(it, i, base, cats, txnByID, rates)))
	}

	onGotIt := ui.UseEvent(func() {
		uistate.MarkVisitCaughtUp(time.Now().Unix())
	})

	return Div(css.Class("wc-card"),
		Attr("data-testid", "dash-whatchanged-card"),
		Attr("role", "complementary"),
		Attr("aria-label", uistate.T("dashboard.wcAria")),
		Div(css.Class("wc-head"),
			Strong(uistate.T("dashboard.wcTitle")),
			Span(css.Class("wc-since"),
				uistate.T("dashboard.wcSince", time.Unix(baseline, 0).Format("Mon, Jan 2"))),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("aria-label", uistate.T("dashboard.wcGotItTitle")),
				Attr("title", uistate.T("dashboard.wcGotItTitle")),
				Attr("data-testid", "wc-gotit"),
				OnClick(onGotIt),
				uistate.T("dashboard.wcGotIt"),
			),
		),
		Div(css.Class("wc-rows"), rows),
	)
}

// wcRowProp phrases one attribution finding into row props: lead, why line,
// and evidence lines, all through the i18n catalog.
func wcRowProp(it attribution.Item, idx int, base string, cats []domain.Category,
	txnByID map[string]domain.Transaction, rates currency.Rates) wcRowProps {

	p := wcRowProps{
		Amount: money.New(it.AmountMinor, base),
		TestID: fmt.Sprintf("wc-row-%d-%s", idx, string(it.Kind)),
	}
	switch it.Kind {
	case attribution.KindNetWorth:
		p.Icon, p.Lead = "📊", uistate.T("dashboard.wcLeadNet")
		p.Why = wcJoin(wcParts(it.Parts, base), wcCount(it.Count))
	case attribution.KindAccount:
		p.Icon, p.Lead = "🏦", it.AccountName
		p.Why = wcJoin(wcParts(it.Parts, base), wcCount(it.Count))
		p.Nav, p.AcctID = wcNavAccount, it.AccountID
		p.NavTitle = uistate.T("dashboard.wcViewAccountTitle", it.AccountName)
	case attribution.KindCategory:
		name := categoryNameByID(cats, it.CategoryID)
		if name == "" {
			name = uistate.T("dashboard.wcUncategorized")
		}
		p.Icon, p.Lead = "🧾", uistate.T("dashboard.wcLeadCategory", name)
		p.Why = wcCount(it.Count)
		p.Nav, p.NavTitle = wcNavTxns, uistate.T("dashboard.wcViewTxnsTitle")
	case attribution.KindIncome:
		p.Icon, p.Lead = "💵", uistate.T("dashboard.wcLeadIncome")
		if it.Count == 1 {
			p.Why = uistate.T("dashboard.wcDepositOne")
		} else {
			p.Why = uistate.T("dashboard.wcDeposits", it.Count)
		}
		p.Nav, p.NavTitle = wcNavTxns, uistate.T("dashboard.wcViewTxnsTitle")
	case attribution.KindLargeTxn:
		p.Icon, p.Lead = "❗", uistate.T("dashboard.wcLeadLarge", it.Payee)
		p.Nav, p.NavTitle = wcNavTxns, uistate.T("dashboard.wcViewTxnsTitle")
	case attribution.KindNewPayee:
		p.Icon, p.Lead = "🆕", uistate.T("dashboard.wcLeadNew", it.Payee)
		if it.Count > 1 {
			p.Why = uistate.T("dashboard.wcNewMore", it.Count-1)
		}
		p.Nav, p.NavTitle = wcNavTxns, uistate.T("dashboard.wcViewTxnsTitle")
	}
	for _, id := range it.TxnIDs {
		t, ok := txnByID[id]
		if !ok {
			continue
		}
		label := t.Payee
		if label == "" {
			label = t.Desc
		}
		amt := t.Amount
		if conv, err := rates.Convert(t.Amount, base); err == nil {
			amt = conv
		}
		p.Evidence = append(p.Evidence,
			t.Date.Format("Jan 2")+" · "+label+" · "+fmtMoney(amt))
	}
	return p
}

// wcParts renders the nonzero decomposition parts as "cash flow $X" phrases.
func wcParts(parts []attribution.Part, base string) []string {
	out := make([]string, 0, len(parts))
	for _, pt := range parts {
		amt := fmtMoney(money.New(pt.AmountMinor, base))
		switch pt.Kind {
		case attribution.PartFlow:
			out = append(out, uistate.T("dashboard.wcPartFlow", amt))
		case attribution.PartAdjustments:
			out = append(out, uistate.T("dashboard.wcPartAdj", amt))
		case attribution.PartOther:
			out = append(out, uistate.T("dashboard.wcPartOther", amt))
		}
	}
	return out
}

// wcCount phrases a contributing-transaction count ("" for zero).
func wcCount(n int) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return uistate.T("dashboard.wcTxnCountOne")
	default:
		return uistate.T("dashboard.wcTxnCount", n)
	}
}

// wcJoin joins nonempty fragments with the card's separator.
func wcJoin(parts []string, extra string) string {
	if extra != "" {
		parts = append(parts, extra)
	}
	out := ""
	for _, s := range parts {
		if s == "" {
			continue
		}
		if out != "" {
			out += " · "
		}
		out += s
	}
	return out
}
