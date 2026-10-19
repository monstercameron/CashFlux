// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
			if f.Flow == "out" {
				f.Flow = ""
			} else {
				f.Flow = "out"
			}
		})
	}))
	onIncome := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			if f.Flow == "in" {
				f.Flow = ""
			} else {
				f.Flow = "in"
			}
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
	categoryFiltered := filter.Category != ""
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

	secondaryText := []string{txnCountTransactionCount(props.Shown)}
	if props.MovedRows > 0 {
		label := uistate.T("transactions.scopeMoved", fmtMoney(props.Moved))
		secondaryText = append(secondaryText, label)
		if props.UnpairedRows > 0 {
			label = uistate.T("transactions.scopeUnpaired", props.UnpairedRows)
			secondaryText = append(secondaryText, label)
		}
	}
	if props.Unconverted > 0 {
		label := uistate.T("transactions.scopeUnconverted", props.Unconverted)
		secondaryText = append(secondaryText, label)
	}
	for _, issue := range txnConversionIssueCounts(props.ConversionIssues) {
		label := uistate.T("tc2e.rowsCannotCompare", issue.count, issue.currency)
		secondaryText = append(secondaryText, label)
	}
	if props.Excluded > 0 {
		label := uistate.T("transactions.scopeExcluded", props.Excluded)
		secondaryText = append(secondaryText, label)
	}
	if props.Lens != "" {
		label := uistate.T("transactions.scopeLens", props.Lens)
		secondaryText = append(secondaryText, label)
	}
	var upcoming ui.Node
	if !categoryFiltered && txnCountShouldShowUpcoming(filter) {
		upcoming = ui.CreateElement(txnUpcomingInline, struct{}{})
	}
	var searchWidenAction ui.Node
	preset := filter.PeriodPreset
	if preset == "" {
		preset = txnfilter.InferPeriodPreset(filter.From, filter.To, time.Now())
	}
	if q := strings.TrimSpace(filter.Text); q != "" && preset != txnfilter.PeriodAllDates && preset != txnfilter.PeriodCustom {
		allDates := filter
		allDates.From, allDates.To, allDates.PeriodPreset = "", "", txnfilter.PeriodAllDates
		allCount := txnSearchAllDatesCount(allDates)
		searchWidenAction = txnSearchAllDatesAction(allCount, searchWidenClick)
	}
	if searchWidenAction != nil {
		band = append(band, searchWidenAction)
	}
	var visibleFacts ui.Node
	if len(secondaryText) > 0 {
		factNodes := make([]ui.Node, 0, len(secondaryText))
		for _, fact := range secondaryText {
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

type txnCategoryTotal struct {
	ID    string
	Name  string
	Minor int64
	Share float64
}

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
	categories, selected := props.Categories, props.Selected
	var total int64
	for _, category := range categories {
		if category.Minor <= 0 {
			continue
		}
		var err error
		total, err = money.Add(total, category.Minor)
		if err != nil {
			total = 0
			break
		}
	}
	barCategories := txnCategoryTopFive(categories, selected)
	segments := make([]ui.Node, 0, len(barCategories))
	items := make([]ui.Node, 0, len(categories))
	accessibleItems := make([]ui.Node, 0, len(categories))
	visibleCount := 3
	for _, category := range barCategories {
		category.Share = txnCategoryShare(category.Minor, total)
		isSelected := selected == "__uncategorized__" && category.ID == "" || selected != "" && selected != "__uncategorized__" && category.ID == selected
		props := txnCategorySegmentProps{Category: category, Selected: isSelected, Dimmed: selected != "" && !isSelected}
		segments = append(segments, ui.CreateElement(txnCategorySegment, props))
	}
	for _, category := range categories {
		category.Share = txnCategoryShare(category.Minor, total)
		isSelected := selected == "__uncategorized__" && category.ID == "" || selected != "" && selected != "__uncategorized__" && category.ID == selected
		itemProps := txnCategorySegmentProps{Category: category, Selected: isSelected, Dimmed: selected != "" && !isSelected}
		items = append(items, ui.CreateElement(txnCategoryListItem, itemProps))
		accessibleItems = append(accessibleItems, Div(Attr("role", "listitem"), category.Name, " ", fmtMoney(money.New(category.Minor, txnCountBaseCurrency()))))
	}
	if len(categories) > visibleCount {
		for i, category := range categories {
			if category.ID == "__other__" && i >= visibleCount {
				items[visibleCount-1], items[i] = items[i], items[visibleCount-1]
				break
			}
		}
		items = items[:visibleCount]
		label := uistate.T("v5sum.more", len(categories)-visibleCount)
		if expanded.Get() {
			label = uistate.T("tv8c.closeList")
		}
		items = append(items, Button(css.Class("txn-category-more"), Type("button"), Attr("data-testid", "txn-category-more"), Attr("aria-controls", "txn-category-popover"), Attr("aria-haspopup", "dialog"), Attr("aria-expanded", ariaBool(expanded.Get())), toggleClick, label))
	}
	breakdownClass := "txn-category-breakdown"
	if selected != "" {
		breakdownClass += " is-filtered"
	}
	if expanded.Get() {
		breakdownClass += " is-expanded"
	}
	var popover ui.Node
	if expanded.Get() {
		allItems := make([]ui.Node, 0, len(categories))
		for _, category := range categories {
			category.Share = txnCategoryShare(category.Minor, total)
			isSelected := selected == "__uncategorized__" && category.ID == "" || selected != "" && selected != "__uncategorized__" && category.ID == selected
			allItems = append(allItems, ui.CreateElement(txnCategoryPopoverItem, txnCategorySegmentProps{Category: category, Selected: isSelected, Dimmed: selected != "" && !isSelected}))
		}
		popover = Div(css.Class("txn-category-popover"), Attr("id", "txn-category-popover"), Attr("role", "dialog"), Attr("aria-label", uistate.T("tv8c.categoryList")), Attr("data-testid", "txn-category-popover"), allItems)
	}
	return Div(css.Class(breakdownClass),
		Div(css.Class("txn-category-bar"), Attr("role", "group"), Attr("aria-label", uistate.T("uxt1.categoryBreakdown")), segments),
		Div(css.Class("txn-category-list"), items),
		popover,
		Div(css.Class("sr-only", "txn-category-accessible-list"), Attr("role", "list"), accessibleItems))
}

func txnCategoryPopoverItem(props txnCategorySegmentProps) ui.Node {
	item := txnCategoryListItem(props)
	return Div(css.Class("txn-category-popover-row"), item, Span(css.Class("txn-category-share"), fmt.Sprintf("%.1f%%", props.Category.Share)))
}

type txnCategorySegmentProps struct {
	Category txnCategoryTotal
	Selected bool
	Dimmed   bool
}

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
	setTxFilterOn(atom, func(filter *uistate.TxFilter) {
		if category == "" {
			filter.Uncategorized = !filter.Uncategorized
			filter.Category = ""
			return
		}
		if filter.Category == category {
			filter.Category = ""
		} else {
			filter.Category = category
		}
	})
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
	viewScope := txnDerivedScope(app, criteria)
	criteria.Flow, criteria.Category, criteria.Uncategorized = "", "", false
	criteria.Sort, criteria.Dir, criteria.Page, criteria.PageSize = "", "", 0, 0
	base, rates := txnCountCurrency()
	scope := fmt.Sprintf("%#v|%s", criteria, strings.Join(memberLens, "\x00"))
	rev := app.Rev()
	memo := &txnCountCategoryMemo
	if memo.valid && memo.app == app && memo.rev == rev && memo.scope == scope && memo.currency == base {
		return append([]txnCategoryTotal(nil), memo.rows...)
	}
	categories := app.Categories()
	categoryNames := make(map[string]string, len(categories))
	for _, category := range categories {
		categoryNames[category.ID] = category.Name
	}
	accounts := app.Accounts()
	accountNames := make(map[string]string, len(accounts))
	for _, account := range accounts {
		accountNames[account.ID] = account.Name
	}
	var categoryTotals []txnview.CategoryTotal
	if view, ok := txnDerivedViewCache.Lookup(rev, viewScope); ok {
		categoryTotals = view.Categories
	} else {
		labels := txnfilter.Labels{Account: accountNames, Category: categoryNames, Payee: app.PayeeResolver().Resolve, BaseCurrency: base, ToBase: func(value money.Money) (money.Money, error) { return rates.Convert(value, base) }}
		visible := txnfilter.ApplyWithLabels(app.Transactions(), criteria, labels)
		view, err := txnview.Summarize(visible, rates, base)
		if err != nil {
			return nil
		}
		categoryTotals = view.Categories
	}
	result := make([]txnCategoryTotal, 0, len(categoryTotals))
	for _, category := range categoryTotals {
		id, amount := category.ID, category.Minor
		name := categoryNames[id]
		if name == "" {
			name = uistate.T("uxt1.uncategorized")
		}
		result = append(result, txnCategoryTotal{ID: id, Name: name, Minor: amount})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Minor == result[j].Minor {
			return result[i].Name < result[j].Name
		}
		return result[i].Minor > result[j].Minor
	})
	memo.app, memo.rev, memo.scope, memo.currency, memo.rows, memo.valid = app, rev, scope, base, result, true
	return result
}

