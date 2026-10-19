// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverformula"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// BudgetEditFormProps drives the budget editor form rendered inside the shell-root
// flip modal (see internal/app BudgetEditHost). Mode selects which editor to show.
type BudgetEditFormProps struct {
	BudgetID string
	Mode     string // one of uistate.BudgetEditMode*
	OnDone   func() // clears the atom (closes the modal); called after a save/cancel
}

// BudgetEditForm renders the budget editor (full edit, or top-up) as the body of the
// flip modal. It owns all its form state and its own Save/Cancel buttons; the host's
// FlipPanel is NoFooter. Because the host only renders this when the atom is set, the
// component mounts fresh on each open, so the useState initializers seed correctly
// from the budget. It lives at the shell root, outside the transformed bento/tile
// ancestors, so the modal centers on the viewport.
func BudgetEditForm(props BudgetEditFormProps) ui.Node {
	editData := useBudgetEditData(props)
	nameS := ui.UseState(editData.b.Name)
	editDraft := useBudgetEditDraft(budgetEditDraftProps{
		nameS: nameS, b: editData.b, limitMajor: editData.limitMajor, props: props, coverDefaultStr: editData.coverDefaultStr,
		found: editData.found, dec: editData.dec,
	})
	onToggleSrc, onWeightSrc, onToggleMax := budgetEditSourceCallbacks(editDraft.coverSelS, editDraft.coverWtS, editDraft.coverMaxS)
	submitCover := useBudgetEditCoverSave(budgetEditCoverSaveProps{
		app: editData.app, done: editData.done, amtFxS: editDraft.amtFxS, pr: editData.pr, coverPayAnchor: editData.coverPayAnchor,
		amtFormulaS: editDraft.amtFormulaS, b: editData.b, errS: editDraft.errS, coverAmtS: editDraft.coverAmtS, dec: editData.dec,
		coverSrcs: editData.coverSrcs, coverSelS: editDraft.coverSelS, recurringS: editDraft.recurringS, coverWtS: editDraft.coverWtS,
		wtFxS: editDraft.wtFxS, wtFormulaS: editDraft.wtFormulaS, coverMaxS: editDraft.coverMaxS, cur: editData.cur, props: props,
		viewAnchor: editData.viewAnchor, coverCustomVals: editDraft.coverCustomVals,
	})
	cancel, onCustom, onCoverCustom := useBudgetEditCustomCallbacks(editData.done, editDraft.customEditVals, editDraft.coverCustomVals)

	saveEdit := useBudgetEditSave(budgetEditSaveProps{
		app: editData.app, done: editData.done, props: props, nameS: nameS, ev: editDraft.ev, errS: editDraft.errS,
		limitS: editDraft.limitS, dec: editData.dec, cur: editData.cur, periodS: editDraft.periodS, ownerS: editDraft.ownerS,
		rolloverS: editDraft.rolloverS, rolloverCapS: editDraft.rolloverCapS, methodologyS: editDraft.methodologyS,
		customEditVals: editDraft.customEditVals, targetKindS: editDraft.targetKindS, targetAmtS: editDraft.targetAmtS,
		targetDateS: editDraft.targetDateS, linkedGoalS: editDraft.linkedGoalS,
	})

	saveNotes, removeNote := useBudgetEditNotesSave(budgetEditNotesSaveProps{
		app: editData.app, props: props, errS: editDraft.errS, done: editData.done, notesS: editDraft.notesS,
	})

	submitTopup := useBudgetEditTopupSave(budgetEditTopupSaveProps{
		app: editData.app, done: editData.done, topupAmt: editDraft.topupAmt, dec: editData.dec, errS: editDraft.errS,
		topupPermanentS: editDraft.topupPermanentS, b: editData.b, viewAnchor: editData.viewAnchor, coverPayAnchor: editData.coverPayAnchor,
		pr: editData.pr, coverSelS: editDraft.coverSelS, coverSrcs: editData.coverSrcs, cur: editData.cur, props: props,
	})

	if editData.app == nil || !editData.found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if editDraft.errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), editDraft.errS.Get())
	}

	// --- Notes: a free-text note on the budget. The textarea grows to fill the modal. ---
	if props.Mode == uistate.BudgetEditModeNotes {
		return ui.CreateElement(budgetEditNotes, budgetEditNotesProps{
			b: editData.b, removeNote: removeNote, saveNotes: saveNotes, app: editData.app, notesS: editDraft.notesS,
			onNotes: editDraft.onNotes, errLine: errLine, cancel: cancel,
		})
	}

	// --- Formulas: the budget's engine variables + live values, each copyable. ---
	if props.Mode == uistate.BudgetEditModeFormulas {
		return budgetFormulasView(editData.b, editData.selfStatus, editData.selfFound, editData.cur, editData.dec, cancel)
	}

	// --- Cover: spread the overspend across one or more source budgets. ---
	if props.Mode == uistate.BudgetEditModeCover {
		return ui.CreateElement(budgetEditCover, budgetEditCoverProps{
			coverSrcs: editData.coverSrcs, coverIncomplete: editData.coverIncomplete, app: editData.app, pr: editData.pr,
			coverPayAnchor: editData.coverPayAnchor, amtFxS: editDraft.amtFxS, amtFormulaS: editDraft.amtFormulaS, b: editData.b,
			cur: editData.cur, coverAmtS: editDraft.coverAmtS, dec: editData.dec, coverWtS: editDraft.coverWtS, wtFxS: editDraft.wtFxS,
			coverSelS: editDraft.coverSelS, wtFormulaS: editDraft.wtFormulaS, coverMaxS: editDraft.coverMaxS, onToggleSrc: onToggleSrc,
			onWeightSrc: onWeightSrc, onToggleMax: onToggleMax, submitCover: submitCover, coverShortfallStr: editData.coverShortfallStr,
			onAmtFormula: editDraft.onAmtFormula, onCoverAmt: editDraft.onCoverAmt, coverDefaultStr: editData.coverDefaultStr,
			fullCover: editDraft.fullCover, toggleAmtFx: editDraft.toggleAmtFx, toggleWtFx: editDraft.toggleWtFx,
			onWtFormula: editDraft.onWtFormula, onToggleRecurring: editDraft.onToggleRecurring, recurringS: editDraft.recurringS,
			coverCustomVals: editDraft.coverCustomVals, onCoverCustom: onCoverCustom, errLine: errLine, cancel: cancel,
		})
	}

	// --- Top-up: raise the budget's cap (this period only or permanently), optionally
	// funded by pulling from other budgets. ---
	if props.Mode == uistate.BudgetEditModeTopup {
		return ui.CreateElement(budgetEditRaiseLimit, budgetEditRaiseLimitProps{
			topupPermanentS: editDraft.topupPermanentS, b: editData.b, selfEffCap: editData.selfEffCap, selfCapMath: editData.selfCapMath,
			coverSelS: editDraft.coverSelS, coverSrcs: editData.coverSrcs, topupCoverOpenS: editDraft.topupCoverOpenS,
			topupAmt: editDraft.topupAmt, dec: editData.dec, vw: editData.vw, submitTopup: submitTopup, onTopupAmt: editDraft.onTopupAmt,
			onToggleTopupCover: editDraft.onToggleTopupCover, cur: editData.cur, onToggleSrc: onToggleSrc,
			coverIncomplete: editData.coverIncomplete, errLine: errLine, cancel: cancel,
		})
	}

	// --- Full edit. Single-column form (.acct-edit-form) — a clean vertical stack so
	// the rollover explanation and Method sit full-width beneath their controls, and the
	// action row's margin-top:auto pins Save/Cancel to the modal's bottom. ---
	// G6: consequence previews — when the period or method is CHANGED from the saved
	// value, say what will happen before the user commits, instead of reshaping the
	// current period's figures silently on save.
	return ui.CreateElement(budgetEditFields, budgetEditFieldsProps{
		periodS: editDraft.periodS, b: editData.b, methodologyS: editDraft.methodologyS, limitS: editDraft.limitS, dec: editData.dec,
		cur: editData.cur, advEditOpen: editDraft.advEditOpen, catsOpen: editDraft.catsOpen, props: props, closeCats: editDraft.closeCats,
		saveEdit: saveEdit, ev: editDraft.ev, nameS: nameS, onLimit: editDraft.onLimit, app: editData.app, selfStatus: editData.selfStatus,
		viewAnchor: editData.viewAnchor, targetKindS: editDraft.targetKindS, targetAmtS: editDraft.targetAmtS,
		targetDateS: editDraft.targetDateS, linkedGoalS: editDraft.linkedGoalS, onFillPick: editDraft.onFillPick,
		selfFound: editData.selfFound, onTargetAmt: editDraft.onTargetAmt, onTargetDate: editDraft.onTargetDate, ownerS: editDraft.ownerS,
		openCats: editDraft.openCats, onRollover: editDraft.onRollover, rolloverS: editDraft.rolloverS,
		rolloverCapS: editDraft.rolloverCapS, customEditVals: editDraft.customEditVals, onCustom: onCustom,
		toggleAdvEdit: editDraft.toggleAdvEdit, errLine: errLine, cancel: cancel,
	})
}

