// SPDX-License-Identifier: MIT

//go:build js && wasm

// A suggested limit for ONE budget (SM-12).
//
// The auto-budget flow already proposes limits for every category at once, behind
// a slider and a review step. That is the right shape for setting up a budget set
// and the wrong shape for the far commoner moment: looking at one budget that is
// obviously mis-sized and wanting it to match reality. This is that moment — one
// line, one figure, one click.
//
// The arithmetic is budgeting.SuggestLimit, the same trailing-average the
// auto-budget flow uses, so accepting the hint and running auto-budget can never
// propose two different numbers for the same category.
package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// suggestLimitCode gates SM-12. B12 is "Healthy budget average" — a Free feature,
// so the hint runs with no provider and defaults on.
const suggestLimitCode = "SMART-B12"

// suggestLimitMonths is how much history the average is taken over. Three months
// is long enough to absorb one odd month and short enough to still describe how
// the household lives now.
const suggestLimitMonths = 3

// budgetSuggestProps carries the budget the hint is about.
type budgetSuggestProps struct {
	Budget domain.Budget
}

type suggestLimitCacheEntry struct {
	recommendation budgeting.LimitRecommendation
	displayAmount  money.Money
	err            error
	valid          bool
}

var suggestLimitCache = map[string]suggestLimitCacheEntry{}
var suggestSettingsCache struct {
	revision int
	settings smart.Settings
	loaded   bool
}

// budgetSuggestHint renders an evidence-backed, directional limit recommendation.
// Its own component: it renders inside the budget list and owns a click hook.
func budgetSuggestHint(props budgetSuggestProps) ui.Node {
	dataRevision := uistate.UseDataRevision().Get()
	app := appstate.Default
	settings := smartSettingsForRevision(dataRevision)
	viewedPeriod := uistate.UsePeriod().Get()
	memberLens := uistate.UseMemberLens()
	memberID := ""
	if len(memberLens) == 1 {
		memberID = memberLens[0]
	}
	b := props.Budget

	recommendation, displayAmount, ok := budgetRecommendationAtMember(app, b, budgetViewAnchor(viewedPeriod, budgetNow()), memberID)

	apply := func() {
		a := appstate.Default
		if a == nil || !ok {
			return
		}
		// Re-read the budget at click time: the list behind this hint may have been
		// re-rendered from newer data since it was drawn.
		var cur domain.Budget
		found := false
		for _, x := range a.Budgets() {
			if x.ID == b.ID {
				cur, found = x, true
				break
			}
		}
		if !found {
			return
		}
		auditview.CaptureNow()
		cur.Limit = displayAmount
		if err := a.PutBudgetAt(cur, budgetViewAnchor(viewedPeriod, budgetNow())); err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("sm12.appliedUndo", cur.Name, fmtMoney(cur.Limit)))
	}

	if !ok {
		return Fragment()
	}
	amount := fmtMoney(displayAmount)
	verbKey := "sm12.raise"
	verb := uistate.T(verbKey)
	if recommendation.Direction == budgeting.SuggestionLower {
		verbKey, verb = "sm12.lower", uistate.T("sm12.lower")
	}
	reasonKey := "sm12.reasonNoOutliers"
	if len(recommendation.Outliers) == 1 {
		reasonKey = "sm12.reasonOneOutlier"
	} else if len(recommendation.Outliers) > 1 {
		reasonKey = "sm12.reasonManyOutliers"
	}
	return budgetSuggestRowHint(settings, budgetSuggestRowHintProps{
		Key:                   "sm12:" + b.ID,
		TestID:                "sm12-" + b.ID,
		Text:                  uistate.T("sm12.directionalHint", verb, amount),
		Detail:                budgetSuggestionEvidence(reasonKey, len(recommendation.MonthsUsed)+len(recommendation.Outliers), len(recommendation.Outliers)),
		ActionLabel:           uistate.T(verbKey+"AccessibleAction", b.Name, amount),
		AccessibleActionLabel: uistate.T(verbKey+"AccessibleAction", b.Name, amount),
		OnAction:              apply,
		Dismissible:           true,
	})
}

// budgetSuggestionEvidence formats the selected evidence template with exactly
// the values that template uses. The no-outlier variant needs only the window;
// the other variants also need the excluded-month count.
func budgetSuggestionEvidence(key string, window, outliers int) string {
	switch key {
	case "sm12.reasonNoOutliers":
		return uistate.T(key, window)
	case "sm12.reasonOneOutlier":
		return uistate.T(key, window)
	default:
		return uistate.T(key, window, outliers)
	}
}

