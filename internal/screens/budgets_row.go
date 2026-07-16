// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
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
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeEdit})
	}))
	openCategories := ui.UseEvent(Prevent(func() {
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
	openNotes := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeNotes})
	}))
	openFormulas := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeFormulas})
	}))
	// Transactions drill navigates to the filtered ledger.
	drillMenu := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Budget.TrackedCategoryIDs())
		}
	}))
	// Notes render as a readable, clickable-to-expand line on the card (like /accounts).
	notesExpanded := ui.UseState(false)
	toggleNotes := ui.UseEvent(Prevent(func() { notesExpanded.Set(!notesExpanded.Get()) }))
	// Jump to the To-dos page when this budget has linked to-dos (Task.RelatedType=budget).
	openTodos := ui.UseEvent(Prevent(func() {
		if props.OnViewTodos != nil {
			props.OnViewTodos()
		}
	}))
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
		switch {
		case props.LastMonthOver:
			fillClass = "bar-fill over"
		case props.LastMonthPct >= 85:
			fillClass = "bar-fill near"
		default:
			fillClass = "bar-fill"
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
	var envLine ui.Node = Fragment()
	if props.Envelope != "" {
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
	if props.EnvelopeDebtStart != "" {
		envDebtLine = Span(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "budget-envdebt-"+s.Budget.ID),
			Attr("role", "status"), props.EnvelopeDebtStart)
	}

	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if props.PaceOver != "" {
		paceLine = Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.paceOver", props.PaceOver))
	}

	// BG3: the even-pace caption ("on pace" / "running $38 hot" / "$12 under pace so
	// far"). A hot budget reads in the caution amber; on/under pace stays faint. Only
	// shown while in progress and when the pace overspend warning isn't already owning
	// the message (paceOver is the stronger signal).
	var paceMarkLine ui.Node = Fragment()
	if props.PaceCaption != "" && props.PaceOver == "" {
		tone := tw.TextFaint
		if props.PaceHot {
			tone = tw.TextWarn
		}
		paceMarkLine = Span(css.Class("budget-sub", tone), Attr("data-testid", "budget-pace-caption-"+s.Budget.ID),
			Attr("role", "status"), props.PaceCaption)
	}

	var rolloverLine ui.Node = Fragment()
	if props.RolloverCarry != "" {
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
	if props.EffectiveCap != "" {
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
	if props.ProratedRest != "" {
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
	// card reads top-to-bottom: title → amount → bar → status → actions. The everyday
	// actions are labelled inline tool buttons; the destructive ones (Remove recurring,
	// Delete) stay in the ⋯ overflow at the end — delete never sits exposed on the row.
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	actionsRow := Div(css.Class("budget-actions"),
		coverBtn,
		topupBtn,
		If(canDrill, Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-view-txns-"+s.Budget.ID), Title(uistate.T("budgets.reviewTitle")), OnClick(drillMenu),
			uiw.Icon(icon.Receipt, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions")))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "edit-budget-btn-"+s.Budget.ID), Title(uistate.T("budgets.editTitle")), OnClick(openEdit),
			uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.editAction"))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "edit-budget-cats-btn-"+s.Budget.ID), Title(uistate.T("budgets.catsTitle")), OnClick(openCategories),
			uiw.Icon(icon.Tag, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.catsAction"))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-notes-btn-"+s.Budget.ID), Title(uistate.T("budgets.notesTitle")), OnClick(openNotes),
			uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.notesAction"))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-formulas-btn-"+s.Budget.ID), Title(uistate.T("budgets.formulasTitle")), OnClick(openFormulas),
			uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.formulasAction"))),
		// Destructive actions live in the ⋯ overflow at the end (standing directive:
		// delete always stays in the kebab).
		Div(css.Class("add-wrap"), Attr("id", menuID),
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-kebab-"+s.Budget.ID), Attr("title", uistate.T("budgets.moreActions")), Attr("aria-label", uistate.T("budgets.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
			Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
			Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
				If(hasRecurring, Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "remove-recurring-btn-"+s.Budget.ID), OnClick(removeRecurring), uistate.T("budgets.removeRecurring"))),
				Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-budget-btn-"+s.Budget.ID), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uistate.T("budgets.deleteAction")),
			),
		),
	)

	// Readable, clickable-to-expand notes line (the attached note itself), shown on the
	// card when the budget has a note — mirrors the /accounts notes affordance.
	var notesNode ui.Node = Fragment()
	if notes := strings.TrimSpace(s.Budget.Notes); notes != "" {
		notesCls := "acct-notes budget-notes"
		toggleLabel := uistate.T("accounts.notesReadMore")
		if notesExpanded.Get() {
			notesCls += " open"
			toggleLabel = uistate.T("accounts.notesReadLess")
		}
		notesNode = Button(ClassStr(notesCls), Type("button"), Attr("data-testid", "budget-notes-"+s.Budget.ID),
			Attr("aria-expanded", ariaBool(notesExpanded.Get())), Attr("aria-label", uistate.T("budgets.notesAction")),
			Title(toggleLabel), OnClick(toggleNotes),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), notes),
		)
	}

	// A quiet link to the To-dos page when this budget has linked to-dos.
	var todosLine ui.Node = Fragment()
	if props.LinkedTodos > 0 {
		todosLine = Span(css.Class("budget-sub"),
			Button(css.Class("budget-drill"), Type("button"), Attr("data-testid", "budget-todos-link-"+s.Budget.ID), OnClick(openTodos),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				uistate.T("budgets.viewTodos", props.LinkedTodos)))
	}

	// A quick, scannable metric strip — the full-width card has room for the TIME
	// dimension the prose lines don't make glanceable: how much is left to spend per
	// remaining day, how many days remain (and when it resets), and how far through the
	// period we are. Hidden in the last-month overlay (which shows a different period).
	var metricsStrip ui.Node = Fragment()
	if !lastMonthMode {
		nowT := time.Now()
		pStart, pEnd := budgeting.PeriodRange(s.Budget.Period, nowT, prefs.WeekStartWeekday())
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

	return Div(css.Class("budget "+budgetRowStateClass(s, props.PaceOver)),
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
		metricsStrip,
		todosLine,
		notesNode,
		actionsRow,
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
