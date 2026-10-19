// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status
	dataRevision := uistate.UseDataRevision().Get()

	rowDisclosure := useBudgetRowDisclosure()

	rowLimitDraft := useBudgetRowLimitDraft(s)
	limitKeyDown, onLimitDraft := useBudgetRowLimitKeyboard(budgetRowLimitKeyboardProps{
		limitRestoreFocus: rowLimitDraft.limitRestoreFocus, limitEditing: rowLimitDraft.limitEditing, limitDraft: rowLimitDraft.limitDraft,
		limitDec: rowLimitDraft.limitDec, s: s,
	})
	saveLimitEdit := useBudgetRowLimitSave(budgetRowLimitSaveProps{
		limitDraft: rowLimitDraft.limitDraft, limitDec: rowLimitDraft.limitDec, s: s, limitEditing: rowLimitDraft.limitEditing,
		limitRestoreFocus: rowLimitDraft.limitRestoreFocus, limitButtonRef: rowLimitDraft.limitButtonRef,
	})

	rowNavigation := useBudgetRowNavigation(rowDisclosure.menuOpen, props, s)
	snoozeLabel, toggleSnooze, releaseUnused := useBudgetRowFundingActions(s, props, rowDisclosure.menuOpen)

	rowDetailActions := useBudgetRowDetailActions(s, rowDisclosure.menuOpen, props, rowNavigation.drillScope)
	hasRecurring, coverageLine := budgetRowCoverage(s, props)

	limit := s.Limit

	// When the "Last month's spend" overlay is on, the tile leads with LAST period's spend
	// (against this month's budget): the big bar shows it and this month drops to a small
	// reference line below — so last month "takes over the tile" instead of a tiny bar.
	lastMonthMode := props.LastMonthSpent != ""

	rowBarValues := budgetRowBarValues(s, props, lastMonthMode)
	rowTitle := budgetRowTitle(s, rowBarValues.title, rowNavigation.drillScope, props)
	customLine, envLine, envDebtLine := budgetRowMetadata(props, s, lastMonthMode)

	rowPaceNotes := budgetRowPaceNotes(lastMonthMode, props, s, rowDisclosure.prefs)

	rowPeriodNotes := budgetRowPeriodNotes(lastMonthMode, props, s, rowDetailActions.openCover)
	kebabNode, actionsRow := budgetRowActions(budgetRowActionsProps{
		isOver: rowPeriodNotes.isOver, s: s, openTopup: rowNavigation.openTopup, menuOpen: rowDisclosure.menuOpen,
		menuID: rowDisclosure.menuID, title: rowBarValues.title, toggleMenu: rowDisclosure.toggleMenu, closeMenu: rowDisclosure.closeMenu,
		props: props, openCover: rowDetailActions.openCover, openEdit: rowNavigation.openEdit, openCategories: rowNavigation.openCategories,
		notesLabel: rowTitle.notesLabel, openNotes: rowDetailActions.openNotes, openFormulas: rowDetailActions.openFormulas,
		releaseUnused: releaseUnused, toggleSnooze: toggleSnooze, snoozeLabel: snoozeLabel, hasRecurring: hasRecurring,
		removeRecurring: rowDetailActions.removeRecurring, del: rowNavigation.del, coverBtn: rowPeriodNotes.coverBtn,
		canDrill: rowNavigation.canDrill, drillTitle: rowTitle.drillTitle, drillMenu: rowDetailActions.drillMenu,
	})
	if s.Error != nil {
		return ui.CreateElement(budgetRowCalculationError, budgetRowCalculationErrorProps{
			Status: s, Title: rowBarValues.title, Actions: kebabNode, OpenEdit: rowNavigation.openEdit,
		})
	}

	fundedLine, fundedChip := budgetRowGoalFunding(lastMonthMode, s, props, dataRevision)

	if props.Compact {
		return budgetRowCompact(budgetRowCompactProps{
			budgetRowProps: props, Status: s, Title: rowBarValues.title, LastMonthMode: lastMonthMode,
			BarSpent: rowBarValues.barSpent, Width: rowBarValues.width, FillClass: rowBarValues.fillClass,
			Limit: limit, Label: rowBarValues.label, CanDrill: rowNavigation.canDrill, DrillTitle: rowTitle.drillTitle,
			Kebab: kebabNode, OnLimitEdit: rowLimitDraft.startLimitEdit, OnDrill: rowNavigation.drill,
			LimitEditing: rowLimitDraft.limitEditing.Get(), LimitDraft: rowLimitDraft.limitDraft.Get(), OnLimitDraft: onLimitDraft,
			OnLimitSave: saveLimitEdit, OnLimitCancel: rowLimitDraft.cancelLimitEdit, OnLimitKeyDown: limitKeyDown,
			LimitInputRef:  rowLimitDraft.limitInputRef,
			LimitButtonRef: rowLimitDraft.limitButtonRef,
			LimitErrorID:   rowLimitDraft.limitErrID, LimitError: rowLimitDraft.limitErrMsg, FundedChip: fundedChip,
			HasRecurring:   hasRecurring,
			RolloverDetail: rowPaceNotes.rolloverLine,
			ProjectionLine: budgetRowProjectionLine(s, props),
			HasNote:        rowTitle.hasNote, NotesLabel: rowTitle.notesLabel, NoteText: rowTitle.noteText, OnOpenNotes: rowDetailActions.openNotesFn,
		})
	}
	// Readable, clickable-to-expand notes line (the attached note itself), shown on the
	// card when the budget has a note — mirrors the /accounts notes affordance.
	notesNode, hasNotes := budgetRowNotes(budgetRowNotesProps{
		hasNote: rowTitle.hasNote, s: s, notesLabel: rowTitle.notesLabel, title: rowBarValues.title, openNotes: rowDetailActions.openNotes,
		noteText: rowTitle.noteText,
	})

	metricsStrip := budgetRowMetrics(lastMonthMode, props, s, rowDisclosure.prefs)

	// A composite budget (multi-category, cats+tags, or multi-tag) gets a spend-composition
	// donut UNDER the full-width status bar; a single-dimension budget shows none.
	pieNode, hasPie := budgetPie(s.Budget, props.Anchor, dataRevision)
	cardCls := "budget " + budgetRowStateClass(s, props.PaceOver)
	if hasPie {
		cardCls += " budget-has-pie"
	}

	// The side column holds the attached note and linked follow-ups.
	todosPanel, hasSide := budgetRowFollowUps(budgetRowFollowUpsProps{
		s: s, todosOpen: rowDetailActions.todosOpen, toggleTodos: rowDetailActions.toggleTodos,
		openBudgetTodos: rowDetailActions.openBudgetTodos, openTodos: rowDetailActions.openTodos, hasNotes: hasNotes,
	})

	return ui.CreateElement(budgetRowCard, budgetRowCardProps{
		cardCls: cardCls, s: s, title: rowBarValues.title, lastMonthTag: rowPeriodNotes.lastMonthTag, width: rowBarValues.width,
		fillClass: rowBarValues.fillClass, lastMonthMode: lastMonthMode, props: props, limitEditing: rowLimitDraft.limitEditing,
		saveLimitEdit: saveLimitEdit, barSpent: rowBarValues.barSpent, limit: limit, limitKeyDown: limitKeyDown, onLimitDraft: onLimitDraft,
		limitErrID: rowLimitDraft.limitErrID, limitErrMsg: rowLimitDraft.limitErrMsg, limitDraft: rowLimitDraft.limitDraft,
		limitSaveTitle: rowLimitDraft.limitSaveTitle, cancelLimitEdit: rowLimitDraft.cancelLimitEdit,
		startLimitEdit: rowLimitDraft.startLimitEdit, barPct: rowBarValues.barPct, paceLegend: rowPaceNotes.paceLegend,
		label: rowBarValues.label, effectiveCapLine: rowPaceNotes.effectiveCapLine, envLine: envLine, envDebtLine: envDebtLine,
		fundedLine: fundedLine, paceLine: rowPaceNotes.paceLine, detailsOpen: rowDisclosure.detailsOpen,
		toggleDetails: rowDisclosure.toggleDetails, pieNode: pieNode, thisMonthRef: rowPeriodNotes.thisMonthRef, coverageLine: coverageLine,
		ownerLine: rowTitle.ownerLine, methodLine: rowTitle.methodLine, customLine: customLine, paceMarkLine: rowPaceNotes.paceMarkLine,
		proratedLine: rowPeriodNotes.proratedLine, rolloverLine: rowPaceNotes.rolloverLine, metricsStrip: metricsStrip,
		actionsRow: actionsRow, hasSide: hasSide, hasNotes: hasNotes, notesNode: notesNode, todosPanel: todosPanel,
	})
}

