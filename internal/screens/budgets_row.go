// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status

	// The everyday actions are labelled tool buttons surfaced inline (the full-width
	// card has the room); the DESTRUCTIVE actions (Remove recurring, Delete) live in a
	// "⋯" overflow menu — a standing directive: delete always stays in the kebab, never
	// as an always-visible row button. Escape + outside-pointerdown dismiss the menu;
	// AnchorPopover flips it near the viewport edge.
	menuOpen := ui.UseState(false)
	menuID := ui.UseId()
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	uiw.DismissPopover(menuOpen.Get(), menuID, func() { menuOpen.Set(false) })
	uiw.AnchorPopover(menuOpen.Get(), menuID)
	prefs := uistate.UsePrefs().Get()

	// G1: inline limit editing — clicking the limit figure swaps it for a compact
	// number input (Enter/✓ saves, ✕ cancels), so the single most frequent budget
	// edit never costs a modal round-trip. Edits the BASE limit; the effective-cap
	// line explains any carry/boost delta.
	limitEditing := ui.UseState(false)
	limitDraft := ui.UseState("")
	limitDec := currency.Decimals(s.Budget.Limit.Currency)
	startLimitEdit := ui.UseEvent(Prevent(func() {
		// Seed from the STORED base limit — s.Budget here is the evaluation copy whose
		// Limit has rollover carry / period boosts folded in (the effective cap). If the
		// draft seeded from that, an untouched save would silently rewrite the base
		// limit to the cap, destroying the carry distinction. Look up the real budget.
		seed := s.Budget.Limit
		if app := appstate.Default; app != nil {
			for _, bb := range app.Budgets() {
				if bb.ID == s.Budget.ID {
					seed = bb.Limit
					break
				}
			}
		}
		limitDraft.Set(money.FormatMinor(seed.Amount, limitDec))
		limitEditing.Set(true)
	}))
	cancelLimitEdit := ui.UseEvent(Prevent(func() { limitEditing.Set(false) }))
	onLimitDraft := ui.UseEvent(func(v string) { limitDraft.Set(v) })
	saveLimitEdit := ui.UseEvent(Prevent(func() {
		amt, perr := money.ParseMinor(strings.TrimSpace(limitDraft.Get()), limitDec)
		if perr != nil || amt <= 0 {
			limitEditing.Set(false)
			return
		}
		if app := appstate.Default; app != nil {
			for _, bb := range app.Budgets() {
				if bb.ID != s.Budget.ID {
					continue
				}
				if bb.Limit.Amount != amt {
					bb.Limit = money.New(amt, bb.Limit.Currency)
					if err := app.PutBudget(bb); err == nil {
						uistate.PostUndoable(uistate.T("budgets.limitChangedToast", fmtMoney(bb.Limit)))
						uistate.BumpDataRevision()
					}
				}
				break
			}
		}
		limitEditing.Set(false)
	}))

	del := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnDelete(s.Budget.ID) }))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			// Pass EVERY tracked category so a multi-category budget drills to
			// transactions in all of them, not just the primary one.
			props.OnDrill(s.Budget.TrackedCategoryIDs())
		}
	}))
	// The drill affordances show whenever the budget tracks any category (a
	// multi-category budget may have no single primary CategoryID).
	canDrill := len(s.Budget.TrackedCategoryIDs()) > 0
	// Edit and Top up open the shell-root flip modal (BudgetEditHost) rather than an
	// inline row form: a row sits under transformed bento/tile ancestors, which threw an
	// in-row modal off-centre. SetBudgetEdit updates the atom the host captured. Edit is
	// the lower-frequency action, so it lives in the ⋯ menu (and closes it); Top up is a
	// visible card button.
	openEdit := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeEdit})
	}))
	openCategories := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.SetBudgetCategoriesEdit(s.Budget.ID)
	}))
	openTopup := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeTopup})
	}))

	// "Cover…" opens the shell-root flip modal (BudgetEditHost cover mode), which picks
	// a source budget + amount and moves the limit — no longer an inline row form.
	openCover := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeCover})
	}))
	// Notes + Formulas open their own modes of the same shell-root editor modal.
	// (They also close the kebab, which is where they now live.)
	openNotes := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeNotes})
	}))
	openFormulas := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeFormulas})
	}))
	// Transactions drill navigates to the filtered ledger.
	drillMenu := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Budget.TrackedCategoryIDs())
		}
	}))
	// Jump to the To-do list PRE-FILTERED to this budget's follow-ups (link=budget +
	// the specific budget id), used by both a to-do row's title and the "+N more" link.
	openBudgetTodos := func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkBudget)
		uistate.SetTodoFilterLinkID(s.Budget.ID)
		if props.OnViewTodos != nil {
			props.OnViewTodos()
		}
	}
	openTodos := ui.UseEvent(Prevent(func() { openBudgetTodos() }))
	removeRecurring := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		if props.OnRemoveRecurring != nil {
			props.OnRemoveRecurring(s.Budget.ID)
		}
	}))
	hasRecurring := s.Budget.RecurringCover != nil
	// Coverage badge — differentiate continual (recurring) from a one-time cover this
	// period. Recurring wins (it's inherently covered), so the two never both show.
	var coverageLine ui.Node = Fragment()
	if hasRecurring {
		coverageLine = Span(css.Class("budget-sub", "budget-recurring"), Attr("data-testid", "recurring-badge-"+s.Budget.ID), uistate.T("budgets.recurringBadge"))
	} else if props.Covered {
		coverageLine = Span(css.Class("budget-sub", "budget-covered"), Attr("data-testid", "covered-badge-"+s.Budget.ID), uistate.T("budgets.coveredBadge"))
	}

	limit, _ := s.Spent.Add(s.Remaining) // limit in base currency

	// When the "Last month's spend" overlay is on, the tile leads with LAST period's spend
	// (against this month's budget): the big bar shows it and this month drops to a small
	// reference line below — so last month "takes over the tile" instead of a tiny bar.
	lastMonthMode := props.LastMonthSpent != ""

	// Bar figures + fill (this month by default; swapped to last month below).
	barSpent := fmtMoney(s.Spent)
	barPct := s.Percent
	width := s.Percent
	fillClass := "bar-fill"
	label := uistate.T("budgets.onTrack")
	switch s.State {
	case budgeting.StateNear:
		fillClass = "bar-fill near"
		label = uistate.T("budgets.nearLimit")
	case budgeting.StateOver:
		fillClass = "bar-fill over"
		label = uistate.T("budgets.overBudget")
	default:
		// Not over/near yet, but the pace projection says this budget is trending
		// to overspend — don't claim "On track" while also warning of an overspend
		// (the L35 contradiction). Call it "At risk" instead.
		if props.PaceOver != "" {
			fillClass = "bar-fill near"
			label = uistate.T("budgets.atRisk")
		}
	}
	if lastMonthMode {
		barSpent, barPct, width = props.LastMonthSpent, props.LastMonthPct, props.LastMonthFill
		if props.LastMonthOver {
			// An actual overrun stays danger red — it's a fact, whenever it happened.
			fillClass = "bar-fill over"
		} else {
			// History fills NEUTRAL: the healthy-green live tone (and the amber "near"
			// warning) under a LAST MONTH tag would read as a statement about now
			// (design critique: green was doing too many jobs).
			fillClass = "bar-fill is-hist"
		}
	}
	if width > 100 {
		width = 100
	}

	// Show "name · category" only when they add information (see budgetTitle).
	title := budgetTitle(s.Budget.Name, props.Category)

	// Owner tag (L106 learning): an INDIVIDUAL budget only counts its owner's spending, so a household
	// can't otherwise tell why a shared expense didn't move it. Flag whose it is — but only for
	// individual budgets (OwnerID matches a real member); shared/household budgets (the common default,
	// OwnerID = group) stay unlabeled to keep rows clean.
	var ownerLine ui.Node = Fragment()
	for _, m := range props.Members {
		if m.ID == s.Budget.OwnerID {
			ownerLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.individualOwner", m.Name))
			break
		}
	}

	// C118: show a small method badge when this budget has its own method override,
	// so the user can see at a glance which budget uses a different approach from
	// the household default. Hidden when the budget inherits the global method.
	var methodLine ui.Node = Fragment()
	if s.Budget.Methodology != "" {
		methodLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.methodOverrideRow", budgetMethodLabel(budgeting.ParseMethodology(s.Budget.Methodology))))
	}

	// Custom-field summary (e.g. "Priority: High · Review: Q3") — shown when the budget
	// has any user-defined field values, so custom data stays visible on the row.
	var customLine ui.Node = Fragment()
	if cs := customSummary(props.BudgetDefs, s.Budget.Custom); cs != "" {
		customLine = Span(css.Class("budget-sub", tw.TextFaint), cs)
	}

	// Envelope methodology: show the carried-forward balance under the period row.
	// Like every this-month caption below, it hides in last-month mode: the bar then
	// shows a DIFFERENT period's spend, and a this-month pace/carry/cap line under a
	// last-month bar reads as an internal contradiction (the L-overlay critique).
	var envLine ui.Node = Fragment()
	if !lastMonthMode && props.Envelope != "" {
		cls := "budget-sub " + tw.Fold(tw.FontDisplay)
		if props.EnvelopeNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		envLine = Span(ClassStr(cls), uistate.T("budgets.envelopeRow", props.Envelope))
	}

	// BG5: an overdrawn envelope names what next period starts down, so the overdraft
	// doesn't quietly vanish at the boundary. Caution amber (not danger red): it's a
	// heads-up about where next period STARTS, not a "you're over now" alarm.
	var envDebtLine ui.Node = Fragment()
	if !lastMonthMode && props.EnvelopeDebtStart != "" {
		envDebtLine = Span(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "budget-envdebt-"+s.Budget.ID),
			Attr("role", "status"), props.EnvelopeDebtStart)
	}

	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if !lastMonthMode && props.PaceOver != "" {
		paceLine = Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.paceOver", props.PaceOver))
	}

	// BG3: the even-pace caption ("on pace" / "running $38 hot" / "$12 under pace so
	// far"). A hot budget reads in the caution amber; on/under pace stays faint. Only
	// shown while in progress and when the pace overspend warning isn't already owning
	// the message (paceOver is the stronger signal).
	var paceMarkLine ui.Node = Fragment()
	if !lastMonthMode && props.PaceCaption != "" && props.PaceOver == "" {
		tone := tw.TextFaint
		if props.PaceHot {
			tone = tw.TextWarn
		}
		paceMarkLine = Span(css.Class("budget-sub", tone), Attr("data-testid", "budget-pace-caption-"+s.Budget.ID),
			Attr("role", "status"), props.PaceCaption)
	}

	var rolloverLine ui.Node = Fragment()
	if !lastMonthMode && props.RolloverCarry != "" {
		// A carried-in deficit is historical context about where the period STARTED, not a
		// live alert — keep it quiet/neutral (TextDim) so an over-budget card has ONE colored
		// signal (the red status line) rather than several caption lines competing for the eye.
		cls := "budget-sub " + tw.Fold(tw.FontDisplay, tw.TextDim)
		rolloverLine = Span(ClassStr(cls), uistate.T("budgets.rolloverCarry", props.RolloverCarry))
	}

	// C136: show the effective cap when it differs from the base limit — WITH its
	// arithmetic (base limit ± carry-over ± top-up), so the number is explainable at a
	// glance instead of an opaque figure. Hidden when cap == base limit (no note needed).
	var effectiveCapLine ui.Node = Fragment()
	if !lastMonthMode && props.EffectiveCap != "" {
		if props.EffectiveCapMath != "" {
			effectiveCapLine = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-capmath-"+s.Budget.ID),
				uistate.T("budgets.effectiveCapMath", props.EffectiveCap, props.EffectiveCapMath))
		} else {
			effectiveCapLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.effectiveCap", props.EffectiveCap))
		}
	}

	// C143: even-pace guidance — how much of what's left can be spent over the days
	// still in the period, so the user knows the sustainable daily-ish pace instead
	// of seeing only a lump remaining. Quiet faint line; only set while in-progress.
	var proratedLine ui.Node = Fragment()
	if !lastMonthMode && props.ProratedRest != "" {
		proratedLine = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-prorated"),
			uistate.T("budgets.proratedRest", props.ProratedRest))
	}

	// "Last month's spend" overlay: an overline tag above the (now last-month) bar, plus a
	// faint reference line showing what THIS month has actually spent — so the tile leads
	// with last month while this month stays visible for comparison.
	var lastMonthTag, thisMonthRef ui.Node = Fragment(), Fragment()
	if lastMonthMode {
		lastMonthTag = Div(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budget-lastmonth-"+s.Budget.ID),
			Attr("aria-label", uistate.T("budgets.lastMonthAria", props.LastMonthSpent, props.LastMonthDelta)),
			uistate.T("budgets.lastMonthCap"))
		thisMonthRef = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-thismonth-ref-"+s.Budget.ID),
			uistate.T("budgets.thisMonthRef", fmtMoney(s.Spent)))
	}

	// "Cover…" is offered on an over-budget row and opens the flip modal (which lists
	// the other budgets to pull from). Top up is offered when not over.
	isOver := s.State == budgeting.StateOver
	var coverBtn ui.Node = Fragment()
	if isOver {
		coverBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-cover-btn-"+s.Budget.ID), Title(uistate.T("budgets.coverTitle")), OnClick(openCover), uistate.T("budgets.coverBtn"))
	}
	// Top up is a visible card button (the frequent proactive action) on budgets that
	// aren't over.
	var topupBtn ui.Node = Fragment()
	if !isOver {
		topupBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-topup-btn-"+s.Budget.ID), Title(uistate.T("budgets.topupTitle")), OnClick(openTopup), uistate.T("budgets.topupBtn"))
	}

	// The row actions, rendered as the card's footer (pinned to the bottom by CSS) so the
	// card reads top-to-bottom: title → amount → bar → status → actions. The footer keeps
	// only the EVERYDAY money moves inline — Cover/Top-up and the Transactions drill —
	// and everything configurational (Edit budget, Edit tracking, Notes, Formulas) folds
	// into the ⋯ overflow with the destructive actions at its bottom (standing directive:
	// delete never sits exposed on the row). Seven visible buttons made every card a
	// toolbar; three keep it a card (design critique #8).
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	// The ⋯ overflow is shared by both densities. In COMPACT the row has no footer, so
	// the money moves (Cover/Top-up, Transactions) join the top of the menu; the
	// destructive group stays pinned at the bottom either way.
	kebabNode := Div(css.Class("add-wrap"), Attr("id", menuID),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-kebab-"+s.Budget.ID), Attr("title", uistate.T("budgets.moreActions")), Attr("aria-label", uistate.T("budgets.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
		Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
		Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
			If(props.Compact && isOver, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-cover-btn-"+s.Budget.ID), Title(uistate.T("budgets.coverTitle")), OnClick(openCover), uistate.T("budgets.coverBtn"))),
			If(props.Compact && !isOver, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-topup-btn-"+s.Budget.ID), Title(uistate.T("budgets.topupTitle")), OnClick(openTopup), uistate.T("budgets.topupBtn"))),
			If(props.Compact && canDrill, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-view-txns-"+s.Budget.ID), Title(uistate.T("budgets.reviewTitle")), OnClick(drillMenu), uistate.T("nav.transactions"))),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-budget-btn-"+s.Budget.ID), Title(uistate.T("budgets.editTitle")), OnClick(openEdit), uistate.T("budgets.editAction")),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-budget-cats-btn-"+s.Budget.ID), Title(uistate.T("budgets.catsTitle")), OnClick(openCategories), uistate.T("budgets.catsAction")),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-notes-btn-"+s.Budget.ID), Title(uistate.T("budgets.notesTitle")), OnClick(openNotes), uistate.T("budgets.notesAction")),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-formulas-btn-"+s.Budget.ID), Title(uistate.T("budgets.formulasTitle")), OnClick(openFormulas), uistate.T("budgets.formulasAction")),
			If(hasRecurring, Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "remove-recurring-btn-"+s.Budget.ID), OnClick(removeRecurring), uistate.T("budgets.removeRecurring"))),
			Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-budget-btn-"+s.Budget.ID), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uistate.T("budgets.deleteAction")),
		),
	)
	actionsRow := Div(css.Class("budget-actions"),
		coverBtn,
		topupBtn,
		If(canDrill, Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-view-txns-"+s.Budget.ID), Title(uistate.T("budgets.reviewTitle")), OnClick(drillMenu),
			uiw.Icon(icon.Receipt, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions")))),
		kebabNode,
	)

	// COMPACT density: one scannable line — name, mini bar, spent/limit, what's left,
	// a status chip, and the ⋯ menu (which absorbs the money moves above). Everything
	// analytical (pie, metrics, captions, notes, to-dos) belongs to the card layout;
	// compact is for reading fifteen budgets without scrolling (design critique #9).
	if props.Compact {
		crowCls := "budget-crow " + budgetRowStateClass(s, props.PaceOver)
		var crowTitle ui.Node
		if canDrill {
			crowTitle = Button(css.Class("budget-crow-name budget-drill"), Type("button"), Title(uistate.T("budgets.drillTitle", props.Category)), OnClick(drill), title)
		} else {
			crowTitle = Span(css.Class("budget-crow-name"), title)
		}
		var crowLeft, crowChip ui.Node
		if lastMonthMode {
			crowLeft = Span(ClassStr("budget-crow-left"+lastMonthSubTone(props.LastMonthOver)), props.LastMonthDelta)
			crowChip = Span(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budget-lastmonth-"+s.Budget.ID), uistate.T("budgets.lastMonthCap"))
		} else {
			leftCls := "budget-crow-left"
			if s.Remaining.IsNegative() {
				leftCls += " " + tw.Fold(tw.TextDown)
			}
			crowLeft = Span(ClassStr(leftCls), budgetRemainPhrase(s.Remaining))
			chipTone := ""
			switch {
			case s.State == budgeting.StateOver:
				chipTone = " is-danger"
			case s.State == budgeting.StateNear || props.PaceOver != "":
				chipTone = " is-warn"
			}
			crowChip = Span(ClassStr("pill budget-crow-chip"+chipTone), label)
		}
		return Div(ClassStr(crowCls), Attr("data-testid", "budget-card-"+s.Budget.ID),
			crowTitle,
			Div(css.Class("budget-crow-bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")),
				Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
			Span(css.Class("budget-crow-amt fig"), barSpent+" / "+fmtMoney(limit)),
			crowLeft,
			crowChip,
			kebabNode,
		)
	}

	// Readable, clickable-to-expand notes line (the attached note itself), shown on the
	// card when the budget has a note — mirrors the /accounts notes affordance.
	var notesNode ui.Node = Fragment()
	hasNotes := strings.TrimSpace(s.Budget.Notes) != ""
	if notes := strings.TrimSpace(s.Budget.Notes); notes != "" {
		// Clicking the note opens the full note in the flip modal (handy when it's long) —
		// the card shows a clamped preview.
		notesNode = Button(ClassStr("acct-notes budget-notes"), Type("button"), Attr("data-testid", "budget-notes-"+s.Budget.ID),
			Attr("aria-label", uistate.T("budgets.notesAction")),
			Title(uistate.T("budgets.notesTitle")), OnClick(openNotes),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), notes),
		)
	}

	// A quiet link to the To-dos page when this budget has linked to-dos.
	// A quick, scannable metric strip — the full-width card has room for the TIME
	// dimension the prose lines don't make glanceable: how much is left to spend per
	// remaining day, how many days remain (and when it resets), and how far through the
	// period we are. Hidden in the last-month overlay (which shows a different period).
	var metricsStrip ui.Node = Fragment()
	if !lastMonthMode {
		// QA CF-05: the period must come from the VIEW's anchor, not today — a
		// June card paged into July was showing "14 days left · 52% elapsed ·
		// Aug 1" (all current-month pacing). Completed periods state final facts;
		// future periods say when they start; only a period containing today gets
		// the live per-day / days-left / elapsed race.
		nowT := time.Now()
		anchorT := props.Anchor
		if anchorT.IsZero() {
			anchorT = nowT
		}
		pStart, pEnd := budgeting.PeriodRange(s.Budget.Period, anchorT, prefs.WeekStartWeekday())
		switch {
		case !nowT.Before(pEnd): // completed period — history has no pacing
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.ended"), prefs.FormatDate(pEnd.AddDate(0, 0, -1)), ""),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), "100%", ""),
			)
		case nowT.Before(pStart): // future period — nothing has elapsed yet
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.startsOn"), prefs.FormatDate(pStart), ""),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), "0%", ""),
			)
		default: // the viewed period contains today — live pacing
			daysLeft := int(pEnd.Sub(nowT).Hours() / 24)
			if daysLeft < 1 {
				daysLeft = 1
			}
			elapsed := 0
			if total := pEnd.Sub(pStart).Hours(); total > 0 {
				elapsed = int(nowT.Sub(pStart).Hours() / total * 100)
				if elapsed < 0 {
					elapsed = 0
				} else if elapsed > 100 {
					elapsed = 100
				}
			}
			perDay := int64(0)
			if s.Remaining.Amount > 0 {
				perDay = s.Remaining.Amount / int64(daysLeft)
			}
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.perDay"), fmtMoney(money.New(perDay, s.Remaining.Currency)), ""),
				budgetMetric(uistate.T("budgetMetrics.daysLeft"), strconv.Itoa(daysLeft), prefs.FormatDate(pEnd)),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), fmt.Sprintf("%d%%", elapsed), ""),
			)
		}
	}

	// A composite budget (multi-category, cats+tags, or multi-tag) gets a spend-composition
	// donut UNDER the full-width status bar; a single-dimension budget shows none.
	pieNode, hasPie := budgetPie(s.Budget, props.Anchor)
	cardCls := "budget " + budgetRowStateClass(s, props.PaceOver)
	if hasPie {
		cardCls += " budget-has-pie"
	}

	// The right column holds the note (if any) and the budget's linked follow-up to-dos (if
	// any) — check-off in place, like the transaction follow-ups. Shown whenever either
	// exists; otherwise the column is omitted and the main content fills the width.
	linkedTodos := budgetLinkedTodos(s.Budget.ID)
	var todosPanel ui.Node = Fragment()
	if len(linkedTodos) > 0 {
		const maxTodos = 5
		open := 0
		for _, it := range linkedTodos {
			if !it.Done {
				open++
			}
		}
		kids := []any{css.Class("budget-todos"), Attr("data-testid", "budget-todos-" + s.Budget.ID),
			Span(css.Class("budget-todos-head"), uistate.T("budgets.followUpsHead", open, len(linkedTodos)))}
		for i, it := range linkedTodos {
			if i >= maxTodos {
				break
			}
			kids = append(kids, ui.CreateElement(txnFollowUpItem, txnFollowUpItemProps{ID: it.ID, Title: it.Title, Done: it.Done, Due: it.Due, OnOpen: openBudgetTodos}))
		}
		if extra := len(linkedTodos) - maxTodos; extra > 0 {
			kids = append(kids, Button(css.Class("txnfu-pop-foot"), Type("button"), Attr("data-testid", "budget-todos-more-"+s.Budget.ID),
				OnClick(openTodos), uistate.T("budgets.followUpsMore", extra)))
		}
		todosPanel = Div(kids...)
	}
	hasSide := hasNotes || len(linkedTodos) > 0

	return Div(css.Class(cardCls), Attr("data-testid", "budget-card-"+s.Budget.ID),
		Div(css.Class("budget-head"),
			// The title gets the whole header line now (the spent/limit amount and the
			// percent moved INTO the bar below), so a long budget name has room to breathe.
			Div(css.Class("budget-head-main"),
				IfElse(canDrill,
					Button(css.Class("row-desc budget-drill"), Type("button"), Title(uistate.T("budgets.drillTitle", props.Category)), OnClick(drill),
						Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "text-align": "left", "cursor": "pointer"}),
						title),
					Span(css.Class("row-desc"), title)),
			),
		),
		// Overline tag naming the bar as last month's, when the overlay is on.
		lastMonthTag,
		// The card's "loader": a taller progress bar with the spent/limit amount (left) and
		// the percent-used (right) rendered inside it, over the fill. In last-month mode the
		// figures are last period's spend against this month's budget.
		Div(css.Class("budget-card-loader"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")),
			Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width))),
			// XC4: the committed band sits just past the spent fill (hidden in last-month
			// mode, where the bar shows a different period's spend).
			If(!lastMonthMode && props.HasCommitted && props.Committed.CommittedPct > 0,
				Div(css.Class("bar-committed"), Attr("data-testid", "budget-committed-seg-"+s.Budget.ID),
					Attr("style", fmt.Sprintf("left:%d%%;width:%d%%", props.Committed.SpentPct, props.Committed.CommittedPct)))),
			// BG3: the even-pace tick — a thin vertical marker at where discretionary
			// spending SHOULD be right now (committed money excluded). Hidden in
			// last-month mode (the bar then shows a different period).
			If(!lastMonthMode && props.PaceMarkerPct > 0,
				Div(css.Class("bar-pace-tick"), Attr("data-testid", "budget-pace-tick-"+s.Budget.ID),
					Attr("aria-hidden", "true"),
					Attr("style", fmt.Sprintf("position:absolute;top:0;bottom:0;left:%d%%;width:2px;background:var(--text);opacity:0.55;pointer-events:none", props.PaceMarkerPct)))),
			Div(css.Class("budget-card-loader-figs"),
				// Spent carries foreground weight; the "/ limit" reads as muted context.
				// The limit figure is a direct affordance: click it to edit in place (G1) —
				// the most frequent budget change never costs a modal round-trip.
				IfElse(limitEditing.Get(),
					Form(css.Class("budget-amount", "budget-limit-editform"), OnSubmit(saveLimitEdit),
						Span(css.Class("budget-spent"), barSpent), Span(" / "),
						// When carry/boost make the cap differ from the base limit, the button
						// face shows the CAP but this input edits the BASE — label it so the
						// number swap on open doesn't read as a glitch (the capmath line below
						// carries the arithmetic).
						If(props.EffectiveCap != "", Span(css.Class("budget-limit-basetag"), Attr("data-testid", "budget-limit-basetag-"+s.Budget.ID), uistate.T("budgets.baseLimitTag"))),
						Input(css.Class("field", "budget-limit-input"), Attr("autofocus", ""), Type("number"),
							Attr("data-testid", "budget-limit-input-"+s.Budget.ID), Attr("aria-label", uistate.T("budgets.limitLabel")),
							Value(limitDraft.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onLimitDraft)),
						Button(css.Class("btn btn-sm", "budget-limit-save"), Type("submit"), Attr("data-testid", "budget-limit-save-"+s.Budget.ID),
							Attr("aria-label", uistate.T("action.save")), Title(uistate.T("action.save")), uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))),
						Button(css.Class("btn btn-sm", "budget-limit-cancel"), Type("button"), Attr("data-testid", "budget-limit-cancel-"+s.Budget.ID),
							Attr("aria-label", uistate.T("action.cancel")), Title(uistate.T("action.cancel")), OnClick(cancelLimitEdit), uiw.Icon(icon.Close, css.Class(tw.W35, tw.H35))),
					),
					Span(css.Class("budget-amount"),
						Span(css.Class("budget-spent"), barSpent), Span(" / "),
						Button(css.Class("budget-limit-btn"), Type("button"), Attr("data-testid", "budget-limit-btn-"+s.Budget.ID),
							Title(uistate.T("budgets.limitEditTitle")), Attr("aria-label", uistate.T("budgets.limitEditTitle")),
							OnClick(startLimitEdit), fmtMoney(limit)),
					)),
				// Percent-used, capped for display (e.g. "112%" when over).
				Span(css.Class("budget-pct"), strconv.Itoa(barPct)+"%"),
			),
		),
		// Everything below the bar sits in a flex row so a budget WITH a note can put the
		// note in a right-hand column (the lower-left content is short, leaving that space
		// empty). No note → the note column is omitted and the main content fills the width.
		Div(css.Class("budget-lower"),
			Div(css.Class("budget-lower-main"),
				// Spend-composition donut for a composite budget, under the full-width bar.
				pieNode,
				// One quiet metadata line beneath the bar. In last-month mode it reads the over/
				// under-budget gap + period; otherwise the health status · money left · period.
				IfElse(lastMonthMode,
					Span(css.Class("budget-sub"+lastMonthSubTone(props.LastMonthOver)), props.LastMonthDelta+" · "+periodLabel(s.Budget.Period)),
					Span(css.Class("budget-sub"), uistate.T("budgets.rowPrimary", label, budgetRemainPhrase(s.Remaining))+" · "+periodLabel(s.Budget.Period))),
				// Multi-category budgets: list the tracked categories so the combined total reads clearly.
				If(props.TrackedCats != "", Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-tracked-cats-"+s.Budget.ID),
					uistate.T("budgets.catsTracking", props.TrackedCats))),
				// Cross-category tag tracking: the tags this budget also counts, whatever the category.
				If(len(s.Budget.TrackedTags) > 0, budgetTagLine(s.Budget.ID, s.Budget.TrackedTags)),
				// XC4: quiet committed-vs-free caption; XC3: the plain-English set-aside explainer.
				// (A landing-month entry may carry only the explainer — no committed split.)
				If(!lastMonthMode && props.HasCommitted && props.Committed.CommittedStr != "",
					Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-committed-caption-"+s.Budget.ID),
						uistate.T("budgets.committedCaption", props.Committed.CommittedStr, props.Committed.FreeStr))),
				If(!lastMonthMode && props.HasCommitted && props.Committed.SetAsideNote != "",
					Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-setaside-note-"+s.Budget.ID),
						props.Committed.SetAsideNote)),
				// BG1: funding-target summary ("Refill to $200 · $60 to go").
				budgetTargetLine(s),
				thisMonthRef,
				coverageLine,
				ownerLine,
				methodLine,
				customLine,
				paceLine,
				paceMarkLine,
				proratedLine,
				rolloverLine,
				effectiveCapLine,
				envLine,
				envDebtLine,
				// "What's driving this?" — the analytical link (top charges → their ledger /
				// subscriptions), offered only when a budget is near or over, where knowing
				// WHY is what you actually want. Collapsed and lazy, so it never crowds a card.
				If(!lastMonthMode && (s.State == budgeting.StateOver || s.State == budgeting.StateNear),
					ui.CreateElement(budgetDriversPanel, budgetDriversPanelProps{Budget: s.Budget, Anchor: props.Anchor})),
				metricsStrip,
				actionsRow,
			),
			If(hasSide, Div(css.Class("budget-side-col"),
				If(hasNotes, notesNode),
				todosPanel,
			)),
		),
	)
}

