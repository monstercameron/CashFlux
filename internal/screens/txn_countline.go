// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	"github.com/monstercameron/CashFlux/internal/txnview"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// txnCountLineProps carries the numbers the summary states and the scope that
// produced them.
type txnCountLineProps struct {
	Shown int
	Total int
	Range string // retained for the caller; page ranges are not shown here
	Net   money.Money
	Lens  string
	// Unconverted counts visible rows that could not be folded into the base currency.
	Unconverted int
	// ConversionIssues are rows excluded by a filter because they could not be
	// converted to the household currency for comparison.
	ConversionIssues []txnfilter.ConversionIssue
	// Excluded counts visible rows intentionally omitted from reports and budgets.
	Excluded int
	// Moved is the total magnitude of each visible transfer once.
	Moved money.Money
	// MovedRows counts movements (or visible orphan legs) in the current view.
	MovedRows int
	// UnpairedRows counts transfers without a counterpart in the current view.
	UnpairedRows int
	// Spent and Income summarize the same filtered rows as Shown and Net.
	Spent, Income money.Money
	// CashflowProvided remains for callers compiled against the prior props shape.
	CashflowProvided bool
	// Revision identifies the app snapshot used to calculate the summary.
	Revision uint64
	// CategoryTotals carries the category breakdown calculated for this revision.
	CategoryTotals []txnCategoryTotal
	// FactsProvidedByTable avoids repeating count, transfer and exclusion facts.
	FactsProvidedByTable bool
}

// txnCountLine summarizes the visible transaction scope and its totals.
func txnCountLine(props txnCountLineProps) ui.Node {
	return ui.CreateElement(txnCountLineComponent, props)
}