type budgetRowCalculationErrorProps struct {
	Status   budgeting.Status
	Title    string
	Actions  ui.Node
	OpenEdit ui.Handler
}

func budgetRowCalculationError(props budgetRowCalculationErrorProps) ui.Node {
	openRates := ui.UseEvent(Prevent(func() { uistate.OpenGlobalSettingsAt("household") }))
	missingRate := budgetErrorHasUnknownRate(props.Status.Error)
	fixAction := props.OpenEdit
	fixLabel := uistate.T("bud2.fixBudget")
	if missingRate {
		fixAction = openRates
		fixLabel = uistate.T("bud2.fixRates")
	}
	return Div(css.Class("budget-crow is-risk"), Attr("data-testid", "budget-card-"+props.Status.Budget.ID),
		Div(css.Class("budget-crow-head"), H3(css.Class("budget-crow-name"), props.Title)),
		Span(css.Class("pill budget-crow-chip is-warn"), uistate.T("bud2.cantCalculate")),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budget-fix-"+props.Status.Budget.ID), OnClick(fixAction), fixLabel),
		Div(css.Class("budget-crow-actions"), props.Actions),
	)
}

func budgetErrorHasUnknownRate(err error) bool { return errors.Is(err, currency.ErrUnknownRate) }

type budgetRowCompactProps struct {
	budgetRowProps
	Status                                                   budgeting.Status
	Title                                                    string
	LastMonthMode                                            bool
	BarSpent                                                 string
	Width                                                    int
	FillClass                                                string
	Limit                                                    money.Money
	Label                                                    string
	CanDrill                                                 bool
	DrillTitle                                               string
	Kebab                                                    ui.Node
	OnLimitEdit, OnDrill                                     any
	LimitEditing                                             bool
	LimitDraft                                               string
	OnLimitDraft, OnLimitSave, OnLimitCancel, OnLimitKeyDown any
	LimitInputRef                                            ui.DOMRef
	LimitButtonRef                                           ui.DOMRef
	LimitErrorID, LimitError                                 string
	FundedChip                                               string
	HasRecurring                                             bool
	RolloverDetail                                           ui.Node
	ProjectionLine                                           ui.Node
	HasNote                                                  bool
	NotesLabel, NoteText                                     string
	OnOpenNotes                                              func()
}