// budgetTopDriversFor computes the up-to-n largest charges driving a budget's spend
// this period, reconciled with the shown Spent (same rollup covers as EvaluateRollup).
// Returns the drivers and the base currency. Reads appstate directly (like the
// composition pie), so the card component stays self-contained.
func budgetTopDriversFor(b domain.Budget, n int, anchor time.Time) ([]budgeting.Driver, string) {
	app := appstate.Default
	if app == nil {
		return nil, "USD"
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	// The drivers describe the VIEWED period (QA CF-05), not necessarily today's.
	start, end := budgeting.PeriodRange(b.Period, anchor, uistate.LoadPrefs().WeekStartWeekday())
	descendants := categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs())
	covers := func(id string) bool { return descendants[id] }
	// Group charges by the clean merchant name (payee aliases + rule-pack normalization)
	// so a store under several raw descriptors, or many small charges, reads as one line.
	normalize := app.PayeeResolver().Resolve
	drivers, err := budgeting.TopDrivers(b, app.Transactions(), start, end, rates, covers, n, normalize)
	if err != nil {
		return nil, base
	}
	return drivers, base
}

// recurringLabelSet returns the household's recurring-charge labels, lower-cased —
// so a budget driver can be recognized as a recurring bill / subscription and offer
// the tighter "manage it" path instead of just showing it as a one-off charge.
func recurringLabelSet() map[string]bool {
	app := appstate.Default
	if app == nil {
		return nil
	}
	out := map[string]bool{}
	for _, r := range app.Recurring() {
		if l := strings.ToLower(strings.TrimSpace(r.Label)); l != "" && r.Amount.IsNegative() {
			out[l] = true
		}
	}
	return out
}