// budgetTopupCapCopy builds the two sentences the top-up dialog opens with (C669):
// the arithmetic from the base limit to this period's effective cap, and which of
// those two numbers the typed amount is added to.
//
// The dialog used to say "Increase Groceries (currently $500.00)" while the card
// beside it showed a cap of $721.71 after rollover, and named neither figure as
// the one about to change. Both facts come from the SAME view the card renders —
// effCap and capMath are budgetView.RollEffCap / RollEffCapMath — so the dialog
// cannot quote a cap the card disagrees with. An empty effCap means nothing is
// carried in or boosted, and the base limit IS the cap.
func budgetTopupCapCopy(baseLimit, effCap, capMath string, permanent bool) (capLine, changesLine string) {
	capLine = uistate.T("budgets.topupCapPlain", baseLimit)
	effective := baseLimit
	if effCap != "" {
		effective = effCap
		capLine = uistate.T("budgets.topupCapMath", capMath, effCap)
	}
	if permanent {
		return capLine, uistate.T("budgets.topupChangesPerm", baseLimit)
	}
	return capLine, uistate.T("budgets.topupChangesMonth", effective, baseLimit)
}

// budgetTopupSourceRowProps carries data + the toggle callback for one fundable source
// budget in the top-up "cover from other budgets" checklist.
type budgetTopupSourceRowProps struct {
	ID, Label, AvailStr string
	// CommittedStr / AfterStr carry the same two facts as the cover list (C606):
	// how much of the movable money is already claimed by this period's recurring
	// charges, and what the source would have left once its share is taken.
	CommittedStr, AfterStr string
	Selected               bool
	OnToggle               func(string) // plain func — never an On* hook (no-On*-in-loop rule)
}