// budgetRowCompact renders the dense row layout. All row interactions arrive as
// callbacks from BudgetRow so this component owns no hooks of its own.
func budgetRowCompact(props budgetRowCompactProps) ui.Node {
	s := props.Status
	title := props.Title
	lastMonthMode := props.LastMonthMode
	barSpent, width, fillClass := props.BarSpent, props.Width, props.FillClass
	limit, label := props.Limit, props.Label
	canDrill, drillTitle, kebabNode := props.CanDrill, props.DrillTitle, props.Kebab
	startLimitEdit, drill := props.OnLimitEdit, props.OnDrill
	onLimitDraft, saveLimitEdit, cancelLimitEdit, limitKeyDown := props.OnLimitDraft, props.OnLimitSave, props.OnLimitCancel, props.OnLimitKeyDown
	limitErrID, limitErrMsg := props.LimitErrorID, props.LimitError
	crowCls := "budget-crow " + budgetRowStateClass(s, props.PaceOver)
	// C591: the name is a HEADING, not a second copy of the Transactions
	// action. It used to be an unlabelled button that navigated to a filtered
	// ledger — the same thing the row's Transactions control does, with none of
	// the discoverability and nothing announcing it to a screen reader. One
	// action, labelled, sits in the row's action cell beside the ⋯ menu.
	// The compact row's name cell is narrow by design and ellipsizes ("Groce…",
	// "Transpor…"). Carrying the full name as a native title means the browser
	// recovers it on hover with no popover of our own — and the attribute is
	// harmless when the text already fits (Cam, 2026-08-31).
	crowTitle := Span(css.Class("budget-crow-name"), Attr("title", title), title)
	var crowLeft, crowChip ui.Node
	if lastMonthMode {
		crowLeft = Span(ClassStr("budget-crow-left"+lastMonthSubTone(props.LastMonthOver)), props.LastMonthDelta)
		crowChip = Span(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budget-lastmonth-"+s.Budget.ID), uistate.T("budgets.lastMonthCap"))
	} else {
		leftCls := "budget-crow-left"
		if s.Remaining.IsNegative() {
			leftCls += " " + tw.Fold(tw.TextDown)
		}
		crowLeft = Span(ClassStr(leftCls), budgetRowRemainPhrase(s))
		chipTone := ""
		switch {
		case s.State == budgeting.StateOver:
			chipTone = " is-danger"
		case s.State == budgeting.StateNear:
			chipTone = " is-warn"
		}
		statusText := Fragment()
		if label != uistate.T("budgets.onTrack") {
			statusText = Span(css.Class("budget-crow-status-text"), label)
		}
		crowChip = Fragment(
			If(chipTone != "", Span(ClassStr("pill budget-crow-chip"+chipTone), label)),
			If(chipTone == "", statusText),
		)
	}
	row := Div(ClassStr(crowCls), Attr("data-testid", "budget-card-"+s.Budget.ID),
		// Name, status and actions stay on the first line at compact widths. The
		// figures are grouped below them so the row can narrow without hiding the
		// remaining amount or forcing a marker into the kebab column.
		//
		// C545: the note's TEXT belongs to the card layout, but showing no trace
		// of it in compact meant saving a note in the DEFAULT density produced no
		// visible change at all — the write looked like it had failed, which is
		// exactly how it was reported. The marker is the smallest honest signal:
		// it confirms the note saved, carries the text as its tooltip, and
		// reopens the editor on click.
		//
		// It rides in the name cell rather than the action cell for two reasons.
		// The action cell is a fixed-width column, and a third control there
		// overflowed it and clipped the status chip to "On trac". And the drill
		// action's receipt icon is another lined document outline, so at 16px the
		// two sat side by side as near-identical shapes. Here it reads as what it
		// is — something the budget HAS, next to its name — rather than a third
		// thing to do.
		budgetCompactNameCell(props, crowTitle),
		crowChip,
		// One action cell: the labelled Transactions drill (icon + accessible
		// name) and the ⋯ overflow, so the compact row keeps a one-click path to
		// the ledger without the name pretending to be one (C591).
		Div(css.Class("budget-crow-actions"),
			If(canDrill, Button(css.Class("btn btn-tool budget-crow-drill"), Type("button"),
				Attr("data-testid", "budget-view-txns-"+s.Budget.ID),
				Attr("aria-label", uistate.T("budgets.drillAria", title)),
				Title(drillTitle), OnClick(drill),
				uiw.Icon(icon.Receipt, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			kebabNode,
		),
		Div(css.Class("budget-crow-figures"),
			Span(css.Class("budget-crow-amt fig"),
				Span(css.Class("budget-crow-spent"), barSpent),
				Span(css.Class("budget-crow-limit"), " of "),
				Button(css.Class("budget-limit-btn", "budget-crow-limit-btn"), Type("button"), Ref(props.LimitButtonRef), Attr("data-testid", "budget-limit-btn-"+s.Budget.ID),
					Title(uistate.T("budgets.limitEditTitle")), Attr("aria-label", uistate.T("budgets.limitEditTitle")+" — "+title),
					OnClick(startLimitEdit), fmtMoney(limit)),
				If(budgetCadenceSuffix(s.Budget.Period) != "", Span(css.Class("budget-cadence"), budgetCadenceSuffix(s.Budget.Period))),
				Span(css.Class("budget-crow-fig-separator"), " · "), crowLeft,
			),
		),
		Div(css.Class("budget-crow-bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")+" — "+title),
			Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width))),
			If(!lastMonthMode && s.PeriodStatus != budgeting.PeriodEnded && props.PaceMarkerPct > 0 && s.Spent.Amount > 0,
				Div(css.Class("bar-pace-tick"), Attr("data-testid", "budget-pace-tick-"+s.Budget.ID), Attr("aria-hidden", "true"),
					Attr("style", fmt.Sprintf("position:absolute;top:-3px;height:12px;left:%d%%;width:2px;background:var(--text-1,var(--text));pointer-events:none", props.PaceMarkerPct)))),
		),
		Div(css.Class("budget-crow-sub"), props.ProjectionLine),
		Div(css.Class("budget-crow-rollover"), props.RolloverDetail),
		If(props.LimitEditing, budgetLimitEditor(
			s.Budget.ID, s.Budget.Name, props.LimitInputRef, props.LimitDraft, onLimitDraft, saveLimitEdit, cancelLimitEdit,
			limitKeyDown, limitErrID, limitErrMsg, limit,
		)),
	)
	return row
}

