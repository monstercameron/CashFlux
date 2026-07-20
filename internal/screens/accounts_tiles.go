// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/acctseries"
	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/idlecash"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// accounts_tiles.go holds the Native widget bodies the /accounts surface host
// composes (see accounts_widget.go). Each is a self-contained engine tile: it reads
// the live store (props.App) and the shared accounts-page atoms (filter / transfer
// sub-view), never surface-local closures, so the host can place each tile through
// the same spec/render pipeline the dashboard and /transactions use. The rich
// per-account row (AccountRow) is reused verbatim — these tiles only restructure the
// page around it.
//
// AC7 sweep rules: the config manager lives in the "acct-sweep-btn" toolbar modal
// (SweepRulesHost → SweepRulesForm, accounts_sweep_config.go), and the
// preview-approve proposal card renders above the bento in accounts_widget.go
// (accountsSweepCards, accounts_sweep_card.go), mirroring the GL1 waterfall card.

// --- shared tile props ----------------------------------------------------------

type acctSummaryProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctToolbarProps struct{ App *appstate.App }

// AccountPageTransferProps drives the page-level transfer form rendered inside the
// shell-root flip modal (see internal/app AccountTransferHost). OnDone closes it.
type AccountPageTransferProps struct {
	App    *appstate.App
	OnDone func()
}

type acctListProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctArchivedProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctWelcomeProps struct{ App *appstate.App }

type acctFormulaProps struct{ App *appstate.App }

// --- shared helpers -------------------------------------------------------------

// acctActiveMemberID resolves the legacy single-owner filter from the top-bar active
// scope: when exactly one owner is scoped, return its id (rows are filtered to that
// member); otherwise "" (all members). It calls the UseActiveScope hook, so invoke it
// at a stable position near the top of a tile body.
func acctActiveMemberID() string {
	if s := uistate.UseActiveScope().Get(); len(s.Owners) == 1 {
		return s.Owners[0]
	}
	return ""
}

// partitionAssetAccounts splits the account set into the visible asset rows and the
// archived asset rows, applying owner scoping and excluding liabilities (which live
// on /debt). Assets are sorted by base-converted balance, largest first, so the
// accounts that move net worth most sit at the top. Pure (no hooks) so any tile can
// call it after fetching its data.
func partitionAssetAccounts(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, base, activeMemberID string) (assets, archived []domain.Account) {
	for _, ac := range accounts {
		if !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		if ac.Class == domain.ClassLiability {
			continue
		}
		if ac.Archived {
			archived = append(archived, ac)
		} else {
			assets = append(assets, ac)
		}
	}
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		if c, err := rates.Convert(bal, base); err == nil {
			return c.Amount
		}
		return bal.Amount
	}
	sort.SliceStable(assets, func(i, j int) bool { return convBal(assets[i]) > convBal(assets[j]) })
	return assets, archived
}

// acctRowCallbacks bundles the per-row mutation handlers the asset/archived list
// tiles hand to each AccountRow. They are plain funcs (no hooks) that mutate the
// store and bump the shared data revision so the whole surface re-renders in step;
// OnView is supplied separately by the tile (it closes over the router + tx filter).
type acctRowCallbacks struct {
	OnDelete     func(string)
	OnArchive    func(domain.Account)
	OnRefresh    func(domain.Account)
	OnSave       func(domain.Account)
	OnSetBalance func(ac domain.Account, current money.Money, newBalStr, catID string)
	OnTransfer   func(fromID, toID, amountStr, dateStr, desc string)
}

// buildAcctRowCallbacks wires the store mutations for AccountRow. Errors surface
// through the global notice/toast (PostNotice) rather than a screen-local error
// state, matching the rest of the widgetized surfaces.
func buildAcctRowCallbacks(app *appstate.App) acctRowCallbacks {
	return acctRowCallbacks{
		OnDelete: func(accountID string) {
			// Refuse to delete an account that still has transactions (including the far
			// leg of a transfer): deleting it would orphan them. Archive keeps history.
			for _, t := range app.Transactions() {
				if t.AccountID == accountID || t.TransferAccountID == accountID {
					uistate.PostNotice(uistate.T("accounts.deleteHasTxns"), true)
					return
				}
			}
			name := ""
			for _, ac := range app.Accounts() {
				if ac.ID == accountID {
					name = ac.Name
					break
				}
			}
			// C8: confirm THEN undo — the same destructive-action contract as goals and
			// budgets, so delete behaves identically across sibling pages.
			uistate.ConfirmModal(uistate.T("accounts.deleteConfirm", name), true, func(ok bool) {
				if !ok {
					return
				}
				restoreFocus := captureRowFocus(".rows", ".row")
				if err := app.DeleteAccount(accountID); err != nil {
					uistate.PostNotice(err.Error(), true)
					return
				}
				uistate.BumpDataRevision()
				restoreFocus()
				auditview.CaptureNow()
				uistate.PostUndoable(uistate.T("toast.accountDeleted"))
			})
		},
		OnArchive: func(ac domain.Account) {
			ac.Archived = !ac.Archived
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			// G4: archiving used to be silent — the row just vanished. Name what
			// happened and offer the one-click way back.
			if ac.Archived {
				uistate.PostUndoable(uistate.T("accounts.archivedToast", ac.Name))
			} else {
				uistate.PostUndoable(uistate.T("accounts.restoredToast", ac.Name))
			}
		},
		OnRefresh: func(ac domain.Account) {
			ac.BalanceAsOf = time.Now()
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("accounts.markUpdatedToast", ac.Name), false)
		},
		OnSave: func(ac domain.Account) {
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
		},
		OnSetBalance: func(ac domain.Account, currentBal money.Money, newStr, catID string) {
			dec := currency.Decimals(ac.Currency)
			target, err := money.ParseMinor(strings.TrimSpace(newStr), dec)
			if err != nil {
				uistate.PostNotice(uistate.T("accounts.invalidBalance"), true)
				return
			}
			// Post an adjustment transaction for the difference so the computed balance
			// equals the figure entered (e.g. matching a statement); the optional catID
			// attaches a category so it doesn't land as an uncategorized spike (L57/L30).
			if amount, ok := ledger.AdjustmentToTarget(currentBal, target); ok {
				adj := domain.Transaction{
					ID: id.New(), AccountID: ac.ID, Date: time.Now(), Desc: uistate.T("accounts.balanceAdjustment"),
					Amount: amount, Cleared: true, CategoryID: catID, Source: domain.TxnSourceManual,
				}
				if err := app.PutTransaction(adj); err != nil {
					uistate.PostNotice(err.Error(), true)
					return
				}
			}
			ac.BalanceAsOf = time.Now()
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			// G3: a balance set posts a REAL adjustment transaction — one-click reversible.
			uistate.PostUndoable(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(target, ac.Currency))))
		},
		OnTransfer: func(fromID, toID, amountStr, dateStr, desc string) {
			doAccountTransfer(app, fromID, toID, amountStr, "", "", dateStr, desc)
		},
	}
}