// budgetTopupSourceRow is one checkbox row in the top-up funding list. Its own component
// so the checkbox's change hook stays at a stable call-site (per the no-On*-in-loop rule).
func budgetTopupSourceRow(props budgetTopupSourceRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	return Label(css.Class("budget-topup-src"),
		Input(append([]any{Type("checkbox"), Attr("data-testid", "topup-src-"+props.ID), OnChange(toggle)}, checkedAttr(props.Selected)...)...),
		Div(css.Class("row-main"),
			Span(props.Label),
			// C606: the amount that can actually be moved, not the budget's limit.
			Span(css.Class("row-meta", tw.TextDim), Attr("data-testid", "topup-src-avail-"+props.ID),
				uistate.T("budgets.topupSrcAvail", props.AvailStr)),
			If(props.CommittedStr != "", Span(css.Class("row-meta", "cover-src-committed"),
				Attr("data-testid", "topup-src-committed-"+props.ID), props.CommittedStr)),
			If(props.AfterStr != "", Span(css.Class("row-meta", "cover-src-after"),
				Attr("data-testid", "topup-src-after-"+props.ID), props.AfterStr)),
		),
	)
}

// budgetFormulaRowProps carries one engine-variable name + its formatted value and the
// raw value to copy.
type budgetFormulaRowProps struct {
	Name, Value, Raw string
}