func txnCountLineComponent(props txnCountLineProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	filter := filterAtom.Get()
	scope := txnAnnouncementScope(filter)
	lastScope := ui.UseState(scope)
	announceFilterChange := lastScope.Get() != scope
	ui.UseEffect(func() func() {
		if announceFilterChange {
			lastScope.Set(scope)
		}
		return nil
	}, scope)

	filter, _ = filter.WithOwnerLens(memberLens)
	futurePeriod, scheduledCount := txnFutureScheduleCount(filter, time.Now())
	onSpent := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			*f = txntoolbar.ToggleFlow(*f, "out")
		})
	}))
	onIncome := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			*f = txntoolbar.ToggleFlow(*f, "in")
		})
	}))
	onSearchAllDates := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			f.From, f.To, f.PeriodPreset = "", "", txnfilter.PeriodAllDates
		})
	}))
	searchWidenClick := OnClick(onSearchAllDates)
	if props.Total == 0 && !futurePeriod {
		return Span(css.Class("txn-countline-slot"), Attr("aria-hidden", "true"), Style(map[string]string{"display": "none"}))
	}

	period := txnCountPeriod(filter)
	if futurePeriod && props.Shown == 0 {
		items := txnFutureScheduledItems(filter, time.Now())
		base, rates := txnCountCurrency()
		total, unconverted := txnFutureTotal(items, rates, base)
		parts := []ui.Node{Span(css.Class("txn-count-main"), uistate.T("tv7b.due", fmtMoney(total)))}
		parts = append(parts, txnSecondaryFact("txn-count-future-scheduled", "tv7b.scheduled", txnCountNumber(scheduledCount)))
		if unconverted > 0 {
			parts = append(parts, txnSecondaryFact("txn-count-unconverted", "transactions.scopeUnconverted", unconverted))
		}
		accessibleFuture := uistate.T("tv7b.futureAccessible", txnCountNumber(props.Shown), txnCountNumber(scheduledCount), fmtMoney(total))
		return txnCountRoot(announceFilterChange, accessibleFuture,
			Div(css.Class("txn-countline txn-countline-future", tw.MinW0), Attr("data-testid", "txn-count-line"), Div(css.Class("txn-count-band", "txn-count-future-band"), parts)))
	}

	base, _ := txnCountCurrency()
	spent := txnCountWithCurrency(props.Spent, base)
	income := txnCountWithCurrency(props.Income, base)
	net := txnCountWithCurrency(props.Net, base)
	categories := props.CategoryTotals
	if categories == nil {
		categories = txnCountCategories(filter, memberLens)
	}
	categoryFiltered := txntoolbar.CategoryFiltered(filter)
	figures := []ui.Node{txnCountFigure("spent", uistate.T("tc3w13.spent"), fmtMoney(spent), true, filter.Flow == "out", true, onSpent)}
	if !categoryFiltered && !filter.Uncategorized {
		figures = append(figures,
			txnCountFigure("income", uistate.T("tc3w13.income"), fmtMoney(income), false, filter.Flow == "in", true, onIncome),
			txnCountFigure("net", uistate.T("tc3w13.net"), signedTxnMoney(net), net.Amount < 0, false, false, ui.Handler{}),
		)
	}
	band := []ui.Node{Div(css.Class("txn-count-figures"), figures)}
	if len(categories) > 0 {
		selectedCategory := filter.Category
		if filter.Uncategorized {
			selectedCategory = "__uncategorized__"
		}
		band = append(band, txnCategoryBreakdown(categories, selectedCategory, spent.Amount))
	}

	secondaryText := txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney}.SummaryFacts(txntoolbar.Facts{Shown: props.Shown, MovedRows: props.MovedRows, UnpairedRows: props.UnpairedRows, Unconverted: props.Unconverted, Excluded: props.Excluded, Moved: props.Moved, Issues: props.ConversionIssues, Lens: props.Lens})
	var upcoming ui.Node
	if !categoryFiltered && txnCountShouldShowUpcoming(filter) {
		upcoming = ui.CreateElement(txnUpcomingInline, struct{}{})
	}
	var searchWidenAction ui.Node

	if allDates, ok := txntoolbar.WidenSearch(filter, time.Now()); ok {
		searchWidenAction = txnSearchAllDatesAction(txnSearchAllDatesCount(allDates), searchWidenClick)
	}
	if searchWidenAction != nil {
		band = append(band, searchWidenAction)
	}
	var visibleFacts ui.Node
	visibleText := secondaryText
	if props.FactsProvidedByTable {
		// The table owns the complete visible facts band, including filter
		// conversion failures and the perspective label. Keep the full accessible summary.
		visibleText = nil
	}
	if len(visibleText) > 0 {
		factNodes := make([]ui.Node, 0, len(visibleText))
		for _, fact := range visibleText {
			factNodes = append(factNodes, Span(css.Class("txn-count-fact"), fact))
		}
		visibleFacts = Div(css.Class("txn-count-secondary", "txn-count-visible-facts"), Attr("data-testid", "txn-count-visible-facts"), factNodes)
	}
	if upcoming != nil {
		band = append(band, Div(css.Class("txn-count-upcoming-fact"), upcoming))
	}
	accessibleSummary := uistate.T("uxt1.accessibleSummary", period, txnCountTransactionCount(props.Shown), fmtMoney(spent), fmtMoney(income), signedTxnMoney(net))
	if categoryFiltered {
		accessibleSummary = uistate.T("v5sum.filteredSummary", period, txnCountTransactionCount(props.Shown), fmtMoney(spent))
	}
	accessible := []string{accessibleSummary}
	if len(secondaryText) > 1 {
		accessible = append(accessible, secondaryText[1:]...)
	}
	return txnCountRoot(announceFilterChange, strings.Join(accessible, ". "),
		Div(css.Class("txn-countline", tw.MinW0), Attr("data-testid", "txn-count-line"),
			Div(css.Class("txn-count-band"), Attr("data-testid", "txn-count-band"), band), visibleFacts))
}

func txnSearchAllDatesAction(count int, click any) ui.Node {
	if count <= 0 {
		return nil
	}
	label := uistate.T("r4cnt.searchAllDates", txnCountNumber(count))
	return Button(css.Class("btn-link txn-count-search-widen"), Type("button"), Attr("data-testid", "txn-search-all-dates"), click, label)
}

func txnCountRoot(announce bool, sentence string, content ui.Node) ui.Node {
	return Div(css.Class("txn-countline-wrap"),
		Span(css.Class("sr-only"), Attr("data-testid", "txn-count-line"), Attr("role", "status"), Attr("aria-live", liveMode(announce)), Attr("aria-atomic", "true"), sentence),
		content)
}

