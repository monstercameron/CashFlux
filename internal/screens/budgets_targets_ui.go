// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetTargetKindOptions builds the funding-target picker options (BG1). The first
// option stores the empty value (TargetNone) so saving it clears any target.
func budgetTargetKindOptions(selected string) []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: string(domain.TargetNone), Label: uistate.T("budgets.targetNone")},
		{Value: string(domain.TargetRefillUpTo), Label: uistate.T("budgets.targetRefill")},
		{Value: string(domain.TargetSetAside), Label: uistate.T("budgets.targetSetAside")},
		{Value: string(domain.TargetByDate), Label: uistate.T("budgets.targetByDate")},
	}
}

// budgetGoalOptions lists the active financial goals a by-date budget target can
// borrow its pace from, prefixed by a "no linked goal" option.
func budgetGoalOptions(app *appstate.App, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("budgets.targetLinkGoalNone")}}
	if app == nil {
		return opts
	}
	for _, g := range app.Goals() {
		if g.Archived || !g.IsFinancial() {
			continue
		}
		opts = append(opts, uiw.SelectOption{Value: g.ID, Label: g.Name})
	}
	return opts
}

// budgetTargetDraft holds the funding-target form state threaded from the edit form.
type budgetTargetDraft struct {
	Kind     string
	Amount   string // major-units text
	Date     string // ISO "2006-01-02"
	GoalID   string
	Decimals int
	Currency string
}

// budgetTargetSection renders the funding-target editor: a kind picker plus the
// shape-specific fields (amount, date, and a goal link for by-date targets). It is
// a plain render helper called once at a stable position in the edit form, so the
// select/input On* handlers register safely (the no-On*-in-loop rule only bans
// variable-length loops).
func budgetTargetSection(app *appstate.App, d budgetTargetDraft,
	onKind func(string), onGoal func(string), onAmount any, onDate any) ui.Node {
	kind := domain.TargetKind(d.Kind)
	needsAmount := kind == domain.TargetRefillUpTo || kind == domain.TargetSetAside || kind == domain.TargetByDate
	isByDate := kind == domain.TargetByDate

	return Div(css.Class("budget-target-section"),
		labeledField(uistate.T("budgets.targetLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: budgetTargetKindOptions(d.Kind), Selected: d.Kind,
				OnChange: onKind, AriaLabel: uistate.T("budgets.targetLabel"),
			})),
		If(kind == domain.TargetNone, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.targetHint"))),
		If(needsAmount, Div(css.Class("budget-edit-row"),
			labeledField(uistate.T("budgets.targetAmountLabel"),
				Input(css.Class("field"), Type("number"), Attr("data-testid", "budget-target-amount"),
					Placeholder(uistate.T("budgets.targetAmountLabel")), Step("0.01"), OnInput(onAmount), uiw.FieldValue(d.Amount))),
			If(isByDate, labeledField(uistate.T("budgets.targetDateLabel"),
				Input(css.Class("field"), Type("date"), Attr("data-testid", "budget-target-date"),
					Attr("aria-label", uistate.T("budgets.targetDateLabel")), OnInput(onDate), uiw.FieldValue(d.Date)))),
		)),
		If(isByDate, labeledField(uistate.T("budgets.targetLinkGoalLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: budgetGoalOptions(app, d.GoalID), Selected: d.GoalID,
				OnChange: onGoal, AriaLabel: uistate.T("budgets.targetLinkGoalLabel"),
			}))),
		If(isByDate, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.targetLinkGoalHint"))),
	)
}

// budgetDraftTarget applies a target draft onto a copy of the budget so its
// underfunding can be previewed as the user edits (BG4's "To target" chip).
func budgetDraftTarget(b domain.Budget, d budgetTargetDraft) domain.Budget {
	b.TargetKind = domain.TargetKind(d.Kind)
	if !b.TargetKind.Valid() {
		b.TargetKind = domain.TargetNone
	}
	if m, err := money.ParseMinor(d.Amount, d.Decimals); err == nil {
		b.TargetAmount = money.New(m, d.Currency)
	} else {
		b.TargetAmount = money.Zero(d.Currency)
	}
	if t, err := time.Parse("2006-01-02", d.Date); err == nil {
		b.TargetDate = t
	} else {
		b.TargetDate = time.Time{}
	}
	b.LinkedGoalID = d.GoalID
	return b
}

