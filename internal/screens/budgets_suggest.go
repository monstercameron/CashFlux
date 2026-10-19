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
	"strings"
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
	b := props.Budget

	recommendation, displayAmount, ok := budgetRecommendationAt(app, b, viewedPeriod.From)

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
		if err := a.PutBudget(cur); err != nil {
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
	if key == "sm12.reasonNoOutliers" {
		return uistate.T(key, window)
	}
	return uistate.T(key, window, outliers)
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
	if app == nil || b.Direction.IsSaving() {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// Match the budget meter's category descendants and tracked-tag scope. The
	// budgeting suggestion engine accepts category sets, so tag-matched rows get
	// private synthetic category IDs while ordinary rows retain their categories.
	covers := categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs())
	scope := budgeting.ScopeOf(b, covers)
	if scope.IsEmpty() {
		return budgeting.LimitRecommendation{}, money.Money{}, false
	}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	userPrefs := uistate.LoadPrefs()
	var payCycleAnchor time.Time
	if userPrefs.PayCycleAnchor != "" {
		payCycleAnchor, _ = time.Parse("2006-01-02", userPrefs.PayCycleAnchor)
	}
	start, end := budgeting.PeriodRangeAnchored(b.Period, anchor, userPrefs.WeekStartWeekday(), payCycleAnchor)
	periodKey := start.Format("2006-01-02")
	cacheKey := fmt.Sprintf("%p|%d|%s|%s", app, app.Rev(), b.ID, periodKey)
	entry, cached := suggestLimitCache[cacheKey]
	if !cached {
		suggestedTxns, suggestedCovers := suggestTransactionsForScope(covers, scope.Tags, app.Transactions())
		status, err := budgeting.EvaluateRollup(b, suggestedTxns, start, end, rates, budgeting.DefaultNearThreshold, suggestedCovers)
		var recommendation budgeting.LimitRecommendation
		var displayAmount money.Money
		if err == nil {
			projected := budgeting.ProjectPace(status, start, end, time.Now()).Projected
			projected, err = budgeting.PeriodEquivalent(projected, b.Period, domain.PeriodMonthly)
			if err == nil {
				recommendation, err = budgeting.RecommendBudgetLimit(b, suggestedCovers, suggestedTxns, anchor, suggestLimitMonths, projected, rates)
			}
			if err == nil && recommendation.HasEvidence && recommendation.Direction != budgeting.SuggestionKeep {
				periodAmount, convErr := budgeting.PeriodEquivalent(money.New(recommendation.Suggested, base), domain.PeriodMonthly, b.Period)
				if convErr == nil {
					displayCurrency := b.Limit.Currency
					if displayCurrency == "" {
						displayCurrency = base
					}
					displayAmount, err = rates.Convert(periodAmount, displayCurrency)
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

// suggestTransactionsForScope adapts the established budget scope to the
// category-set suggestion engine. Tag matches are marked with transaction-local
// synthetic categories so unrelated transactions in those real categories do
// not leak into the suggestion.
func suggestTransactionsForScope(covers map[string]bool, tags []string, txns []domain.Transaction) ([]domain.Transaction, map[string]bool) {
	allCovers := make(map[string]bool, len(covers))
	for id, on := range covers {
		if on {
			allCovers[id] = true
		}
	}
	tagSet := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if key := strings.ToLower(strings.TrimSpace(tag)); key != "" {
			tagSet[key] = true
		}
	}
	out := make([]domain.Transaction, 0, len(txns))
	for _, txn := range txns {
		if !txn.CountsInReports() {
			continue
		}
		tagMatch := false
		for _, tag := range txn.Tags {
			if tagSet[strings.ToLower(strings.TrimSpace(tag))] {
				tagMatch = true
				break
			}
		}
		if !tagMatch {
			out = append(out, txn)
			continue
		}
		copyTxn := txn
		prefix := "__budget_tag__" + txn.ID
		if txn.HasSplits() {
			copyTxn.Splits = append([]domain.CategorySplit(nil), txn.Splits...)
			for i := range copyTxn.Splits {
				id := fmt.Sprintf("%s:%d", prefix, i)
				copyTxn.Splits[i].CategoryID = id
				allCovers[id] = true
			}
		} else {
			copyTxn.CategoryID = prefix
			allCovers[prefix] = true
		}
		out = append(out, copyTxn)
	}
	return out, allCovers
}
