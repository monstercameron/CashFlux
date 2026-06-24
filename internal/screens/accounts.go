// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Accounts lists assets and liabilities with live balances, a net-worth summary,
// an add form, and per-row delete.
func Accounts() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Revision atom: bumping it after a mutation re-renders this screen.
	rev := state.UseAtom("rev:accounts", 0)

	errMsg := ui.UseState("")
	noticeAtom := uistate.UseNotice()
	notifyErr := func(text string) { noticeAtom.Set(noticeAtom.Get().With(text, true)) }

	bump := func() { rev.Set(rev.Get() + 1) }

	deleteAccount := func(accountID string) {
		// Refuse to delete an account that still has transactions (including the far
		// leg of a transfer): deleting the row would orphan them. Steer the user to
		// Archive, which retires the account but keeps its history.
		for _, t := range app.Transactions() {
			if t.AccountID == accountID || t.TransferAccountID == accountID {
				errMsg.Set(uistate.T("accounts.deleteHasTxns"))
				return
			}
		}
		if err := app.DeleteAccount(accountID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	archiveAccount := func(ac domain.Account) {
		ac.Archived = !ac.Archived
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	refreshAccount := func(ac domain.Account) {
		ac.BalanceAsOf = time.Now()
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	loadSample := ui.UseEvent(Prevent(func() {
		if err := app.LoadSample(); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}))
	markAllUpdated := ui.UseEvent(Prevent(func() {
		w := app.FreshnessWindows()
		now := time.Now()
		n := 0
		for _, ac := range app.Accounts() {
			if ac.Archived || !freshness.IsStale(ac, w, now) {
				continue
			}
			ac.BalanceAsOf = now
			if err := app.PutAccount(ac); err != nil {
				notifyErr(uistate.T("accounts.markErr", err.Error()))
				continue
			}
			n++
		}
		errMsg.Set("")
		if n > 0 {
			noticeAtom.Set(noticeAtom.Get().With(uistate.T("accounts.markedUpdated", plural(n, "balance")), false))
		}
		bump()
	}))

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// Explainable roll-up (L4): an account whose currency has no FX rate is excluded
	// with a notice rather than silently collapsing the whole total to zero.
	nw, _ := ledger.NetWorthExplained(accounts, txns, rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities

	var assetList, liabList, archivedList []domain.Account
	for _, ac := range accounts {
		switch {
		case ac.Archived:
			archivedList = append(archivedList, ac)
		case ac.Class == domain.ClassLiability:
			liabList = append(liabList, ac)
		default:
			assetList = append(assetList, ac)
		}
	}
	// Sort assets and liabilities by current balance, largest first (G3 §7), so the
	// accounts that move net worth most sit at the top of each group instead of in
	// insertion order. Balances are converted to base so multi-currency rows sort
	// comparably; a missing FX rate falls back to raw minor units.
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		if c, err := rates.Convert(bal, base); err == nil {
			return c.Amount
		}
		return bal.Amount
	}
	sort.SliceStable(assetList, func(i, j int) bool { return convBal(assetList[i]) > convBal(assetList[j]) })
	sort.SliceStable(liabList, func(i, j int) bool { return convBal(liabList[i]) > convBal(liabList[j]) })

	// Net-worth month-to-date delta (G3 §3): the change in net worth since the
	// first of the current month, so Theo can answer "up or down this month?" at a
	// glance. Computed honestly from the two net-worth snapshots — no proxy.
	nowTS := time.Now()
	monthStart := time.Date(nowTS.Year(), nowTS.Month(), 1, 0, 0, 0, 0, nowTS.Location())
	var nwDelta money.Money
	haveDelta := false
	if series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{monthStart, nowTS.AddDate(0, 0, 1)}, rates); err == nil && len(series) == 2 {
		if d, derr := series[1].Sub(series[0]); derr == nil {
			nwDelta, haveDelta = d, true
		}
	}

	saveAccount := func(ac domain.Account) {
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	setBalance := func(ac domain.Account, currentBal money.Money, newStr, catID string) {
		dec := currency.Decimals(ac.Currency)
		target, err := money.ParseMinor(strings.TrimSpace(newStr), dec)
		if err != nil {
			errMsg.Set(uistate.T("accounts.invalidBalance"))
			return
		}
		// Post an adjustment transaction for the difference, so the computed
		// balance equals the figure entered (e.g. matching a statement). The
		// optional catID lets the user attach a category to the adjustment so it
		// doesn't land as an uncategorized spike in reports (L57/L30).
		if amount, ok := ledger.AdjustmentToTarget(currentBal, target); ok {
			adj := domain.Transaction{
				ID: id.New(), AccountID: ac.ID, Date: time.Now(), Desc: uistate.T("accounts.balanceAdjustment"),
				Amount: amount, Cleared: true, CategoryID: catID,
			}
			if err := app.PutTransaction(adj); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		ac.BalanceAsOf = time.Now()
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
		// Confirm the update through the toast live region (a polite notice), so the
		// new balance is announced to screen readers and visibly acknowledged — the
		// reconcile flow was previously silent on success.
		noticeAtom.Set(noticeAtom.Get().With(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(target, ac.Currency))), false))
	}

	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTransactions := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	doTransfer := func(fromID, toID string, amountStr, dateStr, desc string) {
		dec := currency.Decimals("")
		for _, ac := range accounts {
			if ac.ID == fromID {
				dec = currency.Decimals(ac.Currency)
				break
			}
		}
		amtMinor, err := money.ParseMinor(strings.TrimSpace(amountStr), dec)
		if err != nil || amtMinor <= 0 {
			notifyErr(uistate.T("accounts.transferInvalidAmount"))
			return
		}
		var when time.Time
		if d, e := time.Parse("2006-01-02", strings.TrimSpace(dateStr)); e == nil {
			when = d
		}
		d := strings.TrimSpace(desc)
		if d == "" {
			d = uistate.T("accounts.transferDefaultDesc")
		}
		if _, _, err := app.CreateTransferPair(appstate.TransferParams{
			FromAccountID: fromID,
			ToAccountID:   toID,
			AmountMinor:   amtMinor,
			Date:          when,
			Desc:          d,
		}); err != nil {
			notifyErr(err.Error())
			return
		}
		bump()
		noticeAtom.Set(noticeAtom.Get().With(uistate.T("accounts.transferDone"), false))
	}

	windows := app.FreshnessWindows()
	now := time.Now()
	staleCount := 0
	for _, ac := range accounts {
		if freshness.IsStale(ac, windows, now) {
			staleCount++
		}
	}
	categories := app.Categories()
	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now),
			Members: app.Members(), Accounts: accounts, Categories: categories,
			OnDelete: deleteAccount, OnArchive: archiveAccount, OnRefresh: refreshAccount,
			OnSave: saveAccount, OnView: viewTransactions, OnSetBalance: setBalance,
			OnTransfer: doTransfer,
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	return Div(
		If(len(accounts) == 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("accounts.welcomeTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("accounts.welcomeDesc")),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(loadSample), uistate.T("accounts.loadSample")),
			),
		})),
		// Net-worth-dominant summary (G3 §2/§3): net worth is the household's
		// north-star figure, so it gets a larger hero tile spanning the full height
		// with a month-to-date trend subtitle; assets and liabilities sit beside it
		// as secondary tiles.
		Div(css.Class("nw-summary"),
			Div(css.Class("stat stat-hero"),
				Div(css.Class("stat-label"), uistate.T("dashboard.netWorth")),
				Div(ClassStr("stat-value "+accentFor(net)), fmtMoney(net)),
				netWorthDeltaLine(nwDelta, haveDelta),
			),
			stat(uistate.T("accounts.assets"), fmtMoney(assets), "pos"),
			stat(uistate.T("dashboard.liabilities"), fmtMoney(liabilities), "neg"),
		),
		If(len(nw.MissingCurrencies) > 0, P(css.Class("err"), Attr("role", "alert"),
			"Net worth excludes "+plural(len(nw.ExcludedAccounts), "account")+" — no exchange rate for "+strings.Join(nw.MissingCurrencies, ", ")+". Add it in Settings to include them.")),
		If(staleCount > 0, Div(Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(css.Class("btn btn-stale"), Type("button"), Title(uistate.T("accounts.markAllTitle")), OnClick(markAllUpdated),
				Text(uistate.T("accounts.markAll", plural(staleCount, "account")))),
		)),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("accounts.assets"),
			Body:  IfElse(len(assetList) == 0, P(css.Class("empty"), uistate.T("accounts.noAssets")), Div(css.Class("rows"), MapKeyed(assetList, keyOf, renderRow))),
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("dashboard.liabilities"),
			Body:  IfElse(len(liabList) == 0, P(css.Class("empty"), uistate.T("accounts.noLiabilities")), Div(css.Class("rows"), MapKeyed(liabList, keyOf, renderRow))),
		}),
		If(len(archivedList) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("accounts.archived"),
			Rows:  MapKeyed(archivedList, keyOf, renderRow),
		})),
	)
}