// budgetFormulaRow renders one copyable variable → value line: clicking the variable
// name copies the name, the copy button copies the value. Its own component so the copy
// hooks sit at stable call-sites inside the keyed list.
func budgetFormulaRow(props budgetFormulaRowProps) ui.Node {
	copyName := ui.UseEvent(Prevent(func() { copyToClipboard(props.Name, uistate.T("budgets.formulaCopied", props.Name)) }))
	copyVal := ui.UseEvent(Prevent(func() { copyToClipboard(props.Raw, uistate.T("budgets.formulaCopied", props.Name)) }))
	return Div(css.Class("budget-formula-row"),
		Button(css.Class("budget-formula-name"), Type("button"), Attr("data-testid", "budget-formula-name-"+props.Name),
			Title(uistate.T("budgets.copyName")), OnClick(copyName), props.Name),
		Span(css.Class("budget-formula-val"), props.Value),
		Button(css.Class("btn btn-sm btn-ghost budget-formula-copy"), Type("button"), Attr("data-testid", "budget-formula-copy-"+props.Name),
			Title(uistate.T("budgets.copyValue")), OnClick(copyVal), uiw.Icon(icon.Copy, css.Class(tw.W4, tw.H4))),
	)
}

// budgetFormulasView is the read-only "Formulas" modal body: the budget's engine
// variables (budget_<slug>_limit/spent/remaining/over/percent) with their live values,
// each copyable — a quick reference for building formulas/widgets over this budget.
func budgetFormulasView(b domain.Budget, st budgeting.Status, haveStatus bool, cur string, dec int, cancel ui.Handler) ui.Node {
	slugSrc := b.VarName
	if slugSrc == "" {
		slugSrc = b.Name
	}
	prefix := "budget_" + engineenv.BudgetVarSlug(slugSrc) + "_"
	var rows []any
	if haveStatus {
		limit, _ := st.Spent.Add(st.Remaining)
		over := money.New(0, cur)
		if st.Remaining.IsNegative() {
			over = st.Remaining.Abs()
		}
		pairs := []struct{ suffix, val, raw string }{
			{"limit", fmtMoney(limit), money.FormatMinor(limit.Amount, dec)},
			{"spent", fmtMoney(st.Spent), money.FormatMinor(st.Spent.Amount, dec)},
			{"remaining", fmtMoney(st.Remaining), money.FormatMinor(st.Remaining.Amount, dec)},
			{"over", fmtMoney(over), money.FormatMinor(over.Amount, dec)},
			{"percent", strconv.Itoa(st.Percent) + "%", strconv.Itoa(st.Percent)},
		}
		for _, p := range pairs {
			rows = append(rows, ui.CreateElement(budgetFormulaRow, budgetFormulaRowProps{Name: prefix + p.suffix, Value: p.val, Raw: p.raw}))
		}
	}
	listArgs := []any{css.Class("budget-formulas"), Attr("data-testid", "budget-formulas")}
	listArgs = append(listArgs, rows...)
	return Div(css.Class("acct-edit-form"),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.25rem"}), uistate.T("budgets.formulasHint")),
			If(!haveStatus, P(css.Class("empty"), uistate.T("common.notReady"))),
			Div(listArgs...),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.done")),
		),
	)
}