func txnCountFigure(kind, label, amount string, negative, pressed, interactive bool, onClick ui.Handler) ui.Node {
	click := OnClick(onClick)
	valueClass := "txn-count-value"
	if negative || kind == "spent" {
		valueClass += " txn-count-negative"
	}
	value := Span(css.Class(valueClass), Attr("data-testid", "txn-count-"+kind+"-value"), amount)
	if kind == "net" {
		tone := "txn-count-net-neutral"
		if negative {
			tone = "txn-count-negative"
		} else if strings.HasPrefix(amount, "+") {
			tone = "txn-count-positive"
		}
		value = Span(css.Class("txn-count-value", tone), Attr("data-testid", "txn-count-net-value"), amount)
	}
	var figure ui.Node
	if interactive {
		buttonClass := "txn-count-figure txn-count-figure-button"
		if kind == "spent" {
			buttonClass += " text-down"
		}
		figure = Button(css.Class(buttonClass), Type("button"), Attr("data-testid", "txn-count-"+kind), Attr("aria-label", label+" "+amount), Attr("aria-pressed", ariaBool(pressed)), click,
			value, Span(css.Class("txn-count-label"), label))
	} else {
		figure = Div(css.Class("txn-count-figure"), Attr("data-testid", "txn-count-"+kind), value, Span(css.Class("txn-count-label"), label))
	}
	return figure
}

type txnCategoryTotal = txntoolbar.CategoryTotal

func txnCategoryBreakdown(categories []txnCategoryTotal, selected string, _ int64) ui.Node {
	return ui.CreateElement(txnCategoryBreakdownComponent, txnCategoryBreakdownProps{Categories: categories, Selected: selected})
}

type txnCategoryBreakdownProps struct {
	Categories []txnCategoryTotal
	Selected   string
}

func txnCategoryBreakdownComponent(props txnCategoryBreakdownProps) ui.Node {
	expanded := ui.UseState(false)
	onToggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	toggleClick := OnClick(onToggle)
	layout := txntoolbar.Presentation{Text: uistate.T}.CategoryLayout(props.Categories, props.Selected)
	segments := make([]ui.Node, 0, len(layout.Bar))
	for _, item := range layout.Bar {
		segments = append(segments, ui.CreateElement(txnCategorySegment, item))
	}
	items := make([]ui.Node, 0, len(layout.Visible)+1)
	for _, item := range layout.Visible {
		items = append(items, ui.CreateElement(txnCategoryListItem, item))
	}
	accessibleItems := make([]ui.Node, 0, len(layout.All))
	for _, item := range layout.All {
		accessibleItems = append(accessibleItems, Div(Attr("role", "listitem"), item.Category.Name, " ", fmtMoney(money.New(item.Category.Minor, txnCountBaseCurrency()))))
	}
	if layout.More > 0 {
		label := uistate.T("v5sum.more", layout.More)
		if expanded.Get() {
			label = uistate.T("tv8c.closeList")
		}
		items = append(items, Button(css.Class("txn-category-more"), Type("button"), Attr("data-testid", "txn-category-more"), Attr("aria-controls", "txn-category-popover"), Attr("aria-haspopup", "dialog"), Attr("aria-expanded", ariaBool(expanded.Get())), toggleClick, label))
	}
	breakdownClass := "txn-category-breakdown"
	if props.Selected != "" {
		breakdownClass += " is-filtered"
	}
	if expanded.Get() {
		breakdownClass += " is-expanded"
	}
	var popover ui.Node
	if expanded.Get() {
		allItems := make([]ui.Node, 0, len(layout.All))
		for _, item := range layout.All {
			allItems = append(allItems, ui.CreateElement(txnCategoryPopoverItem, item))
		}
		popover = Div(css.Class("txn-category-popover"), Attr("id", "txn-category-popover"), Attr("role", "dialog"), Attr("aria-label", uistate.T("tv8c.categoryList")), Attr("data-testid", "txn-category-popover"), allItems)
	}
	return Div(css.Class(breakdownClass), Div(css.Class("txn-category-bar"), Attr("role", "group"), Attr("aria-label", uistate.T("uxt1.categoryBreakdown")), segments), Div(css.Class("txn-category-list"), items), popover, Div(css.Class("sr-only", "txn-category-accessible-list"), Attr("role", "list"), accessibleItems))
}