// acctTransferOptions builds the From/To pickers for BOTH transfer forms with one
// eligibility rule (C2): money moves FROM liquid cash only (a 401(k) or the condo
// can't be a source of a spending transfer), and it lands anywhere except a
// valuation-only holding (property/vehicle) — a transfer TO a liability is a payment
// and says so on the option label. Each side still excludes the other's selection.
func acctTransferOptions(accounts []domain.Account, fromID, toID string) (fromOpts, toOpts []uiw.SelectOption) {
	fromOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
	toOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
	for _, ac := range accounts {
		if ac.Archived {
			continue
		}
		// Annotate the currency — unless the account NAME already carries it (e.g. the
		// sample "Travel Card (EUR)"), which would read as "Travel Card (EUR) (EUR)".
		lbl := ac.Name
		if !strings.Contains(strings.ToUpper(ac.Name), "("+strings.ToUpper(ac.Currency)+")") {
			lbl += " (" + ac.Currency + ")"
		}
		if ac.ID != toID && earmarkEligibleType(ac.Type) {
			fromOpts = append(fromOpts, uiw.SelectOption{Value: ac.ID, Label: lbl})
		}
		if ac.ID != fromID && ac.Type != domain.TypeProperty && ac.Type != domain.TypeVehicle {
			toLbl := lbl
			if ac.Class == domain.ClassLiability {
				toLbl += uistate.T("accounts.transferPaymentSuffix")
			}
			toOpts = append(toOpts, uiw.SelectOption{Value: ac.ID, Label: toLbl})
		}
	}
	return fromOpts, toOpts
}

// acctTransferFXNote surfaces the cross-currency semantics BEFORE the transfer is
// posted (G7): the amount is denominated in the source currency, and either lands
// converted at the saved rate (with a live ≈ preview of the typed amount) or — when
// no rate exists — a warning that it would land unconverted, pointing at Manage
// exchange rates. Renders nothing for same-currency pairs or incomplete picks.
func acctTransferFXNote(app *appstate.App, fromID, toID, amountStr string) ui.Node {
	if app == nil || fromID == "" || toID == "" {
		return Fragment()
	}
	var fromAc, toAc domain.Account
	for _, ac := range app.Accounts() {
		if ac.ID == fromID {
			fromAc = ac
		}
		if ac.ID == toID {
			toAc = ac
		}
	}
	if fromAc.ID == "" || toAc.ID == "" || fromAc.Currency == toAc.Currency {
		return Fragment()
	}
	s := app.Settings()
	rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
	dec := currency.Decimals(fromAc.Currency)
	amtMinor, perr := money.ParseMinor(strings.TrimSpace(amountStr), dec)
	if perr != nil || amtMinor <= 0 {
		amtMinor = int64(1)
		for i := 0; i < dec; i++ {
			amtMinor *= 10
		} // preview 1.00 until an amount is typed
	}
	conv, cerr := rates.Convert(money.New(amtMinor, fromAc.Currency), toAc.Currency)
	if cerr != nil {
		return P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "xfer-fx-norate"),
			uistate.T("accounts.fxNoteNoRate", fromAc.Currency, toAc.Currency))
	}
	return P(css.Class("budget-sub"), Attr("data-testid", "xfer-fx-note"),
		Style(map[string]string{"margin": "0"}),
		uistate.T("accounts.fxNoteAmountIn", fromAc.Currency, toAc.Currency, fmtMoney(conv)))
}

// acctTransferBalancePreview shows what the transfer would do to both booked
// balances before it posts — "Checking: $1,000.00 → $950.00" for each side —
// via appstate.PreviewTransferPair, which shares the exact leg math (FX
// conversion + liability payment sign) with the real post. Renders nothing
// until both accounts are picked and a valid positive amount is typed.
// receivedStr/feeStr are the optional landed-amount override (destination
// currency) and fee (source currency); pass "" for none — invalid text is
// treated as absent here (the form gates submit on it separately).
func acctTransferBalancePreview(app *appstate.App, fromID, toID, amountStr, receivedStr, feeStr string) ui.Node {
	if app == nil || fromID == "" || toID == "" || fromID == toID {
		return Fragment()
	}
	fromCcy := acctCurrencyOf(app, fromID)
	amtMinor, perr := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(fromCcy))
	if perr != nil || amtMinor <= 0 {
		return Fragment()
	}
	recvMinor, _ := acctParseOptionalMinor(receivedStr, acctCurrencyOf(app, toID))
	feeMinor, _ := acctParseOptionalMinor(feeStr, fromCcy)
	pv, err := app.PreviewTransferPair(appstate.TransferParams{
		FromAccountID: fromID, ToAccountID: toID, AmountMinor: amtMinor,
		ReceivedMinor: recvMinor, FeeMinor: feeMinor,
	})
	if err != nil {
		return Fragment()
	}
	var fromName, toName string
	for _, ac := range app.Accounts() {
		switch ac.ID {
		case fromID:
			fromName = ac.Name
		case toID:
			toName = ac.Name
		}
	}
	// #63: state the transfer's SEMANTICS in one plain sentence — what each side
	// does and that neither leg is spending. A liability destination reads as
	// paying down what's owed, not as an increase.
	amtStr := money.FormatMinor(amtMinor, currency.Decimals(fromCcy))
	semantics := uistate.T("accounts.xferSemanticsAsset", fromName, amtStr, toName)
	for _, ac := range app.Accounts() {
		if ac.ID == toID && ac.Class == domain.ClassLiability {
			semantics = uistate.T("accounts.xferSemanticsLiability", fromName, amtStr, toName)
			break
		}
	}
	return Div(css.Class("budget-sub"), Attr("data-testid", "xfer-balance-preview"),
		Style(map[string]string{"margin": "0", "display": "grid", "gap": "0.15rem"}),
		Span(uistate.T("accounts.xferPreviewTitle")),
		Span(Attr("data-testid", "xfer-preview-from"),
			uistate.T("accounts.xferPreviewLine", fromName, fmtMoney(pv.FromBefore), fmtMoney(pv.FromAfter))),
		Span(Attr("data-testid", "xfer-preview-to"),
			uistate.T("accounts.xferPreviewLine", toName, fmtMoney(pv.ToBefore), fmtMoney(pv.ToAfter))),
		Span(css.Class(tw.TextDim), Attr("data-testid", "xfer-semantics"), semantics),
	)
}

// acctCurrencyOf returns the currency code of the given account ("" when the
// id isn't picked yet or unknown).
func acctCurrencyOf(app *appstate.App, accountID string) string {
	if app == nil || accountID == "" {
		return ""
	}
	for _, ac := range app.Accounts() {
		if ac.ID == accountID {
			return ac.Currency
		}
	}
	return ""
}

// acctTransferAmountOK reports whether amountStr parses as a positive amount in
// the source account's minor units — the shared submit-enable rule for both
// transfer forms (the button used to enable with a blank amount; 2026-07-18
// assessment, High: validation defect).
func acctTransferAmountOK(app *appstate.App, fromID, amountStr string) bool {
	dec := currency.Decimals(acctCurrencyOf(app, fromID))
	amtMinor, err := money.ParseMinor(strings.TrimSpace(amountStr), dec)
	return err == nil && amtMinor > 0
}

// acctParseOptionalMinor parses an optional amount field: blank or an explicit
// zero both mean "none" (0, true) — a typed "0" fee is a statement, not an
// error; otherwise it must parse as a positive amount in the given currency.
func acctParseOptionalMinor(s, ccy string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	v, err := money.ParseMinor(s, currency.Decimals(ccy))
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// doAccountTransfer creates a transfer pair from the page/row transfer forms. Pure
// (no hooks): validates the amounts, defaults the date/desc, and bumps the revision.
// receivedStr/feeStr are the optional cross-currency landed amount (destination
// currency) and transfer fee (source currency); blank means none.
func doAccountTransfer(app *appstate.App, fromID, toID, amountStr, receivedStr, feeStr, dateStr, desc string) {
	fromCcy := acctCurrencyOf(app, fromID)
	dec := currency.Decimals(fromCcy)
	amtMinor, err := money.ParseMinor(strings.TrimSpace(amountStr), dec)
	if err != nil || amtMinor <= 0 {
		uistate.PostNotice(uistate.T("accounts.transferInvalidAmount"), true)
		return
	}
	recvMinor, recvOK := acctParseOptionalMinor(receivedStr, acctCurrencyOf(app, toID))
	feeMinor, feeOK := acctParseOptionalMinor(feeStr, fromCcy)
	if !recvOK || !feeOK {
		uistate.PostNotice(uistate.T("accounts.transferInvalidExtra"), true)
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
		FromAccountID: fromID, ToAccountID: toID, AmountMinor: amtMinor,
		ReceivedMinor: recvMinor, FeeMinor: feeMinor, Date: when, Desc: d,
	}); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.BumpDataRevision()
	// G3: a transfer posts two real transactions — one-click reversible.
	uistate.PostUndoable(uistate.T("accounts.transferDone"))
}

