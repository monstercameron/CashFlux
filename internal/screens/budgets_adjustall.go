// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strconv"
	"strings"
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// AdjustAllBody is the "Adjust all" form (C592), mounted at the shell root by
// app.AdjustAllHost.
//
// It replaces a bare `PromptModal` — an unlabelled dialog whose entire content
// was the sentence "Adjust every budget's limit by what percent? (e.g. 5 or
// -10)", followed by a confirm quoting the number back. Nothing in that flow said
// how many budgets were in scope, what the household's total was, or what it
// would become; the user typed a number into the dark and found out afterwards.
//
// This form has a heading, a labelled numeric field with its bounds stated,
// inline rejection of blank/invalid/extreme values, a live before→after preview
// of the total AND of every affected budget, and an explicit acknowledgement for
// the changes that take money out of every plan at once. Nothing is written until
// Apply.
func AdjustAllBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetAdjustOpen()
	activeMemberID := uistate.UseActiveMember().Get()

	// C587: opened from "bring the plan down to what has arrived", the reduction
	// is already known — the form starts with it filled in and previewed, rather
	// than asking the user to compute a percentage from two figures.
	//
	// An action can seed a scope when it has made a specific promise. An ordinary
	// Adjust-all open defaults to this period so past limits are safe.
	seedPct, seedScope := uistate.TakeBudgetAdjustSeed()
	pctStr := ui.UseState(seedPct)
	onPct := ui.UseEvent(func(v string) { pctStr.Set(v) })
	scopeS := ui.UseState(defaultAdjustScope(seedScope))
	ack := ui.UseState(false)
	toggleAck := ui.UseEvent(func() { ack.Set(!ack.Get()) })
	ackOnChange := OnChange(toggleAck)
	// The scope is deliberately NOT reset here. AdjustAllHost renders nothing while
	// closed, so this body remounts on every open and re-reads the seed — and if
	// that ever stopped being true, a reset here would overwrite the scope the next
	// reconcile hands over, which is the exact defect C671 is about.
	close := func() {
		pctStr.Set("")
		ack.Set(false)
		openAtom.Set(false)
	}
	onCancel := ui.UseEvent(Prevent(func() { close() }))

	// The budgets in scope: the ones visible under the current member perspective,
	// which is the set the page is showing. Scoping to what is on screen is what
	// makes "every budget" a claim the user can check.
	scoped := adjustAllScope(app, activeMemberID)

	scope := budgeting.AdjustScope(scopeS.Get())
	if !scope.Valid() {
		scope = budgeting.AdjustEveryPeriod
	}
	impactReach := fundsReachThisPeriod
	if scope.IsPermanent() {
		impactReach = fundsReachPermanent
	}
	pr := uistate.UsePrefs().Get()
	vw := uistate.UsePeriod().Get()
	// Each budget's current period start, for a this-period write — the shared
	// resolver, so the period this form writes to is the one every other caller
	// means by "this period".
	periodStart := budgetPeriodStarts(vw, pr)
	// Whether the window being viewed has already closed — the same test the
	// budgets hero uses to decide between "hasn't arrived yet" and "never arrived".
	_, viewedEnd := vw.Range()
	viewedClosed := adjustAllPeriodClosed(viewedEnd, time.Now())
	// How far each budget's cap sits off its stored limit this period. Needed
	// under BOTH scopes: a this-period write moves the cap directly, and a
	// permanent one has to check that the limit it writes still clears whatever
	// one-off change is already recorded on top of it.
	var overlays map[string]int64
	if app != nil {
		overlays = budgetAdjustOverlays(computeBudgetView(app, activeMemberID, vw, pr, false), scoped, periodStart)
	}

	raw := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(pctStr.Get()), "%"))
	pct, parseErr := strconv.ParseFloat(raw, 64)
	valid := raw != "" && parseErr == nil && budgeting.ValidAdjustPct(pct)
	preview := budgeting.AdjustPreview{}
	var normalizationErr error
	if valid {
		// Previewed on the basis the chosen scope will write, never on the other
		// one — that mismatch is what made a this-period preview a promise the
		// write could not keep.
		base := budgetBaseCurrency("")
		fx := currency.Rates{Base: base}
		if app != nil && app.Settings().BaseCurrency != "" {
			base = app.Settings().BaseCurrency
			fx = currency.Rates{Base: base, Rates: app.Settings().FXRates}
		}
		preview, normalizationErr = budgeting.AdjustAllPreviewNormalized(scoped, pct, scope, overlays, domain.PeriodMonthly,
			fx)
	}
	normalizedDelta := money.Zero(preview.NormalizedAfter.Currency)
	var normalizedDeltaErr error
	if normalizationErr == nil && preview.Error == nil {
		normalizedDelta, normalizedDeltaErr = preview.NormalizedAfter.Sub(preview.NormalizedBefore)
	}
	// C671: a permanent adjustment is always acknowledged, whatever its size. The
	// old rule looked only at magnitude, so a 5% cut that reached every future
	// period passed without a word while a 30% one that did the same was stopped.
	needsAck := valid && budgeting.AdjustNeedsAck(pct, scope)
	canApply := valid && normalizationErr == nil && preview.Error == nil && normalizedDeltaErr == nil && preview.Count() > 0 && (!needsAck || ack.Get())

	apply := ui.UseEvent(Prevent(func() {
		if !canApply || app == nil {
			return
		}
		writes, err := budgeting.ApplyAdjustChecked(preview, scope, periodStart)
		if err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
			return
		}
		if err := app.PutBudgetsAtomicAt(writes, vw.From); err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
			return
		}
		n := len(writes)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostUndoable(uistate.T(adjustAppliedKey(scope), budgetCountPhrase(n), formatAdjustPctCopy(pct)))
		close()
	}))

	inlineErr := adjustAllInlineError(raw, parseErr, pct, normalizationErr, preview, normalizedDeltaErr)

	base := budgetBaseCurrency("")
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	previewNode := adjustAllPreviewNode(app, preview, pct, valid, base, normalizationErr, normalizedDeltaErr, normalizedDelta)

	skipNode := adjustAllSkippedNode(app, preview, valid)
	ackNode := adjustAllAckNode(needsAck, scope, pct, preview.Count(), ack.Get(), ackOnChange)

	return adjustAllFormNode(adjustAllFormProps{
		Scope: scope, PeriodLabel: vw.FromLabel(), ViewedClosed: viewedClosed, PctText: pctStr.Get(), InlineError: inlineErr,
		CanApply: canApply, Preview: previewNode, Skipped: skipNode, Ack: ackNode,
		ImpactHint: fundsImpactLine(budgeting.ImpactAdjustAll, fundsImpactOption{reach: impactReach}),
		OnPct:      onPct, OnScope: func(value string) { scopeS.Set(value); ack.Set(false) }, OnCancel: onCancel, OnApply: apply,
	})
}

