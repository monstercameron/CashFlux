// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"slices"
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
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/valuation"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type accountRowProps struct {
	Account domain.Account
	Balance money.Money
	Cleared money.Money
	Stale   bool
	// DetailContext lazily prepares the production list's selected chart window,
	// period flow and projection. Direct row consumers can still supply the
	// individual display facts below.
	DetailContext *accountDetailContext
	// DaysStale is the number of days since the balance was last confirmed, or -1 if
	// it never was. The stale badge shows it so the row says how stale the balance is.
	// Only read when Stale is true.
	DaysStale int
	// StaleCollapsed is set by the list when more than half the visible accounts are
	// stale: in that state the loud per-row STALE badge collapses to a subdued dot and
	// a single "Most accounts need a balance update — Mark all updated" summary line
	// leads the list, so the warning stands out instead of becoming wallpaper. Below
	// that threshold it is false and the full badge shows.
	StaleCollapsed          bool
	Members                 []domain.Member
	Accounts                []domain.Account // all non-archived accounts (for Transfer to-picker)
	Categories              []domain.Category
	OnDelete                func(string)
	OnArchive               func(domain.Account)
	OnRefresh               func(domain.Account)
	OnSave                  func(domain.Account)
	OnView                  func(string)
	DefaultHousehold        bool
	DefaultMembers          []string
	OnSetDefaultHousehold   func(string)
	OnClearDefaultHousehold func()
	DefaultMemberID         string
	DefaultMemberName       string
	OnSetDefaultMember      func(accountID, memberID string)
	OnClearDefaultMember    func(memberID string)
	// OnSetBalance posts a balance adjustment that brings the account to newBalStr, the
	// typed target. catID is the optional category (empty = uncategorized), and asOfISO
	// is the date the balance was true as of (empty = today).
	OnSetBalance func(ac domain.Account, current money.Money, newBalStr, catID, asOfISO string)
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
	// flows. When it shows a dip below today's balance, the row renders a
	// "→ ~$X low on <date>" line that expands to list the drivers. Zero value renders
	// nothing.
	Projection    acctproject.Projection
	ProjectionErr error
	// ForwardProjection carries recurring drivers through the forward chart's 90-day
	// horizon. Projection above is the separate 30-day summary.
	ForwardProjection    acctproject.Projection
	ForwardProjectionErr error
	// Sparkline is the account's 90-day end-of-day balance series, oldest first,
	// in the account's currency (minor units). Rendered as an inline SVG polyline; a
	// flat run is itself the "nothing has posted since your last update" signal. Fewer
	// than two points renders nothing.
	Sparkline []int64
	// Sparkline12m / SparklineAll are the wider balance windows for the account-detail
	// chart's range picker: the trailing 12 months and the account's full
	// history. Populated only when HasRange is set (the account has more than 90 days
	// of history); otherwise the detail chart shows just the plain 90-day sparkline.
	Sparkline12m []int64
	SparklineAll []int64
	HasRange     bool
	// Flow is this account's money-in / money-out / net for the current period,
	// with transfers counted separately (never as income or spend). Rendered as compact
	// row figures when ShowFlow is set.
	Flow     accountflow.Flow
	ShowFlow bool
	// InstByID indexes the household's institution directory by id, so the
	// row can color its edge and show a chip for Account.InstitutionID. Nil or a
	// miss renders nothing extra — free-text-only institutions are unaffected.
	InstByID map[string]domain.Institution
}

func defaultAccountPillTitle(props accountRowProps) string {
	if props.DefaultHousehold && len(props.DefaultMembers) > 0 {
		return uistate.T("defaultAccount.pillTitleBoth", strings.Join(defaultMemberNames(props.Members, props.DefaultMembers), ", "))
	}
	if props.DefaultHousehold {
		return uistate.T("defaultAccount.pillTitleHousehold")
	}
	return uistate.T("defaultAccount.pillTitleMembers", strings.Join(defaultMemberNames(props.Members, props.DefaultMembers), ", "))
}

func defaultMemberNames(members []domain.Member, ids []string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, member := range members {
			if member.ID == id {
				names = append(names, member.Name)
				break
			}
		}
	}
	return names
}

