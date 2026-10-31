// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/idlecash"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
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

// acctActiveMemberID resolves the single-owner filter from the top-bar active
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
	OnDelete  func(string)
	OnArchive func(domain.Account)
	// OnRefresh marks the account's balance as confirmed today and posts a notice. OnRerender only
	// redraws the surface; the reconcile panel uses it after a change that needs no stamp.
	OnRefresh               func(domain.Account)
	OnRerender              func(domain.Account)
	OnSave                  func(domain.Account) bool
	OnSetBalance            func(ac domain.Account, newBalStr, catID, asOfISO string) bool
	OnTransfer              func(fromID, toID, amountStr, dateStr, desc string) bool
	OnSetDefaultHousehold   func(string)
	OnClearDefaultHousehold func()
	OnSetDefaultMember      func(accountID, memberID string)
	OnClearDefaultMember    func(memberID string)
}

// deleteTemplatesFor removes the quick-add templates that post to the account, so a deleted account
// leaves no favourite that points at it. The ids are gathered first because deleting rewrites the list
// being read.
func deleteTemplatesFor(accountID string) {
	var ids []string
	for _, t := range uistate.TxnTemplates() {
		if t.AccountID == accountID {
			ids = append(ids, t.ID)
		}
	}
	for _, id := range ids {
		uistate.DeleteTxnTemplate(id)
	}
}