// budgetDriversPanelProps configures the "what's driving this" disclosure.
type budgetDriversPanelProps struct {
	Budget domain.Budget
	Anchor time.Time // the view's period anchor (QA CF-05); zero falls back to now
}

// budgetDriversPanel is the budget card's "what's driving this?" disclosure — the
// analytical link that answers WHY a budget is over, not just that it is. Collapsed
// by default (the card is already dense, and this is opt-in detail); when opened it
// lists the largest charges driving the overspend, each drilling to the ledger for
// that merchant — and a charge that's a recurring bill/subscription links straight to
// where it can be reviewed or cancelled. Its own component so the disclosure hook and
// the drivers computation stay lazy and at stable positions.
func budgetDriversPanel(props budgetDriversPanelProps) ui.Node {
	expanded := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	open := expanded.Get()

	discLabel := uistate.T("budgets.driversShow")
	if open {
		discLabel = uistate.T("budgets.driversHide")
	}
	listID := "budget-drivers-list-" + props.Budget.ID
	head := Button(css.Class("budget-drivers-toggle"), Type("button"),
		Attr("data-testid", "budget-drivers-toggle-"+props.Budget.ID),
		Attr("aria-expanded", ariaBool(open)), Attr("aria-controls", listID), OnClick(toggle),
		Span(discLabel),
		uiw.Icon(icon.ChevronDown, css.Class("budget-drivers-chev", tw.W35, tw.H35)))

	var body ui.Node = Fragment()
	if open {
		drivers, base := budgetTopDriversFor(props.Budget, 3, props.Anchor)
		if len(drivers) == 0 {
			body = P(css.Class("budget-drivers-empty"), Attr("id", listID), Attr("data-testid", "budget-drivers-empty-"+props.Budget.ID),
				uistate.T("budgets.driversNone"))
		} else {
			recSet := recurringLabelSet()
			rows := make([]ui.Node, 0, len(drivers))
			for _, d := range drivers {
				label := d.Label
				isRec := recSet[strings.ToLower(strings.TrimSpace(label))]
				drill := func(payee string, recurring bool) func() {
					return func() {
						if recurring {
							nav.Navigate(uistate.RoutePath("/subscriptions"))
							return
						}
						f := txFilter.Get()
						f.Text = payee
						f = f.Normalize()
						txFilter.Set(f)
						uistate.PersistTxFilter(f)
						nav.Navigate(uistate.RoutePath("/transactions"))
					}
				}(label, isRec)
				rows = append(rows, ui.CreateElement(budgetDriverRow, budgetDriverRowProps{
					BudgetID: props.Budget.ID, Label: label,
					Amount: fmtMoney(money.New(d.Amount, base)), Recurring: isRec, OnDrill: drill,
				}))
			}
			body = Div(css.Class("budget-drivers-list"), Attr("id", listID), rows)
		}
	}
	return Div(css.Class("budget-drivers"), head, body)
}

