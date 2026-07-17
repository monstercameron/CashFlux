// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/valuation"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type accountRowProps struct {
	Account    domain.Account
	Balance    money.Money
	Cleared    money.Money
	Stale      bool
	// DaysStale is the days since the balance was last confirmed (−1 = never), shown
	// on the stale badge so the row says HOW stale it is — matching the "it's been N
	// days" wording of the balance-update notification. Only read when Stale is true.
	DaysStale int
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
	// BillPayment is this account's most recent linked bill payment (any transaction the
	// user marked as a bill payment toward it). Shown as a "Bill $X · N →" line so the
	// linkage is visible for every account, not just debts. OnViewBills drills to them.
	BillPayment ledger.BillPaymentInfo
	OnViewBills func(string)
	// Projection is this account's 30-day balance projection from its recurring cash
	// flows (AC13). When it shows a dip below today's balance, the row renders a
	// "→ ~$X low on <date>" line that expands to list the drivers. Zero value renders
	// nothing.
	Projection acctproject.Projection
	// Sparkline is the account's 90-day end-of-day balance series (AC2), oldest first,
	// in the account's currency (minor units). Rendered as an inline SVG polyline; a
	// flat run is itself the "nothing has posted since your last update" signal. Fewer
	// than two points renders nothing.
	Sparkline []int64
	// Flow is this account's money-in / money-out / net for the current period (AC9),
	// with transfers counted separately (never as income or spend). Rendered as compact
	// row figures when ShowFlow is set.
	Flow     accountflow.Flow
	ShowFlow bool
	// InstByID indexes the household's institution directory by id (AC10), so the
	// row can color its edge and show a chip for Account.InstitutionID. Nil or a
	// miss renders nothing extra — free-text-only institutions are unaffected.
	InstByID map[string]domain.Institution
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

// staleBadgeText renders the stale badge label with the days-since-confirmed
// suffix ("Stale · 24d") so the row says HOW stale it is — mirroring the balance-
// update notification. days < 0 means the balance was never confirmed.
func staleBadgeText(t domain.AccountType, days int) string {
	base := uistate.T(staleBadgeKey(t))
	if days < 0 {
		return base + uistate.T("accounts.staleNeverSuffix")
	}
	if days == 0 {
		return base
	}
	return base + uistate.T("accounts.staleDaysSuffix", days)
}