// isValuationType reports whether an account's balance is a periodic estimate (an
// investment, retirement, crypto, property, vehicle or other asset) rather than a
// reconciled cash balance. Those accounts read better as "Out of date / Update value"
// than as the banking wording "Stale / Update balance".
func isValuationType(t domain.AccountType) bool {
	return t == domain.TypeInvestment || t == domain.TypeRetirement ||
		t == domain.TypeCrypto || t == domain.TypeProperty ||
		t == domain.TypeVehicle || t == domain.TypeOther
}

// staleBadgeKey returns the label key for the stale badge: the valuation wording for
// valuation types, and the cash wording otherwise.
func staleBadgeKey(t domain.AccountType) string {
	if isValuationType(t) {
		return "accounts.staleValue"
	}
	return "accounts.stale"
}

// updateActionKey returns the label key for the update action, using the same
// valuation or cash split as staleBadgeKey.
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
	return base + uistate.T("accounts.staleDaysSuffix", plural(days, "day"))
}

// staleBadgeTitle is the badge's hover tooltip — a full-sentence version of the
// same freshness the notification states.
func staleBadgeTitle(t domain.AccountType, days int) string {
	if days < 0 {
		return uistate.T("accounts.staleNeverTitle")
	}
	return uistate.T("accounts.staleDaysTitle", days)
}

// staleBadgeNode renders the account's freshness marker on the name line. Normally
// that's the full "Stale · 24d" badge; when the list is in the collapsed state
// (StaleCollapsed — more than half the visible accounts are stale) it shrinks to a
// subdued neutral dot, and the one summary line at the top of the list carries the
// call to action instead. Renders nothing when the account isn't stale.
func staleBadgeNode(props accountRowProps) ui.Node {
	if !props.Stale {
		return Fragment()
	}
	a := props.Account
	if props.StaleCollapsed {
		return Span(css.Class("acct-stale-dot"), Attr("data-testid", "acct-stale-dot-"+a.ID),
			Role("img"), AriaLabel(uistate.T("accounts.staleDotAria")),
			Title(staleBadgeTitle(a.Type, props.DaysStale)))
	}
	return Span(css.Class("badge badge-prio prio-med"), Attr("data-testid", "acct-stale-badge-"+a.ID),
		Title(staleBadgeTitle(a.Type, props.DaysStale)), staleBadgeText(a.Type, props.DaysStale))
}

// isSharedAccount reports whether an account is shared with the household: shared scope,
// group-owned, or split by ownership shares.
func isSharedAccount(a domain.Account) bool {
	return a.Scope == domain.ScopeShared || a.OwnerID == domain.GroupOwnerID || len(a.OwnershipShares) > 0
}

// acctOwnerName resolves the display name of the member that owns an account, or ""
// when the id is empty / group-owned / not a known member. Used to show ownership on
// individually-owned rows (the Shared chip already marks group/shared ones).
func acctOwnerName(members []domain.Member, ownerID string) string {
	if ownerID == "" || ownerID == domain.GroupOwnerID {
		return ""
	}
	for _, m := range members {
		if m.ID == ownerID {
			return m.Name
		}
	}
	return ""
}