// budgetDriverRowProps is one driver line in the "what's driving this" list.
type budgetDriverRowProps struct {
	BudgetID  string
	Label     string
	Amount    string
	Recurring bool
	OnDrill   func()
}

// budgetDriverRow renders one driving charge: the merchant, its amount, and (when it's
// a recurring bill/subscription) a quiet "recurring" cue. The whole row is the drill —
// to the merchant's charges, or to Subscriptions for a recurring one. Own component so
// its click hook never registers inside the parent's driver loop (framework rule).
func budgetDriverRow(props budgetDriverRowProps) ui.Node {
	onDrill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill()
		}
	}))
	aria := uistate.T("budgets.driverDrillAria", props.Label)
	if props.Recurring {
		aria = uistate.T("budgets.driverDrillRecurringAria", props.Label)
	}
	return Button(css.Class("budget-driver-row"), Type("button"),
		Attr("data-testid", "budget-driver-"+props.BudgetID),
		Attr("aria-label", aria), Title(aria), OnClick(onDrill),
		Span(css.Class("budget-driver-name"), props.Label),
		If(props.Recurring, Span(css.Class("budget-driver-recurring"), uistate.T("budgets.driverRecurring"))),
		Span(css.Class("budget-driver-amt"), props.Amount),
	)
}