// budgetCoverSource is one budget offered as a funding source in the cover editor:
// its id, a display label, its remaining room, and its base limit (used to validate a
// share can't zero it out).
type budgetCoverSource struct {
	ID          string
	Label       string
	RemainMinor int64
	LimitMinor  int64
	// C606: the three honest figures, resolved by budgeting.SourceFundsOf from
	// the same model the budget's card shows. MovableMinor is what can actually
	// be transferred; CommittedMinor is the part of that already claimed by this
	// period's recurring charges; FreeMinor is what is left with no consequence.
	// The list used to display LimitMinor under the word "available" — the size
	// of the plan, most of which had already been spent.
	MovableMinor   int64
	CommittedMinor int64
	FreeMinor      int64
}

// coverMaxGive is the most a source can contribute: what budgeting.SourceFundsOf
// resolved as movable — its remaining room, capped one cent below its limit so
// the cover leaves it a positive one.
func coverMaxGive(sc budgetCoverSource) int64 { return sc.MovableMinor }

// committedCaveat names the part of a source's movable money that this period's
// recurring charges have already claimed (C606), or "" when nothing is.
//
// It is a caveat rather than a cap: the money IS movable and the user may have a
// good reason. What must not happen is moving it without being told, which is
// what "$59.16 available" said when only $20.16 was actually free.
func committedCaveat(sc budgetCoverSource, cur string) string {
	if sc.CommittedMinor <= 0 {
		return ""
	}
	return uistate.T("budgets.coverCommittedNote",
		fmtMoney(money.New(sc.CommittedMinor, cur)), fmtMoney(money.New(sc.FreeMinor, cur)))
}

func budgetRecommendationLine(app *appstate.App, b domain.Budget, status budgeting.Status, anchor time.Time) ui.Node {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	projected, err := budgeting.PeriodEquivalent(status.Projection.Projected, b.Period, domain.PeriodMonthly)
	if err != nil {
		return P(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "budget-limit-recommendation-incomplete"), uistate.T("budgets.recommendationIncomplete"))
	}
	projected, err = rates.Convert(projected, base)
	if err != nil {
		return P(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "budget-limit-recommendation-incomplete"), uistate.T("budgets.recommendationIncomplete"))
	}
	covers := make(map[string]bool)
	for _, id := range b.TrackedCategoryIDs() {
		covers[id] = true
	}
	recommendation, err := budgeting.RecommendBudgetLimit(b, covers, app.Transactions(), anchor, 6, projected, rates)
	if err != nil {
		return P(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "budget-limit-recommendation-incomplete"), uistate.T("budgets.recommendationIncomplete"))
	}
	if !recommendation.HasEvidence {
		return Fragment()
	}
	suggestedPeriod, err := budgeting.PeriodEquivalent(money.New(recommendation.Suggested, base), domain.PeriodMonthly, b.Period)
	if err != nil {
		return P(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "budget-limit-recommendation-incomplete"), uistate.T("budgets.recommendationIncomplete"))
	}
	suggestedPeriod, err = rates.Convert(suggestedPeriod, b.Limit.Currency)
	if err != nil {
		return P(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "budget-limit-recommendation-incomplete"), uistate.T("budgets.recommendationIncomplete"))
	}
	return P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "budget-limit-recommendation"),
		budgetRecommendationCopy(recommendation.Direction, fmtMoney(suggestedPeriod), len(recommendation.MonthsUsed)+len(recommendation.Outliers), len(recommendation.Outliers)))
}

// sourceAfterGiving is what a source would have left once its share is taken —
// the post-move balance the funding list never showed.
func sourceAfterGiving(sc budgetCoverSource, shareMinor int64, cur string) string {
	if shareMinor <= 0 {
		return ""
	}
	left := sc.MovableMinor - shareMinor
	if left < 0 {
		left = 0
	}
	return uistate.T("budgets.coverAfterNote", fmtMoney(money.New(left, cur)))
}