// budgetTargetNeed evaluates a budget's funding-target need (BG1), resolving a
// by-date target's pace from its linked goal when one is set.
func budgetTargetNeed(app *appstate.App, b domain.Budget, status budgeting.Status, now time.Time) budgeting.TargetNeed {
	var linked money.Money
	hasLinked := false
	if b.TargetKind == domain.TargetByDate && b.LinkedGoalID != "" && app != nil {
		for _, g := range app.Goals() {
			if g.ID != b.LinkedGoalID {
				continue
			}
			if m, ok, err := goals.MonthlyNeeded(g, now); err == nil && ok {
				linked, hasLinked = m, true
			}
			break
		}
	}
	return budgeting.Needed(b, status, now, linked, hasLinked)
}

// budgetTargetLine renders the one-line target summary beneath a budget's bar (BG1)
// — e.g. "Refill to $200 · $60 to go". Returns an empty fragment when the budget
// has no target. Resolves app/goals/clock internally so the row edit stays minimal.
func budgetTargetLine(s budgeting.Status) ui.Node {
	b := s.Budget
	if !b.HasTarget() {
		return Fragment()
	}
	app := appstate.Default
	need := budgetTargetNeed(app, b, s, time.Now())
	target := fmtMoney(need.Target)
	remaining := fmtMoney(need.Needed)
	pr := uistate.LoadPrefs()

	// WF17: a paused target says it is paused, at what level, and until when. The
	// alternative — showing "nothing needed" — is indistinguishable from a target
	// that is already met, which is the opposite reading.
	if need.Snoozed {
		return Span(ClassStr("budget-sub "+tw.ColorClass("text-dim")),
			Attr("data-testid", "budget-target-snoozed-"+b.ID),
			uistate.T("budgets.targetSnoozed", target, pr.FormatDate(b.TargetSnoozedUntil)))
	}

	var text string
	switch b.TargetKind {
	case domain.TargetRefillUpTo:
		if need.Needed.IsZero() {
			text = uistate.T("budgets.targetRefillMet", target)
		} else {
			text = uistate.T("budgets.targetRefillRow", target, remaining)
		}
	case domain.TargetSetAside:
		text = uistate.T("budgets.targetSetAsideRow", target)
	case domain.TargetByDate:
		dateLabel := pr.FormatDate(b.TargetDate)
		if need.Needed.IsZero() {
			text = uistate.T("budgets.targetByDateMet", target, dateLabel)
		} else {
			text = uistate.T("budgets.targetByDateRow", target, dateLabel, remaining)
		}
	default:
		return Fragment()
	}
	return Span(css.Class("budget-sub", tw.TextDim), Attr("data-testid", "budget-target-"+b.ID), text)
}

// budgetQuickFillChipProps carries one computed fill suggestion plus a plain pick
// callback (never an On* hook — the chip owns its click hook, per the loop rule).
type budgetQuickFillChipProps struct {
	Key    string
	Label  string
	Kind   string // localized "what this figure is" phrase, for the accessible name
	Value  string // formatted money for display
	Major  string // major-units string applied to the amount field on pick
	OnPick func(string)
}

// budgetQuickFillChip is one quick-fill chip. Its own component so the click hook
// stays at a stable call-site inside the variable-length chip list.
//
// C667: the accessible name says what the figure IS as well as what it is worth.
// Picking a chip overwrites the budget's target, and "Prior limit · $1,300.00"
// read aloud beside three spend figures gave a screen-reader user nothing to
// separate a plan from a fact.
func budgetQuickFillChip(props budgetQuickFillChipProps) ui.Node {
	pick := ui.UseEvent(Prevent(func() { props.OnPick(props.Major) }))
	return Button(css.Class("budget-fill-chip"), Type("button"),
		Attr("data-testid", "budget-fill-"+props.Key),
		Attr("aria-label", uistate.T("budgets.fillApplyKind", props.Label, props.Kind, props.Value)),
		OnClick(pick),
		Span(css.Class("budget-fill-chip-label"), props.Label),
		Span(css.Class("budget-fill-chip-value", tw.TextDim), " · "+props.Value),
	)
}

// budgetQuickFillLabel maps a QuickFill key to its localized chip label.
func budgetQuickFillLabel(key string) string {
	switch key {
	case budgeting.QuickFillLastPeriod:
		return uistate.T("budgets.fillLastPeriod")
	case budgeting.QuickFillAvg3:
		return uistate.T("budgets.fillAvg3")
	case budgeting.QuickFillAvg6:
		return uistate.T("budgets.fillAvg6")
	case budgeting.QuickFillPriorLimit:
		return uistate.T("budgets.fillPriorLimit")
	case budgeting.QuickFillUnderfunded:
		return uistate.T("budgets.fillUnderfunded")
	}
	return key
}