// labeledField wraps a form control in a <label> with persistent visible text, so
// the field stays self-describing after a placeholder would have vanished (C49).
// The wrapping <label> also associates the text with the control for a11y. Styled
// inline (stacked text-over-control) to avoid a stylesheet dependency.
func labeledField(label string, control ui.Node) ui.Node {
	return Label(css.Class("labeled-field"),
		Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
		Span(css.Class("t-caption", tw.TextDim), label),
		control,
	)
}

// ariaBool renders a Go bool as the "true"/"false" string an ARIA state attribute
// (e.g. aria-expanded) expects, keeping disclosure toggles screen-reader-correct.
func ariaBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// currencyOptions builds the account-currency picker's SelectOptions: every known
// registry currency, plus any code already in play (the base currency, the FX-table
// currencies, and the current selection) so an in-use code is never dropped. Each
// option reads "CODE — Name". A validated picker (vs the old free-text input) keeps
// typos from silently breaking FX.
func currencyOptions(app *appstate.App, selected string) []uiw.SelectOption {
	seen := map[string]bool{}
	var codes []string
	add := func(c string) {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		codes = append(codes, c)
	}
	for _, c := range currency.List() {
		add(c.Code)
	}
	add(app.Settings().BaseCurrency)
	for code := range app.Settings().FXRates {
		add(code)
	}
	add(selected)
	sort.Strings(codes)

	opts := make([]uiw.SelectOption, 0, len(codes))
	for _, c := range codes {
		label := c
		if cur, ok := currency.Lookup(c); ok {
			label = c + " — " + cur.Name
		}
		opts = append(opts, uiw.SelectOption{Value: c, Label: label})
	}
	return opts
}

