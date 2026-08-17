// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strconv"
	"strings"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
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
	pctStr := ui.UseState(uistate.TakeBudgetAdjustSeed())
	onPct := ui.UseEvent(func(v string) { pctStr.Set(v) })
	ack := ui.UseState(false)
	toggleAck := ui.UseEvent(func() { ack.Set(!ack.Get()) })
	close := func() {
		pctStr.Set("")
		ack.Set(false)
		openAtom.Set(false)
	}
	onCancel := ui.UseEvent(Prevent(func() { close() }))

	// The budgets in scope: the ones visible under the current member perspective,
	// which is the set the page is showing. Scoping to what is on screen is what
	// makes "every budget" a claim the user can check.
	var scoped []domain.Budget
	if app != nil {
		for _, b := range app.Budgets() {
			if ownerVisibleTo(b.OwnerID, activeMemberID) {
				scoped = append(scoped, b)
			}
		}
	}

	raw := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(pctStr.Get()), "%"))
	pct, parseErr := strconv.ParseFloat(raw, 64)
	valid := raw != "" && parseErr == nil && budgeting.ValidAdjustPct(pct)
	preview := budgeting.AdjustPreview{}
	if valid {
		preview = budgeting.AdjustAllPreview(scoped, pct)
	}
	needsAck := valid && budgeting.IsLargeAdjust(pct)
	canApply := valid && preview.Count() > 0 && (!needsAck || ack.Get())

	apply := ui.UseEvent(Prevent(func() {
		if !canApply || app == nil {
			return
		}
		n := 0
		for _, line := range preview.Lines {
			b := line.Budget
			b.Limit = money.New(line.After, b.Limit.Currency)
			if err := app.PutBudget(b); err != nil {
				uistate.PostNotice(err.Error(), true)
				continue
			}
			n++
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostUndoable(uistate.T("budgets.adjustAllApplied", plural(n, "budget"), formatAdjustPct(pct)))
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
	case preview.Count() == 0:
		inlineErr = uistate.T("budgets.adjustAllNothingToChange")
	}

	base := "USD"
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
		if !preview.MixedCurrency {
			totalNode = Div(css.Class("adjustall-total"), Attr("data-testid", "adjustall-total"),
				Span(css.Class("adjustall-total-lbl"), uistate.T("budgets.adjustAllTotalLabel")),
				Span(css.Class("adjustall-before", tw.TextFaint), fmtMoney(money.New(preview.TotalBefore, cur))),
				Span(css.Class("adjustall-arrow", tw.TextFaint), "→"),
				Span(css.Class("adjustall-after", tw.FontDisplay), fmtMoney(money.New(preview.TotalAfter, cur))),
				Span(ClassStr("adjustall-delta"+adjustDeltaTone(preview.TotalDelta())),
					uistate.T(adjustDeltaKey(preview.TotalDelta()), fmtMoney(money.New(absMinor(preview.TotalDelta()), cur)))),
			)
		} else {
			totalNode = P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "adjustall-mixed"),
				uistate.T("budgets.adjustAllMixedCurrency"))
		}
		previewNode = Div(css.Class("adjustall-preview"), Attr("data-testid", "adjustall-preview"),
			P(css.Class("adjustall-count"), Attr("data-testid", "adjustall-count"),
				uistate.T("budgets.adjustAllCount", plural(preview.Count(), "budget"), formatAdjustPct(pct))),
			totalNode,
			Div(css.Class("adjustall-rows"), Attr("data-testid", "adjustall-rows"), rows),
		)
	}

	// --- the acknowledgement for consequential changes ------------------------
	var ackNode ui.Node = Fragment()
	if needsAck {
		key := "budgets.adjustAllAckRaise"
		if pct < 0 {
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
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.adjustAllIntro")),
			// C597: the same reach vocabulary the page's other funds-moving actions
			// use, so "Adjust all" can be compared with them rather than read on its
			// own terms.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "adjustall-impact"),
				Style(map[string]string{"margin": "0"}), fundsImpactLine(budgeting.ImpactAdjustAll)),
			labeledField(uistate.T("budgets.adjustAllFieldLabel"),
				Fragment(
					Div(css.Class("adjustall-field"),
						uiw.NumField(pctStr.Get(), css.Class("field"), Type("number"), Attr("data-testid", "adjustall-pct"),
							Attr("autofocus", ""), Attr("inputmode", "decimal"),
							Attr("aria-label", uistate.T("budgets.adjustAllFieldLabel")),
							Attr("min", formatAdjustPct(budgeting.AdjustMinPct)),
							Attr("max", formatAdjustPct(budgeting.AdjustMaxPct)),
							Step("0.5"), Placeholder("5"), OnInput(onPct)),
						Span(css.Class("adjustall-suffix", tw.TextFaint), "%"),
					),
					Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "adjustall-hint"),
						uistate.T("budgets.adjustAllFieldHint",
							formatAdjustPct(budgeting.AdjustMinPct), formatAdjustPct(budgeting.AdjustMaxPct))))),
			If(inlineErr != "", P(ClassStr("form-err"), Attr("data-testid", "adjustall-err"), Attr("role", "alert"), inlineErr)),
			previewNode,
			ackNode,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "adjustall-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			buttonWithDisabled(!canApply, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "adjustall-apply")},
				uistate.T("budgets.adjustAllApply")),
		),
	)
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