func adjustAllPreviewNode(app *appstate.App, preview budgeting.AdjustPreview, pct float64, valid bool, base string, normalizationErr, normalizedDeltaErr error, normalizedDelta money.Money) ui.Node {
	if !valid || preview.Count() == 0 {
		return Fragment()
	}
	rows := MapKeyed(preview.Lines,
		func(l budgeting.AdjustLine) any { return l.Budget.ID },
		func(l budgeting.AdjustLine) ui.Node {
			return Div(css.Class("adjustall-row"), Attr("data-testid", "adjustall-row-"+l.Budget.ID),
				Span(css.Class("adjustall-name"), budgetTitle(l.Budget.Name, budgetCategoryName(app, l.Budget.CategoryID))),
				Span(css.Class("adjustall-before", tw.TextFaint), fmtMoney(money.New(l.Before, l.Budget.Limit.Currency))),
				Span(css.Class("adjustall-arrow", tw.TextFaint), "→"),
				Span(css.Class("adjustall-after"), fmtMoney(money.New(l.After, l.Budget.Limit.Currency))))
		})
	var totalNode ui.Node = Fragment()
	if normalizationErr == nil && preview.Error == nil && normalizedDeltaErr == nil {
		delta := normalizedDelta.Amount
		totalNode = Div(css.Class("adjustall-total"), Attr("data-testid", "adjustall-total"),
			Span(css.Class("adjustall-total-lbl"), uistate.T("budgets.adjustAllTotalLabel")),
			Span(css.Class("adjustall-before", tw.TextFaint), fmtMoney(preview.NormalizedBefore)),
			Span(css.Class("adjustall-arrow", tw.TextFaint), "→"),
			Span(css.Class("adjustall-after", tw.FontDisplay), fmtMoney(preview.NormalizedAfter)),
			Span(ClassStr("adjustall-delta"+adjustDeltaTone(delta)), uistate.T(adjustDeltaKey(delta), fmtMoney(money.New(absMinor(delta), base)))))
	} else if preview.MixedCurrency {
		totalNode = P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-mixed"), uistate.T("budgets.adjustAllMixedCurrency"))
	} else {
		totalNode = P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-error"), uistate.T("budgets.incomplete"))
	}
	var normalizedSummary, normalizedLines ui.Node = Fragment(), Fragment()
	if normalizationErr == nil && preview.Error == nil {
		normalizedSummary = P(css.Class("adjustall-normalized"), Attr("data-testid", "adjustall-normalized"),
			uistate.T("budq4.adjustNormalized", fmtMoney(preview.NormalizedBefore), fmtMoney(preview.NormalizedAfter)))
	}
	if len(preview.NormalizedLines) > 0 {
		normalizedLines = Div(css.Class("adjustall-normalized-lines"), Attr("data-testid", "adjustall-normalized-lines"),
			MapKeyed(preview.NormalizedLines, func(line budgeting.AdjustNormalizedLine) any { return line.BudgetID },
				func(line budgeting.AdjustNormalizedLine) ui.Node {
					return P(css.Class("budget-owner-hint", tw.TextFaint), line.Name, ": ", fmtMoney(line.Before), " → ", fmtMoney(line.After))
				}))
	}
	return Div(css.Class("adjustall-preview"), Attr("data-testid", "adjustall-preview"),
		P(css.Class("adjustall-count"), Attr("data-testid", "adjustall-count"), uistate.T("budgets.adjustAllCount", budgetCountPhrase(preview.Count()), formatAdjustPctCopy(pct))),
		totalNode, normalizedSummary, normalizedLines, Div(css.Class("adjustall-rows"), Attr("data-testid", "adjustall-rows"), rows))
}