// buildAcctRowCallbacks wires the store mutations for AccountRow. Errors surface
// through the global notice/toast (PostNotice) rather than a screen-local error
// state, matching the rest of the widgetized surfaces.
func buildAcctRowCallbacks(app *appstate.App) acctRowCallbacks {
	return acctRowCallbacks{
		OnDelete: func(accountID string) {
			// Refuse to delete an account that still has transactions, including the far
			// leg of a transfer, because deleting it would orphan them. Archiving keeps history.
			if app.AccountHasTransactions(accountID) {
				uistate.PostNotice(uistate.T("accounts.deleteHasTxns"), true)
				return
			}
			name := ""
			for _, ac := range app.Accounts() {
				if ac.ID == accountID {
					name = ac.Name
					break
				}
			}
			// Confirm first, then offer undo. Goals and budgets follow the same rule, so
			// deleting behaves the same on every page. The row that had focus is noted before
			// the modal opens, because opening it moves focus away.
			restoreFocus := captureRowFocus(".rows", ".row")
			uistate.ConfirmModal(uistate.T("accounts.deleteConfirm", name), true, func(ok bool) {
				if !ok {
					return
				}
				if err := app.DeleteAccount(accountID); err != nil {
					postAcctWriteError("delete account", err)
					return
				}
				deleteTemplatesFor(accountID)
				uistate.BumpDataRevision()
				restoreFocus()
				auditview.CaptureNow()
				uistate.PostUndoable(uistate.T("toast.accountDeleted"))
			})
		},
		OnArchive: func(ac domain.Account) {
			ac.Archived = !ac.Archived
			if err := app.PutAccount(ac); err != nil {
				postAcctWriteError("archive account", err)
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
			if _, err := app.MarkBalancesUpdated([]string{ac.ID}, time.Now()); err != nil {
				postAcctWriteError("mark balance updated", err)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("accounts.markUpdatedToast", ac.Name), false)
		},
		OnRerender: func(domain.Account) { uistate.BumpDataRevision() },
		OnSave: func(ac domain.Account) bool {
			if err := app.PutAccount(ac); err != nil {
				postAcctWriteError("save account", err)
				return false
			}
			uistate.BumpDataRevision()
			return true
		},
		OnSetDefaultHousehold: func(accountID string) {
			if err := app.SetDefaultAccountID(accountID); err != nil {
				postAcctWriteError("set default account", err)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.householdSet"), false)
		},
		OnClearDefaultHousehold: func() {
			if err := app.ClearDefaultAccountID(); err != nil {
				postAcctWriteError("clear default account", err)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.householdCleared"), false)
		},
		OnSetDefaultMember: func(accountID, memberID string) {
			name, err := app.SetMemberDefaultAccount(memberID, accountID)
			if err != nil {
				postMemberDefaultError(err)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.memberSet", name), false)
		},
		OnClearDefaultMember: func(memberID string) {
			name, err := app.ClearMemberDefaultAccount(memberID)
			if err != nil {
				postMemberDefaultError(err)
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("defaultAccount.memberCleared", name), false)
		},
		OnSetBalance: func(ac domain.Account, newStr, catID, asOfISO string) bool {
			typed, err := money.ParseMinor(strings.TrimSpace(newStr), currency.Decimals(ac.Currency))
			if err != nil {
				uistate.PostNotice(uistate.T("accounts.invalidBalance"), true)
				return false
			}
			// The date the balance was true as of. A figure read on Friday and entered on Monday
			// belongs to Friday; a blank one means today, and a date that does not parse is refused.
			asOf, asOfOK := balanceAsOfDate(asOfISO, time.Now())
			if !asOfOK {
				uistate.PostNotice(uistate.T("accounts.setBalanceAsOfInvalid"), true)
				return false
			}
			// The account's own edits are saved first, so the preview reads the account as it now is.
			if err := app.PutAccount(ac); err != nil {
				postAcctWriteError("save account before balance update", err)
				return false
			}
			// The account's own edits are stored from here on, so the surface redraws even when
			// the balance step below fails.
			uistate.BumpDataRevision()
			// The difference is posted as one provisional balance checkpoint, which replaces any
			// earlier checkpoint for the account in the same atomic commit. It moves the balance but
			// stays out of reports, because the money was not earned or spent.
			preview, err := app.PreviewAccountReconciliation(ac.ID, typed, asOf)
			if err != nil {
				postAcctWriteError("preview balance update", err)
				return false
			}
			if _, err := app.CommitAccountReconciliation(preview.WithCategory(catID), uistate.T("accounts.balanceCheckpointDesc")); err != nil {
				postAcctWriteError("commit balance update", err)
				return false
			}
			uistate.BumpDataRevision()
			// A debt with no opening balance has no recorded sign convention, so the sign it
			// was entered in was inferred from the booked balance. The toast says so.
			_, conventionKnown := reconcile.ConventionOf(ac, accountSnapshot(app).Balances[ac.ID].Amount)
			key := "accounts.balanceUpdated"
			if !conventionKnown {
				key = "accounts.balanceUpdatedGuessed"
			}
			// The toast offers undo.
			uistate.PostUndoable(uistate.T(key, ac.Name, fmtMoney(money.New(typed, ac.Currency))))
			return true
		},
		OnTransfer: func(fromID, toID, amountStr, dateStr, desc string) bool {
			return doAccountTransfer(app, fromID, toID, amountStr, "", "", dateStr, desc)
		},
	}
}

// postMemberDefaultError tells the user why a member default could not be saved.
func postMemberDefaultError(err error) {
	switch {
	case errors.Is(err, appstate.ErrAccountNotFound), errors.Is(err, appstate.ErrDefaultAccountArchived):
		uistate.PostNotice(uistate.T("defaultAccount.archivedError"), true)
	case errors.Is(err, appstate.ErrMemberNotFound):
		uistate.PostNotice(uistate.T("defaultAccount.memberMissing"), true)
	default:
		postAcctWriteError("set member default account", err)
	}
}

// acctTransferOptions builds the From/To pickers for both transfer forms from the
// shared eligibility rule in accountselect.TransferCandidates. A transfer TO a
// liability is a payment, and the option label says so.
func acctTransferOptions(accounts []domain.Account, fromID, toID string) (fromOpts, toOpts []uiw.SelectOption) {
	from, to := accountselect.TransferCandidates(accounts, fromID, toID)
	fromOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
	toOpts = []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
	for _, c := range from {
		fromOpts = append(fromOpts, uiw.SelectOption{Value: c.ID, Label: c.Label})
	}
	for _, c := range to {
		label := c.Label
		if c.LiabilityPayment {
			label += uistate.T("accounts.transferPaymentSuffix")
		}
		toOpts = append(toOpts, uiw.SelectOption{Value: c.ID, Label: label})
	}
	return fromOpts, toOpts
}

// acctTransferFXNote shows the cross-currency meaning of a transfer before it posts.
// The amount is in the source currency. It lands converted at the saved rate, with a
// live ≈ preview of the typed amount. When no rate exists, a warning says it would land
// unconverted and points to Manage exchange rates. Renders nothing for same-currency
// pairs or incomplete picks.
func acctTransferFXNote(app *appstate.App, fromID, toID, amountStr string) ui.Node {
	if app == nil {
		return Fragment()
	}
	_, rates := accountBaseAndRates(app)
	note := accountselect.TransferFXNote(app.Accounts(), rates, fromID, toID, amountStr)
	switch note.Kind {
	case accountselect.FXNoRate:
		return uiw.Callout(uiw.CalloutProps{
			Tone: uiw.CalloutDanger, Live: uiw.CalloutLiveAlert, TestID: "xfer-fx-norate",
			Body: Span(uistate.T("accounts.fxNoteNoRate", note.From, note.To)),
		})
	case accountselect.FXConverted:
		return uiw.Callout(uiw.CalloutProps{
			Tone: uiw.CalloutInfo, TestID: "xfer-fx-note",
			Body: Span(uistate.T("accounts.fxNoteAmountIn", note.From, note.To, fmtMoney(note.Landed))),
		})
	default:
		return Fragment()
	}
}

// acctTransferBalancePreview shows what the transfer would do to both booked balances
// before it posts, such as "Checking: $1,000.00 → $950.00" for each side. It uses
// appstate.PreviewTransferPair, which shares the exact leg math (FX conversion and the
// liability payment sign) with the real post. Renders nothing until both accounts are
// picked and a valid positive amount is typed. receivedStr and feeStr are the optional
// landed-amount override (destination currency) and fee (source currency). Pass ""
// for none. Invalid text counts as absent here, since the form gates submit separately.
func acctTransferBalancePreview(app *appstate.App, fromID, toID, amountStr, receivedStr, feeStr string) ui.Node {
	if app == nil {
		return Fragment()
	}
	pending, ok := accountselect.PreviewTransfer(app.Accounts(), fromID, toID, amountStr, receivedStr, feeStr)
	if !ok {
		return Fragment()
	}
	preview, err := app.PreviewTransferPair(pending.Params)
	if err != nil {
		return Fragment()
	}
	// One plain sentence states what each side does and that neither leg is spending.
	// A liability destination reads as paying down what is owed, not as an increase.
	amtStr := money.FormatMinor(pending.Params.AmountMinor, currency.Decimals(accountselect.CurrencyOf(app.Accounts(), fromID)))
	semanticsKey := "accounts.xferSemanticsAsset"
	if pending.ToLiability {
		semanticsKey = "accounts.xferSemanticsLiability"
	}
	return uiw.Callout(uiw.CalloutProps{
		Tone: uiw.CalloutInfo, TestID: "xfer-balance-preview",
		Body: Div(css.Class("acct-xfer-preview"),
			Span(uistate.T("accounts.xferPreviewTitle")),
			Span(Attr("data-testid", "xfer-preview-from"),
				uistate.T("accounts.xferPreviewLine", pending.FromName, fmtMoney(preview.FromBefore), fmtMoney(preview.FromAfter))),
			Span(Attr("data-testid", "xfer-preview-to"),
				uistate.T("accounts.xferPreviewLine", pending.ToName, fmtMoney(preview.ToBefore), fmtMoney(preview.ToAfter))),
			Span(css.Class(tw.TextDim), Attr("data-testid", "xfer-semantics"),
				uistate.T(semanticsKey, pending.FromName, amtStr, pending.ToName)),
		),
	})
}

// amountStep is the step of a money input in the currency: one minor unit, so the field accepts
// exactly the precision the currency has.
func amountStep(ccy string) string {
	dec := currency.Decimals(ccy)
	if dec <= 0 {
		return "1"
	}
	return "0." + strings.Repeat("0", dec-1) + "1"
}

// acctTransferAmountOK reports whether amountStr parses as a positive amount in the
// source account's minor units. Both transfer forms use it to decide whether submit is
// enabled, so a blank amount never enables the button.
func acctTransferAmountOK(app *appstate.App, fromID, amountStr string) bool {
	return accountselect.AmountOK(app.Accounts(), fromID, amountStr)
}

// doAccountTransfer creates a transfer pair from the page or row transfer forms and reports
// whether it posted, so a form can stay open with its input when it did not. It has no hooks.
// accountselect.ParseTransfer validates the amounts and the date and fills in the description when
// it is blank, then the revision is bumped. receivedStr and feeStr are the optional cross-currency
// landed amount (destination currency) and transfer fee (source currency). Blank means none.
func doAccountTransfer(app *appstate.App, fromID, toID, amountStr, receivedStr, feeStr, dateStr, desc string) bool {
	params, err := accountselect.ParseTransfer(app.Accounts(), accountselect.TransferInput{
		FromID: fromID, ToID: toID, Amount: amountStr, Received: receivedStr, Fee: feeStr,
		Date: dateStr, Desc: desc, DefaultDesc: uistate.T("accounts.transferDefaultDesc"),
	})
	if err != nil {
		uistate.PostNotice(transferFormError(err), true)
		return false
	}
	if _, _, err := app.CreateTransferPair(params); err != nil {
		postAcctWriteError("transfer", err)
		return false
	}
	uistate.BumpDataRevision()
	// A transfer posts two real transactions, so the toast offers one-click undo.
	uistate.PostUndoable(uistate.T("accounts.transferDone"))
	return true
}

// transferFormError is the user-facing text for a transfer form that did not validate.
func transferFormError(err error) string {
	switch {
	case errors.Is(err, accountselect.ErrTransferAmount):
		return uistate.T("accounts.transferInvalidAmount")
	case errors.Is(err, accountselect.ErrTransferExtra):
		return uistate.T("accounts.transferInvalidExtra")
	case errors.Is(err, accountselect.ErrTransferDate):
		return uistate.T("accounts.transferInvalidDate")
	case errors.Is(err, accountselect.ErrTransferSameAccount):
		return uistate.T("accounts.transferSameAccountErr")
	case errors.Is(err, accountselect.ErrTransferUnknownAccount):
		return uistate.T("accounts.transferUnknownAccount")
	case errors.Is(err, accountselect.ErrTransferAccountNotAllowed):
		return uistate.T("accounts.transferNotAllowed")
	default:
		return acctWriteErrorText(err)
	}
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
	mtdKnown := nw.Complete()
	if nw.Complete() {
		var err error
		if mtd, err = nwchange.MonthToDateChange(accounts, txns, props.Rates, time.Now()); err != nil {
			slog.Warn("accounts: month-to-date net-worth change is unavailable", "err", err)
			mtdKnown = false
		}
	}
	deltaMinor, deltaErr := mtd.DeltaMinorChecked()
	nwDelta, haveDelta := money.New(deltaMinor, props.Base), mtdKnown && deltaErr == nil
	netText, assetsText, liabilitiesText, tone := "—", "—", "—", tw.ColorClass("text-dim")
	// The liabilities figure reads as a negative amount: it is what the debts take off net worth.
	if nw.Known {
		netText, assetsText, liabilitiesText, tone = fmtMoney(net), fmtMoney(assets), fmtMoney(money.New(reconcile.StatedOwed(liabilities.Amount), liabilities.Currency)), accentFor(net)
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

	body := Div(css.Class("acct-summary-body"),
		Div(css.Class("acct-summary-frame"),
			// One Add control for both things the page can add. Each item names what it creates. It sits
			// pinned to the summary's top-right corner (see rules_pageheadline.go) as the green main action.
			Div(css.Class("acct-add-menu"),
				uiw.OverflowMenu(uiw.OverflowMenuProps{
					TriggerText:    uistate.T("accounts.addMenu"),
					TriggerLabel:   uistate.T("accounts.addMenuTitle"),
					TriggerTestID:  "accounts-add",
					TriggerVariant: "primary",
					TriggerIcon:    icon.Plus,
					Items: []uiw.OverflowMenuItem{
						{Label: uistate.T("accounts.addAccountItem"), Icon: icon.Accounts, TestID: "accounts-add-account", OnSelect: func() { uistate.SetAddTarget("account") }},
						{Label: uistate.T("accounts.addServiceItem"), Icon: icon.Repeat, TestID: "accounts-add-service", OnSelect: func() { uistate.SetAddTarget("service") }},
					},
				})),
			acctHeadline(acctHeadlineProps{
				NetText: netText, AssetsText: assetsText, LiabilitiesText: liabilitiesText, NetTone: tone,
				Delta: nwDelta, HaveDelta: haveDelta,
				NetTip: smartTooltipFor(smartSettings, "accounts-net", uistate.T("dashboard.netWorth"), uistate.T("smart.tipAccountsNet")),
				Visual: trendVisual, Footer: acctLiquidityLine(app, props.Base, props.Rates),
			}),
		),
		If(!nw.Complete(), uiw.Callout(uiw.CalloutProps{
			Tone: uiw.CalloutWarn, Live: uiw.CalloutLiveStatus, TestID: "acct-networth-note",
			Body: Span(netWorthAvailabilityNote(nw)),
		})),
		// Disclose accounts the household has left out of net worth.
		If(len(nw.ExcludedByChoice) > 0, uiw.Callout(uiw.CalloutProps{
			Tone: uiw.CalloutInfo, Live: uiw.CalloutLiveStatus, TestID: "acct-nw-excludes-by-choice",
			Body: Span(uistate.T(excludesByChoiceKey(len(nw.ExcludedByChoice)), len(nw.ExcludedByChoice))),
		})),
		// A quiet idle-cash line: liquid cash beyond this month's bills and goal set-asides,
		// priced at the user's own benchmark rate (never a live feed). It renders nothing
		// until a benchmark is set in Settings and there is enough idle cash to be worth naming.
		acctIdleCashLine(app, props.Rates, goToAllocate),
		// Where the app's own numbers disagree: a transfer with one leg, or holdings that
		// do not add up to their account's balance. It is absent when the data agrees.
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

// acctIdleCashLine renders the idle-cash summary: liquid cash beyond this month's
// committed bills and goal set-asides, priced at the user's own benchmark rate. It is
// a quiet line linking to /allocate. It renders nothing until a benchmark is set
// (Settings, then Preferences) and the idle amount clears idlecash.DefaultThresholdMinor,
// so a small cash cushion never reads as a nag, and nothing when the figures cannot
// be computed exactly.
func acctIdleCashLine(app *appstate.App, rates currency.Rates, onAllocate ui.Handler) ui.Node {
	benchmark := uistate.LoadPrefs().IdleCashBenchmarkAPR
	if benchmark <= 0 {
		return Fragment()
	}
	res, err := idlecash.FromLedger(idlecash.LedgerInputs{
		Accounts: app.Accounts(), Balances: accountSnapshot(app).Balances,
		Recurring: app.Recurring(), Goals: app.Goals(),
		Rates: rates, Now: time.Now(), BenchmarkAPRPercent: benchmark,
	})
	if err != nil {
		slog.Warn("accounts: idle cash is unavailable", "err", err)
		return Fragment()
	}
	if !res.Flag {
		return Fragment()
	}
	idle := money.New(res.IdleMinor, rates.Base)
	forgone := money.New(res.ForgoneAnnualMinor, rates.Base)
	benchStr := strconv.FormatFloat(res.BenchmarkAPRPercent, 'f', -1, 64)
	return P(css.Class("t-caption", tw.TextDim, tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Attr("data-testid", "acct-idle-cash-line"),
		Span(uistate.T("accounts.idleCashLine", fmtMoney(idle), fmtMoney(forgone), benchStr)),
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-idle-cash-link"), OnClick(onAllocate), uistate.T("accounts.idleCashLink")),
	)
}

// liquidityBucketCopy names the copy and test id of each liquidity bucket.
var liquidityBucketCopy = map[liquidity.Class]struct{ key, testID string }{
	liquidity.Available:   {"accounts.liqAvailable", "acct-liq-available"},
	liquidity.Restricted:  {"accounts.liqRestricted", "acct-liq-restricted"},
	liquidity.Investments: {"accounts.liqInvestments", "acct-liq-investments"},
	liquidity.Held:        {"accounts.liqHeld", "acct-liq-held"},
}

// acctLiquidityLine renders the asset side grouped by how usable the money is right
// now (internal/liquidity), each bucket base-converted. accountview decides which
// buckets show and in what order. Rate-less accounts are skipped, consistent with the
// net-worth exclusion disclosure above.
func acctLiquidityLine(app *appstate.App, base string, rates currency.Rates) ui.Node {
	v := accountview.Liquidity(app.Accounts(), accountSnapshot(app).Balances, rates, base, time.Now())
	if !v.Visible() {
		return Fragment()
	}
	kids := []any{css.Class("t-caption", tw.TextDim, "acct-liquidity"), Attr("data-testid", "acct-liquidity-line"),
		Title(uistate.T("accounts.liqTitle"))}
	for _, b := range v.Buckets() {
		copyOf := liquidityBucketCopy[b.Class]
		kids = append(kids, Span(Attr("data-testid", copyOf.testID),
			uistate.T(copyOf.key, fmtMoney(money.New(b.Amount, base)))))
	}
	if v.Incomplete {
		kids = append(kids, Span(Attr("data-testid", "acct-liquidity-incomplete"), uistate.T("accounts.liquidityIncomplete")))
	}
	return P(kids...)
}

// --- acct-toolbar ---------------------------------------------------------------

// markAllStalePrompt is the shared "Mark all updated" action. It collects the stale
// accounts and names them in the confirm, including how many balances it will touch.
// On confirm it stamps every balance as updated now, as one undoable batch. It holds
// no hooks, so any component can call it from a UseEvent, and it does nothing when no
// account is stale. The toolbar, the account list's stale summary and the net-worth
// quality prompt all use it, so they stay in step.
func markAllStalePrompt(app *appstate.App) {
	stale := freshness.StaleAccounts(app.Accounts(), app.FreshnessWindows(), time.Now())
	if len(stale) == 0 {
		return
	}
	ids := make([]string, 0, len(stale))
	names := make([]string, 0, 3)
	for i, ac := range stale {
		ids = append(ids, ac.ID)
		if i < 3 {
			names = append(names, ac.Name)
		}
	}
	list := strings.Join(names, ", ")
	if len(stale) > 3 {
		list = uistate.T("accounts.markAllMore", list, len(stale)-3)
	}
	confirm := uistate.TN("accounts.markAllConfirmOne", "accounts.markAllConfirmMany", len(stale), list)
	uistate.ConfirmModalLabeled(confirm+" "+uistate.T("acctxn.markAllMeaning"),
		uistate.T("accounts.markAll"), false, func(ok bool) {
			if !ok {
				return
			}
			written, err := app.MarkBalancesUpdated(ids, time.Now())
			if err != nil {
				postAcctWriteError("mark balances updated", err)
			}
			uistate.BumpDataRevision()
			if written > 0 {
				auditview.CaptureNow()
				uistate.PostUndoable(uistate.TN("accounts.markedUpdatedOne", "accounts.markedUpdatedMany", written))
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
	// The transfer and sweep pickers need a valid source and a different valid destination: archived
	// accounts are left out, and property or vehicle accounts can neither send nor receive.
	canTransfer := accountselect.HasTransferPair(accounts)
	base, _ := accountBaseAndRates(app)

	// Type filter options: the types actually present in the scoped set.
	choices := accountview.FilterOptions(accounts, activeMemberID, base, app.FreshnessWindows(), time.Now())
	typeOpts := make([]uiw.PickerOption, 0, len(choices.Types))
	for _, t := range choices.Types {
		typeOpts = append(typeOpts, uiw.PickerOption{Value: t, Label: humanizeType(t)})
	}
	typeSummary := uistate.T("accounts.allTypes")
	switch n := len(f.Types); {
	case n > 2:
		typeSummary = uistate.TN("accounts.typesSelectedOne", "accounts.typesSelectedMany", n)
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
	removeChip := func(key string) { filterAtom.Set(filterAtom.Get().Without(key).Normalize()) }
	clearAll := func() {
		filterAtom.Set(uistate.AccountsFilter{ShowArchived: filterAtom.Get().ShowArchived}.Normalize())
	}

	showFX := choices.HasForeign || len(app.Settings().FXRates) > 0

	// Mark all updated appears only while some balances are stale. An empty actions row
	// would still reserve a gap under the search, so the list is built only when it has an entry.
	// OnClick registers a hook, so it runs on every render whether or not the button shows.
	markAllClick := OnClick(markAll)
	var staleActions []ui.Node
	if choices.StaleCount > 0 {
		staleActions = append(staleActions, uiw.ToolButton(uiw.ToolButtonProps{
			Label: uistate.T("accounts.markAll"), Icon: icon.Refresh, Title: uistate.T("acctxn.markAllMeaning"),
			Variant: "stale", TestID: "acct-markall-btn",
		}, markAllClick))
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
			return uistate.TN("accounts.filtersActiveAriaOne", "accounts.filtersActiveAriaMany", n)
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
					{Label: uistate.T("accounts.transferMoney"), Icon: icon.Repeat, TestID: "page-transfer-btn", OnSelect: openTransfer, Hidden: !canTransfer},
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
					{Label: uistate.T("acctSweepCfg.title"), Icon: icon.TrendingUp, TestID: "acct-sweep-btn", OnSelect: openSweep, Hidden: !canTransfer},
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
	dateS := ui.UseState(time.Now().Format(dateutil.Layout))
	descS := ui.UseState("")
	cancel := ui.UseEvent(Prevent(func() { done() }))
	// The received amount only applies to a cross-currency pair. For any other pair the field is
	// hidden, so whatever it still holds is left out and cannot block the post.
	formInput := func() accountselect.TransferInput {
		received := ""
		if accountselect.IsCrossCurrency(app.Accounts(), fromS.Get(), toS.Get()) {
			received = recvS.Get()
		}
		return accountselect.TransferInput{
			FromID: fromS.Get(), ToID: toS.Get(), Amount: amtS.Get(), Received: received, Fee: feeS.Get(),
			Date: dateS.Get(), Desc: descS.Get(),
		}
	}
	submit := ui.UseEvent(Prevent(func() {
		in := formInput()
		if !accountselect.CanPostTransfer(app.Accounts(), in) {
			return
		}
		if doAccountTransfer(app, in.FromID, in.ToID, in.Amount, in.Received, in.Fee, in.Date, in.Desc) {
			done()
		}
	}))

	pfrom, pto := fromS.Get(), toS.Get()
	// One shared eligibility rule for both transfer forms (liquid sources; no
	// property/vehicle destinations; liabilities labelled as payments).
	accounts := app.Accounts()
	fromOpts, toOpts := acctTransferOptions(accounts, pfrom, pto)
	sameAcct := pfrom != "" && pto != "" && pfrom == pto
	fromCcy, toCcy := accountselect.CurrencyOf(accounts, pfrom), accountselect.CurrencyOf(accounts, pto)
	crossCcy := accountselect.IsCrossCurrency(accounts, pfrom, pto)
	_, recvOK := accountselect.ParseOptionalMinor(recvS.Get(), toCcy)
	recvOK = recvOK || !crossCcy
	_, feeOK := accountselect.ParseOptionalMinor(feeS.Get(), fromCcy)
	// The button stays disabled until the form would actually post.
	submitDisabled := !accountselect.CanPostTransfer(accounts, formInput())

	return Form(css.Class("acct-edit-form", "acct-xfer-form"), Attr("data-testid", "page-transfer-form"),
		AriaLabel(uistate.T("accounts.transferFormLabel")), OnSubmit(submit),
		Div(css.Class("modal-scroll"),
			uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferFromLabel"),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: pfrom, ID: a.ID, OnChange: func(v string) { fromS.Set(v) },
						AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "page-xfer-from-select"})
				},
			}),
			uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferToLabel"),
				Error: ifStr(sameAcct, uistate.T("accounts.transferSameAccountErr"), ""),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: pto, ID: a.ID, OnChange: func(v string) { toS.Set(v) },
						AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "page-xfer-to-select"})
				},
			}),
			uiw.Field(uiw.FieldProps{
				ID: "page-xfer-amt", Label: uistate.T("accounts.transferAmount"),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.MoneyInput(uiw.MoneyInputProps{
						Value: amtS.Get(), Currency: fromCcy, ID: a.ID, TestID: "page-xfer-amt",
						Placeholder: uistate.T("accounts.transferAmount"), AriaLabel: uistate.T("accounts.transferAmount"),
						OnInput: func(v string) { amtS.Set(v) },
						Extra:   append(acctXferDescribeAttrs(a), Attr("inputmode", "decimal"), Attr("autofocus", ""), Attr("min", amountStep(fromCcy))),
					})
				},
			}),
			// Cross-currency: say what the amount means, with a live converted
			// preview at the saved rate, or a no-rate warning before anything posts.
			acctTransferFXNote(app, pfrom, pto, amtS.Get()),
			// Cross-currency only: the actual landed amount, overriding the saved
			// rate; hidden for same-currency pairs where it can't apply.
			If(crossCcy, uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferReceivedLabel", toCcy),
				Error: ifStr(!recvOK, uistate.T("accounts.transferInvalidExtra"), ""),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.NumberInput(uiw.TextFieldProps{
						Value: recvS.Get(), OnInput: func(v string) { recvS.Set(v) },
						Extra: append(acctXferControlAttrs(a), Attr("data-testid", "page-xfer-received"),
							Step(amountStep(toCcy)), Attr("min", amountStep(toCcy)), Title(uistate.T("accounts.transferReceivedHint"))),
					})
				},
			})),
			If(fromCcy != "", uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferFeeLabel", fromCcy),
				Error: ifStr(!feeOK, uistate.T("accounts.transferInvalidExtra"), ""),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.NumberInput(uiw.TextFieldProps{
						Value: feeS.Get(), OnInput: func(v string) { feeS.Set(v) },
						Extra: append(acctXferControlAttrs(a), Attr("data-testid", "page-xfer-fee"),
							Step(amountStep(fromCcy)), Attr("min", "0"), Title(uistate.T("accounts.transferFeeHint"))),
					})
				},
			})),
			// Before/after balances for both sides, straight from the same leg
			// math the post will use (PreviewTransferPair) — including the typed
			// received override and fee.
			acctTransferBalancePreview(app, pfrom, pto, amtS.Get(), recvS.Get(), feeS.Get()),
			uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferDateLabel"),
				Control: func(a uiw.FieldA11y) ui.Node {
					return Input(append(acctXferControlAttrs(a), css.Class("field"), Type("date"),
						AriaLabel(uistate.T("accounts.transferDateLabel")), OnInput(func(v string) { dateS.Set(v) }), uiw.FieldValue(dateS.Get()))...)
				},
			}),
			uiw.Field(uiw.FieldProps{
				Label: uistate.T("accounts.transferDescLabel"),
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.TextInput(uiw.TextFieldProps{
						Value: descS.Get(), Placeholder: uistate.T("accounts.transferDefaultDesc"), OnInput: func(v string) { descS.Set(v) },
						Extra: acctXferControlAttrs(a),
					})
				},
			}),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// acctXferControlAttrs are the id and accessibility attributes Field hands to a transfer control.