func txnCategoryTopFive(categories []txnCategoryTotal, selected string) []txnCategoryTotal {
	if len(categories) <= 5 {
		return categories
	}
	result := append([]txnCategoryTotal(nil), categories[:5]...)
	selectedFound := false
	for _, category := range result {
		selectedFound = selectedFound || selected != "" && category.ID == selected
	}
	other := txnCategoryTotal{ID: "__other__", Name: uistate.T("v5sum.other")}
	for _, category := range categories[5:] {
		if selected != "" && category.ID == selected && !selectedFound {
			result = append(result, category)
			selectedFound = true
			continue
		}
		var err error
		other.Minor, err = money.Add(other.Minor, category.Minor)
		if err != nil {
			return categories
		}
	}
	if other.Minor == 0 {
		return result
	}
	return append(result, other)
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
	if value.Currency == "" {
		value.Currency = base
	}
	return value
}

func txnCountBaseCurrency() string {
	base, _ := txnCountCurrency()
	return base
}

func absTxnAmount(amount int64) int64 {
	if amount < 0 {
		return -amount
	}
	return amount
}

func txnSecondaryFact(class, key string, value any) ui.Node {
	return Span(css.Class("txn-count-fact", class), uistate.T(key, value))
}

func strconvTxnPercent(value float64) string {
	return fmt.Sprintf("%.4f%%", value)
}

