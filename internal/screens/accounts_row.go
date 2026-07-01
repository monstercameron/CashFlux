// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
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
	// ValuationHistory holds the balance snapshots for illiquid-asset accounts,
	// pre-loaded by the accounts screen so AccountRow has no store access.
	// Nil or empty = no history panel rendered.
	ValuationHistory []domain.BalanceSnapshot
	// AccountDefs are the user-defined custom-field definitions for the "account"
	// entity, passed in so the inline editor can render their inputs and the row can
	// show their values. Empty = no custom fields configured.
	AccountDefs []customfields.Def
}

// moneyMajorOrEmpty renders a money value as a major-unit string, or "" when zero.
// isValuationType reports whether an account's balance is a periodically
// estimated valuation (an investment, retirement, crypto, or other/illiquid
// asset) rather than a reconciled cash balance. C226/C73: these read better
// with "Out of date / Update value" than the banking-flavoured "Stale / Update
// balance".
func isValuationType(t domain.AccountType) bool {
	return t == domain.TypeInvestment || t == domain.TypeRetirement ||
		t == domain.TypeCrypto || t == domain.TypeProperty ||
		t == domain.TypeVehicle || t == domain.TypeOther
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
	// Keep the ⋯ menu inside the viewport: the trigger sits near the right edge of the
	// row, so flip the menu to open leftward / upward when it would otherwise overflow.
	uiw.AnchorPopover(menuOpen.Get(), menuID)

	del := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnDelete(a.ID) }))
	arch := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnArchive(a) }))
	refresh := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnRefresh(a) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(a.ID) }))
	// The editors (edit / update-balance / reconcile / transfer) open in a flip modal
	// rendered by the shell-root AccountEditHost (uistate account-editor atom) — not
	// inline in the row — so the modal centers on the viewport instead of resolving
	// against the transformed bento tile. Each opener sets the atom; the host renders
	// the matching form.
	acctEditAtom := uistate.UseAccountEdit()
	openEditor := func(mode string) func() {
		return func() {
			menuOpen.Set(false)
			acctEditAtom.Set(uistate.AccountEdit{ID: a.ID, Mode: mode})
		}
	}
	setBal := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeSetBal)))
	startEdit := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeEdit)))
	startReconcile := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeReconcile)))
	startTransfer := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeTransfer)))
	// The encrypted credential vault opens in its own shell-root modal (CredentialVaultHost).
	acctCredsAtom := uistate.UseAccountCredentials()
	startCredentials := ui.UseEvent(Prevent(func() { menuOpen.Set(false); acctCredsAtom.Set(a.ID) }))
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

	// Valuation history panel — displayed below the row for illiquid-asset
	// accounts when at least 2 snapshots are available (C225 [F31]).
	// Display-only: no On* hooks inside the MapKeyed loop.
	var historyPanel ui.Node = Fragment()
	if isValuationType(a.Type) && len(props.ValuationHistory) >= 2 {
		histDec := dec // capture for closure
		histCur := a.Currency
		// Show up to the last 6 snapshots, most recent first.
		snaps := props.ValuationHistory
		start := 0
		if len(snaps) > 6 {
			start = len(snaps) - 6
		}
		recent := snaps[start:]
		// Reverse so most recent appears at the top of the list.
		reversed := make([]domain.BalanceSnapshot, len(recent))
		for i, s := range recent {
			reversed[len(recent)-1-i] = s
		}
		keyOfSnap := func(s domain.BalanceSnapshot) any { return s.ID }
		renderSnap := func(s domain.BalanceSnapshot) ui.Node {
			dateStr := s.AsOf.Format("Jan 2, 2006")
			valStr := money.FormatMinor(s.BalanceMinor, histDec)
			cur := s.Currency
			if cur == "" {
				cur = histCur
			}
			return Div(css.Class("val-hist-row"),
				Span(css.Class("val-hist-date", tw.TextDim), dateStr),
				Span(css.Class("val-hist-val"), valStr+" "+cur),
			)
		}
		historyPanel = Div(css.Class("val-hist-panel"),
			Attr("data-testid", "val-hist-panel-"+a.ID),
			P(css.Class("val-hist-title", tw.TextDim), uistate.T("accounts.valuationHistoryTitle")),
			Div(css.Class("val-hist-rows"), MapKeyed(reversed, keyOfSnap, renderSnap)),
		)
	}

	return Div(
		Div(css.Class("row"),
			// Account-type glyph (G3 §5): a quick visual tag so Checking / Investment /
			// Credit Card are distinguishable without reading the meta-line.
			Span(css.Class("acct-type-icon", tw.TextDim), Attr("aria-hidden", "true"),
				uiw.Icon(accountTypeIcon(a.Type), css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), a.Name,
					If(props.Stale, Span(css.Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), uistate.T(staleBadgeKey(a.Type)))),
					// A quiet note glyph when the account has notes attached.
					If(strings.TrimSpace(a.Notes) != "", Span(css.Class("acct-notes-dot", tw.TextDim), Style(map[string]string{"margin-left": "0.4rem"}),
						Attr("data-testid", "acct-notes-dot-"+a.ID), Attr("aria-label", uistate.T("accounts.notesBadge")), Title(strings.TrimSpace(a.Notes)),
						uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
					smartBadgeFor(props.SmartSettings, props.SmartByEntity, a.ID),
					smartOverlayFor(props.SmartSettings, props.SmartByEntity, a.ID),
				),
				Span(css.Class("row-meta"), meta),
				// Read-only summary of the account's custom-field values (a compact
				// "Label: value · …" line), shown only when any are set.
				If(customSummary(props.AccountDefs, a.Custom) != "",
					Span(css.Class("row-meta", tw.TextDim), Attr("data-testid", "acct-custom-summary-"+a.ID),
						customSummary(props.AccountDefs, a.Custom))),
				// MIA-extend (#445-10): nudge to fill missing institution.
				If(a.Institution == "" && !a.Archived,
					Button(css.Class("btn-link t-caption", tw.TextDim), Type("button"),
						Attr("data-testid", "set-institution-"+a.ID),
						Title(uistate.T("accounts.setInstitution")),
						OnClick(startEdit),
						uistate.T("accounts.setInstitution"),
					)),
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
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "creds-start-btn-"+a.ID), OnClick(startCredentials), uistate.T("creds.menuItem")),
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("title", archTitle), OnClick(arch), archLabel),
					// Delete moved out of the standalone ✕ column and into the menu as a
					// destructive item (last, red) so a row's actions are all in one place.
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-account-btn-"+a.ID), Attr("aria-label", uistate.T("accounts.deleteTitle")), Title(uistate.T("accounts.deleteTitle")), OnClick(del), uistate.T("accounts.deleteAction")),
				),
			),
		),
		historyPanel,
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