// acctViewTransactions returns an OnView handler that pins the ledger filter to one
// account and navigates to /transactions. nav + txFilter are hooks, so the caller
// resolves them at a stable position and passes them in.
func acctViewTransactions(nav router.Navigator, txFilter state.Atom[uistate.TxFilter]) func(string) {
	return func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
}

// acctViewBillPayments drills to the transactions the user marked as bill payments
// toward an account (Transaction.BillAccountID) — the proof behind the row's
// "last bill" line.
func acctViewBillPayments(nav router.Navigator, txFilter state.Atom[uistate.TxFilter]) func(string) {
	return func(accountID string) {
		f := uistate.TxFilter{BillAccount: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
}

// --- acct-summary ---------------------------------------------------------------

// acctSummaryWidget is the net-worth-dominant summary tile: a hero net-worth figure
// (with a smart explainer + month-to-date trend) beside assets and a liabilities
// total that links through to /debt, plus a missing-exchange-rate notice (L4).
func acctSummaryWidget(props acctSummaryProps) ui.Node {
	// Subscribe to the shared data revision so a mutation in any tile (a transfer, a
	// balance update, a delete) re-renders this tile — its props are the same *App
	// pointer across host renders, so without this the engine would memoize it stale.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	// AC15: client-side nav for the idle-cash line's "See allocation options" link,
	// declared unconditionally (stable hook position) like acctListWidget's goToDebt.
	goToAllocate := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/allocate")) }))
	accounts := app.Accounts()
	txns := app.Transactions()

	nw, _ := ledger.NetWorthExplained(accounts, txns, props.Rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities

	// Month-to-date net-worth delta (G3 §3): the honest change since the 1st.
	// The boundary must be UTC midnight (dateutil.MonthStart), not the local
	// month start: txn dates are UTC-midnight calendar dates, so a local
	// boundary (Jul 1 00:00-04:00 = Jul 1 04:00Z) excluded first-of-month
	// transactions and this tile said "No change this month" while the
	// dashboard hero (already UTC) showed the real delta (C341).
	nowTS := time.Now()
	monthStart := dateutil.MonthStart(nowTS)
	var nwDelta money.Money
	haveDelta := false
	if series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{monthStart, nowTS.AddDate(0, 0, 1)}, props.Rates); err == nil && len(series) == 2 {
		if d, derr := series[1].Sub(series[0]); derr == nil {
			nwDelta, haveDelta = d, true
		}
	}

	smartSettings := uistate.LoadSmartSettings()

	body := Div(
		Div(css.Class("nw-summary"),
			Div(css.Class("stat stat-hero"),
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("dashboard.netWorth"),
					smartTooltipFor(smartSettings, "accounts-net", uistate.T("dashboard.netWorth"), uistate.T("smart.tipAccountsNet")),
				),
				Div(ClassStr("stat-value "+accentFor(net)), fmtMoney(net)),
				netWorthDeltaLine(nwDelta, haveDelta),
			),
			stat(uistate.T("accounts.assets"), fmtMoney(assets), "pos"),
			Div(css.Class("stat"),
				A(css.Class("stat-label"), Href(uistate.RoutePath("/debt")), uistate.T("dashboard.liabilities")),
				Div(ClassStr("stat-value neg"), fmtMoney(liabilities)),
			),
		),
		If(len(nw.MissingCurrencies) > 0, P(css.Class("err"), Attr("role", "alert"),
			uistate.T("accounts.nwExcludes", plural(len(nw.ExcludedAccounts), "account"), strings.Join(nw.MissingCurrencies, ", ")))),
		// AC11: disclose accounts left out of net worth by the household's choice.
		If(len(nw.ExcludedByChoice) > 0, P(css.Class("t-caption", tw.TextDim), Attr("role", "status"), Attr("data-testid", "acct-nw-excludes-by-choice"),
			uistate.T(excludesByChoiceKey(len(nw.ExcludedByChoice)), len(nw.ExcludedByChoice)))),
		// AC15: a quiet idle-cash line — liquid cash beyond this month's bills + goal
		// set-asides, priced at the user's own benchmark rate (never a live feed).
		// Renders nothing until a benchmark is set in Settings and there's enough
		// idle cash to be worth naming.
		acctIdleCashLine(app, props.Base, goToAllocate),
		// Liquidity breakdown: the asset side grouped by how usable the money is.
		acctLiquidityLine(app, props.Base, props.Rates),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// excludesByChoiceKey picks the singular or plural AC11 disclosure key for n
// accounts left out of net worth by the household's choice.
func excludesByChoiceKey(n int) string {
	if n == 1 {
		return "accountsstmt.excludesByChoice"
	}
	return "accountsstmt.excludesByChoicePlural"
}

// moneyFromMajor rebuilds a money.Money from an engine-variable major-unit float
// (the surface liveEngineVars/engineenv.Vars returns), rounding to the currency's
// minor unit. Display-only — never used for ledger arithmetic.
func moneyFromMajor(v float64, base string) money.Money {
	dec := currency.Decimals(base)
	mult := 1.0
	for i := 0; i < dec; i++ {
		mult *= 10
	}
	return money.New(int64(math.Round(v*mult)), base)
}

// acctIdleCashLine renders the AC15 idle-cash summary — liquid cash beyond this
// month's committed bills and goal set-asides, priced at the user's own benchmark
// rate — as a quiet line linking to /allocate. It renders nothing until a benchmark
// is set (Settings → Preferences) and the idle amount clears the same buffer
// idlecash.DefaultThresholdMinor uses, so a small cash cushion never reads as a nag.
func acctIdleCashLine(app *appstate.App, base string, onAllocate ui.Handler) ui.Node {
	vars := liveEngineVars(app)
	benchmark := vars["idle_cash_benchmark"]
	if benchmark <= 0 {
		return Fragment()
	}
	idle := moneyFromMajor(vars["idle_cash"], base)
	if idle.Amount < idlecash.DefaultThresholdMinor {
		return Fragment()
	}
	forgone := moneyFromMajor(vars["idle_cash_forgone_annual"], base)
	benchStr := strconv.FormatFloat(benchmark, 'f', -1, 64)
	return P(css.Class("t-caption", tw.TextDim, tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Attr("data-testid", "acct-idle-cash-line"),
		Span(uistate.T("accounts.idleCashLine", fmtMoney(idle), fmtMoney(forgone), benchStr)),
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-idle-cash-link"), OnClick(onAllocate), uistate.T("accounts.idleCashLink")),
	)
}

// acctLiquidityLine renders the asset side grouped by how usable the money is
// right now (internal/liquidity): Available cash · Restricted · Investments ·
// Held assets, each base-converted. Empty buckets stay silent, and the whole
// line renders nothing until at least two buckets hold money — a single-bucket
// household learns nothing from it. Rate-less accounts are skipped (consistent
// with the net-worth exclusion disclosure above).
func acctLiquidityLine(app *appstate.App, base string, rates currency.Rates) ui.Node {
	txns := app.Transactions()
	totals := liquidity.Totals(app.Accounts(), time.Now(), func(a domain.Account) (int64, bool) {
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			return 0, false
		}
		c, cerr := rates.Convert(bal, base)
		if cerr != nil {
			return 0, false
		}
		return c.Amount, true
	})
	type item struct {
		cls    liquidity.Class
		key    string
		testid string
	}
	items := []item{
		{liquidity.Available, "accounts.liqAvailable", "acct-liq-available"},
		{liquidity.Restricted, "accounts.liqRestricted", "acct-liq-restricted"},
		{liquidity.Investments, "accounts.liqInvestments", "acct-liq-investments"},
		{liquidity.Held, "accounts.liqHeld", "acct-liq-held"},
	}
	nonzero := 0
	for _, it := range items {
		if totals[it.cls] != 0 {
			nonzero++
		}
	}
	if nonzero < 2 {
		return Fragment()
	}
	kids := []any{css.Class("t-caption", tw.TextDim), Attr("data-testid", "acct-liquidity-line"),
		Style(map[string]string{"display": "flex", "flex-wrap": "wrap", "gap": "0.25rem 1rem"}),
		Title(uistate.T("accounts.liqTitle"))}
	for _, it := range items {
		if totals[it.cls] == 0 {
			continue
		}
		kids = append(kids, Span(Attr("data-testid", it.testid),
			uistate.T(it.key, fmtMoney(money.New(totals[it.cls], base)))))
	}
	return P(kids...)
}

// --- acct-toolbar ---------------------------------------------------------------

// acctToolbarGlyph renders one accounts-toolbar action as a standard labeled button
// (the .btn-tool treatment shared with the transactions toolbar): a slightly-grayed
// leading glyph, an always-visible text label, and a small muted trailing badge that
// says what the button DOES — kind "modal" opens a flip modal (⧉), "nav" navigates to
// another page (↗), "action" acts in place (no badge). The visible label removes the
// hover-to-decode step of the old icon-only treatment. variant tints it ("" neutral,
// "primary" accent, "stale" amber); open keeps a modal opener highlighted while its
// modal is showing.
func acctToolbarGlyph(testID string, ic icon.Name, label, title, kind, variant string, open bool, onClick ui.Handler) ui.Node {
	cls := "btn btn-tool"
	switch variant {
	case "primary":
		cls += " btn-primary"
	case "stale":
		cls += " bt-stale"
	}
	if open {
		cls += " is-open"
	}
	// The hover tooltip explains what the action MEANS when the label alone is
	// ambiguous (e.g. "Mark all updated" is a powerful global trust action); an empty
	// title falls back to the label so the tooltip is never blank.
	if title == "" {
		title = label
	}
	// A single leading glyph + the text label — no trailing kind badge (the button's
	// behaviour is conveyed by aria + the label, not a second glyph on the right).
	args := []any{
		css.Class(cls), Type("button"), Attr("data-testid", testID),
		Attr("aria-label", label), Attr("title", title), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(label),
	}
	if kind == "modal" {
		args = append(args, Attr("aria-haspopup", "dialog"), Attr("aria-expanded", boolStr(open)))
	}
	return Button(args...)
}

// markAllStalePrompt is the shared "Mark all updated" action (#77): it collects the
// non-archived stale accounts, previews the blast radius in the confirm (how many
// balances and which), and — on confirm — stamps every balance now as one undoable
// batch. Pure of hooks so any component can call it from a UseEvent; a no-op when
// nothing is stale. Used by both the toolbar's Mark-all button and the accounts
// list's stale-summary line, so the two stay in lockstep.
func markAllStalePrompt(app *appstate.App) {
	w := app.FreshnessWindows()
	now := time.Now()
	var staleAccts []domain.Account
	for _, ac := range app.Accounts() {
		if !ac.Archived && freshness.IsStale(ac, w, now) {
			staleAccts = append(staleAccts, ac)
		}
	}
	if len(staleAccts) == 0 {
		return
	}
	names := make([]string, 0, 3)
	for i, ac := range staleAccts {
		if i == 3 {
			break
		}
		names = append(names, ac.Name)
	}
	list := strings.Join(names, ", ")
	if len(staleAccts) > 3 {
		list = uistate.T("accounts.markAllMore", list, len(staleAccts)-3)
	}
	uistate.ConfirmModalLabeled(uistate.T("accounts.markAllConfirm", plural(len(staleAccts), "balance"), list)+" "+uistate.T("acctxn.markAllMeaning"),
		uistate.T("accounts.markAll"), false, func(ok bool) {
			if !ok {
				return
			}
			n := 0
			for _, ac := range staleAccts {
				ac.BalanceAsOf = time.Now()
				if err := app.PutAccount(ac); err != nil {
					uistate.PostNotice(uistate.T("accounts.markErr", err.Error()), true)
					continue
				}
				n++
			}
			uistate.BumpDataRevision()
			if n > 0 {
				auditview.CaptureNow()
				uistate.PostUndoable(uistate.T("accounts.markedUpdated", plural(n, "balance")))
			}
		})
}

// acctToolbarWidget is the filter toolbar tile, modelled on the /transactions
// toolbar: a search box, a Filters popover (account type + show-archived), active
// chips, and the page actions (Transfer money, Mark all updated, Manage exchange
// rates). It writes the shared accounts filter + transfer-open atoms so the list and
// transfer tiles react in step.
func acctToolbarWidget(props acctToolbarProps) ui.Node {
	// Subscribe to the data revision so the stale-count badge + FX affordance refresh
	// after a mark-all / balance update (the tile's props are revision-independent).
	_ = uistate.UseDataRevision().Get()
	app := props.App
	filterAtom := uistate.UseAccountsFilter()
	transferAtom := uistate.UseAcctTransferOpen()
	formulasAtom := uistate.UseAcctShowFormulas()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	setFilter := func(mut func(*uistate.AccountsFilter)) {
		nf := filterAtom.Get()
		mut(&nf)
		filterAtom.Set(nf.Normalize())
	}
	onSearch := func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.Search = v }) }

	onToggleArch := ui.UseEvent(Prevent(func() { setFilter(func(x *uistate.AccountsFilter) { x.ShowArchived = !x.ShowArchived }) }))
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	openTransfer := ui.UseEvent(Prevent(func() { transferAtom.Set(true) }))
	// G1: page-local account creation — every sibling page anchors its toolbar with Add.
	openAdd := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("account") }))
	// AC1: open the account-group manager. When at least one group exists, jump
	// straight to editing the first (its header carries per-group edit too); with
	// none, start a new group. Plain funcs (not UseEvent): they run from the
	// Manage overflow menu, whose item component owns the click hook.
	groupEditAtom := uistate.UseAccountGroupEdit()
	openGroups := func() {
		target := "new"
		if gs := app.AccountGroups(); len(gs) > 0 {
			target = gs[0].ID
		}
		groupEditAtom.Set(target)
	}
	// AC10: open the institution directory (add/rename/color/delete banks and lenders).
	institutionsAtom := uistate.UseInstitutionsManager()
	openInstitutions := func() { institutionsAtom.Set(true) }
	// AC7: open the surplus-sweep rules manager.
	sweepAtom := uistate.UseSweepRulesOpen()
	openSweep := func() { sweepAtom.Set(true) }
	openFX := func() { uistate.OpenGlobalSettingsAt("household") }
	// #77: a high-impact bulk write previews its blast radius first — the confirm
	// names how many balances it touches (and which) — and the apply is undoable
	// (one capture for the whole batch, so a single Undo restores every stamp). The
	// preview/apply body is shared (markAllStalePrompt) so the accounts list's
	// stale-summary line triggers the identical action.
	markAll := ui.UseEvent(Prevent(func() { markAllStalePrompt(app) }))

	accounts := app.Accounts()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	// Type filter options: the asset types actually present in the scoped set.
	typeSet := map[string]bool{}
	hasForeign := false
	windows := app.FreshnessWindows()
	now := time.Now()
	staleCount := 0
	for _, ac := range accounts {
		if freshness.IsStale(ac, windows, now) {
			staleCount++
		}
		if ac.Currency != "" && ac.Currency != base {
			hasForeign = true
		}
		if !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		typeSet[string(ac.Type)] = true
	}
	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Strings(types)
	typeOpts := withAllOption(uistate.T("accounts.allTypes"),
		uiw.OptionsFrom(types, func(s string) string { return s }, func(s string) string { return humanizeType(s) }, ""))

	archLabel := uistate.T("accounts.showArchived")
	if f.ShowArchived {
		archLabel = uistate.T("accounts.hideArchived")
	}
	formulasLabel := uistate.T("accounts.showFormulas")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("accounts.hideFormulas")
	}
	filtersBody := Div(css.Class("filter-fields"),
		filterSelect(uistate.T("accounts.filterType"), f.Type, typeOpts, func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.Type = v }) }),
		withFieldLabel(uistate.T("accounts.archived"),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(f.ShowArchived)),
				Attr("data-testid", "acct-toggle-archived"), OnClick(onToggleArch), Text(archLabel))),
		withFieldLabel(uistate.T("accounts.formulaTitle"),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "acct-toggle-formulas"), OnClick(onToggleFormulas), Text(formulasLabel))),
	)

	chips := []uiw.Chip{}
	if f.Search != "" {
		chips = append(chips, uiw.Chip{Key: "search", Label: uistate.T("accounts.chipSearch", f.Search)})
	}
	if f.Type != "" {
		chips = append(chips, uiw.Chip{Key: "type", Label: uistate.T("accounts.chipType", humanizeType(f.Type))})
	}
	removeChip := func(key string) { filterAtom.Set(f.Without(key).Normalize()) }
	clearAll := func() { filterAtom.Set(uistate.AccountsFilter{ShowArchived: f.ShowArchived}.Normalize()) }

	showFX := hasForeign || len(app.Settings().FXRates) > 0

	toolbar := uiw.FilterToolbar(uiw.FilterToolbarProps{
		Search:       f.Search,
		SearchLabel:  uistate.T("accounts.searchPlaceholder"),
		OnSearch:     onSearch,
		FiltersLabel: uistate.T("accounts.filters"),
		FiltersTitle: uistate.T("accounts.filtersTitle"),
		ActiveAriaLabel: func(n int) string {
			if n == 0 {
				return uistate.T("accounts.filters")
			}
			return uistate.T("accounts.filtersActiveAria", plural(n, "filter"))
		},
		FilterFields:  filtersBody,
		Chips:         chips,
		OnRemoveChip:  removeChip,
		OnClearAll:    clearAll,
		ClearAllLabel: uistate.T("accounts.clearFilters"),
		RemoveLabel:   uistate.T("accounts.removeFilter"),
		Actions: []ui.Node{
			// 2026-07-17 audit: the everyday actions stay labeled buttons — Mark-all
			// (contextual, amber), Transfer money, and the primary "+ Add account" —
			// while the setup/management surfaces (Groups, Institutions, Sweep rules,
			// Exchange rates) fold into ONE labeled "Account settings" menu (renamed
			// from the vague "Manage"), so the row before the account list stops
			// presenting seven equally weighted verbs.
			If(staleCount > 0, acctToolbarGlyph("acct-markall-btn", icon.Refresh,
				uistate.T("accounts.markAll"), uistate.T("acctxn.markAllMeaning"), "action", "stale", false, markAll)),
			If(len(accounts) >= 1, uiw.OverflowMenu(uiw.OverflowMenuProps{
				TriggerText:   uistate.T("acctxn.acctSettingsMenu"),
				TriggerLabel:  uistate.T("acctxn.acctSettingsMenuTitle"),
				TriggerTestID: "acct-manage-btn",
				TriggerClass:  "btn btn-tool",
				Items: []uiw.OverflowMenuItem{
					// QA CF-27: the item's label matches what clicking DOES — with no
					// groups yet it opens the creation form, so it says "New group…";
					// with groups it opens the manager.
					{Label: func() string {
						if len(app.AccountGroups()) == 0 {
							return uistate.T("accounts.newGroupAction")
						}
						return uistate.T("accounts.manageGroupsAction")
					}(), Icon: icon.Tag, TestID: "acct-groups-btn", OnSelect: openGroups},
					{Label: uistate.T("accounts.institutionsAction"), Icon: icon.Landmark, TestID: "acct-institutions-btn", OnSelect: openInstitutions},
					{Label: uistate.T("acctSweepCfg.title"), Icon: icon.TrendingUp, TestID: "acct-sweep-btn", OnSelect: openSweep, Hidden: len(accounts) < 2},
					{Label: uistate.T("accounts.manageFXRates"), Icon: icon.Scale, TestID: "acct-fx-btn", OnSelect: openFX, Hidden: !showFX},
				},
			})),
			If(len(accounts) >= 2, acctToolbarGlyph("page-transfer-btn", icon.Repeat,
				uistate.T("accounts.transferMoney"), "", "modal", "", transferAtom.Get(), openTransfer)),
			// G1 + primary-last convention: "+ Add account" anchors the right end, exactly
			// like the budgets and goals toolbars.
			acctToolbarGlyph("accounts-add", icon.Plus,
				uistate.T("accounts.addTitle"), "", "modal", "primary", false, openAdd),
		},
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- page transfer form ---------------------------------------------------------

// AccountPageTransferForm is the page-level "Transfer money" editor: pick any two
// accounts and move money between them. It owns its own field hooks and reuses
// doAccountTransfer / CreateTransferPair, then calls OnDone. It renders inside the
// shell-root flip modal (AccountTransferHost) — not an inline tile — so it centers on
// the viewport like the other account editors. From/To option lists each exclude the
// other side so the same account can't be picked twice (C69).
func AccountPageTransferForm(props AccountPageTransferProps) ui.Node {
	app := props.App
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	fromS := ui.UseState("")
	toS := ui.UseState("")
	amtS := ui.UseState("")
	recvS := ui.UseState("")
	feeS := ui.UseState("")
	dateS := ui.UseState(time.Now().Format("2006-01-02"))
	descS := ui.UseState("")
	onAmt := ui.UseEvent(func(v string) { amtS.Set(v) })
	onRecv := ui.UseEvent(func(v string) { recvS.Set(v) })
	onFee := ui.UseEvent(func(v string) { feeS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	cancel := ui.UseEvent(Prevent(func() { done() }))
	submit := ui.UseEvent(Prevent(func() {
		from, to := fromS.Get(), toS.Get()
		if from == "" || to == "" || from == to {
			return
		}
		doAccountTransfer(app, from, to, amtS.Get(), recvS.Get(), feeS.Get(), dateS.Get(), descS.Get())
		done()
	}))

	pfrom, pto := fromS.Get(), toS.Get()
	// C2: one shared eligibility rule for both transfer forms (liquid sources; no
	// property/vehicle destinations; liabilities labelled as payments).
	fromOpts, toOpts := acctTransferOptions(app.Accounts(), pfrom, pto)
	sameAcct := pfrom != "" && pto != "" && pfrom == pto
	fromCcy, toCcy := acctCurrencyOf(app, pfrom), acctCurrencyOf(app, pto)
	crossCcy := fromCcy != "" && toCcy != "" && fromCcy != toCcy
	_, recvOK := acctParseOptionalMinor(recvS.Get(), toCcy)
	_, feeOK := acctParseOptionalMinor(feeS.Get(), fromCcy)
	// The button stays disabled until the form would actually post: both
	// accounts, a valid positive amount, and clean optional fields.
	submitDisabled := sameAcct || pfrom == "" || pto == "" ||
		!acctTransferAmountOK(app, pfrom, amtS.Get()) || !recvOK || !feeOK

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "page-transfer-form"),
		Attr("aria-label", uistate.T("accounts.transferFormLabel")), OnSubmit(submit),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.transferFromLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: pfrom, OnChange: func(v string) { fromS.Set(v) },
					AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "page-xfer-from-select"})),
			labeledField(uistate.T("accounts.transferToLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: pto, OnChange: func(v string) { toS.Set(v) },
					AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "page-xfer-to-select"})),
			If(sameAcct, P(css.Class("err"), Attr("role", "alert"), uistate.T("accounts.transferSameAccountErr"))),
			labeledField(uistate.T("accounts.transferAmount"),
				Input(css.Class("field"), Attr("id", "page-xfer-amt"), Attr("data-testid", "page-xfer-amt"), Attr("autofocus", ""),
					Type("number"), Placeholder(uistate.T("accounts.transferAmount")), Value(amtS.Get()),
					Step("0.01"), Attr("min", "0.01"), OnInput(onAmt))),
			// G7: cross-currency semantics said out loud — denomination + live converted
			// preview at the saved rate, or a no-rate warning before anything posts.
			acctTransferFXNote(app, pfrom, pto, amtS.Get()),
			// Cross-currency only: the actual landed amount, overriding the saved
			// rate; hidden for same-currency pairs where it can't apply.
			If(crossCcy, Fragment(
				labeledField(uistate.T("accounts.transferReceivedLabel", toCcy),
					Input(css.Class("field"), Attr("data-testid", "page-xfer-received"),
						Type("number"), Value(recvS.Get()), Step("0.01"), Attr("min", "0.01"),
						Attr("title", uistate.T("accounts.transferReceivedHint")), OnInput(onRecv))),
				If(!recvOK, P(css.Class("err"), Attr("role", "alert"), uistate.T("accounts.transferInvalidExtra"))),
			)),
			If(fromCcy != "", Fragment(
				labeledField(uistate.T("accounts.transferFeeLabel", fromCcy),
					Input(css.Class("field"), Attr("data-testid", "page-xfer-fee"),
						Type("number"), Value(feeS.Get()), Step("0.01"), Attr("min", "0.01"),
						Attr("title", uistate.T("accounts.transferFeeHint")), OnInput(onFee))),
				If(!feeOK, P(css.Class("err"), Attr("role", "alert"), uistate.T("accounts.transferInvalidExtra"))),
			)),
			// Before/after balances for both sides, straight from the same leg
			// math the post will use (PreviewTransferPair) — including the typed
			// received override and fee.
			acctTransferBalancePreview(app, pfrom, pto, amtS.Get(), recvS.Get(), feeS.Get()),
			labeledField(uistate.T("accounts.transferDateLabel"),
				Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.transferDateLabel")),
					Value(dateS.Get()), OnInput(onDate))),
			labeledField(uistate.T("accounts.transferDescLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.transferDefaultDesc")),
					Value(descS.Get()), OnInput(onDesc))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// --- acct-list ------------------------------------------------------------------