// budgetMetric renders one cell of the budget card's quick-metric strip: a small
// uppercase label over a tabular value, with an optional muted sub (e.g. the reset date).
func budgetMetric(label, value, sub string) ui.Node {
	var subNode ui.Node = Fragment()
	if sub != "" {
		subNode = Span(css.Class("budget-metric-sub"), sub)
	}
	return Div(css.Class("budget-metric"),
		Span(css.Class("budget-metric-label"), label),
		Span(css.Class("budget-metric-value"), value),
		subNode,
	)
}

// lastMonthSubTone returns the danger-tone class suffix for the last-month sub-line when
// last month's spend exceeded this month's budget (else no extra class).
func lastMonthSubTone(over bool) string {
	if over {
		return " " + tw.Fold(tw.TextDown)
	}
	return ""
}

// budgetRowStateClass maps a budget's health to the row's visual-state modifier class
// (the /budgets styles use it for the left accent stripe, the over-tint, and the
// percent-chip color): over → is-over, near-limit → is-near, pace-trending-over →
// is-risk, otherwise is-ontrack.
func budgetRowStateClass(s budgeting.Status, paceOver string) string {
	switch s.State {
	case budgeting.StateOver:
		return "is-over"
	case budgeting.StateNear:
		return "is-near"
	}
	if paceOver != "" {
		return "is-risk"
	}
	return "is-ontrack"
}