// splitCoverAmount divides totalMinor across the checked sources, never asking
// any of them for more than it can actually give (C606).
//
// The arithmetic — capping, redistributing what a capped source could not take,
// and reporting a shortfall — lives in budgeting.CoverSplit, where it is
// table-driven-tested. This is the adapter: it turns the checked rows into that
// input and drops the shortfall, which callers surface separately.
//
// Sources in `maxed` are pinned to their full movable amount and give it first,
// so checking "use all it has" on one source back-fills the others.
func splitCoverAmount(totalMinor int64, srcs []budgetCoverSource, sel map[string]bool, weights map[string]int, maxed map[string]bool) map[string]int64 {
	shares, _ := splitCoverAmountShort(totalMinor, srcs, sel, weights, maxed)
	return shares
}

// splitCoverAmountShort is splitCoverAmount plus the amount the selected sources
// could NOT fund, so a caller can refuse rather than quietly move less than was
// asked for.
func splitCoverAmountShort(totalMinor int64, srcs []budgetCoverSource, sel map[string]bool, weights map[string]int, maxed map[string]bool) (map[string]int64, int64) {
	if totalMinor <= 0 {
		return nil, 0
	}
	var in []budgeting.CoverSplitInput
	for _, sc := range srcs {
		if !sel[sc.ID] {
			continue
		}
		in = append(in, budgeting.CoverSplitInput{
			ID: sc.ID, Cap: coverMaxGive(sc), Weight: weights[sc.ID], Pinned: maxed[sc.ID],
		})
	}
	if len(in) == 0 {
		return nil, totalMinor
	}
	return budgeting.CoverSplit(totalMinor, in)
}

// coverSourceRowProps drives one row of the cover editor's source list. On* handlers
// live here (not in the parent's map loop) per the framework's no-hooks-in-loops rule.
type coverSourceRowProps struct {
	ID        string
	Label     string
	RemainStr string
	Selected  bool
	Weight    int
	ShareStr  string // formatted amount this source contributes (when selected)
	Over      bool   // this source's share exceeds what it can give
	AvailStr  string // formatted amount this source can actually give (C606)
	// CommittedStr names the part of the movable money this period's recurring
	// charges have already claimed, e.g. "$39.00 of that is already committed".
	// Empty when nothing is committed. Moving it is allowed — it is the user's
	// money — but doing it unknowingly is how a bill stops fitting (C606).
	CommittedStr string
	// AfterStr is what would be left in this source once its share is taken, so
	// the consequence is on screen before the move, not after it.
	AfterStr     string
	Max          bool // "use all remaining" — pin this source to its full remaining
	WeightLocked bool // ratio is driven by a formula → the number input is read-only
	OnToggle     func(string)
	OnWeight     func(string, int)
	OnToggleMax  func(string)
}