// acctListWidget is the accounts-list tile: the owner-scoped, search/type-filtered
// account rows rendered as AccountRow inside an EntityListSection. A segmented
// "All / Assets / Liabilities" toggle in the section header narrows by class so
// both sides of net worth can be spot-checked in one place (assets and liabilities
// used to be two separate tiles, which listed every asset twice). Rows sort by
// signed balance high to low, so the biggest holdings sit on top and the heaviest
// debts (most negative) at the bottom. The tile owns the per-row callbacks and the
// "view transactions" navigation.
func acctListWidget(props acctListProps) ui.Node {
	// Subscribe to the data revision so balances/rows re-render after any mutation.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	filterAtom := uistate.UseAccountsFilter()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	// Client-side navigation to /debt for the "Manage debts" link. Declared
	// unconditionally (stable hook position); it prevents the anchor's default full
	// page load — which otherwise re-booted the app and, with app-lock on, dropped
	// the user onto the lock screen.
	goToDebt := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/debt")) }))
	// C365: the investment-section header's "Open investments" link. Declared
	// unconditionally (stable hook position) alongside goToDebt.
	goToInvestments := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/investments")) }))
	// The stale-summary line's "Mark all updated" (shown when most accounts are
	// stale) reuses the toolbar's exact action. Declared unconditionally so the hook
	// position stays stable even when the summary line isn't rendered.
	markAllStale := ui.UseEvent(Prevent(func() { markAllStalePrompt(app) }))

	accounts := app.Accounts()
	txns := app.Transactions()

	// Base-converted net-worth contribution: assets positive, liabilities the
	// negative of their owed magnitude. Taking -Abs for liabilities keeps a debt
	// below the assets regardless of how its balance is signed at rest (the sample
	// stores debts negative; the "amount you owe" add form stores them positive).
	// Falls back to the raw amount for rate-less accounts.
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		m := bal.Amount
		if c, err := props.Rates.Convert(bal, props.Base); err == nil {
			m = c.Amount
		}
		if ac.Class == domain.ClassLiability {
			if m < 0 {
				m = -m
			}
			return -m
		}
		return m
	}

	// Active, owner-visible accounts split by class (archived live in their own tile).
	var active, liabs []domain.Account
	for _, ac := range accounts {
		if ac.Archived || !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		active = append(active, ac)
		if ac.Class == domain.ClassLiability {
			liabs = append(liabs, ac)
		}
	}
	hasLiab := len(liabs) > 0

	// The class view. With no liabilities the toggle is pointless, so force the
	// assets-only view (and hide the segment) — the page reads exactly as before.
	classView := f.Class
	if !hasLiab {
		classView = string(domain.ClassAsset)
	}
	assetsOnly := classView == string(domain.ClassAsset)

	var shown []domain.Account
	for _, ac := range active {
		if assetsOnly && ac.Class == domain.ClassLiability {
			continue
		}
		if classView == string(domain.ClassLiability) && ac.Class != domain.ClassLiability {
			continue
		}
		if f.Matches(ac.Name, string(ac.Type)) {
			shown = append(shown, ac)
		}
	}
	// Risk-first ordering (AC-refresh): accounts that need attention — stale ones,
	// past their freshness window — lead the list so the rows worth acting on sit on
	// top, instead of being buried under big healthy balances. Within each freshness
	// tier the familiar "signed high to low" order holds (biggest holdings lead, the
	// heaviest debts trail). The comparison itself is the pure acctsort.RiskFirstLess
	// (native-tested); here we only supply the stale flag + the signed balance.
	windows := app.FreshnessWindows()
	now := time.Now()
	staleOf := func(ac domain.Account) bool { return freshness.IsStale(ac, windows, now) }
	sort.SliceStable(shown, func(i, j int) bool {
		return acctsort.RiskFirstLess(staleOf(shown[i]), staleOf(shown[j]), convBal(shown[i]), convBal(shown[j]))
	})

	// Stale-badge prioritisation collapse: once more than half the visible accounts are
	// stale, a per-row STALE badge on nearly every line ranks nothing — so collapse the
	// badges to subdued dots and lead the list with one summary line + Mark-all instead.
	// Below the threshold each row keeps its full badge.
	staleShown := 0
	for _, ac := range shown {
		if staleOf(ac) {
			staleShown++
		}
	}
	staleCollapsed := len(shown) > 0 && staleShown*2 > len(shown)

	smartSettings := uistate.LoadSmartSettings()
	pr := uistate.UsePrefs().Get()
	smartIn := buildSmartInput(app, pr.WeekStartWeekday())
	accountInsights := smartengine.RunPage(smartIn, smartSettings, smart.PageAccounts)
	accountByEntity := insightsByEntity(accountInsights)

	categories := app.Categories()
	members := app.Members()
	accDefs := app.CustomFieldDefsFor("account")
	cbs := buildAcctRowCallbacks(app)
	viewTxns := acctViewTransactions(nav, txFilter)
	viewBills := acctViewBillPayments(nav, txFilter)
	// AC10: index the institution directory once so every row can color its edge
	// and show a chip for its Account.InstitutionID.
	instByID := domain.InstitutionByID(app.Institutions())

	// AC2/AC9: the 90-day sparkline window and this-month flow window, computed once.
	monthStart, monthEnd := dateutil.MonthRange(now)

	// C413: the earliest transaction date per account, folded once, so each row can
	// size its "all" balance-history window without re-scanning the ledger per row.
	earliestByAcct := map[string]time.Time{}
	for _, t := range txns {
		if e, ok := earliestByAcct[t.AccountID]; !ok || t.Date.Before(e) {
			earliestByAcct[t.AccountID] = t.Date
		}
	}
	// C413: cap the "all" window at ~20 years of daily points so a stray far-past
	// date can't blow up the fold.
	const allWindowCapDays = 366 * 20

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		// C413: size the range picker's windows. Only compute the wider series when
		// the account actually has more than 90 days of history (HasRange), so the
		// picker stays hidden — and the extra folds skipped — for young accounts.
		allDays := acctseries.AllDays(now, acctseries.Days90, allWindowCapDays, earliestByAcct[ac.ID])
		hasRange := acctseries.HasRange(allDays)
		var s12m, sall []int64
		if hasRange {
			s12m = accountflow.BalanceSeries(ac, txns, now, acctseries.Days12m)
			sall = accountflow.BalanceSeries(ac, txns, now, allDays)
		}
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now), DaysStale: freshness.DaysSinceUpdate(ac, now),
			StaleCollapsed: staleCollapsed,
			Members:        members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			SmartSettings: smartSettings, SmartByEntity: accountByEntity, ValuationHistory: app.BalanceHistory(ac.ID),
			AccountDefs: accDefs, InstByID: instByID,
			BillPayment: ledger.BillPaymentForAccount(ac.ID, txns), OnViewBills: viewBills,
			Sparkline:    accountflow.BalanceSeries(ac, txns, now, acctseries.Days90),
			Sparkline12m: s12m, SparklineAll: sall, HasRange: hasRange,
			Flow:       accountflow.PeriodFlow(ac, txns, monthStart, monthEnd),
			ShowFlow:   true,
			Projection: app.ProjectAccount(ac.ID, now, 30),
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	// AC1: user-defined groups render as sections with a net subtotal each, followed
	// by an "Ungrouped" catch-all. Built only for the "default" body case below.
	groups := app.AccountGroups()
	groupedBody := func() ui.Node {
		sections := groupSections(shown, groups, txns, props.Rates, props.Base)
		return Div(css.Class("acct-groups"),
			MapKeyed(sections, func(s acctGroupSection) any {
				if s.Group.ID == "" {
					return "ungrouped"
				}
				return s.Group.ID
			}, func(s acctGroupSection) ui.Node {
				// C412: each section collapses to just its header (name + subtotal); the
				// collapsed state is read fresh here and persisted per group. Collapsed is
				// threaded into the header prop so the memoized header re-renders on change.
				collapsed := isAcctGroupCollapsed(s.Group.ID)
				groupCls := "acct-group"
				if collapsed {
					groupCls += " is-collapsed"
				}
				return Div(ClassStr(groupCls),
					ui.CreateElement(acctGroupHeader, acctGroupHeaderProps{Section: s, Collapsed: collapsed}),
					If(!collapsed, Div(css.Class("rows"), MapKeyed(s.Accounts, keyOf, renderRow))),
				)
			}),
		)
	}

	var bodyContent ui.Node
	switch {
	case len(active) == 0:
		bodyContent = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("accounts.noAssets"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account", Icon: icon.Accounts, ImportLink: true})
	case len(shown) == 0 && f.HasNarrowing():
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noMatch"))
	case len(shown) == 0 && classView == string(domain.ClassLiability):
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noLiabilities"))
	case len(shown) == 0:
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noAssets"))
	case len(groups) > 0:
		bodyContent = groupedBody()
	default:
		bodyContent = Div(css.Class("rows"), MapKeyed(shown, keyOf, renderRow))
	}

	// Collapsed-stale state: one summary line leads the list, its Mark-all reusing the
	// toolbar action. Prepended so it sits above the (grouped or flat) rows.
	if staleCollapsed {
		bodyContent = Fragment(
			Div(css.Class("acct-stale-summary"), Attr("data-testid", "acct-stale-summary"), Attr("role", "status"),
				Span(uistate.T("accounts.staleSummary")),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "acct-stale-summary-markall"),
					Title(uistate.T("acctxn.markAllMeaning")), OnClick(markAllStale), uistate.T("accounts.markAll")),
			),
			bodyContent,
		)
	}

	// Section title tracks the class view; the segmented toggle is the class picker
	// (only shown when there is something to toggle between).
	title := uistate.T("accounts.assets")
	if hasLiab {
		switch classView {
		case string(domain.ClassLiability):
			title = uistate.T("accounts.liabilitiesTitle")
		case string(domain.ClassAsset):
			title = uistate.T("accounts.assets")
		default:
			title = uistate.T("accounts.allAccounts")
		}
	}

	var classSeg ui.Node = Fragment()
	if hasLiab {
		classSeg = uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("accounts.classFilterLabel"),
			Selected: classView,
			Options: []uiw.SegOption{
				{Value: "", Label: uistate.T("accounts.classAll"), TestID: "acct-class-all"},
				{Value: string(domain.ClassAsset), Label: uistate.T("accounts.classAssets"), TestID: "acct-class-assets"},
				{Value: string(domain.ClassLiability), Label: uistate.T("accounts.classLiabilities"), TestID: "acct-class-liabilities"},
			},
			OnSelect: func(v string) {
				nf := filterAtom.Get()
				nf.Class = v
				filterAtom.Set(nf.Normalize())
				uistate.PersistAcctClass(v)
				// Flush now so a quick reload after toggling doesn't race the
				// autosave ticker and lose the choice (same guard as sample load, C2).
				uistate.RequestPersist()
			},
		})
	}

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: title,
		// The class filter (All / Assets / Liabilities) is the only header action; the
		// old "Smart" sparkle shortcut was dropped here as noise (the Smart hub is one
		// nav click away and the page already carries the inline Smart strip up top).
		HeaderAction: classSeg,
		Body:         bodyContent,
	})

	// A shortcut to /debt, where liabilities carry the richer payoff surface (min
	// payment, utilization, payoff order). When the assets-only view hides them, the
	// link names how many are tucked away (C346); otherwise it's a plain shortcut.
	// Left-click navigates client-side (goToDebt prevents the full reload); the href
	// stays for keyboard/middle-click. Built via If (not a Go `if`) so the OnClick
	// hook is always registered at a stable position.
	debtLinkText := uistate.T("accounts.manageDebtLink")
	debtLinkTestID := "acct-manage-debt"
	if assetsOnly {
		debtLinkText = uistate.T("accounts.liabilitiesStub", len(liabs))
		debtLinkTestID = "acct-liabilities-stub"
	}
	debtLink := If(hasLiab,
		Div(css.Class(tw.Mt3),
			A(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
				Href(uistate.RoutePath("/debt")),
				Attr("data-testid", debtLinkTestID),
				OnClick(goToDebt),
				uiw.Icon(icon.CreditCard, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				debtLinkText,
			),
		),
	)

	// C365: an investment-section header linking Accounts to the holdings experience.
	// Shown above the list whenever the household has investment accounts and the
	// liabilities-only view isn't active (investments are assets). Its chips mirror the
	// /investments summary hero (value / gain / return%), so the two pages agree.
	var investBanner ui.Node = Fragment()
	if classView != string(domain.ClassLiability) {
		investBanner = acctInvestmentsBanner(app, goToInvestments)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(investBanner, section, debtLink),
	})
}

