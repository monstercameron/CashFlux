// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/idlecash"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// accounts_tiles.go holds the Native widget bodies the /accounts surface host
// composes (see accounts_widget.go). Each is a self-contained engine tile. It reads
// the live store (props.App) and the shared accounts-page atoms (filter and transfer
// open state), never surface-local closures, so the host can place each tile through
// the same spec/render pipeline the dashboard and /transactions use. The per-account
// row (AccountRow) is reused as is. These tiles only arrange the page around it.
//
// Sweep rules: the config manager opens from the "acct-sweep-btn" toolbar item
// (SweepRulesHost, which renders SweepRulesForm in accounts_sweep_config.go). The
// proposal cards render above the bento (accountsSweepCards, accounts_sweep_card.go).

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

// accountRowKey is the MapKeyed key for an account row.
func accountRowKey(ac domain.Account) any { return ac.ID }

// acctRowCallbacks bundles the per-row mutation handlers that the list tiles pass to
// each AccountRow. They are plain funcs (no hooks). Each one changes the store and
// bumps the shared data revision so the whole surface re-renders together. OnView is
// supplied separately by the tile, since it needs the router.
type acctRowCallbacks struct {
	OnDelete                func(string)
	OnArchive               func(domain.Account)
	OnRefresh               func(domain.Account)
	OnSave                  func(domain.Account)
	OnSetBalance            func(ac domain.Account, current money.Money, newBalStr, catID, asOfISO string)
	OnTransfer              func(fromID, toID, amountStr, dateStr, desc string)
	OnSetDefaultHousehold   func(string)
	OnClearDefaultHousehold func()
	OnSetDefaultMember      func(accountID, memberID string)
	OnClearDefaultMember    func(memberID string)
}

