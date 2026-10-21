// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// unbudgetedStripProps drives the "Unbudgeted spending" strip on /budgets.
type unbudgetedStripProps struct {
	App     *appstate.App
	Base    string
	CatName map[string]string
	// From/To are the VIEWED period's half-open range, and Label names it (C607).
	//
	// The strip used to read time.Now() and say "this month" whatever period the
	// page was showing: on a closed Jul 2026 view in August it listed August's
	// spending under a label claiming otherwise. A supporting module that quietly
	// changes scope while the page's own figures do not is worse than one that
	// simply has no data for the period.
	From, To time.Time
	Label    string
	// Historical is true when the viewed window has already ended, which changes
	// the tense of everything here from an invitation to a record.
	Historical bool
}

// unbudgetedCat is one candidate: an expense category with spending this month
// and no budget tracking it.
type unbudgetedCat = budgeting.UnbudgetedCategory

// unbudgetedStrip surfaces contextual budget creation (G8): the top expense
// categories with real spending this month that no budget tracks, each one click
// from a pre-filled add form (category + a suggested limit). Renders nothing when
// every spending category is already budgeted — the strip is an invitation, not
// a fixture.
func unbudgetedStrip(props unbudgetedStripProps) ui.Node {
	dataRevision := uistate.UseDataRevision().Get()
	app := props.App
	now := time.Now()
	// C607: the VIEWED period, not today's month. A zero range means the caller
	// did not say, and falling back to this month is then honest rather than a
	// silent mismatch — but every caller passes one.
	ms, me := props.From, props.To
	if ms.IsZero() || me.IsZero() {
		ms, me = dateutil.MonthRange(now)
	}
	appRevision := uint64(0)
	if app != nil {
		appRevision = app.Rev()
	}
	data := ui.UseMemo(func() unbudgetedStripData {
		if app == nil {
			return unbudgetedStripData{}
		}
		rates := currency.Rates{Base: props.Base, Rates: app.Settings().FXRates}
		computed, err := budgeting.ComputeUnbudgeted(app.Budgets(), app.Categories(), budgetLedgerForApp(app), ms, me, rates)
		if err != nil {
			return unbudgetedStripData{Err: err}
		}
		candidates := make([]unbudgetedCat, 0, len(computed.Categories))
		for _, candidate := range computed.Categories {
			if candidate.CategoryID == "" {
				continue
			}
			candidates = append(candidates, candidate)
		}
		return unbudgetedStripData{Candidates: candidates}
	}, dataRevision, appRevision, ms.Unix(), me.Unix(), props.Base)
	if app == nil {
		return Fragment()
	}
	if data.Err != nil {
		slog.Error("calculate unbudgeted spending", "error", data.Err)
		return P(css.Class("form-err"), Attr("role", "status"), uistate.T("budgets.incomplete"))
	}
	cands := append([]unbudgetedCat(nil), data.Candidates...)
	if len(cands) == 0 {
		return Fragment()
	}
	// The cap is fine — this is a teaser, not an inventory — but it used to be
	// SILENT: a household with nine untracked categories was shown four and never
	// told the other five existed, so "tidy these up" looked finished when it was
	// not (Cam, 2026-08-31). The count rides on the bulk link below.
	const maxChips = 4
	totalCands := len(cands)
	if len(cands) > maxChips {
		cands = cands[:maxChips]
	}
	settings := app.Settings()
	rates := currency.Rates{Base: props.Base, Rates: settings.FXRates}
	if rates.Base == "" {
		rates.Base = budgetBaseCurrency(rates.Base)
	}
	userPrefs := uistate.LoadPrefs()
	var payAnchor time.Time
	if userPrefs.PayCycleAnchor != "" {
		payAnchor, _ = time.Parse("2006-01-02", userPrefs.PayCycleAnchor)
	}
	ledger := budgeting.NewNettedLedger(budgetLedgerForApp(app), app.TxnLinks())

	chips := MapKeyed(cands, func(c unbudgetedCat) any { return c.CategoryID }, func(c unbudgetedCat) ui.Node {
		// Only the four visible chips need a historical recommendation.
		sug, err := contextCategoryBudgetMedian(c.CategoryID, app.Categories(), ledger.Transactions(), ms,
			rates, userPrefs.WeekStartWeekday(), payAnchor)
		if err != nil {
			slog.Error("suggest unbudgeted category limit", "category_id", c.CategoryID, "error", err)
		}
		if sug <= 0 {
			sug = c.Amount.Amount
		}
		return ui.CreateElement(unbudgetedChip, unbudgetedChipProps{
			CatID: c.CategoryID, CatName: c.Name,
			SpentStr:   fmtMoney(c.Amount),
			LimitMajor: money.FormatMinor(sug, currency.Decimals(rates.Base)),
			Others:     hasMemberBudgetForCategory(app.Budgets(), app.Categories(), c.CategoryID),
		})
	})
	// C607: the hint NAMES the period it describes. "this month" was true only by
	// coincidence, and false in exactly the case a user is most likely to
	// misread — a closed period they have deliberately paged back to.
	hintKey := "budgets.unbudgetedHintPeriod"
	if props.Historical {
		hintKey = "budgets.unbudgetedHintPeriodPast"
	}
	label := props.Label
	if label == "" {
		label = uistate.T("budgets.unbudgetedThisPeriod")
	}
	return Div(css.Class("budget-unbudgeted"), Attr("data-testid", "budgets-unbudgeted"),
		Div(css.Class("budget-unbudgeted-head"),
			H2(css.Class("budget-unbudgeted-title"), uistate.T("budgets.unbudgetedHead")),
			Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budgets-unbudgeted-hint"),
				uistate.T(hintKey, label)),
			// The chips are capped at four and scoped to the viewed period, and
			// neither limit is visible from here. This is the way to the rest: every
			// untracked category over a year, each one handled on its own terms.
			ui.CreateElement(trackAllButton, trackAllButtonProps{Total: totalCands}),
		),
		Ul(css.Class("budget-unbudgeted-chips"), chips),
	)
}