// budgetMethodLabel returns a short, localized label for a methodology value —
// reused by the per-budget method badge and the method select options.
func budgetMethodLabel(m budgeting.Methodology) string {
	switch m {
	case budgeting.MethodZeroBased:
		return uistate.T("settings.budgetMethodZero")
	case budgeting.MethodEnvelope:
		return uistate.T("settings.budgetMethodEnvelope")
	default:
		return uistate.T("settings.budgetMethodSimple")
	}
}

// budgetMethodOptions builds the SelectOptions for the per-budget method
// override picker. The first option ("Use global default") stores an empty
// value so that saving it clears the override, restoring global-method
// inheritance. The remaining options mirror the global method picker labels.
func budgetMethodOptions(selected string) []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("budgets.methodDefault")},
		{Value: string(budgeting.MethodSimple), Label: uistate.T("settings.budgetMethodSimple")},
		{Value: string(budgeting.MethodZeroBased), Label: uistate.T("settings.budgetMethodZero")},
		{Value: string(budgeting.MethodEnvelope), Label: uistate.T("settings.budgetMethodEnvelope")},
	}
}

// budgetTagLine renders the "Tracking #tag …" caption for a tag-tracking budget — the
// cross-category dimension of what it counts. Read-only chips (no handlers), so building
// them in a plain loop here is fine.
func budgetTagLine(budgetID string, tags []string) ui.Node {
	kids := []any{
		css.Class("budget-sub", "budget-tag-line"),
		Attr("data-testid", "budget-tracked-tags-"+budgetID),
		Span(css.Class("budget-tag-line-label"), uistate.T("budgets.tagsTracking")),
	}
	for _, tg := range tags {
		kids = append(kids, Span(css.Class("budget-tag-chip"), "#"+tg))
	}
	return Span(kids...)
}