// coverSourceRow renders a styled checkbox to include a source budget, and — when
// checked — a ratio input, a "use all remaining" toggle, and the live amount it yields.
// Checking "use all remaining" pins the source to its full remaining (the ratio input
// is then disabled) and back-fills the other selected sources' ratios; an over-limit
// share turns amber with an "only $X available" note.
func coverSourceRow(props coverSourceRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	onWeight := ui.UseEvent(func(v string) { n, _ := strconv.Atoi(strings.TrimSpace(v)); props.OnWeight(props.ID, n) })
	onMax := ui.UseEvent(func() { props.OnToggleMax(props.ID) })
	w := props.Weight
	if props.Selected && w <= 0 {
		w = 1
	}
	rowCls := "cover-src-row"
	if props.Selected {
		rowCls += " is-checked"
	}
	shareCls := "cover-src-share"
	if props.Over {
		shareCls += " is-over"
	}
	weightAttrs := []any{css.Class("field", "cover-src-weight"), Type("number"), Attr("min", "0"),
		Attr("aria-label", uistate.T("budgets.coverRatio")), Step("1"), OnInput(onWeight)}
	if props.Max || props.WeightLocked {
		// Pinned to full remaining, or driven by the shared ratio formula — either way the
		// number isn't hand-edited here.
		weightAttrs = append(weightAttrs, Attr("disabled", "disabled"))
	}
	return Div(css.Class(rowCls),
		Label(css.Class("cover-src-main"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "cover-src-"+props.ID), OnChange(onToggle)}, checkedAttr(props.Selected)...)...),
			Span(css.Class("cover-src-name"), props.Label),
			// C606: the figure is what can actually be MOVED, not the budget's
			// limit — and it says so, because "left" beside a limit was the lie.
			Span(css.Class("cover-src-remain"), Attr("data-testid", "cover-src-avail-"+props.ID),
				uistate.T("budgets.coverMovable", props.RemainStr)),
		),
		If(props.CommittedStr != "", Span(css.Class("cover-src-committed"),
			Attr("data-testid", "cover-src-committed-"+props.ID), props.CommittedStr)),
		If(props.Selected, Div(css.Class("cover-src-ratio"),
			Span(css.Class("cover-src-ratio-label"), uistate.T("budgets.coverRatio")),
			Input(append(weightAttrs, uiw.FieldValue(strconv.Itoa(w)))...),
			Label(css.Class("cover-src-maxlabel"), Title(uistate.T("budgets.coverUseAllTitle")),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "cover-max-"+props.ID), OnChange(onMax)}, checkedAttr(props.Max)...)...),
				Span(uistate.T("budgets.coverUseAll")),
			),
			Div(css.Class("cover-src-shares"),
				Span(ClassStr(shareCls), If(props.Over, Span(css.Class("cover-src-warn"), "⚠ ")), props.ShareStr),
				If(props.Over, Span(css.Class("cover-src-avail"), uistate.T("budgets.coverOnlyAvail", props.AvailStr))),
				// The post-move balance: the consequence, before the move (C606).
				If(!props.Over && props.AfterStr != "", Span(css.Class("cover-src-after"),
					Attr("data-testid", "cover-src-after-"+props.ID), props.AfterStr)),
			),
		)),
	)
}

// buildCoverContext assembles the coverformula evaluation context (the global engine
// surface + the live data) for the cover modal's amount/weight formula previews. Same
// shape the recurring boot apply builds.
func buildCoverContext(app *appstate.App, weekStart time.Weekday, payCycleAnchor time.Time) coverformula.Context {
	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	ms, me := dateutil.MonthRange(now)
	surface := engineenv.Vars(engineenv.Data{
		Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
		Rates: rates, Now: now, PeriodStart: ms, PeriodEnd: me, WeekStart: weekStart,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
	})
	return coverformula.Context{
		Base: surface, Txns: app.Transactions(), Rates: rates, Now: now,
		WeekStart: weekStart, PayCycleAnchor: payCycleAnchor, Defs: app.CustomFieldDefs(),
	}
}

// recurringAmountFormula returns the budget's stored recurring cover amount formula
// (empty when none).
func recurringAmountFormula(b domain.Budget) string {
	if b.RecurringCover != nil {
		return b.RecurringCover.AmountFormula
	}
	return ""
}

// coverCapClass hides the "Selected …" caption when nothing is picked (it stays in the
// DOM so toggling it never restructures the source list's siblings).
func coverCapClass(selCount int) string {
	if selCount == 0 {
		return "cover-selected-cap is-empty"
	}
	return "cover-selected-cap"
}

// coverCapText is the running "Selected N · splitting $X" caption (empty when nothing
// is picked, since the span is then hidden).
func coverCapText(selCount int, totalMinor int64, cur string) string {
	if selCount == 0 {
		return ""
	}
	return uistate.T("budgets.coverSelectedCap", selCount, fmtMoney(money.New(totalMinor, cur)))
}