// acctOwnerChip renders a quiet owner chip on an individually-owned account row,
// matching the SHARED chip's weight (badge-muted). Shared / group / fractionally-
// shared accounts already wear the SHARED chip, so they get nothing here; rows with
// no resolvable Member owner also render nothing.
func acctOwnerChip(props accountRowProps) ui.Node {
	a := props.Account
	if isSharedAccount(a) {
		return Fragment()
	}
	name := acctOwnerName(props.Members, a.OwnerID)
	if name == "" {
		return Fragment()
	}
	return Span(css.Class("badge badge-muted"), Attr("data-testid", "acct-owner-badge-"+a.ID),
		Title(uistate.T("accounts.ownerBadgeTitle", name)),
		AriaLabel(uistate.T("accounts.ownerBadgeTitle", name)), name)
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

// rateFieldValue renders an account's APR for a form field, or "" when no rate has been
// recorded, so an empty box reads back as no rate. A recorded 0% shows as "0", not as
// blank, because somebody typed it.
func rateFieldValue(a domain.Account) string {
	r, ok := a.RateAPR()
	if !ok {
		return ""
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// applyRateAPR reads an APR field back onto an account: a blank box records NO
// rate, and any number — including zero — records that number. This is the seam
// where "nobody said" stays distinguishable from "0%", so it must not go through
// ParseFloat, which answers 0 for both.
func applyRateAPR(a domain.Account, raw string) domain.Account {
	v, ok := textutil.ParseOptionalFloat(raw)
	if !ok {
		return a.WithoutRateAPR()
	}
	return a.WithRateAPR(v)
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
	// (primary: Transactions / Edit / ✕); selecting one closes the menu.
	menuID := ui.UseId()
	// The shared menu state: the same fade, placement, dismissal and stacking as the More menu.
	menu := uiw.UseMenu(menuID)
	toggleMenu := ui.UseEvent(Prevent(func() { menu.Toggle() }))

	del := ui.UseEvent(Prevent(func() { menu.Set(false); props.OnDelete(a.ID) }))
	arch := ui.UseEvent(Prevent(func() { menu.Set(false); props.OnArchive(a) }))
	refresh := ui.UseEvent(Prevent(func() { menu.Set(false); props.OnRefresh(a) }))
	view := ui.UseEvent(Prevent(func() { menu.Set(false); props.OnView(a.ID) }))
	setHouseholdDefault := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		if props.OnSetDefaultHousehold != nil {
			props.OnSetDefaultHousehold(a.ID)
		}
	}))
	clearHouseholdDefault := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		if props.OnClearDefaultHousehold != nil {
			props.OnClearDefaultHousehold()
		}
	}))
	setMemberDefault := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		if props.OnSetDefaultMember != nil && props.DefaultMemberID != "" {
			props.OnSetDefaultMember(a.ID, props.DefaultMemberID)
		}
	}))
	clearMemberDefault := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		if props.OnClearDefaultMember != nil && props.DefaultMemberID != "" {
			props.OnClearDefaultMember(props.DefaultMemberID)
		}
	}))
	// Investment accounts deep-link to their holdings and growth card on /investments.
	// The row navigates itself and sets the deep-link focus, so the destination card
	// scrolls into view and flashes once the page settles. The hook is always
	// registered; the link renders only for investment accounts (isInvestmentAccount),
	// so the row needs no extra prop.
	nav := router.UseNavigate()
	isInvest := isInvestmentAccount(a.Type) && !a.Archived
	openHoldings := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		uistate.SetDeepLinkFocus(`[data-testid="invest-acct-` + a.ID + `"]`)
		nav.Navigate(uistate.RoutePath("/investments"))
	}))
	// Notes are shown inline as a readable, clamped line that expands on click (a
	// disclosure), so a note is actually legible in the row rather than hidden in a
	// hover tooltip on a tiny glyph.
	notesExpanded := ui.UseState(false)
	toggleNotes := ui.UseEvent(Prevent(func() { notesExpanded.Set(!notesExpanded.Get()) }))
	// The projected-low "Why?" disclosure owns its state at this render position.
	// AccountRow is itself the per-row component, so these hooks never run in a loop.
	projExpanded := ui.UseState(false)
	projMounted := uiw.UsePanelFade(projExpanded.Get())
	toggleProj := ui.UseEvent(Prevent(func() { projExpanded.Set(!projExpanded.Get()) }))
	// The detail block (90-day trend, this-period flow, projection, filed documents,
	// notes, custom fields) sits behind a quiet per-row disclosure, so the resting list
	// reads as name then balance, and nothing competes.
	detailsOpen := ui.UseState(false)
	// Each disclosure keeps its content mounted for the fade-out, so opening and closing both animate.
	detailsMounted := uiw.UsePanelFade(detailsOpen.Get())
	toggleDetails := ui.UseEvent(Prevent(func() { detailsOpen.Set(!detailsOpen.Get()) }))
	// The detail chart's range picker (90d / 12m / all) keeps its selection in the
	// row's state, at a stable hook position. The choice is view-only, so it is not
	// persisted.
	chartRange := ui.UseState("90d")
	onChartRange := func(v string) { chartRange.Set(v) }
	if detailsOpen.Get() && props.DetailContext != nil {
		facts := props.DetailContext.facts(a)
		props.Flow, props.ShowFlow, props.Projection = facts.Flow, true, facts.Projection
		props.ProjectionErr = facts.ProjectionErr
		props.ForwardProjection = facts.ForwardProjection
		props.ForwardProjectionErr = facts.ForwardProjectionErr
		props.ValuationHistory = facts.Valuations
		series, hasRange := props.DetailContext.series(a, chartRange.Get())
		props.HasRange = hasRange
		switch chartRange.Get() {
		case "12m":
			props.Sparkline12m = series
		case "all":
			props.SparklineAll = series
		default:
			props.Sparkline = series
		}
	}
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
			menu.Set(false)
			acctEditAtom.Set(uistate.AccountEdit{ID: a.ID, Mode: mode})
		}
	}
	setBal := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeSetBal)))
	startEdit := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeEdit)))
	startReconcile := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeReconcile)))
	startTransfer := ui.UseEvent(Prevent(openEditor(uistate.AcctEditModeTransfer)))
	// The encrypted credential vault opens in its own shell-root modal (CredentialVaultHost).
	acctCredsAtom := uistate.UseAccountCredentials()
	startCredentials := ui.UseEvent(Prevent(func() { menu.Set(false); acctCredsAtom.Set(a.ID) }))
	// Quick institution assignment: a prompt seeded with the current value, normalised
	// the same way the add form normalises it, with no modal in between.
	setInstitution := ui.UseEvent(Prevent(func() {
		menu.Set(false)
		uistate.PromptModalResult(uistate.T("accounts.setInstitutionPrompt", a.Name), a.Institution, func(ok bool, v string) {
			if !ok {
				return
			}
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
	// A liability is shown as what it does to net worth. The figure goes through the
	// account's own storage convention rather than -Abs, because -Abs turns an
	// overpaid card (money the lender owes you) into more debt. Editing still works on
	// the raw stored value; only the displayed figures are converted.
	dispBal, dispCleared := props.Balance, props.Cleared
	if a.Class == domain.ClassLiability {
		conv, _ := reconcile.ConventionOf(a, props.Balance.Amount)
		dispBal = money.New(reconcile.Stated(props.Balance.Amount, conv), props.Balance.Currency)
		dispCleared = money.New(reconcile.Stated(props.Cleared.Amount, conv), props.Cleared.Currency)
	}

	meta := accountMeta(a, props.Balance)
	// The cleared figure is a reconciliation aid: what a statement has confirmed so far.
	// Beside the real balance it reads as a second claim about what is owed, so it is
	// shown only for reconcilable accounts. Otherwise a utility or HOA shell with a zero
	// balance would still show a cleared amount and look like a debt. Valuation-type
	// accounts are estimated rather than reconciled, so the same reasoning applies.
	if a.Type.IsReconcilable() && props.Cleared.Amount != props.Balance.Amount {
		meta += uistate.T("accounts.clearedSuffix", fmtMoney(dispCleared))
	}

	// Month-to-date value change for valuation-type accounts (home, car, investment): one
	// signed figure such as "▲ $2,000.00 this month". Shown only when there is history
	// to compare against.
	valChange := ui.If(isValuationType(a.Type) && len(props.ValuationHistory) >= 2, func() ui.Node {
		chg, ok := valuation.MonthToDateChange(props.ValuationHistory, props.Balance, time.Now())
		if !ok {
			return Fragment()
		}
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
		return Span(ClassStr(cls), Attr("data-testid", "val-change-"+a.ID), label)
	})

	// Bill-payment line: the account's most recent linked bill payment, with a drill to
	// the payments. Shown for ANY account the user has linked a bill payment to (the
	// Debt page shows liabilities; this makes the link visible on every account).
	billNode := If(props.BillPayment.HasAny, Span(css.Class("row-meta"), Attr("data-testid", "acct-bill-"+a.ID),
		uistate.T("accounts.billMeta", fmtMoney(props.BillPayment.Latest)),
		Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "acct-bill-link-"+a.ID),
			Title(uistate.T("accounts.billLinkTitle")), OnClick(viewBills),
			uistate.T("accounts.billCount", plural(props.BillPayment.Count, "payment")))))

	// Readable notes line: the note text itself, clamped to a couple of lines and
	// expandable on click, so an attached note is legible in the row instead of being
	// hidden behind a hover tooltip on a tiny glyph (replaces acct-notes-dot).
	notes := strings.TrimSpace(a.Notes)
	notesNode := ui.If(notes != "", func() ui.Node {
		notesCls := ClassNames("acct-notes", When(notesExpanded.Get(), "open"))
		toggleLabel := uistate.T("accounts.notesReadMore")
		if notesExpanded.Get() {
			toggleLabel = uistate.T("accounts.notesReadLess")
		}
		return Button(ClassStr(notesCls), Type("button"), Attr("data-testid", "acct-notes-"+a.ID),
			AriaExpanded(notesExpanded.Get()), AriaLabel(uistate.T("accounts.notesBadge")),
			Title(toggleLabel), OnClick(toggleNotes),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), notes),
		)
	})

	// The projected-balance line: for cash accounts whose 30-day projection dips below
	// today's balance, it shows "→ ~$X low on <date>" with a "Why?" disclosure that lists
	// the recurring drivers ("Rent −$1,400 on Mar 1"). Liabilities and valuation-type
	// assets are skipped, because their balances are not cash curves.
	var projNode ui.Node = Fragment()
	cashCurve := a.Class != domain.ClassLiability && !isValuationType(a.Type)
	if cashCurve && props.ProjectionErr != nil {
		projNode = P(css.Class("row-meta"), Role("status"), Attr("data-testid", "acct-proj-unavailable-"+a.ID), uistate.T("calculation.unavailable"))
	} else if cashCurve && props.Projection.HasLowDip() {
		dec := currency.Decimals(a.Currency)
		low := money.New(props.Projection.Low, a.Currency)
		toggleLabel := uistate.T("accounts.projectedShow")
		if projExpanded.Get() {
			toggleLabel = uistate.T("accounts.projectedHide")
		}
		var driverNode ui.Node = Fragment()
		if projExpanded.Get() || projMounted {
			rows := []any{css.Class(uiw.FadeClass("acct-proj-drivers", projExpanded.Get())), Attr("data-testid", "acct-proj-drivers-"+a.ID),
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
				AriaLabel(uistate.T("accounts.projectedLowAria", fmtMoney(low), fmtShortDate(props.Projection.LowDate))),
				uistate.T("accounts.projectedLow", fmtMoney(low), fmtShortDate(props.Projection.LowDate))),
			uiw.ToolButton(uiw.ToolButtonProps{Label: toggleLabel, Open: projExpanded.Get(), TestID: "acct-proj-toggle-" + a.ID},
				AriaExpanded(projExpanded.Get()), OnClick(toggleProj)),
			driverNode,
		)
	}

	// The row leads with one inline everyday action, "Transactions". The balance figure
	// stays click-to-edit, which is the main update affordance. The explicit "Update
	// value/balance" action sits in the ⋯ menu for the accounts that need it (stale ones
	// and valuation-type assets revalued by hand), so no row has a second prominent button.
	showValueInMenu := (props.Stale || isValuationType(a.Type)) && !a.Archived

	// This-period in, out and net figures. The balance chart is built inside the details
	// block below, so its range picker mounts only while the panel is open.
	flowNode := accountFlowFigures(a, props.Flow, props.ShowFlow)

	// Color the row's left edge by the account's institution when it references one from
	// the directory. A plain border keeps the cue light. Accounts without an institution
	// keep the plain row.
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
	// The revealed block holds the extra detail. It mounts only while open, so the resting
	// row stays a clean name-and-balance line. Child components own their hooks, so
	// conditional mounting here is safe.
	var detailsNode ui.Node = Fragment()
	if detailsOpen.Get() || detailsMounted {
		customText := customSummary(props.AccountDefs, a.Custom)
		var chart ui.Node
		validBalance := true
		if props.DetailContext != nil {
			_, validBalance = props.DetailContext.snapshot.Balances[a.ID]
		}
		if validBalance {
			chart = accountBalanceChart(a, props.Sparkline, props.Sparkline12m, props.SparklineAll, props.HasRange, chartRange.Get(), onChartRange)
		} else {
			chart = P(css.Class("row-meta"), Role("status"), Attr("data-testid", "acct-history-unavailable-"+a.ID), uistate.T("calculation.unavailable"))
		}
		// The panel is a grid: the period figures run across the top, the balance chart and
		// the 90-day projection sit left, the reading and the account's facts sit right, and
		// the documents and the institution share a footer line.
		detailsNode = Div(css.Class(uiw.FadeClass("acct-row-details", detailsOpen.Get())), Attr("id", "acct-details-"+a.ID), Attr("data-testid", "acct-details-"+a.ID),
			flowNode,
			Div(css.Class("acct-panel-chart"),
				// The 90-day balance figure, with a 90d/12m/all range picker
				// when the account has more than 90 days of history.
				chart,
				// Where the balance goes next. The history chart above shows where the
				// account has been; this shows whether it lasts until payday.
				accountForwardLine(a, props.Balance, props.ForwardProjection, props.ForwardProjectionErr, time.Now()),
			),
			Div(css.Class("acct-panel-side"),
				valChange,
				projNode,
				billNode,
				// Warn when goals have earmarked more against this account than it holds.
				// A healthy account renders nothing.
				ui.CreateElement(accountEarmarkWarning, accountEarmarkWarnProps{Account: a, Balance: props.Balance}),
				// Read-only summary of the account's custom-field values (a compact
				// "Label: value" line), shown only when any are set.
				If(customText != "",
					Span(css.Class("row-meta", tw.TextDim), Attr("data-testid", "acct-custom-summary-"+a.ID),
						customText)),
				// Readable, clickable-to-expand notes line (the attached note itself).
				notesNode,
			),
			Div(css.Class("acct-panel-footer"),
				// The filed-documents drawer (statements, contracts, titles,
				// payoff letters), with an attach form that takes an optional renewal date.
				ui.CreateElement(accountDocsDrawer, accountDocsDrawerProps{Account: a}),
				// Nudge to fill in a missing institution. It is styled as a
				// real link action, because the dim caption treatment read as stray plain text in
				// the details panel.
				If(a.Institution == "" && !a.Archived,
					uiw.ToolButton(uiw.ToolButtonProps{Label: uistate.T("accounts.setInstitution"), Icon: icon.Pencil, Class: "acct-set-institution", TestID: "set-institution-" + a.ID},
						OnClick(startEdit))),
			),
		)
	}

	return Div(css.Class("row acct-row"), Attr("data-testid", "acct-row-"+a.ID), Style(rowStyle),
		// PRIMARY line: type glyph + name/badges on the left, the balance figure and the
		// row actions right-aligned. Everything else is demoted to the sub-line or details.
		Div(css.Class("acct-row-head"),
			// Account-type glyph: a quick visual tag so checking, investment and credit card
			// accounts are distinguishable without reading the meta line.
			Span(css.Class("acct-type-icon", tw.TextDim), AriaHidden(true),
				uiw.Icon(accountTypeIcon(a.Type), css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Div(css.Class("acct-row-id"),
				Div(css.Class("acct-row-name"),
					Span(css.Class("row-desc"), a.Name),
					If(props.DefaultHousehold || len(props.DefaultMembers) > 0, uiw.Pill(uiw.PillProps{Label: uistate.T("defaultAccount.pill"), Tone: uiw.PillDim, Title: defaultAccountPillTitle(props)})),
					// Shared, group-owned and fractionally shared accounts wear a quiet badge, so
					// shared and individual ownership is visible in the list. Scope is checked too,
					// because seeded or imported data can mark an account shared while a person
					// administers it (OwnerID set).
					If(isSharedAccount(a),
						Span(css.Class("badge badge-muted"), Attr("data-testid", "acct-shared-badge-"+a.ID),
							Title(uistate.T("accounts.sharedBadgeTitle")), AriaLabel(uistate.T("accounts.sharedBadge")),
							uiw.Icon(icon.Users, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
					// Individually owned rows name their owner (a member) in a quiet chip, the same
					// weight as the shared chip.
					acctOwnerChip(props),
					// Freshness marker: full badge normally; a subdued dot when the list has
					// collapsed the stale badges (StaleCollapsed) into one summary line.
					staleBadgeNode(props),
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
						AriaExpanded(detailsOpen.Get()),
						// The panel is only in the document while open, so the relation is only stated then.
						IfProp(detailsOpen.Get(), AriaControls("acct-details-"+a.ID)),
						AriaLabel(uistate.T("accountsRedesign.detailsAria", a.Name)),
						Title(detailsLabel), OnClick(toggleDetails),
						uiw.Icon(icon.ChevronDown, css.Class("acct-details-chev", tw.ShrinkO, tw.W4, tw.H4))),
					// Investment accounts link to their holdings and growth on /investments. This is
					// a quiet link, not an everyday-action button, so the row still leads with one
					// inline action.
					If(isInvest,
						Button(css.Class("btn-link acct-holdings-link", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
							Attr("data-testid", "acct-holdings-link-"+a.ID),
							AriaLabel(uistate.T("accountsInvest.viewHoldingsTitle")),
							Title(uistate.T("accountsInvest.viewHoldingsTitle")),
							OnClick(openHoldings),
							uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
							Span(uistate.T("accountsInvest.viewHoldings")))),
				),
			),
			// The headline balance has an explicit accessible name, which tells it apart from
			// the dim cleared figure. It is also the one update affordance on every row:
			// clicking it opens the update-balance editor, and a dotted underline on hover
			// shows that it is editable.
			// Archived rows keep a plain figure.
			IfElse(!a.Archived,
				Button(ClassStr(amountClass(dispBal)+" acct-row-figure budget-limit-btn"), Type("button"),
					Attr("data-testid", "acct-balance-btn-"+a.ID),
					Title(uistate.T("accounts.balanceEditTitle")),
					AriaLabel(uistate.T("accounts.balanceEditNamed", a.Name, fmtMoney(dispBal))),
					OnClick(setBal),
					fmtMoney(dispBal)),
				Span(ClassStr(amountClass(dispBal)+" acct-row-figure"),
					Title(uistate.T("accounts.balanceTitle")),
					AriaLabel(uistate.T("accounts.balanceAria", fmtMoney(dispBal))),
					fmtMoney(dispBal))),
			// Quieter rows: exactly ONE inline everyday action ("Transactions"), plus the
			// ⋯ overflow. The balance figure itself is already click-to-edit for updates,
			// and the explicit "Update value/balance" action lives in the menu, so
			// the resting row is a clean name → balance → [Transactions] [⋯] line instead
			// of a wall of equal-weight buttons.
			Div(css.Class("acct-row-actions"),
				// Each row's Transactions button names its account, so the buttons stay
				// distinguishable in a long list.
				uiw.ToolButton(uiw.ToolButtonProps{
					Label: uistate.T("nav.transactions"), Icon: icon.Receipt, Title: uistate.T("accounts.viewTxnsTitle"),
					AriaLabel: uistate.T("nav.transactions") + " — " + a.Name, TestID: "acct-view-txns-" + a.ID,
				},
					OnClick(view),
				),
				Div(css.Class(menu.WrapClass("add-wrap")), Attr("id", menuID),
					uiw.ToolButton(uiw.ToolButtonProps{
						Icon: icon.MoreH, Title: uistate.T("accounts.moreActions"),
						AriaLabel: uistate.T("accounts.moreActions") + " — " + a.Name, Open: menu.Open,
					},
						Attr("aria-haspopup", "menu"), AriaExpanded(menu.Open), OnClick(toggleMenu),
					),
					Div(ClassStr(menu.PanelClass()), Role("menu"),
						If(!a.Archived && !props.DefaultHousehold, Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "acct-default-household-"+a.ID), OnClick(setHouseholdDefault), uistate.T("defaultAccount.makeHousehold"))),
						If(!a.Archived && props.DefaultHousehold, Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "acct-clear-household-default-"+a.ID), OnClick(clearHouseholdDefault), uistate.T("defaultAccount.clearHousehold"))),
						If(!a.Archived && props.DefaultMemberID != "" && !slices.Contains(props.DefaultMembers, props.DefaultMemberID), Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "acct-default-member-"+a.ID), OnClick(setMemberDefault), uistate.T("defaultAccount.makeMember", props.DefaultMemberName))),
						If(!a.Archived && props.DefaultMemberID != "" && slices.Contains(props.DefaultMembers, props.DefaultMemberID), Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "acct-clear-member-default-"+a.ID), OnClick(clearMemberDefault), uistate.T("defaultAccount.clearMember", props.DefaultMemberName))),
						// The "Update value/balance" action leads the menu for the rows that need it
						// (stale rows and valuation assets). It opens the same set-balance editor as
						// the balance figure.
						If(showValueInMenu, Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "update-value-btn-"+a.ID), Title(uistate.T("accounts.updateBalanceTitle")), OnClick(setBal), uistate.T(updateActionKey(a.Type)))),
						// Edit leads the rest of the menu, as the most common of these actions. Balance
						// updates stay on the figure; Edit covers the rarer name, type and attribute
						// changes. It is also available on archived rows.
						Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "edit-account-btn-"+a.ID), Title(uistate.T("accounts.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
						// Reconcile is offered only for reconcilable account types, never for valuation
						// types, which use Update value instead. Offering it on an account whose cleared
						// figure the row hides would be worse than offering nothing.
						If(!a.Archived && !isValuationType(a.Type) && a.Type.IsReconcilable(), Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "reconcile-start-btn-"+a.ID), Title(uistate.T("accounts.reconcileWhen")), OnClick(startReconcile), uistate.T("accounts.reconcileTitle"))),
						// Money moves out of liquid cash, so a property or 401(k) row does not offer a
						// transfer. The page-level Transfer money action picks any eligible source. The
						// action has one name everywhere.
						If(!a.Archived && earmarkEligibleType(a.Type), Button(css.Class("add-item"), Type("button"), Role("menuitem"),
							Attr("data-testid", "transfer-start-btn-"+a.ID), Title(uistate.T("accounts.transferWhen")), OnClick(startTransfer),
							uistate.T("accounts.transferMoney"))),
						If(!a.Archived, Button(css.Class("add-item"), Type("button"), Role("menuitem"), Title(uistate.T("accounts.markUpdatedWhen")), OnClick(refresh), uistate.T("accounts.markUpdated"))),
						If(!a.Archived, Button(css.Class("add-item"), Type("button"), Role("menuitem"), Attr("data-testid", "set-institution-"+a.ID), OnClick(setInstitution), uistate.T("accounts.setInstitution"))),
						// Security + lifecycle, grouped last: the encrypted vault sits beside
						// Archive/Delete, not amid everyday money actions.
						Button(css.Class("add-item"), Type("button"), Role("menuitem"), Title(uistate.T("creds.menuItemTitle")), Attr("data-testid", "creds-start-btn-"+a.ID), OnClick(startCredentials), uistate.T("creds.menuItem")),
						Button(css.Class("add-item"), Type("button"), Role("menuitem"), Title(archTitle), OnClick(arch), archLabel),
						// Delete stays in the menu as the last, destructive item. It is never exposed on
						// the row.
						Button(css.Class("add-item danger"), Type("button"), Role("menuitem"), Attr("data-testid", "delete-account-btn-"+a.ID), AriaLabel(uistate.T("accounts.deleteTitle")), Title(uistate.T("accounts.deleteTitle")), OnClick(del), uistate.T("accounts.deleteAction")),
					),
				),
			),
		),
		detailsNode,
		// The account's own findings, stated inline. The dot above says something is true
		// about this row; these lines say what, and let it be acted on. They show at
		// Standard density and up. At Minimal, the row keeps only the dot.
		smartRowInsightsFor(props.SmartSettings, props.SmartByEntity, a.ID, accountRowSmartCodes),
		// The repeats found on this account, each with the one action worth having on
		// them. Collapsed by default: the row is already dense, and this is opt-in detail,
		// like the budget card's drivers disclosure.
		ui.CreateElement(accountRecurringPanel, accountRecurProps{Account: a}),
	)
}