func txnCategoryPopoverItem(props txnCategorySegmentProps) ui.Node {
	item := txnCategoryListItem(props)
	return Div(css.Class("txn-category-popover-row"), item, Span(css.Class("txn-category-share"), fmt.Sprintf("%.1f%%", props.Category.Share)))
}

type txnCategorySegmentProps = txntoolbar.CategoryItem

func txnCategorySegment(props txnCategorySegmentProps) ui.Node {
	isOther := props.Category.ID == "__other__"
	class := "txn-category-segment"
	if isOther {
		class += " txn-category-segment-other"
	}
	if props.Selected {
		class += " is-selected"
	}
	if props.Category.ID == "" {
		class += " needs-attention"
	}
	if props.Dimmed {
		class += " is-dimmed"
	}
	label := uistate.T("uxt1.categoryFilter", props.Category.Name, fmtMoney(money.New(props.Category.Minor, txnCountBaseCurrency())))
	if isOther {
		label = props.Category.Name + " " + fmtMoney(money.New(props.Category.Minor, txnCountBaseCurrency()))
	}
	filterAtom := uistate.UseTxFilter()
	onClick := ui.UseEvent(Prevent(func() {
		if !isOther {
			toggleTxnCategoryFilter(filterAtom, props.Category.ID)
		}
	}))
	return Button(css.Class(class), Type("button"), Attr("data-testid", "txn-category-segment"), Attr("data-category-id", props.Category.ID), Attr("aria-hidden", "true"), Attr("tabindex", "-1"), Attr("title", label), Style(map[string]string{"width": strconvTxnPercent(props.Category.Share)}), OnClick(onClick))
}

func txnCategoryListItem(props txnCategorySegmentProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	isOther := props.Category.ID == "__other__"
	onClick := ui.UseEvent(Prevent(func() {
		if !isOther {
			toggleTxnCategoryFilter(filterAtom, props.Category.ID)
		}
	}))
	class := "txn-category-item"
	if props.Selected {
		class += " is-selected"
	}
	if props.Category.ID == "" {
		class += " needs-attention"
	}
	if props.Dimmed {
		class += " is-dimmed"
	}
	if isOther {
		class += " txn-category-other"
	}
	options := []any{css.Class(class), Type("button"), Attr("data-testid", "txn-category-item"), Attr("data-category-id", props.Category.ID), Attr("aria-pressed", ariaBool(props.Selected))}
	if isOther {
		options = append(options, Attr("disabled", "disabled"))
	}
	options = append(options, OnClick(onClick),
		Span(css.Class("txn-category-name"), props.Category.Name), Span(css.Class("txn-category-amount"), fmtMoney(money.New(props.Category.Minor, txnCountBaseCurrency()))))
	return Button(options...)
}

func toggleTxnCategoryFilter(atom state.Atom[uistate.TxFilter], category string) {
	setTxFilterOn(atom, func(filter *uistate.TxFilter) { *filter = txntoolbar.ChooseCategory(*filter, category) })
}

var txnCountCategoryMemo struct {
	app      *appstate.App
	rev      uint64
	scope    string
	currency string
	rows     []txnCategoryTotal
	valid    bool
}

func txnCountCategories(filter txnfilter.Criteria, memberLens []string) []txnCategoryTotal {
	app := appstate.Default
	if app == nil {
		return nil
	}

	criteria, _ := filter.WithOwnerLens(memberLens)
	criteria = txntoolbar.CategoryScope(criteria)
	base, rates := txnCountCurrency()
	scope := txnview.Scope(criteria)
	rev := app.Rev()
	memo := &txnCountCategoryMemo
	if memo.valid && memo.app == app && memo.rev == rev && memo.scope == scope && memo.currency == base {
		return append([]txnCategoryTotal(nil), memo.rows...)
	}
	names := txntoolbar.BuildIndexes(app.Accounts(), app.Categories(), nil, nil)
	labels := txnfilter.Labels{Account: names.Accounts, Category: names.Categories, Payee: app.PayeeResolver().Resolve, BaseCurrency: base, ToBase: func(value money.Money) (money.Money, error) { return rates.Convert(value, base) }}
	rows := txnfilter.ApplyWithLabels(app.Transactions(), criteria, labels)
	view, err := txnview.Summarize(rows, rates, base)
	if err != nil {
		return nil
	}
	result := txnCategoriesForView(view, names.Categories)

	memo.app, memo.rev, memo.scope, memo.currency, memo.rows, memo.valid = app, rev, scope, base, result, true
	return result
}