// budgetCompactNameCell renders the compact row's title and in-name markers.
func budgetCompactNameCell(props budgetRowCompactProps, crowTitle ui.Node) ui.Node {
	s := props.Status
	title := props.Title
	hasNote, notesLabel, noteText, openNotesFn := props.HasNote, props.NotesLabel, props.NoteText, props.OnOpenNotes
	hasRecurring := props.HasRecurring
	fundedChip := props.FundedChip
	return Div(css.Class("budget-crow-head"),
		If(s.Budget.Scope == domain.ScopeIndividual, budgetOwnerInitial(s.Budget.OwnerID, props.Members)),
		crowTitle,
		// The note marker rides the SAME hover system as every other marker in
		// this cell: its words preview in the shared popover after the intent
		// delay, and a click still opens the editor. It used to carry a native
		// `title` instead — a third hover behaviour on a row that already had
		// two, and the one its own comment recorded as mouse-only.
		If(hasNote, ui.CreateElement(budgetNotesMarker, budgetNotesMarkerProps{
			BudgetID: s.Budget.ID, Label: notesLabel, Text: noteText,
			RowTitle: title, OnOpen: openNotesFn,
		})),
		If(s.Budget.Rollover, budgetRolloverBadgeFor(props.budgetRowProps)),
		// WF18: and so does goal-funded spending, for the same reason — the
		// card's line lives behind a disclosure this density omits, so a budget
		// that only looks over because a goal paid for part of it would read as
		// a plain overspend here. Exactly the trap WF17 hit one line below.
		// Two renderings of one marker, one visible at a time (CSS decides by
		// content width): the pill while its words fit, the glyph once they
		// do not. Both are always mounted so neither has to re-enter the tree
		// on a resize — and the glyph owns the hover/popover hooks, which must
		// not appear and disappear from a keyed row's hook order.
		// Coverage, which the compact row never reported: the card carries a
		// "Recurring cover" / "Covered this period" line, and dropping to the
		// list lost it entirely — so a budget only looking healthy because
		// another one is paying for it read as plain healthy (Cam, 2026-08-31).
		// Recurring wins over one-time, matching the card: a standing
		// arrangement is inherently covered, so the two never both show.
		If(hasRecurring, ui.CreateElement(budgetGoalGlyph, budgetGoalGlyphProps{
			BudgetID: s.Budget.ID, Kind: "cover", Icon: icon.Scale,
			Title:    uistate.T("budgets.coverMarkerOngoingTitle"),
			Text:     coverOngoingText(s.Budget, s.Limit.Currency),
			PillText: uistate.T("budgets.coverMarkerPill"), PillClass: "pill budget-crow-chip is-cover",
		})),
		If(!hasRecurring && props.Covered, ui.CreateElement(budgetGoalGlyph, budgetGoalGlyphProps{
			BudgetID: s.Budget.ID, Kind: "cover", Icon: icon.Scale,
			Title:    uistate.T("budgets.coverMarkerOnceTitle"),
			Text:     uistate.T("budgets.coverMarkerOnceText"),
			PillText: uistate.T("budgets.coverMarkerPill"), PillClass: "pill budget-crow-chip is-cover",
		})),
		If(fundedChip != "", ui.CreateElement(budgetGoalGlyph, budgetGoalGlyphProps{
			BudgetID: s.Budget.ID, Kind: "goal", Icon: icon.Goals,
			Title: goalGlyphTitle(), Text: fundedChip,
			// No pill: "part funded by a goal" renders at 168px and the name
			// cell is 151px even at 1042px of content — it would need roughly
			// 1400px before the words fit beside a name, so the pill shape
			// existed only to spill 87px past the cell and paint over the bar
			// (Cam, 2026-08-31). The sentence lives in the popover, where it
			// has room; the row keeps the symbol. "Rolls over" is 68px and
			// still earns its words, so rollover keeps both shapes.
		})),
		// WF17: a paused target has to show HERE too. The card's target line
		// lives behind the details disclosure, which compact mode omits — so
		// pausing from the ⋯ menu produced no visible change at all, and the
		// action looked like it had failed. That is precisely the report C545
		// fixed for notes, arriving again through a different door, so it gets
		// the same answer: the smallest honest marker, carrying the resume date
		// as its label.
		If(s.Budget.TargetSnoozed(time.Now()),
			Span(css.Class("pill budget-crow-chip", tw.TextDim),
				Attr("data-testid", "budget-snoozed-chip-"+s.Budget.ID),
				Attr("title", uistate.T("budgets.targetSnoozed",
					fmtMoney(s.Budget.TargetAmount),
					uistate.LoadPrefs().FormatDate(s.Budget.TargetSnoozedUntil))),
				uistate.T("budgets.targetPausedChip"))),
	)
}

func budgetOwnerInitial(ownerID string, members []domain.Member) ui.Node {
	for _, member := range members {
		if member.ID != ownerID {
			continue
		}
		name := strings.TrimSpace(member.Name)
		initial := "?"
		if name != "" {
			initial = strings.ToUpper(string([]rune(name)[0]))
		}
		return Span(css.Class("budget-owner-initial"), Attr("aria-label", member.Name), Attr("title", member.Name), initial)
	}
	return Fragment()
}

func budgetRowProjectionLine(s budgeting.Status, props budgetRowProps) ui.Node {
	if s.PeriodStatus == budgeting.PeriodEnded && s.Projection.EndedOverBy.Amount > 0 {
		return Span(css.Class("budget-sub", tw.TextDown), Attr("data-testid", "budget-projection-ended-"+s.Budget.ID),
			uistate.T("br3row.endedOver", fmtMoney(s.Projection.EndedOverBy)))
	}
	if s.PeriodStatus == budgeting.PeriodEnded {
		return Fragment()
	}
	if s.Projection.Error == nil && s.Projection.CommittedDominates && s.Projection.Committed.Amount > 0 {
		return Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-projection-committed-"+s.Budget.ID),
			uistate.T("br3row.committedDominates", fmtMoney(s.Projection.Committed)))
	}
	if s.Projection.Error == nil && s.Projection.OverBy.Amount > 0 {
		return Span(css.Class("budget-sub", tw.TextDown), Attr("data-testid", "budget-projection-"+s.Budget.ID),
			uistate.T("bud2.projectionOver", fmtMoney(s.Projection.OverBy)))
	}
	return Fragment()
}

func budgetCadenceSuffix(period domain.Period) string {
	switch period {
	case domain.PeriodWeekly:
		return uistate.T("br3row.cadenceWeek")
	case domain.PeriodQuarterly:
		return uistate.T("br3row.cadenceQuarter")
	case domain.PeriodYearly:
		return uistate.T("br3row.cadenceYear")
	default:
		return ""
	}
}