// cloneSharesMap returns a shallow copy of src, or nil when src is nil/empty.
// Used to initialise edit-form state without aliasing the account's map.
func cloneSharesMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// ownerShareRowProps carries data and the callback for one member's percentage
// field in the fractional-ownership sub-form.
type ownerShareRowProps struct {
	Member   domain.Member
	Share    int                                  // current percentage (0–100)
	OnChange func(memberID string, valStr string) // plain func — never an On* hook
}

// OwnerShareRow renders a single member's share-percentage input. It is a
// standalone component so the parent can use MapKeyed over a variable-length
// member slice without calling On* hooks inside a loop (CLAUDE.md §gotchas).
func OwnerShareRow(props ownerShareRowProps) ui.Node {
	onChange := ui.UseEvent(func(v string) { props.OnChange(props.Member.ID, v) })
	shareStr := ""
	if props.Share > 0 {
		shareStr = strconv.Itoa(props.Share)
	}
	return Div(css.Class("form-grid"),
		Style(map[string]string{"grid-template-columns": "1fr auto", "align-items": "center", "gap": "0.5rem"}),
		Span(css.Class(tw.TextDim), props.Member.Name),
		Label(css.Class("labeled-field"),
			Style(map[string]string{"display": "flex", "flex-direction": "row", "align-items": "center", "gap": "0.25rem"}),
			Input(css.Class("field"),
				Style(map[string]string{"width": "5rem"}),
				Type("number"), Attr("min", "0"), Attr("max", "100"), Step("1"),
				Attr("aria-label", props.Member.Name+" "+uistate.T("account.sharePercent")),
				Value(shareStr), OnInput(onChange)),
			Span(css.Class(tw.TextDim), "%"),
		),
	)
}