func budgetRecommendationCopy(direction budgeting.SuggestionDirection, amount string, periods, outliers int) string {
	verbKey := "sm12.keep"
	switch direction {
	case budgeting.SuggestionRaise:
		verbKey = "sm12.raise"
	case budgeting.SuggestionLower:
		verbKey = "sm12.lower"
	}
	reasonKey := "sm12.reasonNoOutliers"
	if outliers == 1 {
		reasonKey = "sm12.reasonOneOutlier"
	} else if outliers > 1 {
		reasonKey = "sm12.reasonManyOutliers"
	}
	target := uistate.T("sm12.directionalHint", uistate.T(verbKey), amount)
	return target + " " + budgetSuggestionEvidence(reasonKey, periods, outliers)
}

// budgetSuggestRowHint renders SM-12 with a contextual accessible action name.
func budgetSuggestRowHint(settings smart.Settings, props budgetSuggestRowHintProps) ui.Node {
	if props.Text == "" || props.Key == "" || !settings.IsEnabled(suggestLimitCode) || settings.IsMuted(suggestLimitCode) ||
		!settings.DensityOrDefault().Shows(smart.AffordanceFieldAssist) || settings.IsDismissed(props.Key) {
		return Fragment()
	}
	return ui.CreateElement(budgetSuggestRowHintInner, props)
}

type budgetSuggestRowHintProps struct {
	Key                   string
	TestID                string
	Text                  string
	Detail                string
	ActionLabel           string
	AccessibleActionLabel string
	OnAction              func()
	Dismissible           bool
}

func budgetSuggestRowHintInner(props budgetSuggestRowHintProps) ui.Node {
	act := ui.UseEvent(func() {
		if props.OnAction != nil {
			props.OnAction()
		}
	})
	dismiss := ui.UseEvent(func() { uistate.DismissSmartInsight(props.Key); uistate.BumpDataRevision() })
	return Div(css.Class("smart-rowhint"), Attr("data-testid", "smart-rowhint-"+props.TestID),
		uiw.Icon(icon.Sparkles, css.Class(tw.W35, tw.H35, tw.ShrinkO)),
		Div(css.Class("smart-rowhint-body"),
			Div(css.Class("smart-rowhint-text"), props.Text),
			If(props.Detail != "", Div(css.Class("smart-rowhint-detail", tw.Text12, tw.TextDim), props.Detail)),
		),
		Button(css.Class("btn btn-sm smart-rowhint-act"), Type("button"), Attr("data-testid", "smart-rowhint-act-"+props.TestID),
			Attr("aria-label", props.AccessibleActionLabel), OnClick(act), props.ActionLabel),
		If(props.Dismissible, Button(css.Class("btn-icon-bare smart-rowhint-x"), Type("button"),
			Attr("data-testid", "smart-rowhint-dismiss-"+props.TestID), Attr("aria-label", uistate.T("smartHint.dismissAria")),
			Title(uistate.T("smartHint.dismiss")), OnClick(dismiss), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3)))),
	)
}

// budgetRecommendationAt returns an evidence-backed direction and a displayed
// limit in the budget's cadence and currency, or ok=false when no change is useful.
// Saving-direction budgets are omitted because their target is an intentional
// contribution rather than a spending cap.
func budgetRecommendationAt(app *appstate.App, b domain.Budget, anchor time.Time) (budgeting.LimitRecommendation, money.Money, bool) {
	return budgetRecommendationAtMember(app, b, anchor, "")
}