// reconcileTxnRowProps holds the data and callback for a single uncleared
// transaction row inside the reconcile-to-statement panel.
type reconcileTxnRowProps struct {
	Txn domain.Transaction
	// OnToggle is a plain func — never an On* hook — so the parent can pass it
	// into MapKeyed without violating the no-On*-in-loop rule (CLAUDE.md §gotchas).
	OnToggle func(domain.Transaction)
}

// ReconcileTxnRow renders a single uncleared transaction for the reconcile panel. It
// owns its click handler, and its "Mark cleared" button label doubles as a status badge.
func ReconcileTxnRow(props reconcileTxnRowProps) ui.Node {
	t := props.Txn
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(t) }))
	return Div(css.Class("row"), Attr("data-testid", "reconcile-txn-row"), Attr("data-id", t.ID),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), t.Desc),
			Span(css.Class("row-meta"), uistate.CurrentPrefs().FormatDate(t.Date)),
		),
		// Standard accounting figure — "$807.45" / "($620.00)" — to match the rest
		// of the app, not a bare "807.45" / "-620.00".
		Span(ClassStr("fig "+amountClass(t.Amount)), fmtMoney(t.Amount)),
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
		css.Class(css.New(css.GridCols(css.Fr(1), css.TrackLen(css.Length("auto"))), css.Items.Center, css.Gap(css.Rem(0.5)))),
		Span(css.Class(tw.TextDim), props.Member.Name),
		Label(css.Class("labeled-field"),
			css.Class(css.New(css.Display.Flex, css.FlexDir.Row, css.Items.Center, css.Gap(css.Rem(0.25)))),
			Input(css.Class("field"),
				css.Class(css.New(css.W(css.Length("5rem")))),
				Type("number"), Attr("min", "0"), Attr("max", "100"), Step("1"),
				AriaLabel(props.Member.Name+" "+uistate.T("account.sharePercent")), OnInput(onChange), uiw.FieldValue(shareStr)),
			Span(css.Class(tw.TextDim), "%"),
		),
	)
}