// budgetSlice is one wedge of a composite budget's spend-composition pie.
type budgetSlice struct {
	Label  string
	Amount int64 // minor units, base currency
	ValStr string
}

// budgetOwnsScope mirrors the budgeting engine's scope rule for the pie: a shared budget
// counts everyone; an individual budget counts only its owner's spend.
func budgetOwnsScope(b domain.Budget, member string) bool {
	if b.Scope != domain.ScopeIndividual {
		return true
	}
	return member == b.OwnerID
}

// budgetCompositionSlices breaks a budget's CURRENT-period spend into wedges by tracked
// dimension — one per tracked category and one per tracked tag. The cats+tags dedupe is the
// careful bit: a charge that matches a tracked tag is attributed WHOLE to that tag (the
// first of the budget's tags it carries, in the budget's tag order) and never also to a
// category — exactly mirroring the engine's tag-priority spend, so the wedges sum to the
// budget's Spent with no double-counting. Wedges are sorted largest-first; zero wedges drop.
func budgetCompositionSlices(b domain.Budget, anchor time.Time) []budgetSlice {
	app := appstate.Default
	if app == nil {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	// Composition follows the VIEWED period (QA CF-05), matching the card's bar.
	start, end := budgeting.PeriodRange(b.Period, anchor, uistate.LoadPrefs().WeekStartWeekday())
	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	catSet := map[string]bool{}
	for _, id := range b.TrackedCategoryIDs() {
		catSet[id] = true
	}
	tagSet := b.TrackedTagSet()

	amt := map[string]int64{}
	label := map[string]string{}
	var order []string
	add := func(key, lbl string, m money.Money) {
		conv, err := rates.Convert(m.Abs(), base)
		if err != nil {
			return
		}
		if _, ok := amt[key]; !ok {
			order = append(order, key)
			label[key] = lbl
		}
		amt[key] += conv.Amount
	}
	for _, t := range app.Transactions() {
		if !t.CountsInReports() || t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		if !budgetOwnsScope(b, t.MemberID) {
			continue
		}
		// Tag priority (dedupe): the first tracked tag this charge carries wins the whole
		// charge; it's never also counted under a category.
		matched := ""
		for _, tg := range b.TrackedTags {
			k := strings.ToLower(strings.TrimSpace(tg))
			if k == "" || !tagSet[k] {
				continue
			}
			has := false
			for _, tt := range t.Tags {
				if strings.ToLower(strings.TrimSpace(tt)) == k {
					has = true
					break
				}
			}
			if has {
				matched = k
				break
			}
		}
		if matched != "" {
			add("tag:"+matched, "#"+matched, t.Amount)
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if catSet[s.CategoryID] {
					add("cat:"+s.CategoryID, catName[s.CategoryID], s.Amount)
				}
			}
		} else if catSet[t.CategoryID] {
			add("cat:"+t.CategoryID, catName[t.CategoryID], t.Amount)
		}
	}
	var slices []budgetSlice
	for _, k := range order {
		if amt[k] > 0 {
			slices = append(slices, budgetSlice{Label: label[k], Amount: amt[k], ValStr: fmtMoney(money.New(amt[k], base))})
		}
	}
	sort.SliceStable(slices, func(i, j int) bool { return slices[i].Amount > slices[j].Amount })
	return slices
}