// acctInvestmentsBanner renders the Accounts investment-section header (C365): a
// labelled strip with the portfolio value / gain / return% chips (from the shared
// invest view, matching the /investments hero) and an "Open investments" link, so
// the holdings experience is one click away. Renders nothing when the household has
// no investment accounts. onOpen navigates to /investments (a stable hook from the
// caller). Styled inline (theme tokens) so it needs no stylesheet rule.
func acctInvestmentsBanner(app *appstate.App, onOpen ui.Handler) ui.Node {
	v := computeInvestView(app)
	if !v.HasAny {
		return Fragment()
	}
	gainTone := tw.ColorClass(gainToneClass(v.SecSummary.TotalGainMinor))
	chip := func(testid, label, value, toneFold string) ui.Node {
		return Div(css.Class("acct-invest-chip"),
			Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.1rem"}),
			Span(css.Class("t-caption", tw.TextDim), label),
			Span(ClassStr("acct-invest-chip-val "+tw.Fold(tw.FontDisplay)+toneFold), Attr("data-testid", testid), value),
		)
	}
	toneCls := ""
	if gainTone != "" {
		toneCls = " " + gainTone
	}
	return Div(css.Class("acct-invest-banner"), Attr("data-testid", "acct-invest-banner"),
		Attr("role", "group"), Attr("aria-label", uistate.T("accountsInvest.bannerAria")),
		Style(map[string]string{
			"display": "flex", "flex-wrap": "wrap", "align-items": "center", "gap": "0.75rem 1.5rem",
			"padding": "0.75rem 1rem", "margin": "0 0 0.75rem", "border": "1px solid var(--border)",
			"border-radius": "0.5rem", "background": "var(--bg-card)",
		}),
		Div(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Style(map[string]string{"margin-right": "auto"}),
			uiw.Icon(icon.Reports, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)),
			Span(css.Class(tw.FontMedium), uistate.T("accountsInvest.sectionTitle")),
		),
		chip("acct-invest-value", uistate.T("accountsInvest.value"), fmtSignedMoney(v.TotalValueMinor, v.Sym, v.Dec), ""),
		chip("acct-invest-gain", uistate.T("accountsInvest.gain"), fmtSignedMoney(v.SecSummary.TotalGainMinor, v.Sym, v.Dec), toneCls),
		chip("acct-invest-return", uistate.T("accountsInvest.returnPct"), strconv.FormatFloat(v.SecSummary.ReturnPct, 'f', 2, 64)+"%", toneCls),
		Button(css.Class("btn-link", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "acct-open-investments"),
			Title(uistate.T("accountsInvest.openTitle")), OnClick(onOpen),
			Span(uistate.T("accountsInvest.open")),
			uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W3, tw.H3))),
		// Reconciliation sub-caption (wraps to its own full-width line): spells out how the
		// tracked-securities value relates to the investment-account total, so this banner's
		// figure can't be mistaken for a second, conflicting "investment value".
		If(investHasReconcile(v),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "acct-invest-reconcile"),
				Title(investReconcileTitle(v)),
				Style(map[string]string{"width": "100%", "flex-basis": "100%", "margin": "0.15rem 0 0"}),
				investReconcileLine(v))),
	)
}