func budgetLimitEditor(id, budgetName string, inputRef ui.DOMRef, draft string, onDraft, save, cancel, keyDown any, errID, errMsg string, effective money.Money) ui.Node {
	return Form(css.Class("budget-limit-editor"), Attr("data-testid", "budget-limit-editor-"+id), OnSubmit(save),
		Strong(css.Class("budget-limit-editor-title"), budgetName+" "+uistate.T("budgets.limitTitleSuffix")),
		Div(css.Class("budget-limit-editor-cap"),
			Strong(Attr("data-testid", "budget-limit-effective-"+id), budgetEffectiveCapCopy(id, effective))),
		Div(css.Class("budget-limit-editor-base"),
			Label(For("budget-limit-input-"+id), uistate.T("budgets.baseLimitLabel")),
			Input(append(append([]any{css.Class("field", "budget-limit-input"), Type("number"),
				ID("budget-limit-input-" + id), Attr("data-testid", "budget-limit-input-"+id),
				Attr("aria-label", budgetName+" "+uistate.T("budgets.baseLimitNameSuffix")), Ref(inputRef), Step("0.01"), Attr("min", "0.01"),
				OnInput(onDraft), OnKeyDown(keyDown)}, errAttrs(errID, errMsg)...), uiw.FieldValue(draft))...),
		),
		Button(append([]any{css.Class("btn btn-sm budget-limit-save"), Type("submit"), Attr("data-testid", "budget-limit-save-"+id),
			Attr("aria-label", uistate.T("action.save")), uistate.T("action.save")}, disabledAttr(errMsg != "")...)...),
		Button(css.Class("btn btn-sm budget-limit-cancel"), Type("button"), Attr("data-testid", "budget-limit-cancel-"+id),
			OnClick(cancel), uistate.T("action.cancel")),
		errText(errID, errMsg),
	)
}

func budgetEffectiveCapCopy(id string, effective money.Money) string {
	base := effective
	if app := appstate.Default; app != nil {
		for _, budget := range app.Budgets() {
			if budget.ID == id {
				base = budget.Limit
				break
			}
		}
	}
	delta, err := money.Sub(effective.Amount, base.Amount)
	if err != nil || delta == 0 {
		return uistate.T("br3row.effectiveBase", fmtMoney(effective))
	}
	sign := "+"
	if delta < 0 {
		sign = "−"
		delta, err = money.Neg(delta)
		if err != nil {
			return uistate.T("br3row.effectiveBase", fmtMoney(effective))
		}
	}
	return uistate.T("br3row.effectiveBaseCarry", fmtMoney(effective), sign, fmtMoney(money.New(delta, effective.Currency)))
}

// budgetRolloverBadgeProps configures the per-row rollover policy badge (C395).
type budgetRolloverBadgeProps struct {
	BudgetID     string
	Rollover     bool   // whether this budget carries unused money into the next period
	CapPeriods   int    // RolloverCapPeriods; > 0 caps the carried surplus at N× the limit
	Carry        string // formatted previous-period remaining ("$50.00 left" / "$32.00 over"); "" when none
	Chain        string // auditable period-by-period carry chain
	EffectiveCap string // this period's effective cap when it differs from the base limit; "" otherwise
	CapMath      string // the effective cap's arithmetic ("$200 limit + $50 carried over")
}

// budgetRolloverBadge renders the small rollover-policy pill on a budget row and,
// on click, a deterministic carryover-math explainer popover — the project's
// "no black boxes" rule applied to rollover. The pill states policy at a glance
// (No rollover / Rolls over / Rolls over · cap N); the popover reconstructs THIS
// period's carryover from the exact figures the engine produced (last period's
// surplus/shortfall, the cap rule, and the resulting effective cap with its
// arithmetic). Its own component so the popover's open-state + dismiss hooks sit
// at a stable position — never inside the row's caption list.
func budgetRolloverBadge(props budgetRolloverBadgeProps) ui.Node {
	// The glance-level label + tone.
	var label, toneCls string
	switch {
	case !props.Rollover:
		label, toneCls = uistate.T("budgetsRollover.badgeOff"), " is-off"
	case props.CapPeriods > 0:
		label, toneCls = uistate.T("budgetsRollover.badgeCapped", props.CapPeriods), " is-capped"
	default:
		label, toneCls = uistate.T("budgetsRollover.badgeOn"), " is-on"
	}

	// The deterministic breakdown, assembled from the same figures shown on the row.
	var parts []string
	if !props.Rollover {
		parts = append(parts, uistate.T("budgetsRollover.offText"))
	} else {
		parts = append(parts, uistate.T("budgetsRollover.onIntro"))
		if props.Carry != "" {
			parts = append(parts, uistate.T("budgetsRollover.carryLine", props.Carry))
		} else {
			parts = append(parts, uistate.T("budgetsRollover.noCarryLine"))
		}
		if props.CapPeriods > 0 {
			parts = append(parts, uistate.T("budgetsRollover.capLine", props.CapPeriods))
		}
		if props.EffectiveCap != "" && props.CapMath != "" {
			parts = append(parts, uistate.T("budgetsRollover.capThisPeriod", props.EffectiveCap, props.CapMath))
		}
		if props.Chain != "" {
			parts = append(parts, uistate.T("budgets.rolloverChain", props.Chain))
		}
	}

	// Same two renderings as the goal marker, one visible at a time by content
	// width: the worded pill while it fits, a glyph once it does not. "Rolls over"
	// is shorter than the goal sentence but it is still a whole word competing with
	// the budget's own name in a cell that has already given up its "left" column
	// (Cam, 2026-08-31). The glyph carries the SAME explainer this badge already
	// builds, so the two paths cannot drift apart.
	// ONE marker, both shapes, one hover system. This used to run its own popover
	// AND nest a glyph that ran a second — two systems for one piece of information,
	// able to open together and anchored to different elements (Cam, 2026-08-31).
	return Span(ClassStr("budget-rollover-badge"+toneCls),
		ui.CreateElement(budgetGoalGlyph, budgetGoalGlyphProps{
			BudgetID: props.BudgetID, Kind: "rollover", Icon: icon.Repeat,
			Title: uistate.T("budgetsRollover.popTitle"), Text: strings.Join(parts, " "),
			PillText: label, PillClass: "budget-rollover-pill",
		}),
	)
}

// budgetRolloverBadgeFor builds the rollover badge element for a row from its status
// and the row props, reading the policy straight off the domain budget.
func budgetRolloverBadgeFor(props budgetRowProps) ui.Node {
	return ui.CreateElement(budgetRolloverBadge, budgetRolloverBadgeProps{
		BudgetID:     props.Status.Budget.ID,
		Rollover:     props.Status.Budget.Rollover,
		CapPeriods:   props.Status.Budget.RolloverCapPeriods,
		Carry:        props.RolloverCarry,
		Chain:        "",
		EffectiveCap: props.EffectiveCap,
		CapMath:      props.EffectiveCapMath,
	})
}