func txnCategoryShare(amount, spent int64) float64 {
	if spent <= 0 || amount <= 0 {
		return 0
	}
	return float64(amount) * 100 / float64(spent)
}

func txnCountShouldShowUpcoming(filter txnfilter.Criteria) bool {
	return strings.TrimSpace(filter.Text) == ""
}

func txnFutureTotal(items []bills.Bill, rates currency.Rates, base string) (money.Money, int) {
	total := money.New(0, base)
	unconverted := 0
	for _, item := range items {
		if item.TransferAccountID != "" {
			continue
		}
		converted, err := rates.Convert(item.Amount, base)
		if err != nil {
			unconverted++
			continue
		}
		total.Amount += converted.Amount
	}
	return total, unconverted
}

func txnCountShowsNet(spent, income money.Money) bool {
	return spent.Amount > 0 && income.Amount > 0
}

func txnCountTransactionCount(n int) string {
	key := "r3n.transactionCountMany"
	if n == 1 {
		key = "r3n.transactionCountOne"
	}
	return uistate.T(key, txnCountNumber(n))
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
	counts := make(map[string]int)
	for _, issue := range issues {
		code := strings.ToUpper(strings.TrimSpace(issue.Transaction.Amount.Currency))
		if code == "" {
			code = "unknown"
		}
		counts[code]++
	}
	currencies := make([]string, 0, len(counts))
	for code := range counts {
		currencies = append(currencies, code)
	}
	sort.Strings(currencies)
	result := make([]txnConversionIssueCount, 0, len(currencies))
	for _, code := range currencies {
		result = append(result, txnConversionIssueCount{currency: code, count: counts[code]})
	}
	return result
}

func txnCountPeriod(filter txnfilter.Criteria) string {
	if filter.From == "" && filter.To == "" {
		return uistate.T("r2a.allDates")
	}
	from, fromErr := time.Parse("2006-01-02", filter.From)
	to, toErr := time.Parse("2006-01-02", filter.To)
	if filter.From == "" && toErr == nil {
		return uistate.T("txnTQ6.throughDate", uistate.LoadPrefs().FormatDate(to))
	}
	if filter.To == "" && fromErr == nil {
		return uistate.T("txnTQ6.fromDate", uistate.LoadPrefs().FormatDate(from))
	}
	if fromErr == nil && toErr == nil && from.Year() == to.Year() && from.Month() == to.Month() && from.Day() == 1 && to.AddDate(0, 0, 1).Day() == 1 {
		return uistate.T("r2a.month", uistate.T("budgets.month."+strings.ToLower(from.Month().String())), from.Year())
	}
	if fromErr == nil && toErr == nil {
		return uistate.T("txnperiod.customRange", uistate.LoadPrefs().FormatDate(from), uistate.LoadPrefs().FormatDate(to))
	}
	return uistate.T("r2a.customRange")
}

func signedTxnMoney(value money.Money) string { return fmtTxnSignedMoney(value) }

func txnAnnouncementScope(filter txnfilter.Criteria) txnfilter.Criteria {
	filter.Sort, filter.Dir, filter.Page, filter.PageSize = "", "", 0, 0
	return filter
}

func liveMode(announce bool) string {
	if announce {
		return "polite"
	}
	return "off"
}