func adjustAllSkippedNode(app *appstate.App, preview budgeting.AdjustPreview, valid bool) ui.Node {
	if !valid {
		return Fragment()
	}
	var lines []ui.Node
	for _, reason := range []budgeting.SkipReason{budgeting.SkipWouldInvert, budgeting.SkipNothingToScale, budgeting.SkipUnknownOverlay} {
		skipped := preview.SkippedFor(reason)
		if len(skipped) == 0 {
			continue
		}
		names := make([]string, 0, len(skipped))
		for _, s := range skipped {
			names = append(names, budgetTitle(s.Budget.Name, budgetCategoryName(app, s.Budget.CategoryID)))
		}
		lines = append(lines, P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-skipped-"+string(reason)),
			uistate.T(adjustSkipKey(reason), budgetCountPhrase(len(skipped)), strings.Join(names, ", "))))
	}
	if len(lines) == 0 {
		return Fragment()
	}
	return Div(css.Class("adjustall-skipped"), Attr("data-testid", "adjustall-skipped"), lines)
}

func adjustAllAckNode(needsAck bool, scope budgeting.AdjustScope, pct float64, count int, ack bool, ackOnChange any) ui.Node {
	if !needsAck {
		return Fragment()
	}
	key := "budgets.adjustAllAckRaise"
	switch {
	case scope.IsPermanent() && pct < 0:
		key = "budgets.adjustAllAckFutureLower"
	case scope.IsPermanent():
		key = "budgets.adjustAllAckFutureRaise"
	case pct < 0:
		key = "budgets.adjustAllAckLower"
	}
	return Label(css.Class("adjustall-ack"), Attr("data-testid", "adjustall-ack-label"),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "adjustall-ack"), Attr("style", "flex-shrink:0"), ackOnChange}, checkedAttr(ack)...)...),
		Span(uistate.T(key, budgetCountPhrase(count), formatAdjustPctCopy(math.Abs(pct)))))
}

