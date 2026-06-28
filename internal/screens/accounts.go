// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
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
		// Restore focus to the next account row after the re-render (§6.7).
		restoreFocus := captureRowFocus(".rows", ".row")
		if err := app.DeleteAccount(accountID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
		restoreFocus()
		auditview.CaptureNow()
		uistate.PostUndoable(uistate.T("toast.accountDeleted"))
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
		uistate.SetSampleActive(true)
		uistate.RequestPersist() // C2: flush before a fast reload can race the ticker
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

	// C84: settings atom + handler so the "Manage exchange rates" affordance can
	// open the Settings panel directly from /accounts, without a separate route.
	settingsAtom := uistate.UseSettings()
	openFXSettings := ui.UseEvent(Prevent(func() { settingsAtom.Set(uistate.Global()) }))

	// C67: page-level transfer form state hooks — declared here (stable position,
	// not in a loop) so hook ordering is consistent across renders. The submit
	// handler captures doTransferFn, a pointer resolved after doTransfer is defined.
	pageXferOpen := ui.UseState(false)
	pageXferFromS := ui.UseState("")
	pageXferToS := ui.UseState("")
	pageXferAmtS := ui.UseState("")
	pageXferDateS := ui.UseState(time.Now().Format("2006-01-02"))
	pageXferDescS := ui.UseState("")
	openPageXfer := ui.UseEvent(Prevent(func() {
		pageXferFromS.Set("")
		pageXferToS.Set("")
		pageXferAmtS.Set("")
		pageXferDateS.Set(time.Now().Format("2006-01-02"))
		pageXferDescS.Set("")
		pageXferOpen.Set(true)
	}))
	cancelPageXfer := ui.UseEvent(Prevent(func() { pageXferOpen.Set(false) }))
	// onPageXferFrom / onPageXferTo: SelectInput OnChange receives a plain string;
	// plain funcs (not On* hooks) are safe to use as SelectInput.OnChange callbacks.
	onPageXferFrom := func(v string) { pageXferFromS.Set(v) }
	onPageXferTo := func(v string) { pageXferToS.Set(v) }
	onPageXferAmt := ui.UseEvent(func(v string) { pageXferAmtS.Set(v) })
	onPageXferDate := ui.UseEvent(func(v string) { pageXferDateS.Set(v) })
	onPageXferDesc := ui.UseEvent(func(v string) { pageXferDescS.Set(v) })
	// doTransferFn is a pointer so submitPageXfer can call doTransfer after it is
	// defined below (Go does not allow forward references in closures).
	var doTransferFn func(fromID, toID, amountStr, dateStr, desc string)
	submitPageXfer := ui.UseEvent(Prevent(func() {
		from, to := pageXferFromS.Get(), pageXferToS.Get()
		if from == "" || to == "" || from == to || doTransferFn == nil {
			return
		}
		doTransferFn(from, to, pageXferAmtS.Get(), pageXferDateS.Get(), pageXferDescS.Get())
		pageXferOpen.Set(false)
	}))

	// C278: scope the displayed list to the active member when one is selected.
	// The atom is read at a stable top-level hook position; filtering is plain code.
	activeMemberID := uistate.UseActiveMember().Get()

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
		if !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
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
	// C67: wire the page-level submit hook to the now-defined doTransfer closure.
	doTransferFn = doTransfer

	windows := app.FreshnessWindows()
	now := time.Now()
	staleCount := 0
	for _, ac := range accounts {
		if freshness.IsStale(ac, windows, now) {
			staleCount++
		}
	}
	categories := app.Categories()

	// Compute page-level smart insights once (not per row) so each AccountRow can
	// cheaply call smartBadgeFor with its own ID. Re-renders when data/settings
	// change via the existing rev atom above.
	_ = uistate.UseDataRevision().Get() // smart re-render hook (idempotent with rev above)
	pr := uistate.UsePrefs().Get()
	smartSettings := uistate.LoadSmartSettings()
	smartIn := buildSmartInput(app, pr.WeekStartWeekday())
	accountInsights := smartengine.RunPage(smartIn, smartSettings, smart.PageAccounts)
	accountByEntity := insightsByEntity(accountInsights)

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now),
			Members: app.Members(), Accounts: accounts, Categories: categories,
			OnDelete: deleteAccount, OnArchive: archiveAccount, OnRefresh: refreshAccount,
			OnSave: saveAccount, OnView: viewTransactions, OnSetBalance: setBalance,
			OnTransfer:       doTransfer,
			SmartSettings:    smartSettings,
			SmartByEntity:    accountByEntity,
			ValuationHistory: app.BalanceHistory(ac.ID),
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
				// Net worth label carries a smart explainer tooltip (key-figure placement).
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("dashboard.netWorth"),
					smartTooltipFor(smartSettings, "accounts-net", uistate.T("dashboard.netWorth"), uistate.T("smart.tipAccountsNet")),
				),
				Div(ClassStr("stat-value "+accentFor(net)), fmtMoney(net)),
				netWorthDeltaLine(nwDelta, haveDelta),
			),
			stat(uistate.T("accounts.assets"), fmtMoney(assets), "pos"),
			stat(uistate.T("dashboard.liabilities"), fmtMoney(liabilities), "neg"),
		),
		If(len(nw.MissingCurrencies) > 0, P(css.Class("err"), Attr("role", "alert"),
			"Net worth excludes "+plural(len(nw.ExcludedAccounts), "account")+" — no exchange rate for "+strings.Join(nw.MissingCurrencies, ", ")+". Add it in Settings to include them.")),
		// C84: FX rate discoverability — show a muted button whenever there are
		// foreign-currency accounts (MissingCurrencies > 0) OR existing rates to
		// review/edit (FXRates > 0). Opens Settings directly so users don't have
		// to hunt for the exchange-rate table.
		If(len(nw.MissingCurrencies) > 0 || len(app.Settings().FXRates) > 0,
			Button(css.Class("btn btn-ghost"), Type("button"),
				Title(uistate.T("accounts.manageFXRatesTitle")),
				OnClick(openFXSettings),
				uistate.T("accounts.manageFXRates"),
			),
		),
		If(staleCount > 0, Div(Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(css.Class("btn btn-stale"), Type("button"), Title(uistate.T("accounts.markAllTitle")), OnClick(markAllUpdated),
				Text(uistate.T("accounts.markAll", plural(staleCount, "account")))),
		)),
		// C67: top-level "Transfer money" action — visible whenever there are at least
		// two non-archived accounts. Clicking opens an inline form that reuses the
		// existing doTransfer / CreateTransferPair flow without duplicating any logic.
		If(len(accounts) >= 2 && !pageXferOpen.Get(),
			Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
				Type("button"),
				Attr("data-testid", "page-transfer-btn"),
				Title(uistate.T("accounts.transferTitle")),
				OnClick(openPageXfer),
				uiw.Icon(icon.Accounts, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("accounts.transferMoney")),
			),
		),
		If(pageXferOpen.Get(), func() ui.Node {
			// Build From/To option lists from all non-archived accounts, excluding the
			// account already selected in the other field so the user can't pick the same
			// account on both sides.
			pfrom, pto := pageXferFromS.Get(), pageXferToS.Get()
			fromOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
			toOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
			for _, ac := range accounts {
				if ac.Archived {
					continue
				}
				lbl := ac.Name + " (" + ac.Currency + ")"
				if ac.ID != pto {
					fromOpts = append(fromOpts, uiw.SelectOption{Value: ac.ID, Label: lbl})
				}
				if ac.ID != pfrom {
					toOpts = append(toOpts, uiw.SelectOption{Value: ac.ID, Label: lbl})
				}
			}
			sameAcct := pfrom != "" && pto != "" && pfrom == pto
			submitDisabled := sameAcct || pfrom == "" || pto == ""
			return Div(css.Class("row-edit"),
				Attr("data-testid", "page-transfer-form"),
				H3(Style(map[string]string{"margin": "0.5rem 0 0.25rem"}),
					uistate.T("accounts.transferTitle")),
				Form(css.Class("form-grid"),
					Attr("aria-label", uistate.T("accounts.transferFormLabel")),
					OnSubmit(submitPageXfer),
					labeledField(uistate.T("accounts.transferFromLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   fromOpts,
							Selected:  pfrom,
							OnChange:  onPageXferFrom,
							AriaLabel: uistate.T("accounts.transferFromLabel"),
							TestID:    "page-xfer-from-select",
						})),
					labeledField(uistate.T("accounts.transferToLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   toOpts,
							Selected:  pto,
							OnChange:  onPageXferTo,
							AriaLabel: uistate.T("accounts.transferToLabel"),
							TestID:    "page-xfer-to-select",
						})),
					If(sameAcct, P(css.Class("err"), Attr("role", "alert"),
						uistate.T("accounts.transferSameAccountErr"))),
					labeledField(uistate.T("accounts.transferAmount"),
						Input(css.Class("field"), Attr("id", "page-xfer-amt"),
							Attr("data-testid", "page-xfer-amt"),
							Type("number"), Placeholder(uistate.T("accounts.transferAmount")),
							Value(pageXferAmtS.Get()), Step("0.01"), Attr("min", "0.01"),
							OnInput(onPageXferAmt))),
					labeledField(uistate.T("accounts.transferDateLabel"),
						Input(css.Class("field"), Type("date"),
							Attr("aria-label", uistate.T("accounts.transferDateLabel")),
							Value(pageXferDateS.Get()), OnInput(onPageXferDate))),
					labeledField(uistate.T("accounts.transferDescLabel"),
						Input(css.Class("field"), Type("text"),
							Placeholder(uistate.T("accounts.transferDefaultDesc")),
							Value(pageXferDescS.Get()), OnInput(onPageXferDesc))),
					IfElse(submitDisabled,
						Button(css.Class("btn btn-primary"), Type("submit"),
							Attr("disabled", "disabled"),
							uistate.T("accounts.transferSubmit")),
						Button(css.Class("btn btn-primary"), Type("submit"),
							uistate.T("accounts.transferSubmit"))),
					Button(css.Class("btn"), Type("button"), OnClick(cancelPageXfer), uistate.T("action.cancel")),
				),
			)
		}()),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title:        uistate.T("accounts.assets"),
			HeaderAction: smartSectionAction(smartSettings),
			// Empty assets gets the first-run icon + "Add your first account" CTA (opens the add-account
			// modal), matching every other entity screen — previously a bare "No asset accounts yet." line
			// with no glyph and no way to act from the page. The liabilities card stays a plain celebratory
			// line below (no CTA — never nudge a new user to add debt).
			Body:         IfElse(len(assetList) == 0, ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("accounts.noAssets"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account", Icon: icon.Accounts, ImportLink: true}), Div(css.Class("rows"), MapKeyed(assetList, keyOf, renderRow))),
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
	case domain.TypeRetirement:
		return icon.TrendingUp
	case domain.TypeCrypto:
		return icon.Scale
	case domain.TypeProperty:
		return icon.Box // closest available glyph for a real-estate/building asset (C224)
	case domain.TypeVehicle:
		return icon.Calculator // closest available glyph for a vehicle asset (C224)
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