// recurringCoverCustom returns the custom-field values stored on a budget's recurring
// cover rule (nil when there's no rule).
func recurringCoverCustom(b domain.Budget) map[string]any {
	if b.RecurringCover == nil {
		return nil
	}
	return b.RecurringCover.Custom
}

// recurringWeightFormula returns the shared per-source weight formula stored on a
// recurring cover (empty when none). All sources carry the same formula, so the first
// non-empty one is the modal-level formula to restore.
func recurringWeightFormula(b domain.Budget) string {
	if b.RecurringCover == nil {
		return ""
	}
	for _, s := range b.RecurringCover.Sources {
		if s.WeightFormula != "" {
			return s.WeightFormula
		}
	}
	return ""
}

// resolveCoverWeights returns the effective per-source ratio weight for each selected
// source. When wtFormula is set, each source's weight is that formula evaluated in the
// source budget's own context (e.g. "cf_budget_priority" weights sources by a custom
// priority field); otherwise the fixed ratio inputs are used. The second return is the
// first formula error encountered (for the live preview), if any.
func resolveCoverWeights(app *appstate.App, ctx coverformula.Context, srcs []budgetCoverSource, sel map[string]bool, fixed map[string]int, wtFormula string) (map[string]int, string) {
	out := make(map[string]int, len(srcs))
	formula := strings.TrimSpace(wtFormula)
	var byID map[string]domain.Budget
	if formula != "" {
		byID = make(map[string]domain.Budget, len(app.Budgets()))
		for _, bb := range app.Budgets() {
			byID[bb.ID] = bb
		}
	}
	errStr := ""
	for _, sc := range srcs {
		if !sel[sc.ID] {
			continue
		}
		if formula != "" {
			w, err := ctx.Weight(formula, byID[sc.ID])
			if err != nil {
				if errStr == "" {
					errStr = err.Error()
				}
				out[sc.ID] = 0
				continue
			}
			out[sc.ID] = w
			continue
		}
		w := fixed[sc.ID]
		if w <= 0 {
			w = 1
		}
		out[sc.ID] = w
	}
	return out, errStr
}

// budgetOwnerScopeHint spells out the owner picker's silent scope consequence (C5):
// group = shared (counts everyone's spending), a member = individual (theirs only).
func budgetOwnerScopeHint(members []domain.Member, ownerID string) string {
	if ownerID == domain.GroupOwnerID {
		return uistate.T("budgets.ownerSharedHint")
	}
	name := ""
	for _, m := range members {
		if m.ID == ownerID {
			name = m.Name
			break
		}
	}
	return uistate.T("budgets.ownerIndividualHint", name)
}

// budgetCategoryName resolves a category's display name (for the top-up hint), or ""
// when the budget has no category or it can't be found.
func budgetCategoryName(app *appstate.App, categoryID string) string {
	if app == nil || categoryID == "" {
		return ""
	}
	for _, c := range app.Categories() {
		if c.ID == categoryID {
			return c.Name
		}
	}
	return ""
}

// rolloverCapOptions builds the BG5 rollover-cap picker options: no cap (the default,
// value "0"), or a ceiling of 1×/2×/3× the period limit on how much surplus can carry
// forward. Values are the RolloverCapPeriods integer as a string.
func rolloverCapOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "0", Label: uistate.T("budgets.rolloverCapNone")},
		{Value: "1", Label: uistate.T("budgets.rolloverCap1x")},
		{Value: "2", Label: uistate.T("budgets.rolloverCap2x")},
		{Value: "3", Label: uistate.T("budgets.rolloverCap3x")},
	}
}

// budgetEditTopupImpact supplies the shared consequence wording for the raise-limit form.
func budgetEditTopupImpact(permanent, funded bool) string {
	reach := fundsReachThisPeriod
	if permanent {
		reach = fundsReachPermanent
	}
	return fundsImpactLine(budgeting.TopUpImpact(permanent, funded), fundsImpactOption{
		reach: reach, movesFromSelectedBudgets: funded,
	})
}