// buildAcctRowCallbacks wires the store mutations for AccountRow. Errors surface
// through the global notice/toast (PostNotice) rather than a screen-local error
// state, matching the rest of the widgetized surfaces.
func buildAcctRowCallbacks(app *appstate.App) acctRowCallbacks {
	return acctRowCallbacks{
		OnDelete: func(accountID string) {
			// Refuse to delete an account that still has transactions, including the far
			// leg of a transfer, because deleting it would orphan them. Archiving keeps history.
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
			// Confirm first, then offer undo. Goals and budgets follow the same rule, so
			// deleting behaves the same on every page.
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
			// Name the change and offer a one-click undo, so archiving does not look like
			// the row simply vanished.
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
		OnSetDefaultHousehold: func(accountID string) {
			if err := app.SetDefaultAccountID(accountID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.householdSet"), false)
		},
		OnClearDefaultHousehold: func() {
			if err := app.ClearDefaultAccountID(); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.householdCleared"), false)
		},
		OnSetDefaultMember: func(accountID, memberID string) {
			accountFound := false
			for _, account := range app.Accounts() {
				if account.ID == accountID && !account.Archived {
					accountFound = true
					break
				}
			}
			if !accountFound {
				uistate.PostNotice(uistate.T("defaultAccount.archivedError"), true)
				return
			}
			name, found, err := setMemberDefaultAccount(app, memberID, accountID)
			if err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			if !found {
				uistate.PostNotice(uistate.T("defaultAccount.memberMissing"), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.memberSet", name), false)
		},
		OnClearDefaultMember: func(memberID string) {
			name, found, err := setMemberDefaultAccount(app, memberID, "")
			if err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			if !found {
				uistate.PostNotice(uistate.T("defaultAccount.memberMissing"), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.memberCleared", name), false)
		},
		OnSetBalance: func(ac domain.Account, currentBal money.Money, newStr, catID, asOfISO string) {
			dec := currency.Decimals(ac.Currency)
			typed, err := money.ParseMinor(strings.TrimSpace(newStr), dec)
			if err != nil {
				uistate.PostNotice(uistate.T("accounts.invalidBalance"), true)
				return
			}
			// The figure is typed in the stated convention (negative is money owed), so convert
			// it to the way this account holds balances at rest before it is posted.
			conv, _ := reconcile.ConventionOf(ac, currentBal.Amount)
			target := reconcile.Stored(typed, conv)
			// The difference is posted as a provisional balance checkpoint, not as an
			// ordinary adjustment. It moves the balance but is left out of reports, because
			// the money was not earned or spent: the account simply held more than the
			// ledger knew. Leaving the user to tag and delete these by hand produced
			// adjustments that counted as income.
			//
			// Only one checkpoint per account stands at a time. A previous one is replaced,
			// so checking a balance twice does not leave two entries explaining one gap.
			now := time.Now()
			// The date the balance was true as of. A figure read on Friday and
			// entered on Monday belongs to Friday; an unparseable or absent one
			// falls back to now rather than refusing the update over a date field.
			asOf := now
			if parsed, perr := dateutil.ParseDate(asOfISO); perr == nil && !parsed.IsZero() {
				asOf = parsed
			}
			if cp, ok := provisional.New(id.New(), ac.ID, uistate.T("accounts.balanceCheckpointDesc"),
				currentBal.Amount, target, ac.Currency, asOf, now); ok {
				cp.CategoryID = catID
				replaced := 0
				app.BulkMutate(func() {
					for _, old := range provisional.Of(ac.ID, app.Transactions()) {
						if err := app.DeleteTransaction(old.ID); err == nil {
							replaced++
						}
					}
					if err := app.PutTransaction(cp); err != nil {
						uistate.PostNotice(err.Error(), true)
					}
				})
				if replaced > 0 {
					uistate.PostNotice(uistate.T("accounts.balanceCheckpointReplaced", replaced), false)
				}
			}
			ac.BalanceAsOf = time.Now()
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			// The balance set is posted as a provisional checkpoint. It moves the balance, stays
			// out of reports, and is retired once a statement covers its date. The toast offers undo.
			uistate.PostUndoable(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(typed, ac.Currency))))
		},
		OnTransfer: func(fromID, toID, amountStr, dateStr, desc string) {
			doAccountTransfer(app, fromID, toID, amountStr, "", "", dateStr, desc)
		},
	}
}

// setMemberDefaultAccount points the member's default account at accountID, or clears
// it when accountID is "". found is false when no member has memberID. name is the
// member's display name, for the notice.
func setMemberDefaultAccount(app *appstate.App, memberID, accountID string) (name string, found bool, err error) {
	for _, member := range app.Members() {
		if member.ID != memberID {
			continue
		}
		member.Prefs.DefaultAccountID = accountID
		err = app.PutMember(member)
		return member.Name, true, err
	}
	return "", false, nil
}

// acctTransferOptions builds the From/To pickers for both transfer forms with one
// eligibility rule. Money moves FROM liquid cash only, so a 401(k) or a condo cannot
// be the source of a transfer. It lands anywhere except a valuation-only holding
// (property or vehicle). A transfer TO a liability is a payment, and the option label
// says so. Each side still excludes the other side's selection.
func acctTransferOptions(accounts []domain.Account, fromID, toID string) (fromOpts, toOpts []uiw.SelectOption) {
	fromOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
	toOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
	// Sorted for picking (acctsort.ForPicker), not in the risk-first order the accounts
	// page uses and not in creation order.
	for _, ac := range acctsort.ForPicker(accounts) {
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

// acctTransferFXNote shows the cross-currency meaning of a transfer before it posts.
// The amount is in the source currency. It lands converted at the saved rate, with a
// live ≈ preview of the typed amount. When no rate exists, a warning says it would land
// unconverted and points to Manage exchange rates. Renders nothing for same-currency
// pairs or incomplete picks.
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
		amtMinor = int64(math.Pow10(dec)) // preview 1.00 until an amount is typed
	}
	conv, cerr := rates.Convert(money.New(amtMinor, fromAc.Currency), toAc.Currency)
	if cerr != nil {
		return P(css.Class("err"), Role("alert"), Attr("data-testid", "xfer-fx-norate"),
			uistate.T("accounts.fxNoteNoRate", fromAc.Currency, toAc.Currency))
	}
	return P(css.Class("budget-sub"), Attr("data-testid", "xfer-fx-note"),
		css.Class(css.New(css.Margin(css.Length("0")))),
		uistate.T("accounts.fxNoteAmountIn", fromAc.Currency, toAc.Currency, fmtMoney(conv)))
}

// acctTransferBalancePreview shows what the transfer would do to both booked balances
// before it posts, such as "Checking: $1,000.00 → $950.00" for each side. It uses
// appstate.PreviewTransferPair, which shares the exact leg math (FX conversion and the
// liability payment sign) with the real post. Renders nothing until both accounts are
// picked and a valid positive amount is typed. receivedStr and feeStr are the optional
// landed-amount override (destination currency) and fee (source currency). Pass ""
// for none. Invalid text counts as absent here, since the form gates submit separately.
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
	preview, err := app.PreviewTransferPair(appstate.TransferParams{
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
	// State what the transfer means in one plain sentence: what each side does, and that
	// neither leg is spending. A liability destination reads as paying down what is
	// owed, not as an increase.
	amtStr := money.FormatMinor(amtMinor, currency.Decimals(fromCcy))
	semantics := uistate.T("accounts.xferSemanticsAsset", fromName, amtStr, toName)
	for _, ac := range app.Accounts() {
		if ac.ID == toID && ac.Class == domain.ClassLiability {
			semantics = uistate.T("accounts.xferSemanticsLiability", fromName, amtStr, toName)
			break
		}
	}
	return Div(css.Class("budget-sub"), Attr("data-testid", "xfer-balance-preview"),
		css.Class(css.New(css.Margin(css.Length("0")), css.Display.Grid, css.Gap(css.Rem(0.15)))),
		Span(uistate.T("accounts.xferPreviewTitle")),
		Span(Attr("data-testid", "xfer-preview-from"),
			uistate.T("accounts.xferPreviewLine", fromName, fmtMoney(preview.FromBefore), fmtMoney(preview.FromAfter))),
		Span(Attr("data-testid", "xfer-preview-to"),
			uistate.T("accounts.xferPreviewLine", toName, fmtMoney(preview.ToBefore), fmtMoney(preview.ToAfter))),
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

// acctTransferAmountOK reports whether amountStr parses as a positive amount in the
// source account's minor units. Both transfer forms use it to decide whether submit is
// enabled, so a blank amount never enables the button.
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

// doAccountTransfer creates a transfer pair from the page or row transfer forms. It
// has no hooks. It validates the amounts, fills in the date and description when
// they are blank, and bumps the revision. receivedStr and feeStr are the optional
// cross-currency landed amount (destination currency) and transfer fee (source
// currency). Blank means none.
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
	description := strings.TrimSpace(desc)
	if description == "" {
		description = uistate.T("accounts.transferDefaultDesc")
	}
	if _, _, err := app.CreateTransferPair(appstate.TransferParams{
		FromAccountID: fromID, ToAccountID: toID, AmountMinor: amtMinor,
		ReceivedMinor: recvMinor, FeeMinor: feeMinor, Date: when, Desc: description,
	}); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.BumpDataRevision()
	// A transfer posts two real transactions, so the toast offers one-click undo.
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
// total that links through to /debt, plus a notice when exchange rates are missing.
func acctSummaryWidget(props acctSummaryProps) ui.Node {
	// Subscribe to the shared data revision so a mutation in any tile (a transfer, a
	// balance update, a delete) re-renders this tile — its props are the same *App
	// pointer across host renders, so without this the engine would memoize it stale.
	rev := uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	// Client-side navigation for the idle-cash line's "See allocation options" link.
	// Declared unconditionally so the hook position is stable, like goToDebt in acctListWidget.
	goToAllocate := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/allocate")) }))
	// The page's add-account action, pinned over the summary's top-right corner.
	accounts := app.Accounts()
	txns := app.Transactions()

	nw := netWorthBalanceSnapshot(app, accounts, accountSnapshot(app).Balances, props.Rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities

	// Month-to-date net-worth delta: the honest change since the 1st, read through the
	// shared nwchange seam so this tile, the dashboard hero, /networth and /reports answer
	// the same question the same way. The window starts at UTC midnight because
	// transaction dates are UTC-midnight calendar dates. A local month start would miss
	// the first-of-month transactions.
	mtd := nwchange.Change{}
	if nw.Complete() {
		mtd, _ = nwchange.MonthToDateChange(accounts, txns, props.Rates, time.Now())
	}
	deltaMinor, deltaErr := mtd.DeltaMinorChecked()
	nwDelta, haveDelta := money.New(deltaMinor, props.Base), deltaErr == nil
	netText, assetsText, liabilitiesText, tone := "—", "—", "—", "text-dim"
	if nw.Known {
		netText, assetsText, liabilitiesText, tone = fmtMoney(net), fmtMoney(assets), fmtMoney(money.New(-liabilities.Amount, liabilities.Currency)), accentFor(net)
	}

	smartSettings := uistate.LoadSmartSettings()

	// The six-month net-worth shape the Dashboard hero also draws, so the two agree.
	now := time.Now()
	// The trend is deferred one settle, as the Dashboard hero defers its own, so the
	// header never adds work to the first paint.
	settled := useAfterDelay("acct-headline", 700*time.Millisecond)
	spark := ui.UseMemo(func() []float64 {
		if !settled || !nw.Complete() {
			return nil
		}
		return dashboardview.BuildNetWorthTrend(accounts, txns, props.Rates, now).Spark
	}, rev, settled, now.Format(dateutil.Layout), nw.Complete())
	trendVisual := uiw.PageHeadlineTrend(uiw.PageHeadlineTrendProps{
		Values: spark, Label: uistate.T("dashboard.netWorth"), GradientID: "acct-headline-trend",
		Pending: nw.Complete() && !settled, PendingLabel: uistate.T("accounts.trendLoading"),
	})

	body := Div(
		Div(css.Class("acct-summary-frame"),
			// One Add control for both things the page can add. Each item names what it creates.
			uiw.OverflowMenu(uiw.OverflowMenuProps{
				TriggerText:   uistate.T("accounts.addMenu"),
				TriggerLabel:  uistate.T("accounts.addMenuTitle"),
				TriggerTestID: "accounts-add",
				TriggerClass:  "primary",
				TriggerIcon:   icon.Plus,
				Items: []uiw.OverflowMenuItem{
					{Label: uistate.T("accounts.addAccountItem"), Icon: icon.Accounts, TestID: "accounts-add-account", OnSelect: func() { uistate.SetAddTarget("account") }},
					{Label: uistate.T("accounts.addServiceItem"), Icon: icon.Repeat, TestID: "accounts-add-service", OnSelect: func() { uistate.SetAddTarget("service") }},
				},
			}),
			acctHeadline(netText, assetsText, liabilitiesText, tone, nwDelta, haveDelta,
				smartTooltipFor(smartSettings, "accounts-net", uistate.T("dashboard.netWorth"), uistate.T("smart.tipAccountsNet")), trendVisual, acctLiquidityLine(app, props.Base, props.Rates)),
		),
		If(!nw.Complete(), P(css.Class("err"), Role("status"), Attr("data-testid", "acct-networth-note"), netWorthAvailabilityNote(nw))),
		// Disclose accounts the household has left out of net worth.
		If(len(nw.ExcludedByChoice) > 0, P(css.Class("t-caption", tw.TextDim), Role("status"), Attr("data-testid", "acct-nw-excludes-by-choice"),
			uistate.T(excludesByChoiceKey(len(nw.ExcludedByChoice)), len(nw.ExcludedByChoice)))),
		// A quiet idle-cash line: liquid cash beyond this month's bills and goal set-asides,
		// priced at the user's own benchmark rate (never a live feed). It renders nothing
		// until a benchmark is set in Settings and there is enough idle cash to be worth naming.
		acctIdleCashLine(app, props.Base, goToAllocate),
		// Liquidity breakdown: the asset side grouped by how usable the money is.
		// Shows where the app's own numbers disagree: a transfer with one leg, or holdings
		// that do not add up to their account's balance. It is absent when the data agrees.
		ui.CreateElement(contradictionStrip, contradictionStripProps{
			Kinds: []string{"account", "transaction"},
		}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// excludesByChoiceKey picks the singular or plural disclosure key for n accounts the
// household has left out of net worth.
func excludesByChoiceKey(n int) string {
	if n == 1 {
		return "accountsstmt.excludesByChoice"
	}
	return "accountsstmt.excludesByChoicePlural"
}

// moneyFromMajor rebuilds a money.Money from a major-unit float (the engine variables
// from liveEngineVars), rounding to the currency's minor unit. It is for display only,
// never for ledger arithmetic.
func moneyFromMajor(v float64, base string) money.Money {
	mult := math.Pow10(currency.Decimals(base))
	return money.New(int64(math.Round(v*mult)), base)
}

// acctIdleCashLine renders the idle-cash summary: liquid cash beyond this month's
// committed bills and goal set-asides, priced at the user's own benchmark rate. It is
// a quiet line linking to /allocate. It renders nothing until a benchmark is set
// (Settings, then Preferences) and the idle amount clears idlecash.DefaultThresholdMinor,
// so a small cash cushion never reads as a nag.
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
	v := accountview.Liquidity(app.Accounts(), accountSnapshot(app).Balances, rates, base, time.Now())
	totals := v.Totals
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
	if nonzero < 2 && !v.Incomplete {
		return Fragment()
	}
	kids := []any{css.Class("t-caption", tw.TextDim), Attr("data-testid", "acct-liquidity-line"),
		css.Class(css.New(css.Display.Flex, tw.FlexWrap, css.Gap(css.Length("0.25rem 1rem")))),
		Title(uistate.T("accounts.liqTitle"))}
	for _, it := range items {
		if totals[it.cls] == 0 {
			continue
		}
		kids = append(kids, Span(Attr("data-testid", it.testid),
			uistate.T(it.key, fmtMoney(money.New(totals[it.cls], base)))))
	}
	if v.Incomplete {
		kids = append(kids, Span(Attr("data-testid", "acct-liquidity-incomplete"), uistate.T("accounts.liquidityIncomplete")))
	}
	return P(kids...)
}

// --- acct-toolbar ---------------------------------------------------------------

// acctToolbarGlyph renders one accounts-toolbar action as a labeled button in the
// shared .btn-tool style. It has a leading glyph and a visible text label, so the
// action reads without hovering. kind "modal" marks a button that opens a dialog and
// adds the matching ARIA attributes. "action" acts in place. variant tints the button:
// "" is neutral, "primary" uses the accent color, and "stale" uses amber.
func acctToolbarGlyph(testID string, ic icon.Name, label, title, kind, variant string, onClick ui.Handler) ui.Node {
	// The tooltip explains what the action means when the label alone is ambiguous, as
	// with "Mark all updated". An empty title falls back to the label. The accessible
	// name is always the label.
	if title == "" {
		title = label
	}
	extra := []any{OnClick(onClick)}
	if kind == "modal" {
		extra = append(extra, Attr("aria-haspopup", "dialog"), Attr("aria-expanded", "false"))
	}
	return uiw.ToolButton(uiw.ToolButtonProps{
		Label: label, Icon: ic, Title: title, AriaLabel: label, Variant: variant, TestID: testID,
	}, extra...)
}

// markAllStalePrompt is the shared "Mark all updated" action. It collects the
// non-archived stale accounts and names them in the confirm, including how many
// balances it will touch. On confirm it stamps every balance as updated now, as one
// undoable batch. It holds no hooks, so any component can call it from a UseEvent,
// and it does nothing when no account is stale. The toolbar and the account list's
// stale summary both use it, so the two stay in step.
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
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	setFilter := func(mut func(*uistate.AccountsFilter)) {
		nf := filterAtom.Get()
		mut(&nf)
		filterAtom.Set(nf.Normalize())
	}
	onSearch := func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.Search = v }) }

	// Menu items take plain funcs (the item owns its click hook), so the transfer opener is one too.
	openTransfer := func() { transferAtom.Set(true) }
	// Open the account-group manager. When a group exists, jump straight to editing the
	// first one (each group header also has its own edit button). With none, start a new
	// group. These are plain funcs, not UseEvent, because the Manage menu's item owns the
	// click hook.
	groupEditAtom := uistate.UseAccountGroupEdit()
	openGroups := func() {
		target := "new"
		if gs := app.AccountGroups(); len(gs) > 0 {
			target = gs[0].ID
		}
		groupEditAtom.Set(target)
	}
	// Open the institution directory, where banks and lenders are added, renamed, colored or deleted.
	institutionsAtom := uistate.UseInstitutionsManager()
	openInstitutions := func() { institutionsAtom.Set(true) }
	// Open the surplus-sweep rules manager.
	sweepAtom := uistate.UseSweepRulesOpen()
	openSweep := func() { sweepAtom.Set(true) }
	openFX := func() { uistate.OpenGlobalSettingsAt("household") }
	// Marking every balance updated is a high-impact bulk write. The confirm names the
	// balances it will touch, and the change is undoable as one batch, so a single Undo
	// restores every balance. The shared markAllStalePrompt runs the action, so the
	// stale summary in the account list does exactly the same thing.
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
	typeOpts := make([]uiw.PickerOption, 0, len(types))
	for _, t := range types {
		typeOpts = append(typeOpts, uiw.PickerOption{Value: t, Label: humanizeType(t)})
	}
	typeSummary := uistate.T("accounts.allTypes")
	switch n := len(f.Types); {
	case n > 2:
		typeSummary = plural(n, "type")
	case n > 0:
		names := make([]string, 0, n)
		for _, t := range f.Types {
			names = append(names, humanizeType(t))
		}
		typeSummary = strings.Join(names, ", ")
	}
	toggleType := func(v string) { setFilter(func(x *uistate.AccountsFilter) { *x = x.ToggleType(v) }) }

	archivedValue := "hide"
	if f.ShowArchived {
		archivedValue = "show"
	}
	filtersBody := Div(css.Class("filter-fields"),
		withFieldLabel(uistate.T("accounts.filterType"), uiw.MultiPicker(uiw.MultiPickerProps{
			ID: "acct-filter-type", Label: uistate.T("accounts.filterType"), Placeholder: typeSummary,
			Options: typeOpts, Selected: f.Types, OnToggle: toggleType, TestID: "acct-type-picker",
		})),
		withFieldLabel(uistate.T("accounts.archived"), uiw.RadioGroup(uiw.RadioGroupProps{
			Label: uistate.T("accounts.archived"), Value: archivedValue,
			Options: []uiw.RadioOption{
				{Value: "hide", Label: uistate.T("accounts.hideArchived"), Icon: icon.EyeOff, TestID: "acct-archived-hide"},
				{Value: "show", Label: uistate.T("accounts.showArchived"), Icon: icon.Eye, TestID: "acct-archived-show"},
			},
			OnChange: func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.ShowArchived = v == "show" }) },
		})),
	)

	chips := []uiw.Chip{}
	if f.Search != "" {
		chips = append(chips, uiw.Chip{Key: "search", Label: uistate.T("accounts.chipSearch", f.Search)})
	}
	for _, t := range f.Types {
		chips = append(chips, uiw.Chip{Key: "type:" + t, Label: uistate.T("accounts.chipType", humanizeType(t))})
	}
	removeChip := func(key string) { filterAtom.Set(f.Without(key).Normalize()) }
	clearAll := func() { filterAtom.Set(uistate.AccountsFilter{ShowArchived: f.ShowArchived}.Normalize()) }

	showFX := hasForeign || len(app.Settings().FXRates) > 0

	// Mark all updated appears only while some balances are stale. An empty actions row
	// would still reserve a gap under the search, so the list is built only when it has an entry.
	var staleActions []ui.Node
	if staleCount > 0 {
		staleActions = append(staleActions, acctToolbarGlyph("acct-markall-btn", icon.Refresh,
			uistate.T("accounts.markAll"), uistate.T("acctxn.markAllMeaning"), "action", "stale", markAll))
	}
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
		// The More menu sits on the search row, beside Filters, so the setup actions are
		// one click away from where the list is filtered.
		PrimaryActions: []ui.Node{
			If(len(accounts) > 0, uiw.OverflowMenu(uiw.OverflowMenuProps{
				TriggerText:   uistate.T("acctxn.acctSettingsMenu"),
				TriggerLabel:  uistate.T("acctxn.acctSettingsMenuTitle"),
				TriggerTestID: "acct-manage-btn",
				Items: []uiw.OverflowMenuItem{
					{Label: uistate.T("accounts.transferMoney"), Icon: icon.Repeat, TestID: "page-transfer-btn", OnSelect: openTransfer, Hidden: len(accounts) < 2},
					// The item's label matches what clicking does. With no
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
		},
		Actions: staleActions,
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
// other side, so the same account cannot be picked for both.
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
	// One shared eligibility rule for both transfer forms (liquid sources; no
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
		AriaLabel(uistate.T("accounts.transferFormLabel")), OnSubmit(submit),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.transferFromLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: pfrom, OnChange: func(v string) { fromS.Set(v) },
					AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "page-xfer-from-select"})),
			labeledField(uistate.T("accounts.transferToLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: pto, OnChange: func(v string) { toS.Set(v) },
					AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "page-xfer-to-select"})),
			If(sameAcct, P(css.Class("err"), Role("alert"), uistate.T("accounts.transferSameAccountErr"))),
			labeledField(uistate.T("accounts.transferAmount"),
				Input(css.Class("field"), Attr("id", "page-xfer-amt"), Attr("data-testid", "page-xfer-amt"), Attr("autofocus", ""),
					Type("number"), Placeholder(uistate.T("accounts.transferAmount")),
					Step("0.01"), Attr("min", "0.01"), OnInput(onAmt), uiw.FieldValue(amtS.Get()))),
			// Cross-currency: say what the amount means, with a live converted
			// preview at the saved rate, or a no-rate warning before anything posts.
			acctTransferFXNote(app, pfrom, pto, amtS.Get()),
			// Cross-currency only: the actual landed amount, overriding the saved
			// rate; hidden for same-currency pairs where it can't apply.
			If(crossCcy, Fragment(
				labeledField(uistate.T("accounts.transferReceivedLabel", toCcy),
					Input(css.Class("field"), Attr("data-testid", "page-xfer-received"),
						Type("number"), Step("0.01"), Attr("min", "0.01"),
						Title(uistate.T("accounts.transferReceivedHint")), OnInput(onRecv), uiw.FieldValue(recvS.Get()))),
				If(!recvOK, P(css.Class("err"), Role("alert"), uistate.T("accounts.transferInvalidExtra"))),
			)),
			If(fromCcy != "", Fragment(
				labeledField(uistate.T("accounts.transferFeeLabel", fromCcy),
					Input(css.Class("field"), Attr("data-testid", "page-xfer-fee"),
						Type("number"), Step("0.01"), Attr("min", "0.01"),
						Title(uistate.T("accounts.transferFeeHint")), OnInput(onFee), uiw.FieldValue(feeS.Get()))),
				If(!feeOK, P(css.Class("err"), Role("alert"), uistate.T("accounts.transferInvalidExtra"))),
			)),
			// Before/after balances for both sides, straight from the same leg
			// math the post will use (PreviewTransferPair) — including the typed
			// received override and fee.
			acctTransferBalancePreview(app, pfrom, pto, amtS.Get(), recvS.Get(), feeS.Get()),
			labeledField(uistate.T("accounts.transferDateLabel"),
				Input(css.Class("field"), Type("date"), AriaLabel(uistate.T("accounts.transferDateLabel")), OnInput(onDate), uiw.FieldValue(dateS.Get()))),
			labeledField(uistate.T("accounts.transferDescLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.transferDefaultDesc")), OnInput(onDesc), uiw.FieldValue(descS.Get()))),
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

// acctListWidget is the accounts-list tile: the owner-scoped, search and type-filtered
// account rows, rendered as AccountRow inside an EntityListSection. A segmented
// "All / Assets / Liabilities" toggle in the section header narrows the list by class.
// Rows sort risk-first: accounts that need attention lead, and within each group the
// signed balance decides the order. The tile owns the per-row callbacks and the
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
	// The stale-summary line's "Mark all updated" (shown when most accounts are
	// stale) reuses the toolbar's exact action. Declared unconditionally so the hook
	// position stays stable even when the summary line isn't rendered.
	markAllStale := ui.UseEvent(Prevent(func() { markAllStalePrompt(app) }))

	accounts := app.Accounts()

	snapshot := accountSnapshot(app)
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
	// Services are recurring bills, not accounts. They have their own class view.
	serviceList := app.Services()
	hasServices := len(serviceList) > 0

	// The class view. Without liabilities the toggle has nothing to switch between, so the page
	// falls back to the assets-only view, as before. The Services view stays on once it is chosen,
	// even before the household has any service, so its empty state can be seen.
	classView := f.Class
	if classView != serviceClassView && !hasLiab {
		classView = string(domain.ClassAsset)
	}
	inServices := classView == serviceClassView
	assetsOnly := classView == string(domain.ClassAsset)

	var shown []domain.Account
	for _, ac := range active {
		if inServices {
			break
		}
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
	// Risk-first ordering: accounts that need attention (stale, or past their freshness
	// window) lead the list, so the rows worth acting on sit at the top instead of being
	// buried under large healthy balances. Within each freshness tier, accounts run from
	// the biggest signed balance down, so the heaviest debts trail. The comparison is the
	// pure acctsort.RiskFirstLess, which has native tests. Here we only supply the stale
	// flag and the signed balance.
	windows := app.FreshnessWindows()
	now := time.Now()
	isStale := func(ac domain.Account) bool { return freshness.IsStale(ac, windows, now) }
	accountview.SortRiskFirst(shown, snapshot.Balances, windows, props.Rates, props.Base, now)

	// Stale-badge prioritisation collapse: once more than half the visible accounts are
	// stale, a per-row STALE badge on nearly every line ranks nothing — so collapse the
	// badges to subdued dots and lead the list with one summary line + Mark-all instead.
	// Below the threshold each row keeps its full badge.
	staleShown := 0
	for _, ac := range shown {
		if isStale(ac) {
			staleShown++
		}
	}
	staleCollapsed := len(shown) > 0 && staleShown*2 > len(shown)

	smartSettings := uistate.LoadSmartSettings()
	pr := uistate.UsePrefs().Get()
	smartIn := buildSmartInput(app, pr.WeekStartWeekday())
	accountInsights := accountsPageInsights(app, smartIn, smartSettings, pr.WeekStartWeekday())
	accountByEntity := insightsByEntity(accountInsights)

	categories := app.Categories()
	members := app.Members()
	accDefs := app.CustomFieldDefsFor("account")
	cbs := buildAcctRowCallbacks(app)
	viewTxns := acctViewTransactions(nav, txFilter)
	viewBills := acctViewBillPayments(nav, txFilter)
	householdDefaultID := app.DefaultAccountID()
	// Index the institution directory once, so each row can color its edge
	// and show a chip for its Account.InstitutionID.
	instByID := domain.InstitutionByID(app.Institutions())

	// Share the existing ledger index. Compact rows need balances only; their
	// chart windows, flows and recurring projections are prepared on disclosure.
	detailContext := newAccountDetailContext(app, snapshot, props.Rates, props.Base, now)

	renderRow := func(ac domain.Account) ui.Node {
		defaultMemberID, defaultMemberName := accountDefaultMemberTarget(ac, members, activeMemberID)
		var defaultMemberIDs []string
		if !ac.Archived {
			for _, member := range members {
				if member.Prefs.DefaultAccountID == ac.ID {
					defaultMemberIDs = append(defaultMemberIDs, member.ID)
				}
			}
		}
		bal := snapshot.Balances[ac.ID]
		cleared := snapshot.Cleared[ac.ID]
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: isStale(ac), DaysStale: freshness.DaysSinceUpdate(ac, now),
			StaleCollapsed: staleCollapsed,
			Members:        members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			DefaultHousehold: !ac.Archived && householdDefaultID == ac.ID, DefaultMembers: defaultMemberIDs,
			DefaultMemberID: defaultMemberID, DefaultMemberName: defaultMemberName,
			OnSetDefaultHousehold: cbs.OnSetDefaultHousehold, OnClearDefaultHousehold: cbs.OnClearDefaultHousehold,
			OnSetDefaultMember:   cbs.OnSetDefaultMember,
			OnClearDefaultMember: cbs.OnClearDefaultMember,
			SmartSettings:        smartSettings, SmartByEntity: accountByEntity,
			AccountDefs: accDefs, InstByID: instByID,
			BillPayment: snapshot.BillPayments[ac.ID], OnViewBills: viewBills,
			DetailContext: detailContext,
		})
	}
	// User-defined groups render as sections, each with a net subtotal, followed
	// by an "Ungrouped" catch-all. Built only for the "default" body case below.
	groups := app.AccountGroups()
	groupedBody := func() ui.Node {
		sections := groupSections(shown, groups, snapshot.Balances, props.Rates, props.Base)
		return Div(css.Class("acct-groups"),
			MapKeyed(sections, func(s acctGroupSection) any {
				if s.Group.ID == "" {
					return "ungrouped"
				}
				return s.Group.ID
			}, func(s acctGroupSection) ui.Node {
				// Each section collapses to just its header (name and subtotal). The
				// collapsed state is read fresh here and persisted per group. Collapsed is
				// threaded into the header prop so the memoized header re-renders on change.
				collapsed := isAcctGroupCollapsed(s.Group.ID)
				groupCls := "acct-group"
				if collapsed {
					groupCls += " is-collapsed"
				}
				return Div(ClassStr(groupCls),
					ui.CreateElement(acctGroupHeader, acctGroupHeaderProps{Section: s, Collapsed: collapsed}),
					If(!collapsed, Div(css.Class("rows"), MapKeyed(s.Accounts, accountRowKey, renderRow))),
				)
			}),
		)
	}

	bodyContent := ui.Match().
		When(inServices, func() ui.Node {
			return servicesBody(app, serviceList, props.Base, props.Rates)
		}).
		When(len(active) == 0, func() ui.Node {
			return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("accounts.noAssets"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account", Icon: icon.Accounts, ImportLink: true})
		}).
		When(len(shown) == 0 && f.HasNarrowing(), func() ui.Node {
			return P(css.Class("empty"), uistate.T("accounts.noMatch"))
		}).
		When(len(shown) == 0 && classView == string(domain.ClassLiability), func() ui.Node {
			return P(css.Class("empty"), uistate.T("accounts.noLiabilities"))
		}).
		When(len(shown) == 0, func() ui.Node {
			return P(css.Class("empty"), uistate.T("accounts.noAssets"))
		}).
		When(len(groups) > 0, groupedBody).
		Default(func() ui.Node {
			return Div(css.Class("rows"), MapKeyed(shown, accountRowKey, renderRow))
		})

	// Collapsed-stale state: one summary line leads the list, its Mark-all reusing the
	// toolbar action. Prepended so it sits above the (grouped or flat) rows.
	if staleCollapsed {
		bodyContent = Fragment(
			Div(css.Class("acct-stale-summary"), Attr("data-testid", "acct-stale-summary"), Role("status"),
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

	classSeg := ui.If(hasLiab || hasServices || inServices, func() ui.Node {
		return uiw.RadioGroup(uiw.RadioGroupProps{
			Label: uistate.T("accounts.classFilterLabel"),
			Value: classView,
			Options: []uiw.RadioOption{
				{Value: "", Label: uistate.T("accounts.classAll"), TestID: "acct-class-all"},
				{Value: string(domain.ClassAsset), Label: uistate.T("accounts.classAssets"), TestID: "acct-class-assets"},
				{Value: string(domain.ClassLiability), Label: uistate.T("accounts.classLiabilities"), TestID: "acct-class-liabilities"},
				{Value: serviceClassView, Label: uistate.T("accounts.classServices"), TestID: "acct-class-services"},
			},
			OnChange: func(v string) {
				nf := filterAtom.Get()
				nf.Class = v
				filterAtom.Set(nf.Normalize())
				// The class view is a viewer preference, not household data, so saving it does not
				// touch the dataset or schedule a save.
				uistate.PersistAcctClass(v)
			},
		})
	})

	if inServices {
		title = uistate.T("accounts.servicesTitle")
	}
	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: title,
		// The class filter (All / Assets / Liabilities / Services) is the header action; the old
		// "Smart" sparkle shortcut was dropped here as noise (the Smart hub is one nav click away
		// and the page already carries the inline Smart strip up top). Adding is in the summary's Add menu.
		HeaderAction: classSeg,
		Body:         bodyContent,
	})

	// A shortcut to /debt, where liabilities carry the richer payoff surface (min
	// payment, utilization, payoff order). When the assets-only view hides them, the
	// link names how many are tucked away; otherwise it is a plain shortcut.
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
			uiw.ToolButton(uiw.ToolButtonProps{
				Label: debtLinkText, Icon: icon.CreditCard, Href: uistate.RoutePath("/debt"), TestID: debtLinkTestID,
			},
				OnClick(goToDebt),
			),
		),
	)

	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(section, debtLink),
	})
}