// netWorthDeltaLine renders the month-to-date net-worth change as a small trend
// subtitle under the hero figure: a colored ↑/↓ glyph + the signed amount + "this
// month" (G3 §3). A zero or unknown delta reads as a calm "no change" caption.
func netWorthDeltaLine(delta money.Money, have bool) ui.Node {
	if !have || delta.Amount == 0 {
		return Span(css.Class("stat-sub", tw.TextDim), uistate.T("accounts.noChangeMonth"))
	}
	up := delta.Amount > 0
	tone, glyph := tw.TextUp, icon.TrendingUp
	if !up {
		tone, glyph = tw.TextDown, icon.TrendingDown
	}
	abs := delta
	if !up {
		abs = money.New(-delta.Amount, delta.Currency)
	}
	sign := "+"
	if !up {
		sign = "−"
	}
	return Span(css.Class("stat-sub", tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tone),
		uiw.Icon(glyph, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("accounts.deltaThisMonth", sign+fmtMoney(abs))))
}

// accountTypeIcon maps an account type to a small leading glyph so Checking /
// Investment / Credit Card are distinguishable at a glance without reading the
// meta-line (G3 §5). Unknown types fall back to the generic accounts glyph.
func accountTypeIcon(t domain.AccountType) icon.Name {
	switch t {
	case domain.TypeCreditCard, domain.TypeLineOfCredit:
		return icon.CreditCard
	case domain.TypeLoan, domain.TypePersonalLoan, domain.TypeMortgage:
		return icon.Landmark
	case domain.TypeInvestment:
		return icon.Reports
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings:
		return icon.Landmark
	default:
		return icon.Accounts
	}
}

// accountMeta builds an account row's subtitle: type · currency, plus credit
// utilization for liability accounts that have a credit limit.
func accountMeta(a domain.Account, bal money.Money) string {
	meta := humanizeType(string(a.Type)) + " · " + a.Currency
	if a.Class == domain.ClassLiability {
		if pct, ok := ledger.Utilization(bal.Amount, a.CreditLimit.Amount); ok {
			meta += fmt.Sprintf(" · %d%% of limit used", pct)
		}
	}
	return meta
}