func acctXferControlAttrs(a uiw.FieldA11y) []any {
	return append([]any{ID(a.ID)}, acctXferDescribeAttrs(a)...)
}

// acctXferDescribeAttrs are the description and validity attributes Field hands to a transfer
// control, for a control that sets its own id.
func acctXferDescribeAttrs(a uiw.FieldA11y) []any {
	var attrs []any
	if a.DescribedBy != "" {
		attrs = append(attrs, Attr("aria-describedby", a.DescribedBy))
	}
	if a.Invalid {
		attrs = append(attrs, Attr("aria-invalid", "true"))
	}
	return attrs
}

// --- account row feed -----------------------------------------------------------

// acctRowFeed holds what every AccountRow in a tile shares: the household records a
// row displays and the handlers it calls. The list and archived tiles build their
// row props from one feed, so a row looks the same in both.
type acctRowFeed struct {
	Members     []domain.Member
	AccountDefs []customfields.Def
	InstByID    map[string]domain.Institution
	Snapshot    accountview.Snapshot
	Windows     freshness.Windows
	Now         time.Time
	Callbacks   acctRowCallbacks
	OnView      func(string)
	OnViewBills func(string)
}

// newAcctRowFeed reads the shared row inputs from the store. snapshot is the ledger
// index the tile already holds, and onView and onViewBills are the drill-down
// handlers the tile built from its router and filter hooks.
func newAcctRowFeed(app *appstate.App, snapshot accountview.Snapshot, onView, onViewBills func(string), now time.Time) acctRowFeed {
	return acctRowFeed{
		Members:     app.Members(),
		AccountDefs: app.CustomFieldDefsFor("account"), InstByID: domain.InstitutionByID(app.Institutions()),
		Snapshot: snapshot, Windows: app.FreshnessWindows(), Now: now,
		Callbacks: buildAcctRowCallbacks(app), OnView: onView, OnViewBills: onViewBills,
	}
}

