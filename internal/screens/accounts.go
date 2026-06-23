//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/textutil"
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
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
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
		If(len(accounts) == 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("accounts.welcomeTitle")),
			P(css.Class("muted"), uistate.T("accounts.welcomeDesc")),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(loadSample), uistate.T("accounts.loadSample")),
		)),
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
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("accounts.assets")),
			IfElse(len(assetList) == 0, P(css.Class("empty"), uistate.T("accounts.noAssets")), Div(css.Class("rows"), MapKeyed(assetList, keyOf, renderRow))),
		),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("dashboard.liabilities")),
			IfElse(len(liabList) == 0, P(css.Class("empty"), uistate.T("accounts.noLiabilities")), Div(css.Class("rows"), MapKeyed(liabList, keyOf, renderRow))),
		),
		If(len(archivedList) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("accounts.archived")),
			Div(css.Class("rows"), MapKeyed(archivedList, keyOf, renderRow)),
		)),
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

// currencyOptions builds the account-currency picker's <option>s: every known
// registry currency, plus any code already in play (the base currency, the FX-table
// currencies, and the current selection) so an in-use code is never dropped. Each
// option reads "CODE — Name"; the chosen code is marked selected. A validated
// picker (vs the old free-text input) keeps typos from silently breaking FX.
func currencyOptions(app *appstate.App, selected string) []ui.Node {
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

	opts := make([]ui.Node, 0, len(codes))
	for _, c := range codes {
		label := c
		if cur, ok := currency.Lookup(c); ok {
			label = c + " — " + cur.Name
		}
		opts = append(opts, Option(Value(c), SelectedIf(selected == c), label))
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

type accountRowProps struct {
	Account    domain.Account
	Balance    money.Money
	Cleared    money.Money
	Stale      bool
	Members    []domain.Member
	Accounts   []domain.Account // all non-archived accounts (for Transfer to-picker)
	Categories []domain.Category
	OnDelete   func(string)
	OnArchive  func(domain.Account)
	OnRefresh  func(domain.Account)
	OnSave     func(domain.Account)
	OnView     func(string)
	// OnSetBalance: newBalStr is the typed target; catID is the optional category
	// to attach to the adjustment transaction (empty = uncategorized).
	OnSetBalance func(ac domain.Account, current money.Money, newBalStr, catID string)
	OnTransfer   func(fromID, toID string, amountStr string, dateStr string, desc string)
}

// moneyMajorOrEmpty renders a money value as a major-unit string, or "" when zero.
func moneyMajorOrEmpty(m money.Money, dec int) string {
	if m.Amount == 0 {
		return ""
	}
	return money.FormatMinor(m.Amount, dec)
}

// floatOrEmpty renders a float as a plain string, or "" when zero.
func floatOrEmpty(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// intOrEmpty renders an int, or "" when zero.
func intOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// AccountRow is a per-account row component. It can be edited inline (name,
// opening balance, and the type-specific asset/liability attributes); it owns all
// its hooks so the list and the edit toggle never disturb hook ordering.
func AccountRow(props accountRowProps) ui.Node {
	a := props.Account
	dec := currency.Decimals(a.Currency)

	// Secondary actions live in a "⋯" overflow menu so each row stays uncluttered
	// (primary: Transactions / Edit / ✕); selecting one closes the menu (C9).
	menuOpen := ui.UseState(false)
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(a.ID) }))
	arch := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnArchive(a) }))
	refresh := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnRefresh(a) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(a.ID) }))
	settingBal := ui.UseState(false)
	setBalAmtS := ui.UseState("")
	setBalCatS := ui.UseState("") // optional category for the adjustment transaction
	setBal := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		setBalAmtS.Set("")
		setBalCatS.Set("")
		settingBal.Set(true)
	}))
	onSetBalAmt := ui.UseEvent(func(v string) { setBalAmtS.Set(v) })
	onSetBalCat := ui.UseEvent(func(e ui.Event) { setBalCatS.Set(e.GetValue()) })
	doSetBal := ui.UseEvent(Prevent(func() {
		if v := strings.TrimSpace(setBalAmtS.Get()); v != "" {
			props.OnSetBalance(a, props.Balance, v, setBalCatS.Get())
		}
		settingBal.Set(false)
	}))
	cancelSetBal := ui.UseEvent(Prevent(func() { settingBal.Set(false) }))

	// Transfer form state (L43): pre-populated with this account as the source.
	transferring := ui.UseState(false)
	xferToS := ui.UseState("")
	xferAmtS := ui.UseState("")
	xferDateS := ui.UseState(time.Now().Format("2006-01-02"))
	xferDescS := ui.UseState("")
	startTransfer := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		xferToS.Set("")
		xferAmtS.Set("")
		xferDateS.Set(time.Now().Format("2006-01-02"))
		xferDescS.Set("")
		transferring.Set(true)
	}))
	cancelTransfer := ui.UseEvent(Prevent(func() { transferring.Set(false) }))
	onXferTo := ui.UseEvent(func(e ui.Event) { xferToS.Set(e.GetValue()) })
	onXferAmt := ui.UseEvent(func(v string) { xferAmtS.Set(v) })
	onXferDate := ui.UseEvent(func(v string) { xferDateS.Set(v) })
	onXferDesc := ui.UseEvent(func(v string) { xferDescS.Set(v) })
	doTransfer := ui.UseEvent(Prevent(func() {
		if props.OnTransfer != nil {
			props.OnTransfer(a.ID, xferToS.Get(), xferAmtS.Get(), xferDateS.Get(), xferDescS.Get())
		}
		transferring.Set(false)
	}))

	// reconcile-to-statement mode (L30): user enters a statement balance; we show
	// per-account uncleared transactions they can mark cleared, recomputing the
	// cleared balance live against the target until the difference reaches zero.
	reconciling := ui.UseState(false)
	stmtBalS := ui.UseState("")
	onStmtBal := ui.UseEvent(func(v string) { stmtBalS.Set(v) })
	startReconcile := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		stmtBalS.Set("")
		reconciling.Set(true)
	}))
	cancelReconcile := ui.UseEvent(Prevent(func() { reconciling.Set(false) }))

	editing := ui.UseState(false)
	nameS := ui.UseState(a.Name)
	balS := ui.UseState(money.FormatMinor(a.OpeningBalance.Amount, dec))
	climS := ui.UseState(moneyMajorOrEmpty(a.CreditLimit, dec))
	aprS := ui.UseState(floatOrEmpty(a.InterestRateAPR))
	minpS := ui.UseState(moneyMajorOrEmpty(a.MinPayment, dec))
	dueS := ui.UseState(intOrEmpty(a.DueDayOfMonth))
	lenderS := ui.UseState(a.Lender)
	retS := ui.UseState(floatOrEmpty(a.ExpectedReturnAPR))
	liqS := ui.UseState(intOrEmpty(a.LiquidityScore))
	stabS := ui.UseState(intOrEmpty(a.StabilityScore))
	lockISO := ""
	if !a.LockUntil.IsZero() {
		lockISO = dateutil.FormatDate(a.LockUntil)
	}
	lockS := ui.UseState(lockISO)
	ownerS := ui.UseState(a.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onBal := ui.UseEvent(func(v string) { balS.Set(v) })
	onClim := ui.UseEvent(func(v string) { climS.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprS.Set(v) })
	onMinp := ui.UseEvent(func(v string) { minpS.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onLender := ui.UseEvent(func(v string) { lenderS.Set(v) })
	onRet := ui.UseEvent(func(v string) { retS.Set(v) })
	onLiq := ui.UseEvent(func(v string) { liqS.Set(v) })
	onStab := ui.UseEvent(func(v string) { stabS.Set(v) })
	onLock := ui.UseEvent(func(v string) { lockS.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(a.Name)
		balS.Set(money.FormatMinor(a.OpeningBalance.Amount, dec))
		climS.Set(moneyMajorOrEmpty(a.CreditLimit, dec))
		aprS.Set(floatOrEmpty(a.InterestRateAPR))
		minpS.Set(moneyMajorOrEmpty(a.MinPayment, dec))
		dueS.Set(intOrEmpty(a.DueDayOfMonth))
		lenderS.Set(a.Lender)
		retS.Set(floatOrEmpty(a.ExpectedReturnAPR))
		liqS.Set(intOrEmpty(a.LiquidityScore))
		stabS.Set(intOrEmpty(a.StabilityScore))
		lockS.Set(lockISO)
		ownerS.Set(a.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		cp := a
		cp.Name = strings.TrimSpace(nameS.Get())
		cp.OwnerID = ownerS.Get()
		if ownerS.Get() == domain.GroupOwnerID {
			cp.Scope = domain.ScopeShared
		} else {
			cp.Scope = domain.ScopeIndividual
		}
		if amt, err := money.ParseMinor(strings.TrimSpace(balS.Get()), dec); err == nil {
			cp.OpeningBalance = money.New(amt, a.Currency)
		}
		if a.Class == domain.ClassLiability {
			cp.CreditLimit = parseMoneyOrZero(climS.Get(), dec, a.Currency)
			cp.InterestRateAPR = textutil.ParseFloat(aprS.Get())
			cp.MinPayment = parseMoneyOrZero(minpS.Get(), dec, a.Currency)
			cp.DueDayOfMonth = textutil.ParseInt(dueS.Get())
			cp.Lender = strings.TrimSpace(lenderS.Get())
		} else {
			cp.ExpectedReturnAPR = textutil.ParseFloat(retS.Get())
			cp.LiquidityScore = textutil.ParseInt(liqS.Get())
			cp.StabilityScore = textutil.ParseInt(stabS.Get())
			if lu := strings.TrimSpace(lockS.Get()); lu != "" {
				if d, err := dateutil.ParseDate(lu); err == nil {
					cp.LockUntil = d
				}
			} else {
				cp.LockUntil = time.Time{}
			}
		}
		props.OnSave(cp)
		editing.Set(false)
	}))

	// Land the cursor in the first field when an inline editor opens (§6.7).
	ui.UseEffect(func() func() {
		switch {
		case settingBal.Get():
			focusByID("acct-setbal-" + a.ID)
		case reconciling.Get():
			focusByID("acct-reconcile-stmt-" + a.ID)
		case transferring.Get():
			focusByID("acct-xfer-amt-" + a.ID)
		case editing.Get():
			focusByID("acct-edit-" + a.ID)
		}
		return nil
	}, fmt.Sprintf("%t-%t-%t-%t", editing.Get(), settingBal.Get(), reconciling.Get(), transferring.Get()))

	if settingBal.Get() {
		// Delta preview: compute the live adjustment so the user sees what will be
		// posted before they confirm (L57/L30). Rendered only when the field has
		// a parseable value.
		dec := currency.Decimals(a.Currency)
		var deltaNode ui.Node = Fragment()
		if rawAmt := strings.TrimSpace(setBalAmtS.Get()); rawAmt != "" {
			if targetMinor, parseErr := money.ParseMinor(rawAmt, dec); parseErr == nil {
				dp := reconcile.PreviewDelta(props.Balance.Amount, targetMinor)
				sign := ""
				if dp.AdjustmentMinor > 0 {
					sign = "+"
				}
				adjLabel := sign + money.FormatMinor(dp.AdjustmentMinor, dec)
				if dp.NeedsAdjustment {
					deltaNode = P(css.Class("t-caption"),
						Attr("data-testid", "setbal-delta-preview"),
						uistate.T("accounts.setBalanceDeltaPreview",
							fmtMoney(money.New(props.Balance.Amount, a.Currency)),
							fmtMoney(money.New(targetMinor, a.Currency)),
							adjLabel),
					)
				} else {
					deltaNode = P(css.Class("t-caption"),
						Attr("data-testid", "setbal-delta-preview"),
						uistate.T("accounts.setBalanceNoAdjNeeded"))
				}
			}
		}

		// Category picker for the adjustment transaction (L57/L30).
		catOpts := []ui.Node{Option(Value(""), SelectedIf(setBalCatS.Get() == ""), uistate.T("accounts.setBalanceNoCategory"))}
		for _, c := range props.Categories {
			catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(setBalCatS.Get() == c.ID), c.Name))
		}

		return Div(css.Class("row-edit"),
			Form(css.Class("form-grid"),
				Attr("id", "acct-setbal-form-"+a.ID),
				Attr("aria-label", uistate.T("accounts.setBalanceFormLabel", a.Name)),
				OnSubmit(doSetBal),
				labeledField(uistate.T("accounts.setBalanceAmount"),
					Input(css.Class("field"), Attr("id", "acct-setbal-"+a.ID),
						Type("number"), Placeholder(uistate.T("accounts.setBalanceAmount")),
						Value(setBalAmtS.Get()), Step("0.01"), OnInput(onSetBalAmt))),
				deltaNode,
				labeledField(uistate.T("accounts.setBalanceCategoryLabel"),
					Select(css.Class("field"),
						Attr("aria-label", uistate.T("accounts.setBalanceCategoryLabel")),
						Attr("data-testid", "setbal-cat-select"),
						OnChange(onSetBalCat), catOpts)),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelSetBal), uistate.T("action.cancel")),
			),
		)
	}
	if reconciling.Get() {
		app := appstate.Default
		var allTxns []domain.Transaction
		if app != nil {
			allTxns = app.Transactions()
		}
		// Re-derive cleared balance live so it updates as the user marks transactions.
		clearedNow, _ := ledger.ClearedBalance(a, allTxns)

		// Parse whatever the user has typed into the statement-balance field.
		dec := currency.Decimals(a.Currency)
		stmtMinor, _ := money.ParseMinor(strings.TrimSpace(stmtBalS.Get()), dec)
		result := reconcile.Diff(clearedNow.Amount, stmtMinor)

		diffLabel := money.FormatMinor(result.DifferenceMinor, dec)
		if result.DifferenceMinor > 0 {
			diffLabel = "+" + diffLabel
		}

		// Collect uncleared transactions for this account so the user can mark them.
		var unclearedTxns []domain.Transaction
		for _, t := range allTxns {
			if t.AccountID == a.ID && !t.Cleared {
				unclearedTxns = append(unclearedTxns, t)
			}
		}

		// onToggleClear is passed as a plain func to each ReconcileTxnRow — the row
		// component owns the On* hook; we never call On* inside the loop below.
		onToggleClear := func(t domain.Transaction) {
			if app == nil {
				return
			}
			t.Cleared = !t.Cleared
			_ = app.PutTransaction(t)
			// Trigger a re-render by bumping rev via the parent's OnSetBalance
			// callback, which already calls bump(). Since we can't access bump()
			// directly here, we re-use the OnRefresh callback (a no-op balance-as-of
			// touch) to propagate the render cycle.
			props.OnRefresh(a)
		}

		keyOfTxn := func(t domain.Transaction) any { return t.ID }
		renderTxnRow := func(t domain.Transaction) ui.Node {
			return ui.CreateElement(ReconcileTxnRow, reconcileTxnRowProps{
				Txn:      t,
				Currency: a.Currency,
				OnToggle: onToggleClear,
			})
		}

		return Div(Attr("data-testid", "reconcile-statement-mode"),
			H3(Style(map[string]string{"margin": "0.5rem 0 0.25rem"}), "Reconcile to statement — ", a.Name),
			Div(css.Class("form-grid"),
				labeledField("Statement balance",
					Input(css.Class("field"), Attr("id", "acct-reconcile-stmt-"+a.ID),
						Attr("data-testid", "reconcile-statement-input"),
						Type("number"), Step("0.01"),
						Placeholder("Enter statement balance"),
						Value(stmtBalS.Get()), OnInput(onStmtBal))),
			),
			Div(Style(map[string]string{"margin": "0.5rem 0"}),
				Span(Style(map[string]string{"margin-right": "1rem"}),
					"Cleared balance: ", fmtMoney(clearedNow)),
				Span(Attr("data-testid", "reconcile-difference"),
					"Difference: ", diffLabel),
				If(result.Reconciled, Span(Style(map[string]string{"margin-left": "1rem", "color": "var(--cf-pos)", "font-weight": "bold"}),
					Attr("data-testid", "reconcile-confirmed"), "Reconciled ✓")),
			),
			If(result.Reconciled,
				Button(css.Class("btn btn-primary"), Type("button"),
					Attr("data-testid", "reconcile-done"),
					OnClick(cancelReconcile), "Done")),
			If(len(unclearedTxns) > 0,
				Div(Style(map[string]string{"margin-top": "0.75rem"}),
					P(css.Class("t-caption"), "Uncleared transactions — mark cleared to reconcile:"),
					Div(css.Class("rows"), MapKeyed(unclearedTxns, keyOfTxn, renderTxnRow)),
				)),
			If(len(unclearedTxns) == 0 && !result.Reconciled,
				P(css.Class("muted"), "No uncleared transactions. Adjust the statement balance to match the cleared balance above.")),
			Button(css.Class("btn"), Type("button"), OnClick(cancelReconcile), uistate.T("action.cancel")),
		)
	}
	if transferring.Get() {
		// Build a "To account" option list: every non-archived account except this one.
		toOpts := []ui.Node{Option(Value(""), SelectedIf(xferToS.Get() == ""), uistate.T("accounts.transferToPlaceholder"))}
		for _, ac := range props.Accounts {
			if ac.ID == a.ID || ac.Archived {
				continue
			}
			label := ac.Name + " (" + ac.Currency + ")"
			toOpts = append(toOpts, Option(Value(ac.ID), SelectedIf(xferToS.Get() == ac.ID), label))
		}
		return Div(css.Class("row-edit"),
			H3(Style(map[string]string{"margin": "0.5rem 0 0.25rem"}),
				uistate.T("accounts.transferTitle", a.Name)),
			Form(css.Class("form-grid"),
				Attr("id", "acct-transfer-form-"+a.ID),
				Attr("aria-label", uistate.T("accounts.transferFormLabel", a.Name)),
				OnSubmit(doTransfer),
				labeledField(uistate.T("accounts.transferAmount"),
					Input(css.Class("field"), Attr("id", "acct-xfer-amt-"+a.ID),
						Type("number"), Placeholder(uistate.T("accounts.transferAmount")),
						Value(xferAmtS.Get()), Step("0.01"), Attr("min", "0.01"),
						OnInput(onXferAmt))),
				labeledField(uistate.T("accounts.transferToLabel"),
					Select(css.Class("field"),
						Attr("aria-label", uistate.T("accounts.transferToLabel")),
						Attr("data-testid", "acct-xfer-to-select"),
						OnChange(onXferTo), toOpts)),
				labeledField(uistate.T("accounts.transferDateLabel"),
					Input(css.Class("field"), Type("date"),
						Attr("aria-label", uistate.T("accounts.transferDateLabel")),
						Value(xferDateS.Get()), OnInput(onXferDate))),
				labeledField(uistate.T("accounts.transferDescLabel"),
					Input(css.Class("field"), Type("text"),
						Placeholder(uistate.T("accounts.transferDefaultDesc")),
						Value(xferDescS.Get()), OnInput(onXferDesc))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelTransfer), uistate.T("action.cancel")),
			),
		)
	}

	if editing.Get() {
		isLiab := a.Class == domain.ClassLiability
		return Div(css.Class("row-edit"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("common.name"),
					Input(css.Class("field"), Attr("id", "acct-edit-"+a.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("common.owner"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("common.owner")), Title(uistate.T("common.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get()))),
				labeledField(uistate.T("accounts.openingBalance"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(balS.Get()), Step("0.01"), OnInput(onBal))),
				If(isLiab, labeledField(uistate.T("accounts.creditLimit"),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Value(climS.Get()), Step("0.01"), OnInput(onClim)))),
				If(isLiab, labeledField(uistate.T("accounts.apr"),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Value(aprS.Get()), Step("0.01"), OnInput(onApr)))),
				If(isLiab, labeledField(uistate.T("accounts.minPayment"),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Value(minpS.Get()), Step("0.01"), OnInput(onMinp)))),
				If(isLiab, labeledField(uistate.T("accounts.dueDay"),
					Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay")), Value(dueS.Get()), OnInput(onDue)))),
				If(isLiab, labeledField(uistate.T("accounts.lender"),
					Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lenderS.Get()), OnInput(onLender)))),
				If(!isLiab, labeledField(uistate.T("accounts.expReturn"),
					Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(retS.Get()), Step("0.01"), OnInput(onRet)))),
				If(!isLiab, labeledField(uistate.T("accounts.liquidity"),
					Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liqS.Get()), OnInput(onLiq)))),
				If(!isLiab, labeledField(uistate.T("accounts.stability"),
					Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stabS.Get()), OnInput(onStab)))),
				If(!isLiab, labeledField(uistate.T("accounts.lockUntilEdit"),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntilEdit")), Title(uistate.T("accounts.lockUntilEdit")), Value(lockS.Get()), OnInput(onLock)))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	archLabel, archTitle := uistate.T("accounts.archive"), uistate.T("accounts.archiveTitle")
	if a.Archived {
		archLabel, archTitle = uistate.T("accounts.restore"), uistate.T("accounts.restoreTitle")
	}
	meta := accountMeta(a, props.Balance)
	if props.Cleared.Amount != props.Balance.Amount {
		meta += uistate.T("accounts.clearedSuffix", fmtMoney(props.Cleared))
	}
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	return Div(css.Class("row"),
		// Account-type glyph (G3 §5): a quick visual tag so Checking / Investment /
		// Credit Card are distinguishable without reading the meta-line.
		Span(css.Class("acct-type-icon", tw.TextDim), Attr("aria-hidden", "true"),
			uiw.Icon(accountTypeIcon(a.Type), css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), a.Name,
				If(props.Stale, Span(css.Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), uistate.T("accounts.stale"))),
			),
			Span(css.Class("row-meta"), meta),
		),
		Span(ClassStr(amountClass(props.Balance)), fmtMoney(props.Balance)),
		// Stale accounts get the reconcile action surfaced inline (G3 §6) rather than
		// buried in the ⋯ menu, since "update my balance" is the whole reason a stale
		// account is flagged.
		If(props.Stale && !a.Archived, Button(css.Class("btn btn-stale", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("accounts.updateBalance")), OnClick(setBal), uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("accounts.updateBalance")))),
		// Primary actions inline; everything else in the ⋯ menu.
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("accounts.viewTitle")), OnClick(view), uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions"))),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("accounts.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Div(css.Class("add-wrap"),
			Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("accounts.moreActions")), Attr("aria-label", uistate.T("accounts.moreActions")), Attr("aria-haspopup", "menu"), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
			Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
			Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
				If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(setBal), uistate.T("accounts.updateBalance"))),
				If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "reconcile-start-btn-"+a.ID), OnClick(startReconcile), "Reconcile to statement")),
				If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "transfer-start-btn-"+a.ID), OnClick(startTransfer),
					uistate.T("accounts.transferAction"))),
				If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(refresh), uistate.T("accounts.markUpdated"))),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("title", archTitle), OnClick(arch), archLabel),
			),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("accounts.deleteTitle")), Title(uistate.T("accounts.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// reconcileTxnRowProps holds the data and callback for a single uncleared
// transaction row inside the reconcile-to-statement panel.
type reconcileTxnRowProps struct {
	Txn      domain.Transaction
	Currency string
	// OnToggle is a plain func — never an On* hook — so the parent can pass it
	// into MapKeyed without violating the no-On*-in-loop rule (CLAUDE.md §gotchas).
	OnToggle func(domain.Transaction)
}

// ReconcileTxnRow renders a single uncleared transaction for the reconcile
// panel. It owns its own OnClick hook (satisfying the per-row component rule),
// and exposes a "Mark cleared" button whose label doubles as a status badge.
func ReconcileTxnRow(props reconcileTxnRowProps) ui.Node {
	t := props.Txn
	dec := currency.Decimals(props.Currency)
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(t) }))
	return Div(css.Class("row"), Attr("data-testid", "reconcile-txn-row"), Attr("data-id", t.ID),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), t.Desc),
			Span(css.Class("row-meta"), t.Date.Format("2006-01-02")),
		),
		Span(ClassStr("fig "+amountClass(t.Amount)), money.FormatMinor(t.Amount.Amount, dec)),
		Button(css.Class("btn"), Type("button"),
			Attr("data-testid", "reconcile-txn-clear-btn"),
			Title("Mark this transaction cleared"),
			OnClick(toggle), "Mark cleared"),
	)
}

// parseMoneyOrZero parses a major-unit amount to money, returning zero on error.
func parseMoneyOrZero(s string, dec int, cur string) money.Money {
	if amt, err := money.ParseMinor(strings.TrimSpace(s), dec); err == nil {
		return money.New(amt, cur)
	}
	return money.Money{Currency: cur}
}