func budgetRolloverTable(view budgeting.RolloverView, id string) ui.Node {
	periods := append([]budgeting.RolloverPeriodState(nil), view.Prior...)
	if !view.Current.Start.IsZero() {
		periods = append(periods, view.Current)
	}
	if len(periods) == 0 {
		return Fragment()
	}
	if len(periods) > 4 {
		periods = periods[len(periods)-4:]
	}
	rows := make([]ui.Node, 0, len(periods))
	for _, state := range periods {
		month := budgetRolloverMonthLabel(state.Start)
		rows = append(rows, Tr(
			Th(Attr("scope", "row"), month), Td(fmtMoney(state.EffectiveCap)), Td(formatSignedMoney(state.CarryIn)),
			Td(fmtMoney(state.Spend)), Td(formatSignedMoney(state.CarryOut)),
		))
	}
	rowArgs := make([]any, len(rows))
	for i := range rows {
		rowArgs[i] = rows[i]
	}
	return Table(Attr("data-testid", "budget-rollover-table-"+id),
		Thead(Tr(Th(uistate.T("bud2.rolloverMonth")), Th(uistate.T("bud2.rolloverLimit")), Th(uistate.T("bud2.rolloverCarriedIn")), Th(uistate.T("bud2.rolloverSpent")), Th(uistate.T("bud2.rolloverCarriedOut")))),
		Tbody(rowArgs...),
	)
}

func budgetRolloverMonthLabel(start time.Time) string {
	return budgetMonthName(start, true) + " " + strconv.Itoa(start.Year())
}

func formatSignedMoney(amount money.Money) string {
	if amount.Amount < 0 {
		magnitude, err := money.Abs(amount.Amount)
		if err != nil {
			return "—"
		}
		return "−" + fmtMoney(money.New(magnitude, amount.Currency))
	}
	return "+" + fmtMoney(amount)
}

func formatFreeMoney(amount money.Money) string {
	if amount.Amount < 0 {
		magnitude, err := money.Abs(amount.Amount)
		if err != nil {
			return "—"
		}
		return "−" + fmtMoney(money.New(magnitude, amount.Currency))
	}
	return fmtMoney(amount)
}

// budgetDrillScope resolves what a budget's "Transactions" drill-through should
// show: exactly the set its spend total is computed over (C585). The descendant
// expansion is the same categorytree.DescendantsOfAll call EvaluateRollup is
// given, so the ledger and the bar cannot describe different sets.
func budgetDrillScope(b domain.Budget) budgeting.Scope {
	app := appstate.Default
	if app == nil {
		return budgeting.ScopeOf(b, nil)
	}
	return budgeting.ScopeOf(b, categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs()))
}

// budgetScopeFilter turns a budget's spend scope into a ledger filter that
// selects exactly the transactions the scope counts.
//
// A single category uses the plain Category field so the ledger's category
// dropdown still reflects the filter; more than one uses the multi-value
// Categories. Tracked tags switch the filter to ScopeAny, because a budget
// counts a charge that is in a tracked category OR carries a tracked tag — an
// AND there would hide the very charges the tag contributed.
func budgetScopeFilter(scope budgeting.Scope) uistate.TxFilter {
	var f uistate.TxFilter
	switch len(scope.CategoryIDs) {
	case 0:
		// nothing tracked by category — tags alone (or an unfiltered ledger)
	case 1:
		f.Category = scope.CategoryIDs[0]
	default:
		f.Categories = strings.Join(scope.CategoryIDs, ",")
	}
	if len(scope.Tags) > 0 {
		f.Tags = strings.Join(scope.Tags, ",")
		f.ScopeAny = true
	}
	return f
}

// budgetTopDriversFor computes the up-to-n largest charges driving a budget's spend
// this period, reconciled with the shown Spent (same rollup covers as EvaluateRollup).
// Returns the drivers and the base currency. Reads appstate directly (like the
// composition pie), so the card component stays self-contained.
func budgetTopDriversFor(b domain.Budget, n int, anchor time.Time) ([]budgeting.Driver, string) {
	app := appstate.Default
	if app == nil {
		return nil, budgetBaseCurrency("")
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	// The drivers describe the VIEWED period (QA CF-05), not necessarily today's.
	prefs := uistate.LoadPrefs()
	start, end := budgeting.PeriodRangeAnchored(b.Period, anchor, prefs.WeekStartWeekday(), budgetPayCycleAnchor(prefs.PayCycleAnchor))
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
	Status budgeting.Status
	Budget domain.Budget
	// Title is the budget's display title ("name · category"), which scopes the
	// toggle's accessible name (C668). Several budgets can be near or over at
	// once, and every one of them offers a control called "What's driving this?".
	Title  string
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
	revision := uistate.UseDataRevision().Get()
	open := expanded.Get()

	discLabel := uistate.T("budgets.driversShow")
	if open {
		discLabel = uistate.T("budgets.driversHide")
	}
	listID := "budget-drivers-list-" + props.Budget.ID
	// C668: name the control by its budget. The page can show several of these at
	// once — one per near-or-over budget — and "What's driving this?" identified
	// none of them, so a role-based lookup resolved two matches and a screen-reader
	// user had no way to tell which card was about to expand. The visible label
	// stays short (the card around it supplies the context a sighted user needs);
	// the accessible name carries the budget, and aria-controls already points at
	// this card's own region.
	discName := discLabel
	if props.Title != "" {
		discName = discLabel + " — " + props.Title
	}
	head := Button(css.Class("budget-drivers-toggle"), Type("button"),
		Attr("data-testid", "budget-drivers-toggle-"+props.Budget.ID),
		Attr("aria-label", discName), Title(discName),
		Attr("aria-expanded", ariaBool(open)), Attr("aria-controls", listID), OnClick(toggle),
		Span(discLabel),
		uiw.Icon(icon.ChevronDown, css.Class("budget-drivers-chev", tw.W35, tw.H35)))

	var body ui.Node = Fragment()
	if open {
		anchor := props.Anchor
		if anchor.IsZero() {
			anchor = time.Now()
		}
		prefs := uistate.LoadPrefs()
		start, end := budgeting.PeriodRangeAnchored(props.Budget.Period, anchor, prefs.WeekStartWeekday(), budgetPayCycleAnchor(prefs.PayCycleAnchor))
		drivers, _ := memoBudgetDrivers(appstate.Default, revision, props.Budget.ID, start, end, func() ([]budgeting.Driver, string) {
			return budgetTopDriversFor(props.Budget, 3, anchor)
		})
		// SM-4: name WHAT KIND of thing happened before listing who. The merchants
		// answer "who", and the reader was left to infer the rest from three names.
		reason := ui.CreateElement(budgetWhyOverHint, budgetWhyOverProps{
			Status: props.Status, Budget: props.Budget, Anchor: props.Anchor, Drivers: drivers,
		})
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
					Amount: fmtMoney(money.New(d.Amount, d.Currency)), Recurring: isRec, OnDrill: drill,
				}))
			}
			body = Div(css.Class("budget-drivers-list"), Attr("id", listID), rows)
		}
		body = Fragment(reason, body)
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
	Label    string
	Amount   int64 // signed minor units
	Currency string
	ValStr   string
}