// staleBadgeTitle is the badge's hover tooltip — a full-sentence version of the
// same freshness the notification states.
func staleBadgeTitle(t domain.AccountType, days int) string {
	if days < 0 {
		return uistate.T("accounts.staleNeverTitle")
	}
	return uistate.T("accounts.staleDaysTitle", days)
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
	view := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnView(a.ID) }))
	// Notes are shown inline as a readable, clamped line that expands on click (a
	// disclosure), so a note is actually legible in the row rather than hidden in a
	// hover tooltip on a tiny glyph.
	notesExpanded := ui.UseState(false)
	toggleNotes := ui.UseEvent(Prevent(func() { notesExpanded.Set(!notesExpanded.Get()) }))
	// AC13: the projected-low "Why?" disclosure owns its own state at this stable
	// render position (AccountRow is itself the per-row component, so this is not a
	// loop-level hook).
	projExpanded := ui.UseState(false)
	toggleProj := ui.UseEvent(Prevent(func() { projExpanded.Set(!projExpanded.Get()) }))
	// AC-series detail (90-day trend, this-period flow, projection, filed documents,
	// notes, custom fields) is folded behind a quiet per-row disclosure so the resting
	// list reads as name → balance and nothing competes; one click reveals the rest.
	detailsOpen := ui.UseState(false)
	toggleDetails := ui.UseEvent(Prevent(func() { detailsOpen.Set(!detailsOpen.Get()) }))
	viewBills := ui.UseEvent(Prevent(func() {
		if props.OnViewBills != nil {
			props.OnViewBills(a.ID)
		}
	}))
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
	// G8: quick institution assignment from the row — a prompt seeded with the current
	// value, normalised the same way the add form normalises it. No modal round-trip.
	setInstitution := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.PromptModal(uistate.T("accounts.setInstitutionPrompt", a.Name), a.Institution, func(v string) {
			ac := a
			ac.Institution = titleCaseWords(strings.TrimSpace(v))
			props.OnSave(ac)
			if ac.Institution == "" {
				uistate.PostNotice(uistate.T("accounts.institutionCleared"), false)
			} else {
				uistate.PostNotice(uistate.T("accounts.institutionSet", ac.Institution), false)
			}
		})
	}))
	archLabel, archTitle := uistate.T("accounts.archive"), uistate.T("accounts.archiveTitle")
	if a.Archived {
		archLabel, archTitle = uistate.T("accounts.restore"), uistate.T("accounts.restoreTitle")
	}
	// A liability is shown as a negative figure — a debt reduces net worth — taking
	// -Abs so it reads correctly whether the balance is stored positive ("amount you
	// owe" add form) or negative (the sample convention). Editing still works on the
	// raw stored value; only the displayed figures are signed.
	dispBal, dispCleared := props.Balance, props.Cleared
	if a.Class == domain.ClassLiability {
		dispBal = props.Balance.Abs().Neg()
		dispCleared = props.Cleared.Abs().Neg()
	}

	meta := accountMeta(a, props.Balance)
	if props.Cleared.Amount != props.Balance.Amount {
		meta += uistate.T("accounts.clearedSuffix", fmtMoney(dispCleared))
	}
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}

	// Month-to-date value change for illiquid-asset accounts (home / car /
	// investment): a single signed figure — "▲ $2,000.00 this month" — in place of
	// the old scrolling valuation-history list, which was noise in the row (C225
	// [F31]). Shown only when there is history to compare against. Display-only.
	var valChange ui.Node = Fragment()
	if isValuationType(a.Type) && len(props.ValuationHistory) >= 2 {
		if chg, ok := valuation.MonthToDateChange(props.ValuationHistory, props.Balance, time.Now()); ok {
			cls := "row-meta acct-val-change"
			label := uistate.T("accounts.valuationNoChangeMonth")
			if chg.Amount != 0 {
				arrow := "▲"
				if chg.IsNegative() {
					arrow = "▼"
				}
				if t := figTone(chg); t != "" {
					cls += " " + tw.ColorClass(t)
				}
				label = uistate.T("accounts.valuationChangeMonth", arrow+" "+fmtMoney(chg.Abs()))
			}
			valChange = Span(ClassStr(cls), Attr("data-testid", "val-change-"+a.ID), label)
		}
	}

	// Bill-payment line: the account's most recent linked bill payment, with a drill to
	// the payments. Shown for ANY account the user has linked a bill payment to (the
	// Debt page shows liabilities; this makes the link visible on every account).
	var billNode ui.Node = Fragment()
	if props.BillPayment.HasAny {
		billNode = Span(css.Class("row-meta"), Attr("data-testid", "acct-bill-"+a.ID),
			uistate.T("accounts.billMeta", fmtMoney(props.BillPayment.Latest)),
			Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "acct-bill-link-"+a.ID),
				Title(uistate.T("accounts.billLinkTitle")), OnClick(viewBills),
				uistate.T("accounts.billCount", plural(props.BillPayment.Count, "payment"))))
	}

	// Readable notes line: the note text itself, clamped to a couple of lines and
	// expandable on click, so an attached note is legible in the row instead of being
	// hidden behind a hover tooltip on a tiny glyph (replaces acct-notes-dot).
	var notesNode ui.Node = Fragment()
	if notes := strings.TrimSpace(a.Notes); notes != "" {
		notesCls := "acct-notes"
		toggleLabel := uistate.T("accounts.notesReadMore")
		if notesExpanded.Get() {
			notesCls += " open"
			toggleLabel = uistate.T("accounts.notesReadLess")
		}
		notesNode = Button(ClassStr(notesCls), Type("button"), Attr("data-testid", "acct-notes-"+a.ID),
			Attr("aria-expanded", ariaBool(notesExpanded.Get())), Attr("aria-label", uistate.T("accounts.notesBadge")),
			Title(toggleLabel), OnClick(toggleNotes),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), notes),
		)
	}

	// AC13 projected-balance line: for cash-like asset accounts whose 30-day
	// projection dips below today's balance, show "→ ~$X low on <date>" with a "Why?"
	// disclosure that lists the recurring drivers ("Rent −$1,400 on Mar 1"). Skipped
	// for liabilities and valuation-type assets, whose balances aren't cash curves.
	var projNode ui.Node = Fragment()
	if a.Class != domain.ClassLiability && !isValuationType(a.Type) && props.Projection.HasLowDip() {
		dec := currency.Decimals(a.Currency)
		low := money.New(props.Projection.Low, a.Currency)
		toggleLabel := uistate.T("accounts.projectedShow")
		if projExpanded.Get() {
			toggleLabel = uistate.T("accounts.projectedHide")
		}
		var driverNode ui.Node = Fragment()
		if projExpanded.Get() {
			rows := []any{css.Class("acct-proj-drivers"), Attr("data-testid", "acct-proj-drivers-"+a.ID),
				Div(css.Class("row-meta", tw.TextDim), uistate.T("accounts.projectedDrivers"))}
			for _, d := range props.Projection.Drivers {
				amt := money.New(d.Amount, a.Currency)
				rows = append(rows, Div(css.Class("row-meta"),
					uistate.T("accounts.projectedDriver", d.Label, signedMoney(amt, dec), fmtShortDate(d.Date))))
			}
			driverNode = Div(rows...)
		}
		projNode = Div(css.Class("acct-proj"), Attr("data-testid", "acct-proj-"+a.ID),
			Span(css.Class("row-meta"), Title(uistate.T("accounts.projectedTitle")),
				Attr("aria-label", uistate.T("accounts.projectedLowAria", fmtMoney(low), fmtShortDate(props.Projection.LowDate))),
				uistate.T("accounts.projectedLow", fmtMoney(low), fmtShortDate(props.Projection.LowDate))),
			Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "acct-proj-toggle-"+a.ID),
				Attr("aria-expanded", ariaBool(projExpanded.Get())), OnClick(toggleProj), toggleLabel),
			driverNode,
		)
	}

	// Inline quick actions (everything else lives in the ⋯ menu). "Edit" is always
	// inline; "Update value/balance" is inline for the accounts you actively maintain —
	// stale ones (emphasized) and valuation-type assets you revalue by hand — and lives
	// in the menu for the rest so it's never duplicated.
	showValueInline := (props.Stale || isValuationType(a.Type)) && !a.Archived
	updBtnCls := "btn"
	if props.Stale {
		updBtnCls = "btn btn-stale"
	}

	// AC2: a 90-day balance sparkline (inline SVG); AC9: this-period in/out/net figures.
	sparkNode := accountSparkline(a, props.Sparkline)
	flowNode := accountFlowFigures(a, props.Flow, props.ShowFlow)

	// AC10: color the row's left edge by its institution when the account
	// references one from the directory; a bare border-left is the least intrusive
	// way to add the cue without disturbing the row's existing layout.
	rowStyle := map[string]string{}
	inst, hasInst := props.InstByID[a.InstitutionID]
	if hasInst {
		rowStyle["border-left"] = "3px solid " + institutionSwatchColor(inst)
	}

	// The detail disclosure sits on the quiet secondary line; its label flips with state.
	detailsLabel := uistate.T("accountsRedesign.detailsShow")
	if detailsOpen.Get() {
		detailsLabel = uistate.T("accountsRedesign.detailsHide")
	}
	// The revealed block collects every AC-series extra. It mounts only while open so the
	// resting row stays a clean name → balance line (child components' hooks are their own,
	// so conditional mounting here is safe).
	var detailsNode ui.Node = Fragment()
	if detailsOpen.Get() {
		detailsNode = Div(css.Class("acct-row-details"), Attr("data-testid", "acct-details-"+a.ID),
			valChange,
			projNode,
			billNode,
			flowNode,
			sparkNode,
			// XC7: warn when goals have earmarked more against this account than it
			// holds (goal money has been spent). Own component; healthy → Fragment().
			ui.CreateElement(accountEarmarkWarning, accountEarmarkWarnProps{Account: a, Balance: props.Balance}),
			// Read-only summary of the account's custom-field values (a compact
			// "Label: value · …" line), shown only when any are set.
			If(customSummary(props.AccountDefs, a.Custom) != "",
				Span(css.Class("row-meta", tw.TextDim), Attr("data-testid", "acct-custom-summary-"+a.ID),
					customSummary(props.AccountDefs, a.Custom))),
			// Readable, clickable-to-expand notes line (the attached note itself).
			notesNode,
			// AC8/AC17: the filed-documents drawer (statements, contracts, titles,
			// payoff letters) with an attach form carrying an optional renewal date.
			ui.CreateElement(accountDocsDrawer, accountDocsDrawerProps{Account: a}),
			// MIA-extend (#445-10): nudge to fill missing institution.
			If(a.Institution == "" && !a.Archived,
				Button(css.Class("btn-link t-caption", tw.TextDim), Type("button"),
					Attr("data-testid", "set-institution-"+a.ID),
					Style(map[string]string{"align-self": "flex-start", "text-align": "left"}),
					Title(uistate.T("accounts.setInstitution")),
					OnClick(startEdit),
					uistate.T("accounts.setInstitution"),
				)),
		)
	}

	return Div(css.Class("row acct-row"), Attr("data-testid", "acct-row-"+a.ID), Style(rowStyle),
		// PRIMARY line: type glyph + name/badges on the left, the balance figure and the
		// row actions right-aligned. Everything else is demoted to the sub-line or details.
		Div(css.Class("acct-row-head"),
			// Account-type glyph (G3 §5): a quick visual tag so Checking / Investment /
			// Credit Card are distinguishable without reading the meta-line.
			Span(css.Class("acct-type-icon", tw.TextDim), Attr("aria-hidden", "true"),
				uiw.Icon(accountTypeIcon(a.Type), css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Div(css.Class("acct-row-id"),
				Div(css.Class("acct-row-name"),
					Span(css.Class("row-desc"), a.Name),
					If(props.Stale, Span(css.Class("badge badge-prio prio-med"), Title(staleBadgeTitle(a.Type, props.DaysStale)), staleBadgeText(a.Type, props.DaysStale))),
					smartBadgeFor(props.SmartSettings, props.SmartByEntity, a.ID),
					smartOverlayFor(props.SmartSettings, props.SmartByEntity, a.ID),
					If(hasInst, institutionChip(props.InstByID, a.InstitutionID)),
				),
				// SECONDARY line: the quiet meta (type · currency · utilization) plus the
				// details disclosure — the only two things allowed to sit under the name.
				Div(css.Class("acct-row-sub"),
					Span(css.Class("row-meta"), meta),
					Button(css.Class("btn-link acct-details-toggle"), Type("button"),
						Attr("data-testid", "acct-details-toggle-"+a.ID),
						Attr("aria-expanded", ariaBool(detailsOpen.Get())),
						Attr("aria-label", uistate.T("accountsRedesign.detailsAria", a.Name)),
						OnClick(toggleDetails), detailsLabel),
				),
			),
			// L100-T1: the headline balance carries an explicit accessible name to
			// disambiguate it from the dim "cleared (…)" figure. G2/C4: the figure is
			// also the ONE consistent update affordance on every row — click it to open
			// the update-balance editor (dotted underline on hover signals editability);
			// archived rows keep a plain, non-interactive figure.
			IfElse(!a.Archived,
				Button(ClassStr(amountClass(dispBal)+" acct-row-figure budget-limit-btn"), Type("button"),
					Attr("data-testid", "acct-balance-btn-"+a.ID),
					Title(uistate.T("accounts.balanceEditTitle")),
					Attr("aria-label", uistate.T("accounts.balanceAria", fmtMoney(dispBal))),
					OnClick(setBal),
					fmtMoney(dispBal)),
				Span(ClassStr(amountClass(dispBal)+" acct-row-figure"),
					Title(uistate.T("accounts.balanceTitle")),
					Attr("aria-label", uistate.T("accounts.balanceAria", fmtMoney(dispBal))),
					fmtMoney(dispBal))),
			// Quick actions inline: ONE primary — Transactions (the highest-frequency
			// navigation) — plus the conditional "Update value/balance" for accounts you
			// actively maintain (stale/valuation, emphasized when stale). Everything else,
			// Edit included, is demoted into the ⋯ menu so the resting row is a clean
			// name → balance → [Transactions] [⋯] line instead of a wall of equal-weight
			// buttons. The balance figure itself is already click-to-edit for updates.
			Div(css.Class("acct-row-actions"),
				If(showValueInline, Button(css.Class(updBtnCls, tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "update-value-btn-"+a.ID), Title(uistate.T("accounts.updateBalanceTitle")), OnClick(setBal), uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T(updateActionKey(a.Type))))),
				Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "acct-view-txns-"+a.ID), Title(uistate.T("accounts.viewTxnsTitle")), OnClick(view), uiw.Icon(icon.Receipt, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions"))),
				Div(css.Class("add-wrap"), Attr("id", menuID),
					Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("accounts.moreActions")), Attr("aria-label", uistate.T("accounts.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
					Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
					Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
						// Edit leads the menu — the most common of the demoted actions. (The
						// everyday balance update stays inline / on the figure; Edit covers the
						// rarer name/type/attribute changes.) Available on archived rows too,
						// matching the prior inline behavior.
						Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-account-btn-"+a.ID), Title(uistate.T("accounts.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
						// C1: reconcile-to-statement only where statements + transactions exist —
						// a valuation account (property/vehicle/investment/…) uses Update value.
						If(!a.Archived && !isValuationType(a.Type), Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "reconcile-start-btn-"+a.ID), Title(uistate.T("accounts.reconcileWhen")), OnClick(startReconcile), uistate.T("accounts.reconcileTitle"))),
						// C2: money moves FROM liquid cash — a property or 401(k) row doesn't
						// offer an outgoing transfer. (The page-level Transfer money picks any
						// eligible source.) C9: one name for the action everywhere.
						If(!a.Archived && earmarkEligibleType(a.Type), Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
							Attr("data-testid", "transfer-start-btn-"+a.ID), Title(uistate.T("accounts.transferWhen")), OnClick(startTransfer),
							uistate.T("accounts.transferMoney"))),
						If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Title(uistate.T("accounts.markUpdatedWhen")), OnClick(refresh), uistate.T("accounts.markUpdated"))),
						If(!a.Archived, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "set-institution-"+a.ID), OnClick(setInstitution), uistate.T("accounts.setInstitution"))),
						// Security + lifecycle, grouped last: the encrypted vault sits beside
						// Archive/Delete, not amid everyday money actions.
						Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Title(uistate.T("creds.menuItemTitle")), Attr("data-testid", "creds-start-btn-"+a.ID), OnClick(startCredentials), uistate.T("creds.menuItem")),
						Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("title", archTitle), OnClick(arch), archLabel),
						// Delete stays in the menu as the last, destructive item (standing rule:
						// delete never sits exposed on a row).
						Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-account-btn-"+a.ID), Attr("aria-label", uistate.T("accounts.deleteTitle")), Title(uistate.T("accounts.deleteTitle")), OnClick(del), uistate.T("accounts.deleteAction")),
					),
				),
			),
		),
		detailsNode,
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
			OnClick(toggle), uistate.T("transactions.markCleared")),
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