// budgetPiePalette colours the composition wedges (theme-agnostic, distinct hues).
var budgetPiePalette = []string{"#4f9d69", "#e0a458", "#5b8def", "#c05e5e", "#8b6fb0", "#3fa7a0", "#b0894f", "#7f8c99"}

// budgetPie renders the right-side spend-composition donut for a COMPOSITE budget (2+
// tracked dimensions — multi-category, cats+tags, or multi-tag). Returns (node, shown):
// shown is false for a single-dimension budget or when there's no spend yet, so the caller
// omits the pie column entirely.
func budgetPie(b domain.Budget, anchor time.Time) (ui.Node, bool) {
	if len(b.TrackedCategoryIDs())+len(b.TrackedTags) < 2 {
		return Fragment(), false // not composite — the bar already tells the whole story
	}
	slices := budgetCompositionSlices(b, anchor)
	var total int64
	for _, s := range slices {
		total += s.Amount
	}
	if total <= 0 || len(slices) == 0 {
		return Fragment(), false // nothing spent yet — an empty pie helps no one
	}
	var stops []string
	legend := []any{css.Class("budget-pie-legend")}
	var acc float64
	for i, s := range slices {
		color := budgetPiePalette[i%len(budgetPiePalette)]
		pct := float64(s.Amount) / float64(total) * 100
		stops = append(stops, fmt.Sprintf("%s %.3f%% %.3f%%", color, acc, acc+pct))
		acc += pct
		legend = append(legend, Div(css.Class("budget-pie-legrow"),
			Span(css.Class("budget-pie-dot"), Attr("style", "background:"+color)),
			Span(css.Class("budget-pie-leglabel"), s.Label),
			Span(css.Class("budget-pie-legval"), s.ValStr)))
	}
	donut := Div(css.Class("budget-pie-donut"), Attr("aria-hidden", "true"),
		Attr("style", "background:conic-gradient("+strings.Join(stops, ", ")+")"),
		Div(css.Class("budget-pie-hole")))
	return Div(css.Class("budget-pie"), Attr("data-testid", "budget-pie-"+b.ID),
		donut, Div(legend...)), true
}

// budgetLinkedTodos returns the to-dos linked to a budget (Task.RelatedType=budget),
// ordered open-first then soonest-due — the list shown in the budget card's side panel.
func budgetLinkedTodos(budgetID string) []followUpItem {
	app := appstate.Default
	if app == nil {
		return nil
	}
	fmtDate := uistate.LoadPrefs().FormatDate
	var out []followUpItem
	for _, t := range app.Tasks() {
		if t.RelatedType != domain.RelatedBudget || t.RelatedID != budgetID {
			continue
		}
		due := ""
		if !t.Due.IsZero() && fmtDate != nil {
			due = fmtDate(t.Due)
		}
		out = append(out, followUpItem{ID: t.ID, Title: t.Title, Done: t.Status == domain.StatusDone, Due: due, dueT: t.Due})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Done != b.Done {
			return !a.Done // open first
		}
		if a.dueT.IsZero() != b.dueT.IsZero() {
			return !a.dueT.IsZero() // dated before undated
		}
		return a.dueT.Before(b.dueT)
	})
	return out
}