type budgetPeriodMemoKey struct {
	app      *appstate.App
	revision int
	budgetID string
	start    string
	end      string
}

type budgetDriverMemoValue struct {
	drivers []budgeting.Driver
	base    string
}

var budgetPeriodMemos = struct {
	sync.Mutex
	goal    map[budgetPeriodMemoKey]goalfunded.Split
	funding map[budgetPeriodMemoKey][]goalfunded.BudgetFunding
	drivers map[budgetPeriodMemoKey]budgetDriverMemoValue
	pie     map[budgetPeriodMemoKey][]budgetSlice
}{goal: make(map[budgetPeriodMemoKey]goalfunded.Split), funding: make(map[budgetPeriodMemoKey][]goalfunded.BudgetFunding), drivers: make(map[budgetPeriodMemoKey]budgetDriverMemoValue), pie: make(map[budgetPeriodMemoKey][]budgetSlice)}

func periodMemoKey(app *appstate.App, revision int, budgetID string, start, end time.Time) budgetPeriodMemoKey {
	return budgetPeriodMemoKey{app: app, revision: revision, budgetID: budgetID,
		start: start.UTC().Format(time.RFC3339Nano), end: end.UTC().Format(time.RFC3339Nano)}
}

func memoGoalFunded(app *appstate.App, revision int, budgetID string, start, end time.Time, calculate func() goalfunded.Split) goalfunded.Split {
	key := periodMemoKey(app, revision, budgetID, start, end)
	budgetPeriodMemos.Lock()
	if value, ok := budgetPeriodMemos.goal[key]; ok {
		budgetPeriodMemos.Unlock()
		return value
	}
	budgetPeriodMemos.Unlock()
	value := calculate()
	budgetPeriodMemos.Lock()
	if len(budgetPeriodMemos.goal) > 256 {
		budgetPeriodMemos.goal = make(map[budgetPeriodMemoKey]goalfunded.Split)
	}
	budgetPeriodMemos.goal[key] = value
	budgetPeriodMemos.Unlock()
	return value
}

func memoBudgetGoalFunding(app *appstate.App, revision int, start, end time.Time, calculate func() []goalfunded.BudgetFunding) []goalfunded.BudgetFunding {
	key := periodMemoKey(app, revision, "goal-funding-allocation", start, end)
	budgetPeriodMemos.Lock()
	if value, ok := budgetPeriodMemos.funding[key]; ok {
		budgetPeriodMemos.Unlock()
		return append([]goalfunded.BudgetFunding(nil), value...)
	}
	budgetPeriodMemos.Unlock()
	value := calculate()
	budgetPeriodMemos.Lock()
	if len(budgetPeriodMemos.funding) > 256 {
		budgetPeriodMemos.funding = make(map[budgetPeriodMemoKey][]goalfunded.BudgetFunding)
	}
	budgetPeriodMemos.funding[key] = append([]goalfunded.BudgetFunding(nil), value...)
	budgetPeriodMemos.Unlock()
	return value
}

func memoBudgetDrivers(app *appstate.App, revision int, budgetID string, start, end time.Time, calculate func() ([]budgeting.Driver, string)) ([]budgeting.Driver, string) {
	key := periodMemoKey(app, revision, budgetID, start, end)
	budgetPeriodMemos.Lock()
	if value, ok := budgetPeriodMemos.drivers[key]; ok {
		budgetPeriodMemos.Unlock()
		return append([]budgeting.Driver(nil), value.drivers...), value.base
	}
	budgetPeriodMemos.Unlock()
	drivers, base := calculate()
	budgetPeriodMemos.Lock()
	if len(budgetPeriodMemos.drivers) > 256 {
		budgetPeriodMemos.drivers = make(map[budgetPeriodMemoKey]budgetDriverMemoValue)
	}
	budgetPeriodMemos.drivers[key] = budgetDriverMemoValue{drivers: append([]budgeting.Driver(nil), drivers...), base: base}
	budgetPeriodMemos.Unlock()
	return drivers, base
}

func memoBudgetComposition(b domain.Budget, anchor time.Time, revision int) []budgetSlice {
	app := appstate.Default
	if app == nil {
		return nil
	}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	start, end := budgeting.PeriodRangeAnchored(b.Period, anchor, uistate.LoadPrefs().WeekStartWeekday(),
		budgetPayCycleAnchor(uistate.LoadPrefs().PayCycleAnchor))
	key := periodMemoKey(app, revision, b.ID, start, end)
	budgetPeriodMemos.Lock()
	if value, ok := budgetPeriodMemos.pie[key]; ok {
		budgetPeriodMemos.Unlock()
		return value
	}
	budgetPeriodMemos.Unlock()
	value := budgetCompositionSlices(b, anchor)
	budgetPeriodMemos.Lock()
	if len(budgetPeriodMemos.pie) > 256 {
		budgetPeriodMemos.pie = make(map[budgetPeriodMemoKey][]budgetSlice)
	}
	budgetPeriodMemos.pie[key] = append([]budgetSlice(nil), value...)
	budgetPeriodMemos.Unlock()
	return value
}

