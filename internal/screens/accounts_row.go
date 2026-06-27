// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

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
	// Smart badge inputs: SmartSettings + byEntity index from the page's insight run.
	// When SmartSettings is zero-value the badge simply renders nothing (safe default).
	SmartSettings smart.Settings
	SmartByEntity map[string][]smart.Insight
}

// moneyMajorOrEmpty renders a money value as a major-unit string, or "" when zero.
// isValuationType reports whether an account's balance is a periodically
// estimated valuation (an investment, retirement, crypto, or other/illiquid
// asset) rather than a reconciled cash balance. C226/C73: these read better
// with "Out of date / Update value" than the banking-flavoured "Stale / Update
// balance".
func isValuationType(t domain.AccountType) bool {
	return t == domain.TypeInvestment || t == domain.TypeRetirement ||
		t == domain.TypeCrypto || t == domain.TypeOther
}

// staleBadgeKey / updateActionKey pick asset-appropriate wording for the stale
// badge and the update action (C226).
func staleBadgeKey(t domain.AccountType) string {
	if isValuationType(t) {
		return "accounts.staleValue"
	}
	return "accounts.stale"
}

func updateActionKey(t domain.AccountType) string {
	if isValuationType(t) {
		return "accounts.updateValue"
	}
	return "accounts.updateBalance"
}

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
	menuID := ui.UseId()
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	// WAI-ARIA dismissal for the ⋯ actions menu: Escape (refocus trigger) + outside
	// pointerdown. The `.add-backdrop` can't be relied on (fixed inside the topbar's
	// sticky stacking context, so it doesn't paint over page content). See uiw.DismissPopover.
	uiw.DismissPopover(menuOpen.Get(), menuID, func() { menuOpen.Set(false) })

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
	// onSetBalCat hook kept for stable hook ordering; SelectInput owns the change event.
	ui.UseEvent(func(e ui.Event) { setBalCatS.Set(e.GetValue()) })
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
	// onXferTo hook kept for stable hook ordering; SelectInput owns the change event.
	ui.UseEvent(func(e ui.Event) { xferToS.Set(e.GetValue()) })
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
	// editAdvOpen tracks whether the advanced asset fields are expanded in the
	// inline-edit form (mirrors the add form's disclosure, C49).
	editAdvOpen := ui.UseState(false)
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
	// onOwner hook kept for stable hook ordering; SelectInput owns the change event.
	ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	onToggleEditAdv := ui.UseEvent(func() { editAdvOpen.Set(!editAdvOpen.Get()) })
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
		// Collapse advanced section on open so returning users start from a clean
		// short form; they can expand again if they need to adjust an advanced field.
		editAdvOpen.Set(false)
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
		catOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.setBalanceNoCategory")}}
		for _, c := range props.Categories {
			catOpts = append(catOpts, uiw.SelectOption{Value: c.ID, Label: c.Name})
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
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   catOpts,
						Selected:  setBalCatS.Get(),
						OnChange:  func(v string) { setBalCatS.Set(v) },
						AriaLabel: uistate.T("accounts.setBalanceCategoryLabel"),
						TestID:    "setbal-cat-select",
					})),
				// C227: for valuation-type assets (investment/retirement/crypto/other),
				// note that values are entered manually and no external API is called.
				If(isValuationType(a.Type), P(css.Class("t-caption", tw.TextDim),
					Attr("data-testid", "valuation-manual-note"),
					uistate.T("accounts.valuationManualNote"))),
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
		toOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
		for _, ac := range props.Accounts {
			if ac.ID == a.ID || ac.Archived {
				continue
			}
			toOpts = append(toOpts, uiw.SelectOption{Value: ac.ID, Label: ac.Name + " (" + ac.Currency + ")"})
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
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   toOpts,
						Selected:  xferToS.Get(),
						OnChange:  func(v string) { xferToS.Set(v) },
						AriaLabel: uistate.T("accounts.transferToLabel"),
						TestID:    "acct-xfer-to-select",
					})),
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
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   ownerSelectOptions(props.Members, ownerS.Get()),
						Selected:  ownerS.Get(),
						OnChange:  func(v string) { ownerS.Set(v) },
						AriaLabel: uistate.T("common.owner"),
					})),
				labeledField(uistate.T("accounts.openingBalance"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(balS.Get()), Step("0.01"), OnInput(onBal))),
				// Liability-specific fields (always shown when editing a liability).
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
				// Asset advanced fields: tucked behind a disclosure so the common edit
				// path (name · owner · balance) stays short — mirrors the add form (C49).
				If(!isLiab, Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(editAdvOpen.Get())), OnClick(onToggleEditAdv),
					IfElse(editAdvOpen.Get(), Text("Hide advanced fields"), Text("Show advanced fields")))),
				If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.expReturn"),
					Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(retS.Get()), Step("0.01"), OnInput(onRet)))),
				If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.liquidity"),
					Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liqS.Get()), OnInput(onLiq)))),
				If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.stability"),
					Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stabS.Get()), OnInput(onStab)))),
				If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.lockUntilEdit"),
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
				If(props.Stale, Span(css.Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), uistate.T(staleBadgeKey(a.Type)))),
				smartBadgeFor(props.SmartSettings, props.SmartByEntity, a.ID),
				smartOverlayFor(props.SmartSettings, props.SmartByEntity, a.ID),
			),
			Span(css.Class("row-meta"), meta),
		),
		// L100-T1: the headline balance sits near the dim "cleared (…)" figure in the meta line and both
		// render parenthesized for liabilities, so give the current balance an explicit accessible name
		// (tooltip + aria-label) — it disambiguates "what I owe now" from the cleared balance for hover
		// and screen-reader users without cluttering the row with a visible label.
		Span(ClassStr(amountClass(props.Balance)),
			Title(uistate.T("accounts.balanceTitle")),
			Attr("aria-label", uistate.T("accounts.balanceAria", fmtMoney(props.Balance))),
			fmtMoney(props.Balance)),
		// Stale accounts get the reconcile action surfaced inline (G3 §6) rather than
		// buried in the ⋯ menu, since "update my balance" is the whole reason a stale
		// account is flagged.
		If(props.Stale && !a.Archived, Button(css.Class("btn btn-stale", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T(updateActionKey(a.Type))), OnClick(setBal), uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T(updateActionKey(a.Type))))),
		// Primary actions inline; everything else in the ⋯ menu.
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("accounts.viewTitle")), OnClick(view), uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions"))),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("accounts.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Div(css.Class("add-wrap"), Attr("id", menuID),
			Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("accounts.moreActions")), Attr("aria-label", uistate.T("accounts.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
			Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
			Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
				If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(setBal), uistate.T(updateActionKey(a.Type)))),
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
			Title(uistate.T("accounts.markClearedTitle")),
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