func adjustAllPeriodClosed(end, now time.Time) bool {
	return !dateutil.WallClockUTC(now).Before(end)
}

func budgetCountPhrase(count int) string {
	if count == 1 {
		return uistate.T("budgets.countOne")
	}
	return uistate.T("budgets.countMany", count)
}

type adjustAllFormProps struct {
	Scope        budgeting.AdjustScope
	PeriodLabel  string
	ViewedClosed bool
	PctText      string
	InlineError  string
	CanApply     bool
	Preview      ui.Node
	Skipped      ui.Node
	Ack          ui.Node
	ImpactHint   string
	OnPct        any
	OnScope      func(string)
	OnCancel     any
	OnApply      any
}

func adjustAllFormNode(props adjustAllFormProps) ui.Node {
	return Form(css.Class("acct-edit-form"), Attr("data-testid", "adjustall-form"), OnSubmit(props.OnApply),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("budgets.adjustAllFieldLabel"), Fragment(
				Div(css.Class("adjustall-field"),
					Input(css.Class("field"), Type("number"), Attr("data-testid", "adjustall-pct"), Attr("autofocus", ""), Attr("inputmode", "decimal"),
						Attr("aria-label", uistate.T("budgets.adjustAllFieldLabel")), Attr("min", formatAdjustPct(budgeting.AdjustMinPct)),
						Attr("max", formatAdjustPct(budgeting.AdjustMaxPct)), Step("0.5"), Placeholder("5"), OnInput(props.OnPct), uiw.FieldValue(props.PctText)),
					Span(css.Class("adjustall-suffix", tw.TextFaint), "%")),
				Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "adjustall-hint"),
					uistate.T("budgets.adjustAllFieldHint", formatAdjustPctCopy(budgeting.AdjustMinPct), formatAdjustPctCopy(budgeting.AdjustMaxPct))))),
			labeledField(uistate.T("bv6g.adjust.scopeLabel"), uiw.Segmented(uiw.SegmentedProps{
				Label: uistate.T("bv6g.adjust.scopeLabel"), Selected: string(props.Scope),
				Options: []uiw.SegOption{
					{Value: string(budgeting.AdjustThisPeriod), Label: uistate.T("bv6g.adjust.thisPeriod"), TestID: "adjustall-scope-this"},
					{Value: string(budgeting.AdjustEveryPeriod), Label: uistate.T("bv6g.adjust.allPeriods"), TestID: "adjustall-scope-every"},
				}, OnSelect: props.OnScope,
			})),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "adjustall-scope-hint"), Style(map[string]string{"margin": "0"}), adjustScopeDescription(props.Scope)),
			If(props.InlineError != "", P(ClassStr("form-err"), Attr("data-testid", "adjustall-err"), Attr("role", "alert"), props.InlineError)),
			props.Preview, props.Skipped, props.Ack),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "adjustall-cancel"), OnClick(props.OnCancel), uistate.T("action.cancel")),
			buttonWithDisabled(!props.CanApply, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "adjustall-apply")}, uistate.T(adjustApplyKey(props.Scope)))))
}

func adjustAllInlineError(raw string, parseErr error, pct float64, normalizationErr error, preview budgeting.AdjustPreview, deltaErr error) string {
	switch {
	case raw == "":
		return ""
	case parseErr != nil:
		return uistate.T("budgets.adjustAllNotANumber")
	case pct == 0:
		return uistate.T("budgets.adjustAllZero")
	case !budgeting.ValidAdjustPct(pct):
		return uistate.T("budgets.adjustAllOutOfRange", formatAdjustPctCopy(budgeting.AdjustMinPct), formatAdjustPctCopy(budgeting.AdjustMaxPct))
	case normalizationErr != nil || preview.Error != nil || deltaErr != nil:
		return uistate.T("budgets.incomplete")
	case preview.Count() == 0:
		return uistate.T("budgets.adjustAllNothingToChange")
	default:
		return ""
	}
}