// budgetCompositionSlices returns the shared budgeting engine's signed contributions
// for the current period. Negative discounts remain visible, and each value keeps the
// budget's currency so the legend reconciles to its Spent figure.
func budgetCompositionSlices(b domain.Budget, anchor time.Time) []budgetSlice {
	app := appstate.Default
	if app == nil {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	// Composition follows the VIEWED period (QA CF-05), matching the card's bar.
	prefs := uistate.LoadPrefs()
	start, end := budgeting.PeriodRangeAnchored(b.Period, anchor, prefs.WeekStartWeekday(), budgetPayCycleAnchor(prefs.PayCycleAnchor))
	slices, err := budgetCompositionData(b, app.Transactions(), app.Categories(), start, end, rates)
	if err != nil {
		slog.Error("calculate budget composition", "budget_id", b.ID, "error", err)
		return nil
	}
	return slices
}

func budgetCompositionData(b domain.Budget, transactions []domain.Transaction, categories []domain.Category, start, end time.Time, rates currency.Rates) ([]budgetSlice, error) {
	catName := map[string]string{}
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	covers := categorytree.DescendantsOfAll(categories, b.TrackedCategoryIDs())
	compositions, err := budgeting.CompositionSlices(b, transactions, start, end, rates, covers)
	if err != nil {
		return nil, err
	}
	slices := make([]budgetSlice, 0, len(compositions))
	for _, composition := range compositions {
		name := catName[composition.CategoryID]
		if name == "" {
			name = composition.CategoryID
		}
		slices = append(slices, budgetSlice{Label: name, Amount: composition.Amount.Amount, Currency: composition.Amount.Currency, ValStr: fmtMoney(composition.Amount)})
	}
	sort.SliceStable(slices, func(i, j int) bool {
		return budgetSliceMagnitude(slices[i].Amount) > budgetSliceMagnitude(slices[j].Amount)
	})
	return slices, nil
}

func budgetSliceMagnitude(amount int64) uint64 {
	if amount < 0 {
		return uint64(-(amount + 1)) + 1
	}
	return uint64(amount)
}

// budgetPiePalette colours the composition wedges (theme-agnostic, distinct hues).
var budgetPiePalette = []string{"#4f9d69", "#e0a458", "#5b8def", "#c05e5e", "#8b6fb0", "#3fa7a0", "#b0894f", "#7f8c99"}

// budgetPie renders the right-side spend-composition donut for a COMPOSITE budget (2+
// tracked dimensions — multi-category, cats+tags, or multi-tag). Returns (node, shown):
// shown is false for a single-dimension budget or when there's no spend yet, so the caller
// omits the pie column entirely.
func budgetPie(b domain.Budget, anchor time.Time, revision int) (ui.Node, bool) {
	if len(b.TrackedCategoryIDs())+len(b.TrackedTags) < 2 {
		return Fragment(), false // not composite — the bar already tells the whole story
	}
	slices := memoBudgetComposition(b, anchor, revision)
	var total int64
	for _, s := range slices {
		if s.Amount > 0 {
			total += s.Amount
		}
	}
	if len(slices) == 0 {
		return Fragment(), false // nothing spent yet — an empty pie helps no one
	}
	var stops []string
	legend := []any{css.Class("budget-pie-legend")}
	var acc float64
	hasNegative := false
	for i, s := range slices {
		color := budgetPiePalette[i%len(budgetPiePalette)]
		if s.Amount > 0 && total > 0 {
			pct := float64(s.Amount) / float64(total) * 100
			stops = append(stops, fmt.Sprintf("%s %.3f%% %.3f%%", color, acc, acc+pct))
			acc += pct
		}
		rowClass := "budget-pie-legrow"
		if s.Amount < 0 {
			rowClass += " is-negative"
			hasNegative = true
		}
		legend = append(legend, Div(css.Class(rowClass),
			Span(css.Class("budget-pie-dot"), Attr("style", "background:"+color)),
			Span(css.Class("budget-pie-leglabel"), s.Label),
			Span(css.Class("budget-pie-legval"), s.ValStr)))
	}
	var donut ui.Node = Fragment()
	if len(stops) > 0 {
		donut = Div(css.Class("budget-pie-donut"), Attr("aria-hidden", "true"),
			Attr("style", "background:conic-gradient("+strings.Join(stops, ", ")+")"),
			Div(css.Class("budget-pie-hole")))
	}
	var negativeNote ui.Node = Fragment()
	if hasNegative {
		negativeNote = P(css.Class("budget-pie-negative-note"), uistate.T("budgets.compositionNegativeNote"))
	}
	return Div(css.Class("budget-pie"), Attr("data-testid", "budget-pie-"+b.ID),
		donut, Div(legend...), negativeNote), true
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

// budgetDetailsKey names the card's details disclosure for its current state
// (C595). One label per state rather than a swapped noun, so the control reads
// the same way the density toggle now does.
func budgetDetailsKey(open bool) string {
	if open {
		return "budgets.detailsHide"
	}
	return "budgets.detailsShow"
}

// coverOngoingText describes a standing cover arrangement for the marker's
// popover. The amount is what makes the badge worth opening: "covered" alone does
// not say whether another budget is quietly carrying $10 of this one or $400.
func coverOngoingText(b domain.Budget, evalCurrency string) string {
	rc := b.RecurringCover
	if rc == nil {
		return uistate.T("budgets.coverMarkerOngoingPlain")
	}
	// The budget's own limit currency when it has one, else the currency the row was
	// evaluated in — never a bare guess at the household base.
	cur := b.Limit.Currency
	if cur == "" {
		cur = evalCurrency
	}
	return uistate.TN("budgets.coverMarkerOngoingTextOne", "budgets.coverMarkerOngoingTextMany",
		len(rc.Sources), fmtMoney(money.New(rc.AmountMinor, cur)))
}

// budgetRowReleaseImpact supplies the shared consequence wording for the release action.
func budgetRowReleaseImpact() string {
	return fundsImpactLine(budgeting.ImpactRelease)
}