// --- acct-archived --------------------------------------------------------------

// acctArchivedWidget is the archived-accounts tile, placed by the host only when the
// "show archived" toggle is on and archived accounts exist. Rows reuse AccountRow
// (so an archived account can still be restored, viewed, or edited).
func acctArchivedWidget(props acctArchivedProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	filterAtom := uistate.UseAccountsFilter()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	accounts := app.Accounts()
	txns := app.Transactions()
	_, archived := partitionAssetAccounts(accounts, txns, props.Rates, props.Base, activeMemberID)

	var shown []domain.Account
	for _, ac := range archived {
		if f.Matches(ac.Name, string(ac.Type)) {
			shown = append(shown, ac)
		}
	}

	windows := app.FreshnessWindows()
	now := time.Now()
	categories := app.Categories()
	members := app.Members()
	accDefs := app.CustomFieldDefsFor("account")
	cbs := buildAcctRowCallbacks(app)
	viewTxns := acctViewTransactions(nav, txFilter)
	viewBills := acctViewBillPayments(nav, txFilter)
	instByID := domain.InstitutionByID(app.Institutions())

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now), DaysStale: freshness.DaysSinceUpdate(ac, now),
			Members: members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			ValuationHistory: app.BalanceHistory(ac.ID), AccountDefs: accDefs, InstByID: instByID,
			BillPayment: ledger.BillPaymentForAccount(ac.ID, txns), OnViewBills: viewBills,
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	var bodyContent ui.Node
	if len(shown) == 0 {
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noMatch"))
	} else {
		bodyContent = Div(css.Class("rows"), MapKeyed(shown, keyOf, renderRow))
	}
	section := uiw.EntityListSection(uiw.EntityListSectionProps{Title: uistate.T("accounts.archived"), Body: bodyContent})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-archived", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// --- acct-welcome ---------------------------------------------------------------

// acctWelcomeWidget is the first-run tile (no accounts yet): a load-sample CTA so a
// new user can populate the app and explore.
func acctWelcomeWidget(props acctWelcomeProps) ui.Node {
	app := props.App
	loadSample := ui.UseEvent(Prevent(func() {
		if err := app.LoadSample(); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.SetSampleActive(true)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))
	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("accounts.welcomeTitle"),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("accounts.welcomeDesc")),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(loadSample), uistate.T("accounts.loadSample")),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-welcome", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- acct-formula ---------------------------------------------------------------

// acctFormulaWidget is the opt-in "Account metrics" tile (revealed by the toolbar's
// Formulas toggle). It embeds the reusable FormulaBuilder, which evaluates against
// the live engine variable surface — account aggregates (assets, liabilities,
// net_worth, asset_accounts, …) plus every number-typed account custom field as a
// cf_acct_<key> variable — so it ties custom fields and formulas together: define a
// numeric custom field on your accounts, then compute over it here.
func acctFormulaWidget(props acctFormulaProps) ui.Node {
	body := Div(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("accounts.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("accounts.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