type unbudgetedStripData struct {
	Candidates []unbudgetedCat
	Err        error
}

func contextCategoryBudgetMedian(categoryID string, categories []domain.Category, txns []domain.Transaction, anchor time.Time,
	rates currency.Rates, weekStart time.Weekday, payAnchor time.Time) (int64, error) {
	covers := categorytree.DescendantsOfAll(categories, []string{categoryID})
	budget := domain.Budget{ID: "suggestion:" + categoryID, CategoryID: categoryID,
		Period: domain.PeriodMonthly, Limit: money.Zero(rates.Base)}
	recommendation, err := budgeting.RecommendBudgetLimitWithContext(budget, covers, txns, anchor, 3,
		money.Zero(rates.Base), rates, weekStart, payAnchor)
	if err != nil {
		return 0, err
	}
	if !recommendation.HasEvidence {
		return 0, nil
	}
	return recommendation.Median, nil
}

type trackAllButtonProps struct{ Total int }

// trackAllButton opens the bulk sheet. Its own component so the click hook sits at
// a stable call-site, matching every other control on this strip.
func trackAllButton(props trackAllButtonProps) ui.Node {
	open := uistate.UseTrackUntrackedOpen()
	onOpen := ui.UseEvent(Prevent(func() { open.Set(true) }))
	// The label carries the number the chips cannot: how many untracked categories
	// there actually are, over a full year rather than just this period.
	label := uistate.T("budgets.trackAll")
	if props.Total > 0 {
		label = uistate.T("budgets.trackAllCategoryCount", props.Total)
	}
	return Button(css.Class("budget-unbudgeted-all"), Type("button"),
		Attr("data-testid", "budgets-track-all"), Attr("aria-label", label), OnClick(onOpen), label)
}

// unbudgetedChipProps drives one category chip in the unbudgeted strip.
type unbudgetedChipProps struct {
	CatID, CatName, SpentStr, LimitMajor string
	Others                               bool
}

func hasMemberBudgetForCategory(budgets []domain.Budget, categories []domain.Category, categoryID string) bool {
	for _, budget := range budgets {
		if budget.Scope != domain.ScopeIndividual {
			continue
		}
		covered := categorytree.DescendantsOfAll(categories, budget.TrackedCategoryIDs())
		if covered[categoryID] {
			return true
		}
	}
	return false
}

// unbudgetedChip is one "<category> · $spent — Budget this" chip. Clicking it opens
// the add-budget modal pre-seeded with the category and a suggested limit. Its own
// component so the click hook sits at a stable call-site (no On* in the map loop).
func unbudgetedChip(props unbudgetedChipProps) ui.Node {
	open := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetAddSeed(uistate.BudgetAddSeed{
			Name: props.CatName, CategoryID: props.CatID, LimitMajor: props.LimitMajor,
		})
		uistate.SetAddTarget("budget")
	}))
	categoryLabel := props.CatName
	if props.Others {
		categoryLabel = uistate.T("br3row.unbudgetedOthers", categoryLabel)
	}
	return Li(css.Class("budget-unbudgeted-item"), Button(css.Class("budget-unbudgeted-chip"), Type("button"),
		Attr("data-testid", "budget-this-"+props.CatID),
		Title(uistate.T("budgets.budgetThis")), OnClick(open),
		Span(css.Class("budget-unbudgeted-cat"), categoryLabel, " — ", props.SpentStr),
		Span(css.Class("budget-unbudgeted-cta"),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			uistate.T("budgets.budgetThis")),
	))
}