func txnCategoryTopFive(categories []txnCategoryTotal, selected string) []txnCategoryTotal {
	return txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}.CategoryTopFive(categories, selected)
}

func txnCountCurrency() (string, currency.Rates) {
	base := "USD"
	var rates currency.Rates
	if app := appstate.Default; app != nil {
		settings := app.Settings()
		if settings.BaseCurrency != "" {
			base = settings.BaseCurrency
		}
		rates = currency.Rates{Base: base, Rates: settings.FXRates}
	}
	return base, rates
}

func txnCountWithCurrency(value money.Money, base string) money.Money {
	return txntoolbar.WithCurrency(value, base)
}

func txnCountBaseCurrency() string {
	base, _ := txnCountCurrency()
	return base
}

func txnSecondaryFact(class, key string, value any) ui.Node {
	return Span(css.Class("txn-count-fact", class), uistate.T(key, value))
}

func strconvTxnPercent(value float64) string {
	return fmt.Sprintf("%.4f%%", value)
}

func txnCategoryShare(amount, spent int64) float64 { return txntoolbar.CategoryShare(amount, spent) }

func txnCountShouldShowUpcoming(filter txnfilter.Criteria) bool {
	return txntoolbar.ShowUpcoming(filter)
}

func txnFutureTotal(items []bills.Bill, rates currency.Rates, base string) (money.Money, int) {
	return txntoolbar.FutureTotal(txntoolbar.BillAmounts(items), rates, base)
}

func txnCountShowsNet(spent, income money.Money) bool { return txntoolbar.ShowsNet(spent, income) }

func txnCountTransactionCount(n int) string {
	return txntoolbar.Presentation{Text: uistate.T}.TransactionCount(n)
}

// txnSearchAllDatesMemo caches the count widened beyond the current date range.
var txnSearchAllDatesMemo struct {
	app    *appstate.App
	rev    uint64
	filter txnfilter.Criteria
	count  int
	valid  bool
}

func txnSearchAllDatesCount(allDates txnfilter.Criteria) int {
	app := appstate.Default
	if app == nil {
		return 0
	}
	rev := app.Rev()
	m := &txnSearchAllDatesMemo
	if m.valid && m.app == app && m.rev == rev && m.filter == allDates {
		return m.count
	}
	count := len(txnfilter.ApplyWithLabels(app.Transactions(), allDates, txnLabelsForApp(app, nil, nil)))
	m.app, m.rev, m.filter, m.count, m.valid = app, rev, allDates, count, true
	return count
}

type txnConversionIssueCount struct {
	currency string
	count    int
}

func txnConversionIssueCounts(issues []txnfilter.ConversionIssue) []txnConversionIssueCount {
	groups := txntoolbar.ConversionIssueCounts(issues)
	result := make([]txnConversionIssueCount, len(groups))
	for i, g := range groups {
		result[i] = txnConversionIssueCount{currency: g.Currency, count: g.Count}
	}
	return result
}

func txnCountPeriod(filter txnfilter.Criteria) string {
	return txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}.CountPeriod(filter)
}

func signedTxnMoney(value money.Money) string { return fmtTxnSignedMoney(value) }

func txnAnnouncementScope(filter txnfilter.Criteria) txnfilter.Criteria {
	return txntoolbar.AnnouncementScope(filter)
}

func liveMode(announce bool) string { return txntoolbar.LiveMode(announce) }

func txnCategoriesForView(view txnview.View, names map[string]string) []txnCategoryTotal {
	return txntoolbar.Categories(view.Categories, names, uistate.T("uxt1.uncategorized"))
}