// budgetQuickFillKind maps a suggestion's kind to the phrase that says what its
// figure actually is — spending, a limit that was set, or a target's shortfall.
func budgetQuickFillKind(kind budgeting.QuickFillKind) string {
	switch kind {
	case budgeting.QuickFillLimit:
		return uistate.T("budgets.fillKindLimit")
	case budgeting.QuickFillTarget:
		return uistate.T("budgets.fillKindTarget")
	default:
		return uistate.T("budgets.fillKindSpend")
	}
}

// budgetQuickFillRow renders the quick-fill chip strip beside the amount field
// (BG4). onPick receives the chosen major-units string to seed the amount input.
//
// C667: the suggestions are computed over the budget's OWN cadence, from the
// SAME period the card is reporting on, and over the SAME category population —
// the tracked categories plus their descendants, the set computeBudgetView hands
// EvaluateRollup. Missing any of the three is what put "Last month $0.00 / Avg
// 3 mo $5.42" under a card reading $1,455.74 spent: the chips read the parent
// category over calendar months while the bar rolled up the sub-categories over
// the budget's period.
//
// anchor is the view's period anchor (budgetViewAnchor) — NOT time.Now(). The
// card evaluates against the viewed window, so paging back to March and opening
// the editor would otherwise offer history counted backwards from today: the same
// mismatch the ticket describes, arriving down the time axis instead of the
// category one. The caption underneath states the window and the population, so a
// figure that still looks surprising can be checked rather than guessed at.
func budgetQuickFillRow(app *appstate.App, b domain.Budget, status budgeting.Status, anchor time.Time, draft budgetTargetDraft, onPick func(string)) ui.Node {
	if app == nil {
		return Fragment()
	}
	pr := uistate.LoadPrefs()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	var payAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, err := time.Parse("2006-01-02", pr.PayCycleAnchor); err == nil {
			payAnchor = t
		}
	}

	// The target's remaining need is a forward-looking figure about today's
	// funding, so it keeps the wall clock — the same reference budgetTargetLine
	// uses on the card, which is the line this chip has to agree with.
	need := budgetTargetNeed(app, budgetDraftTarget(b, draft), status, time.Now())
	in := budgeting.QuickFillInput{
		Now:            anchor,
		WeekStart:      pr.WeekStartWeekday(),
		PayAnchor:      payAnchor,
		Rates:          rates,
		Covers:         categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs()),
		Underfunded:    need.Needed,
		HasUnderfunded: domain.TargetKind(draft.Kind).Valid() && domain.TargetKind(draft.Kind) != domain.TargetNone && need.Needed.IsPositive(),
	}
	fills, win := budgeting.QuickFills(b, app.Transactions(), in)
	if len(fills) == 0 {
		return Fragment()
	}
	// Name the window the spend figures cover, or say plainly that there is none —
	// silence there reads as "this IS your history".
	explain := uistate.T("budgets.fillExplainNone")
	if win.Periods > 0 {
		explain = uistate.T("budgets.fillExplain", win.Periods, strings.ToLower(periodLabel(win.Period)),
			pr.FormatDate(win.From), pr.FormatDate(win.To.AddDate(0, 0, -1)))
	}
	return Div(css.Class("budget-fill-row"),
		Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.fillHeading")),
		Div(css.Class("budget-fill-chips"),
			MapKeyed(fills, func(f budgeting.QuickFill) any { return f.Key }, func(f budgeting.QuickFill) ui.Node {
				return ui.CreateElement(budgetQuickFillChip, budgetQuickFillChipProps{
					Key:    f.Key,
					Label:  budgetQuickFillLabel(f.Key),
					Kind:   budgetQuickFillKind(f.Kind),
					Value:  fmtMoney(f.Amount),
					Major:  money.FormatMinor(f.Amount.Amount, currency.Decimals(f.Amount.Currency)),
					OnPick: onPick,
				})
			}),
		),
		Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budget-quickfill-explain"), explain),
		Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budget-quickfill-limit-note"),
			uistate.T("budgets.fillLimitNote", uistate.T("budgets.fillPriorLimit"))),
	)
}