// props builds the row props every tile shares for one account. A tile sets the
// fields only it needs on the result.
func (f acctRowFeed) props(ac domain.Account) accountRowProps {
	return accountRowProps{
		Account: ac, Balance: f.Snapshot.Balances[ac.ID], Cleared: f.Snapshot.Cleared[ac.ID],
		Stale: freshness.IsStale(ac, f.Windows, f.Now), DaysStale: freshness.DaysSinceUpdate(ac, f.Now),
		Members:  f.Members,
		OnDelete: f.Callbacks.OnDelete, OnArchive: f.Callbacks.OnArchive, OnRefresh: f.Callbacks.OnRefresh,
		OnSave: f.Callbacks.OnSave, OnView: f.OnView,
		AccountDefs: f.AccountDefs, InstByID: f.InstByID,
		BillPayment: f.Snapshot.BillPayments[ac.ID], OnViewBills: f.OnViewBills,
	}
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

	// Client-side navigation to /debt for the "Manage debts" link. It prevents the
	// anchor's default full page load, which would restart the app and, with app-lock
	// on, drop the user onto the lock screen. Declared unconditionally so the hook
	// position is stable.
	goToDebt := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/debt")) }))
	// The stale-summary line's "Mark all updated" reuses the toolbar's exact action.
	// Declared unconditionally so the hook position stays stable even when the summary
	// line isn't rendered.
	markAllStale := ui.UseEvent(Prevent(func() { markAllStalePrompt(app) }))

	snapshot := accountSnapshot(app)
	// Services are recurring bills, not accounts. They have their own class view, which
	// stays on once it is chosen, even before the household has any service, so its
	// empty state can be seen.
	serviceList := app.Services()
	utilities := utilityAccounts(app.Accounts())
	utilityCharged := utilityCharges(utilities, app.Recurring())
	hasServices := len(serviceList) > 0 || len(utilities) > 0
	list := accountview.List(accountview.ListInput{
		Accounts: app.Accounts(), MemberID: activeMemberID, Class: f.Class,
		ServicesClass: serviceClassView, Match: f.Matches,
	})
	hasLiab := len(list.Liabilities) > 0
	classView := list.Class
	shown := list.Shown

	// Risk-first ordering: accounts that need attention (stale, or past their freshness
	// window) lead the list, so the rows worth acting on sit at the top instead of being
	// buried under large healthy balances. accountview.SortRiskFirst ranks each account
	// by freshness, then from the biggest signed balance down so the heaviest debts trail.
	windows := app.FreshnessWindows()
	now := time.Now()
	accountview.SortRiskFirst(shown, snapshot.Balances, windows, props.Rates, props.Base, now)

	// Once more than half the visible accounts are stale, a STALE badge on nearly every
	// row ranks nothing. The rows then collapse their badges to subdued dots and the
	// list leads with one summary line and a Mark-all action.
	stale := accountview.Stale(shown, windows, now)

	smartSettings := uistate.LoadSmartSettings()
	pr := uistate.UsePrefs().Get()
	smartIn := buildSmartInput(app, pr.WeekStartWeekday())
	accountInsights := accountsPageInsights(app, smartIn, smartSettings, pr.WeekStartWeekday())
	accountByEntity := insightsByEntity(accountInsights)

	feed := newAcctRowFeed(app, snapshot, acctViewTransactions(nav, txFilter), acctViewBillPayments(nav, txFilter), now)
	householdDefaultID := app.DefaultAccountID()

	// The row shares the ledger index. Compact rows need balances only; their chart
	// windows, flows and recurring projections are prepared on disclosure.
	detailContext := newAccountDetailContext(app, snapshot, props.Rates, props.Base, now)

	renderRow := func(ac domain.Account) ui.Node {
		defaultMemberID, defaultMemberName := accountDefaultMemberTarget(ac, feed.Members, activeMemberID)
		var defaultMemberIDs []string
		if !ac.Archived {
			for _, member := range feed.Members {
				if member.Prefs.DefaultAccountID == ac.ID {
					defaultMemberIDs = append(defaultMemberIDs, member.ID)
				}
			}
		}
		row := feed.props(ac)
		row.StaleCollapsed = stale.Collapsed
		row.DefaultHousehold = !ac.Archived && householdDefaultID == ac.ID
		row.DefaultMembers = defaultMemberIDs
		row.DefaultMemberID, row.DefaultMemberName = defaultMemberID, defaultMemberName
		row.OnSetDefaultHousehold, row.OnClearDefaultHousehold = feed.Callbacks.OnSetDefaultHousehold, feed.Callbacks.OnClearDefaultHousehold
		row.OnSetDefaultMember, row.OnClearDefaultMember = feed.Callbacks.OnSetDefaultMember, feed.Callbacks.OnClearDefaultMember
		row.SmartSettings, row.SmartByEntity = smartSettings, accountByEntity
		row.DetailContext = detailContext
		return ui.CreateElement(AccountRow, row)
	}
	// User-defined groups render as sections, each with a net subtotal, followed
	// by an "Ungrouped" catch-all. Used as the body only when at least one group exists.
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
		When(list.InServices, func() ui.Node {
			return servicesBody(app, serviceList, utilities, utilityCharged, props.Base, props.Rates)
		}).
		When(len(list.Active) == 0, func() ui.Node {
			return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("accounts.noAssets"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account", Icon: icon.Accounts, ImportLink: true})
		}).
		When(len(shown) == 0 && f.HasNarrowing(), func() ui.Node {
			return acctEmptyCallout("acct-list-nomatch", uistate.T("accounts.noMatch"))
		}).
		When(len(shown) == 0 && classView == string(domain.ClassLiability), func() ui.Node {
			return acctEmptyCallout("acct-list-noliabilities", uistate.T("accounts.noLiabilities"))
		}).
		When(len(shown) == 0, func() ui.Node {
			return acctEmptyCallout("acct-list-noassets", uistate.T("accounts.noAssets"))
		}).
		When(len(groups) > 0, groupedBody).
		Default(func() ui.Node {
			return Div(css.Class("rows"), MapKeyed(shown, accountRowKey, renderRow))
		})

	// Collapsed-stale state: one summary line leads the list, its Mark-all reusing the
	// toolbar action. Prepended so it sits above the (grouped or flat) rows.
	if stale.Collapsed {
		bodyContent = Fragment(
			uiw.Callout(uiw.CalloutProps{
				Tone: uiw.CalloutInfo, Live: uiw.CalloutLiveStatus, TestID: "acct-stale-summary",
				Body: Span(uistate.T("accounts.staleSummary")),
				Action: uiw.ToolButton(uiw.ToolButtonProps{
					Label: uistate.T("accounts.markAll"), Title: uistate.T("acctxn.markAllMeaning"),
					Variant: "stale", TestID: "acct-stale-summary-markall",
				}, OnClick(markAllStale)),
			}),
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

	classSeg := ui.If(hasLiab || hasServices || list.InServices, func() ui.Node {
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

	if list.InServices {
		title = uistate.T("accounts.servicesTitle")
	}
	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: title,
		// The class filter (All / Assets / Liabilities / Services) is the header action.
		// Adding is in the summary's Add menu.
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
	if list.AssetsOnly {
		debtLinkText = uistate.T("accounts.liabilitiesStub", len(list.Liabilities))
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

// acctEmptyCallout is the notice a list tile shows in place of its rows when none are left to show.
func acctEmptyCallout(testID, text string) ui.Node {
	return uiw.Callout(uiw.CalloutProps{Tone: uiw.CalloutInfo, TestID: testID, Body: Span(text)})
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
// "show archived" toggle is on and archived accounts exist. It lists archived assets
// and liabilities alike. Rows reuse AccountRow (so an archived account can still be
// restored, viewed, or edited).
func acctArchivedWidget(props acctArchivedProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	filterAtom := uistate.UseAccountsFilter()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	var shown []domain.Account
	for _, ac := range accountview.Archived(app.Accounts(), activeMemberID) {
		if f.Matches(ac.Name, string(ac.Type)) {
			shown = append(shown, ac)
		}
	}

	feed := newAcctRowFeed(app, accountSnapshot(app), acctViewTransactions(nav, txFilter), acctViewBillPayments(nav, txFilter), time.Now())
	renderRow := func(ac domain.Account) ui.Node {
		row := feed.props(ac)
		row.ValuationHistory = app.BalanceHistory(ac.ID)
		return ui.CreateElement(AccountRow, row)
	}
	bodyContent := ui.If(len(shown) == 0, func() ui.Node {
		return acctEmptyCallout("acct-archived-nomatch", uistate.T("accounts.noMatch"))
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
			postAcctWriteError("load sample", err)
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
		P(css.Class("t-caption", tw.TextDim, "acct-gap-below"), uistate.T("accounts.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("accounts.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// balanceAsOfDate reads the date a balance update was true as of. A blank date means now; any other
// text must be a real YYYY-MM-DD date, and the second result is false when it is not. The edit form's
// date check (valueUpdateProblem) applies the same rule.
func balanceAsOfDate(raw string, now time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now, true
	}
	parsed, err := dateutil.ParseDate(raw)
	if err != nil || parsed.IsZero() {
		return time.Time{}, false
	}
	return parsed, true
}