// accountDefaultMemberTarget picks the member a row's "default account for member"
// action applies to: the member in the active scope when there is one, otherwise the
// account's owner. It returns empty strings when neither is found.
func accountDefaultMemberTarget(account domain.Account, members []domain.Member, activeID string) (string, string) {
	want := strings.TrimSpace(activeID)
	for _, member := range members {
		if want != "" && member.ID == want {
			return member.ID, member.Name
		}
	}
	want = strings.TrimSpace(account.OwnerID)
	for _, member := range members {
		if member.ID == want {
			return member.ID, member.Name
		}
	}
	return "", ""
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
	snapshot := accountSnapshot(app)
	_, archived := accountview.Assets(accounts, snapshot.Balances, props.Rates, props.Base, activeMemberID)

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
		bal := snapshot.Balances[ac.ID]
		cleared := snapshot.Cleared[ac.ID]
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now), DaysStale: freshness.DaysSinceUpdate(ac, now),
			Members: members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			ValuationHistory: app.BalanceHistory(ac.ID), AccountDefs: accDefs, InstByID: instByID,
			BillPayment: snapshot.BillPayments[ac.ID], OnViewBills: viewBills,
		})
	}
	bodyContent := ui.If(len(shown) == 0, func() ui.Node {
		return P(css.Class("empty"), uistate.T("accounts.noMatch"))
	}, func() ui.Node {
		return Div(css.Class("rows"), MapKeyed(shown, accountRowKey, renderRow))
	})
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
		P(css.Class("t-caption", tw.TextDim), css.Class(css.New(css.Margin(css.Length("0 0 0.5rem")))), uistate.T("accounts.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("accounts.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
