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
	viewedClosed := !time.Now().Before(viewedEnd)
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
		uistate.PostUndoable(uistate.T(adjustAppliedKey(scope), plural(n, "budget"), formatAdjustPct(pct)))
		close()
	}))

	// --- what's wrong with the typed value, in the words that fix it -----------
	inlineErr := ""
	switch {
	case raw == "":
		// Not an error yet — the field is simply empty. The hint below carries it.
	case parseErr != nil:
		inlineErr = uistate.T("budgets.adjustAllNotANumber")
	case pct == 0:
		inlineErr = uistate.T("budgets.adjustAllZero")
	case !budgeting.ValidAdjustPct(pct):
		inlineErr = uistate.T("budgets.adjustAllOutOfRange",
			formatAdjustPct(budgeting.AdjustMinPct), formatAdjustPct(budgeting.AdjustMaxPct))
	case normalizationErr != nil:
		inlineErr = uistate.T("budgets.incomplete")
	case preview.Error != nil:
		inlineErr = uistate.T("budgets.incomplete")
	case normalizedDeltaErr != nil:
		inlineErr = uistate.T("budgets.incomplete")
	case preview.Count() == 0:
		inlineErr = uistate.T("budgets.adjustAllNothingToChange")
	}

	base := budgetBaseCurrency("")
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	cur := preview.Currency
	if cur == "" {
		cur = base
	}

	// --- the preview ----------------------------------------------------------
	var previewNode ui.Node = Fragment()
	if valid && preview.Count() > 0 {
		rows := MapKeyed(preview.Lines,
			func(l budgeting.AdjustLine) any { return l.Budget.ID },
			func(l budgeting.AdjustLine) ui.Node {
				return Div(css.Class("adjustall-row"), Attr("data-testid", "adjustall-row-"+l.Budget.ID),
					Span(css.Class("adjustall-name"), budgetTitle(l.Budget.Name, budgetCategoryName(app, l.Budget.CategoryID))),
					Span(css.Class("adjustall-before", tw.TextFaint), fmtMoney(money.New(l.Before, l.Budget.Limit.Currency))),
					Span(css.Class("adjustall-arrow", tw.TextFaint), "→"),
					Span(css.Class("adjustall-after"), fmtMoney(money.New(l.After, l.Budget.Limit.Currency))),
				)
			})
		var totalNode ui.Node = Fragment()
		if normalizationErr == nil && preview.Error == nil && normalizedDeltaErr == nil {
			delta := normalizedDelta.Amount
			totalNode = Div(css.Class("adjustall-total"), Attr("data-testid", "adjustall-total"),
				Span(css.Class("adjustall-total-lbl"), uistate.T("budgets.adjustAllTotalLabel")),
				Span(css.Class("adjustall-before", tw.TextFaint), fmtMoney(preview.NormalizedBefore)),
				Span(css.Class("adjustall-arrow", tw.TextFaint), "→"),
				Span(css.Class("adjustall-after", tw.FontDisplay), fmtMoney(preview.NormalizedAfter)),
				Span(ClassStr("adjustall-delta"+adjustDeltaTone(delta)),
					uistate.T(adjustDeltaKey(delta), fmtMoney(money.New(absMinor(delta), base)))),
			)
		} else if preview.MixedCurrency {
			totalNode = P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-mixed"),
				uistate.T("budgets.adjustAllMixedCurrency"))
		} else {
			totalNode = P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-error"),
				uistate.T("budgets.incomplete"))
		}
		var normalizedSummary ui.Node = Fragment()
		if normalizationErr == nil && preview.Error == nil {
			normalizedSummary = P(css.Class("adjustall-normalized"), Attr("data-testid", "adjustall-normalized"),
				uistate.T("budq4.adjustNormalized", fmtMoney(preview.NormalizedBefore), fmtMoney(preview.NormalizedAfter)))
		}
		var normalizedLines ui.Node = Fragment()
		if len(preview.NormalizedLines) > 0 {
			normalizedLines = Div(css.Class("adjustall-normalized-lines"), Attr("data-testid", "adjustall-normalized-lines"),
				MapKeyed(preview.NormalizedLines, func(line budgeting.AdjustNormalizedLine) any { return line.BudgetID }, func(line budgeting.AdjustNormalizedLine) ui.Node {
					return P(css.Class("budget-owner-hint", tw.TextFaint), line.Name, ": ",
						fmtMoney(line.Before), " → ", fmtMoney(line.After))
				}))
		}
		previewNode = Div(css.Class("adjustall-preview"), Attr("data-testid", "adjustall-preview"),
			P(css.Class("adjustall-count"), Attr("data-testid", "adjustall-count"),
				uistate.T("budgets.adjustAllCount", plural(preview.Count(), "budget"), formatAdjustPct(pct))),
			totalNode,
			normalizedSummary,
			normalizedLines,
			Div(css.Class("adjustall-rows"), Attr("data-testid", "adjustall-rows"), rows),
		)
	}

	// C671: what the adjustment is leaving out, and why. Skipping used to be
	// silent, so a form headed "every budget" could quietly show fewer rows than
	// the household has — and the inversion guard added here would have been the
	// most confusing silence of all: a budget vanishing from the list because a
	// change made earlier in the same period would put its plan below zero.
	var skipNode ui.Node = Fragment()
	if valid {
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
			lines = append(lines, P(css.Class("budget-cat-fate", tw.TextFaint),
				Attr("data-testid", "adjustall-skipped-"+string(reason)),
				uistate.T(adjustSkipKey(reason), plural(len(skipped), "budget"), strings.Join(names, ", "))))
		}
		if len(lines) > 0 {
			skipNode = Div(css.Class("adjustall-skipped"), Attr("data-testid", "adjustall-skipped"), lines)
		}
	}

	// --- the acknowledgement for consequential changes ------------------------
	var ackNode ui.Node = Fragment()
	if needsAck {
		// C671: when the change outlives this period, the sentence has to say so —
		// that is the fact the whole ticket is about, and "lower 12 budgets by 40%"
		// is true of both scopes while meaning very different things.
		key := "budgets.adjustAllAckRaise"
		switch {
		case scope.IsPermanent() && pct < 0:
			key = "budgets.adjustAllAckFutureLower"
		case scope.IsPermanent():
			key = "budgets.adjustAllAckFutureRaise"
		case pct < 0:
			key = "budgets.adjustAllAckLower"
		}
		// The verb ("lower") already carries the direction, so the figure beside it
		// is a magnitude: "lower 10 budgets by 10%", never "by -10%".
		ackNode = Label(css.Class("adjustall-ack"), Attr("data-testid", "adjustall-ack-label"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "adjustall-ack"),
				Attr("style", "flex-shrink:0"), OnChange(toggleAck)}, checkedAttr(ack.Get())...)...),
			Span(uistate.T(key, plural(preview.Count(), "budget"), formatAdjustPct(math.Abs(pct)))))
	}

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "adjustall-form"), OnSubmit(apply),
		Div(css.Class("modal-scroll"),
			// C671: the intro used to say "every budget's limit" whichever scope was
			// chosen, which is false of a this-period change — that one never touches
			// a limit. The sentence follows the scope.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "adjustall-intro"),
				Style(map[string]string{"margin": "0"}), uistate.T(adjustIntroKey(scope))),
			// C597: the same reach vocabulary the page's other funds-moving actions
			// use, so "Adjust all" can be compared with them rather than read on its
			// own terms.
			labeledField(uistate.T("budgets.adjustAllFieldLabel"),
				Fragment(
					Div(css.Class("adjustall-field"),
						Input(css.Class("field"), Type("number"), Attr("data-testid", "adjustall-pct"),
							Attr("autofocus", ""), Attr("inputmode", "decimal"),
							Attr("aria-label", uistate.T("budgets.adjustAllFieldLabel")),
							Attr("min", formatAdjustPct(budgeting.AdjustMinPct)),
							Attr("max", formatAdjustPct(budgeting.AdjustMaxPct)),
							Step("0.5"), Placeholder("5"), OnInput(onPct), uiw.FieldValue(pctStr.Get())),
						Span(css.Class("adjustall-suffix", tw.TextFaint), "%"),
					),
					Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "adjustall-hint"),
						uistate.T("budgets.adjustAllFieldHint",
							formatAdjustPct(budgeting.AdjustMinPct), formatAdjustPct(budgeting.AdjustMaxPct))))),
			// C671: how long it lasts, asked as its own question. The form used to
			// have one answer and never state it, so an action offered as a fix for
			// one underfunded period rewrote the plan for every period after it.
			labeledField(uistate.T("budgets.adjustAllScopeLabel"),
				uiw.Segmented(uiw.SegmentedProps{
					Label:    uistate.T("budgets.adjustAllScopeLabel"),
					Selected: string(scope),
					Options: []uiw.SegOption{
						{Value: string(budgeting.AdjustThisPeriod), Label: uistate.T("budgets.adjustAllScopeThis"), TestID: "adjustall-scope-this"},
						{Value: string(budgeting.AdjustEveryPeriod), Label: adjustEveryPeriodLabel(), TestID: "adjustall-scope-every"},
					},
					OnSelect: func(v string) { scopeS.Set(v); ack.Set(false) },
				})),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "adjustall-scope-hint"),
				Style(map[string]string{"margin": "0"}), fundsImpactLine(budgeting.ImpactAdjustAll, fundsImpactOption{reach: impactReach})),
			// C671: WHICH period "this period" is, said inside the form. The funding
			// callout discloses it before opening, but the toolbar's own Adjust-all is
			// available at any time and carries none of that — so a user browsing a
			// closed month could reach this same write with nothing anywhere saying
			// the month had ended. The warning belongs where both doors lead.
			If(scope == budgeting.AdjustThisPeriod,
				P(ClassStr(adjustPeriodClass(viewedClosed)), Attr("data-testid", "adjustall-period"),
					Style(map[string]string{"margin": "0"}),
					uistate.T(adjustPeriodKey(viewedClosed), vw.FromLabel()))),
			If(inlineErr != "", P(ClassStr("form-err"), Attr("data-testid", "adjustall-err"), Attr("role", "alert"), inlineErr)),
			previewNode,
			skipNode,
			ackNode,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "adjustall-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			// The commit names its own reach, so the last thing read before the
			// click is the thing that was hidden until after it (C671).
			buttonWithDisabled(!canApply, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "adjustall-apply")},
				uistate.T(adjustApplyKey(scope))),
		),
	)
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