// defaultAdjustScope resolves the scope the form opens on: whatever the action
// that opened it promised, or this period when it promised nothing.
//
// The seed exists so a caller cannot advertise one reach and open a form set to
// another — the C671 defect in one sentence.
func defaultAdjustScope(seed string) string {
	if s := budgeting.AdjustScope(seed); s.Valid() {
		return string(s)
	}
	return string(budgeting.AdjustThisPeriod)
}

// adjustEveryPeriodLabel makes the effect on already-closed periods explicit.
func adjustEveryPeriodLabel() string { return uistate.T("budgets.adjustAllScopeEveryPast") }

// adjustIntroKey, adjustScopeHintKey, adjustApplyKey and adjustAppliedKey name
// the copy for each scope: what the form does, what the control will do, what the
// commit is called, and what the undo banner says it did. Four moments, one fact —
// which number changes and for how long.
// adjustSkipKey names the sentence that explains why some budgets were left out
// of a bulk adjustment. Each reason gets its own, because "left out" is the only
// thing they share — one is an empty plan, one is a cap this session could not
// resolve, and one is a budget a change made earlier in this same period would
// push below zero, which is the only one that needs an action from the reader.
func adjustSkipKey(reason budgeting.SkipReason) string {
	switch reason {
	case budgeting.SkipWouldInvert:
		return "budgets.adjustAllSkipInvert"
	case budgeting.SkipUnknownOverlay:
		return "budgets.adjustAllSkipUnknownOverlay"
	default:
		return "budgets.adjustAllSkipEmpty"
	}
}

// adjustPeriodKey and adjustPeriodClass name and tone the line that says WHICH
// period a this-period change lands on. A closed period gets warn tone and a
// sentence about the consequence, because editing a month that is over is a
// different act from correcting the one you are in.
func adjustPeriodKey(closed bool) string {
	if closed {
		return "budgets.adjustAllPeriodPast"
	}
	return "budgets.adjustAllPeriodLive"
}

func adjustPeriodClass(closed bool) string {
	if closed {
		return "t-caption " + tw.ColorClass("text-warn")
	}
	return "t-caption " + tw.ColorClass("text-dim")
}

func adjustIntroKey(scope budgeting.AdjustScope) string {
	if scope.IsPermanent() {
		return "budgets.adjustAllIntro"
	}
	return "budgets.adjustAllIntroThis"
}

func adjustScopeHintKey(scope budgeting.AdjustScope) string {
	if scope.IsPermanent() {
		return "budgets.adjustAllScopeEveryHint"
	}
	return "budgets.adjustAllScopeThisHint"
}

func adjustApplyKey(scope budgeting.AdjustScope) string {
	if scope.IsPermanent() {
		return "budgets.adjustAllApplyEvery"
	}
	return "budgets.adjustAllApplyThis"
}

func adjustAppliedKey(scope budgeting.AdjustScope) string {
	if scope.IsPermanent() {
		return "budgets.adjustAllAppliedEvery"
	}
	return "budgets.adjustAllAppliedThis"
}

// formatAdjustPct renders a percentage without a trailing ".0", so the copy reads
// "5%" and "-12.5%" rather than "5.000000%".
func formatAdjustPct(pct float64) string {
	return strconv.FormatFloat(pct, 'f', -1, 64)
}

func formatAdjustPctCopy(pct float64) string {
	value := formatAdjustPct(pct)
	if strings.HasPrefix(value, "-") {
		return "−" + strings.TrimPrefix(value, "-")
	}
	return value
}

func adjustScopeDescription(scope budgeting.AdjustScope) string {
	if scope.IsPermanent() {
		return uistate.T("bv6g.adjust.allHint")
	}
	return uistate.T("bv6g.adjust.thisHint")
}

// adjustDeltaKey / adjustDeltaTone name and tone the change to the total. A bulk
// lower reduces what the household plans to spend, which is neither good nor bad
// on its own — so the tone marks direction, not health.
func adjustDeltaKey(delta int64) string {
	if delta < 0 {
		return "budgets.adjustAllDeltaDown"
	}
	return "budgets.adjustAllDeltaUp"
}

func adjustDeltaTone(delta int64) string {
	if delta < 0 {
		return " is-down"
	}
	return " is-up"
}