func budgetRecommendationAtMember(app *appstate.App, b domain.Budget, anchor time.Time, memberID string) (budgeting.LimitRecommendation, money.Money, bool) {
	if app == nil || b.Direction.IsSaving() {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	visible := false
	for _, visibleBudget := range budgeting.ViewBudgets(app.Budgets(), memberID) {
		if visibleBudget.ID == b.ID {
			visible = true
			break
		}
	}
	if !visible {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// Match the card's category descendants and tracked tags without rewriting rows.
	covers := categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs())
	scope := budgeting.ScopeOf(b, covers)
	if scope.IsEmpty() {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	if anchor.IsZero() {
		anchor = budgetNow()
	}
	userPrefs := uistate.LoadPrefs()
	weekStart := userPrefs.WeekStartWeekday()
	var payCycleAnchor time.Time
	if userPrefs.PayCycleAnchor != "" {
		payCycleAnchor, _ = time.Parse("2006-01-02", userPrefs.PayCycleAnchor)
	}
	start, end := budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payCycleAnchor)
	periodKey := start.Format("2006-01-02")
	now := budgetNow()
	cacheKey := fmt.Sprintf("%p|%d|%s|%s|%s|%s|%d|%s", app, app.Rev(), b.ID, memberID, periodKey, now.Format("2006-01-02"), weekStart, payCycleAnchor.Format("2006-01-02"))
	entry, cached := suggestLimitCache[cacheKey]
	if !cached {
		ledger := budgetLedgerForApp(app)
		suggestedTxns, suggestedCovers := ledger, covers
		_, err := budgeting.EvaluateRollup(b, suggestedTxns, start, end, rates, budgeting.DefaultNearThreshold, suggestedCovers)
		var recommendation budgeting.LimitRecommendation
		var displayAmount money.Money
		if err == nil {
			projectionBudget := b
			projectionBudget.CategoryIDs = scope.CategoryIDs
			view := budgeting.EvaluateViewBudget(projectionBudget, budgeting.ViewInput{
				Period: budgeting.ViewPeriod{Anchor: anchor, Now: now, WeekStart: userPrefs.WeekStartWeekday(), PayCycleAnchor: payCycleAnchor},
				Ledger: &ledger, Transactions: ledger, Categories: app.Categories(), Recurring: app.Recurring(), Rates: rates, Base: base,
			})
			if view.Projection.Error != nil {
				err = view.Projection.Error
			}
			projected := view.Projection.Projected
			if err == nil {
				projected, err = budgeting.PeriodEquivalent(projected, b.Period, domain.PeriodMonthly)
			}
			if err == nil {
				recommendation, err = budgeting.RecommendBudgetLimitWithContext(b, suggestedCovers, suggestedTxns, anchor, suggestLimitMonths, projected, rates, weekStart, payCycleAnchor)
			}
			if err == nil && recommendation.Direction == budgeting.SuggestionRaise {
				overCount := budgetRecommendationOverspends(b, covers, scope.Tags, ledger, start, weekStart, payCycleAnchor, rates)
				if !budgetRaiseAllowed(overCount, view.Projection.Projected, view.Status.Limit) {
					return budgeting.LimitRecommendation{}, money.Money{}, false
				}
			}
			if err == nil && recommendation.HasEvidence && recommendation.Direction != budgeting.SuggestionKeep {
				periodAmount, convErr := budgeting.PeriodEquivalent(money.New(recommendation.Suggested, base), domain.PeriodMonthly, b.Period)
				if convErr == nil {
					displayCurrency := b.Limit.Currency
					if displayCurrency == "" {
						displayCurrency = base
					}
					displayAmount, err = rates.Convert(periodAmount, displayCurrency)
					if err == nil && recommendation.Direction == budgeting.SuggestionLower && view.Committed.Required.Amount > 0 {
						displayAmount, err = budgetSuggestionBillFloor(displayAmount, view.Committed.Required, displayCurrency, rates)
					}
				} else {
					err = convErr
				}
			}
		}
		entry = suggestLimitCacheEntry{recommendation: recommendation, displayAmount: displayAmount, err: err,
			valid: err == nil && recommendation.HasEvidence && recommendation.Direction != budgeting.SuggestionKeep && displayAmount.Amount > 0}
		if len(suggestLimitCache) > 512 {
			suggestLimitCache = map[string]suggestLimitCacheEntry{}
		}
		suggestLimitCache[cacheKey] = entry
	}
	if entry.err != nil || !entry.valid {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	return entry.recommendation, entry.displayAmount, true
}

// budgetRaiseAllowed permits a raise only after repeated overspending and when
// the current period's projection is over its effective limit.
func budgetRaiseAllowed(overCount int, projected, limit money.Money) bool {
	return overCount >= 2 && projected.Currency == limit.Currency && projected.Amount > limit.Amount
}

func budgetSuggestionBillFloor(suggested, required money.Money, targetCurrency string, rates currency.Rates) (money.Money, error) {
	floor, err := rates.Convert(required, targetCurrency)
	if err != nil {
		return money.Money{}, err
	}
	if suggested.Amount < floor.Amount {
		return floor, nil
	}
	return suggested, nil
}

func budgetRecommendationOverspends(b domain.Budget, covers map[string]bool, tags []string, txns budgeting.NettedLedger, anchor time.Time, weekStart time.Weekday, payCycleAnchor time.Time, rates currency.Rates) int {
	scope := budgeting.ScopeOf(b, covers)
	if scope.IsEmpty() {
		return 0
	}
	scoped, scopedCategories := txns, covers
	count := 0
	for i := 0; i < suggestLimitMonths; i++ {
		start, end := budgeting.PreviousPeriodRangeAnchored(b.Period, anchor, weekStart, payCycleAnchor)
		status, err := budgeting.EvaluateRollup(b, scoped, start, end, rates, budgeting.DefaultNearThreshold, scopedCategories)
		if err == nil && status.State == budgeting.StateOver {
			count++
		}
		anchor = start
	}
	return count
}

func smartSettingsForRevision(revision int) smart.Settings {
	if !suggestSettingsCache.loaded || suggestSettingsCache.revision != revision {
		suggestSettingsCache = struct {
			revision int
			settings smart.Settings
			loaded   bool
		}{revision: revision, settings: uistate.LoadSmartSettings(), loaded: true}
	}
	return suggestSettingsCache.settings
}
