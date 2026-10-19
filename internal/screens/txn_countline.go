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
	if props.Total == 0 && !futurePeriod {
		return Span(css.Class("txn-countline-slot"), Attr("aria-hidden", "true"), Style(map[string]string{"display": "none"}))
	}

	period := txnCountPeriod(filter)
	if futurePeriod && props.Shown == 0 {
		items := txnFutureScheduledItems(filter, time.Now())
		base, rates := txnCountCurrency()
		total, unconverted := txnFutureTotal(items, rates, base)
		parts := []ui.Node{Span(css.Class("txn-count-main"),
			uistate.T("fut2.futureSummary", period, txnCountNumber(props.Shown), txnCountNumber(scheduledCount)),
			Span(css.Class("txn-count-due-amount"), fmtMoney(total)), uistate.T("fut2.dueSuffix"))}
		if unconverted > 0 {
			parts = append(parts, txnSecondaryFact("txn-count-unconverted", "transactions.scopeUnconverted", unconverted))
		}
		return txnCountRoot(announceFilterChange, period+" · "+txnCountNumber(props.Shown)+" posted · "+txnCountNumber(scheduledCount)+" scheduled · "+fmtMoney(total)+" due",
			Div(css.Class("txn-countline txn-countline-future", tw.MinW0), Attr("data-testid", "txn-count-line"), parts))
	}

	base, _ := txnCountCurrency()
	spent := txnCountWithCurrency(props.Spent, base)
	income := txnCountWithCurrency(props.Income, base)
	net := txnCountWithCurrency(props.Net, base)
	categories := txnCountCategories(filter, memberLens)
	parts := []ui.Node{Div(css.Class("txn-count-figures"),
		txnCountFigure("spent", "Spent", fmtMoney(spent), true, filter.Flow == "out", true, onSpent),
		txnCountFigure("income", "Income", fmtMoney(income), false, filter.Flow == "in", true, onIncome),
		txnCountFigure("net", "Net", signedTxnMoney(net), net.Amount < 0, false, false, ui.Handler{}),
	)}
	if len(categories) > 0 {
		parts = append(parts, txnCategoryBreakdown(categories, filter.Category))
	}

	secondary := []ui.Node{
		Span(css.Class("txn-count-fact"), Attr("data-testid", "txn-count-transaction-total"), txnCountTransactionCount(props.Shown)),
	}
	secondaryText := []string{txnCountTransactionCount(props.Shown)}
	if props.MovedRows > 0 {
		label := uistate.T("transactions.scopeMoved", fmtMoney(props.Moved))
		secondary = append(secondary, Span(css.Class("txn-count-fact"), Attr("data-testid", "txn-count-moved"), label))
		secondaryText = append(secondaryText, label)
		if props.UnpairedRows > 0 {
			label = uistate.T("transactions.scopeUnpaired", props.UnpairedRows)
			secondary = append(secondary, Span(css.Class("txn-count-fact"), Attr("data-testid", "txn-count-unpaired"), label))
			secondaryText = append(secondaryText, label)
		}
	}
	if props.Unconverted > 0 {
		label := uistate.T("transactions.scopeUnconverted", props.Unconverted)
		secondary = append(secondary, Span(css.Class("txn-count-fact"), Attr("data-testid", "txn-count-unconverted"), label))
		secondaryText = append(secondaryText, label)
	}
	if props.Excluded > 0 {
		label := uistate.T("transactions.scopeExcluded", props.Excluded)
		secondary = append(secondary, Span(css.Class("txn-count-fact"), Attr("data-testid", "txn-count-excluded"), label))
		secondaryText = append(secondaryText, label)
	}
	if props.Lens != "" {
		label := uistate.T("transactions.scopeLens", props.Lens)
		secondary = append(secondary, Span(css.Class("txn-count-fact"), label))
		secondaryText = append(secondaryText, label)
	}
	var upcoming ui.Node
	if txnCountShouldShowUpcoming(filter) {
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
		label := uistate.T("r4cnt.searchAllDates", txnCountNumber(allCount))
		searchWidenAction = Button(css.Class("btn-link txn-count-search-widen"), Type("button"), Attr("data-testid", "txn-search-all-dates"),
			OnClick(onSearchAllDates), label)
		secondaryText = append(secondaryText, label)
	}
	secondary = append(secondary, searchWidenAction)
	parts = append(parts, Div(css.Class("txn-count-secondary"), Attr("data-testid", "txn-count-secondary"), secondary))
	if upcoming != nil {
		parts = append(parts, Div(css.Class("txn-count-upcoming-fact"), upcoming))
	}
	accessible := []string{uistate.T("uxt1.accessibleSummary", period, txnCountTransactionCount(props.Shown), fmtMoney(spent), fmtMoney(income), signedTxnMoney(net))}
	accessible = append(accessible, secondaryText...)
	return txnCountRoot(announceFilterChange, strings.Join(accessible, ". "),
		Div(css.Class("txn-countline", tw.MinW0), Attr("data-testid", "txn-count-line"), parts))
}

func txnCountRoot(announce bool, sentence string, content ui.Node) ui.Node {
	return Div(css.Class("txn-countline-wrap"),
		Span(css.Class("sr-only"), Attr("data-testid", "txn-count-line"), Attr("role", "status"), Attr("aria-live", liveMode(announce)), Attr("aria-atomic", "true"), sentence),
		content)
}

func txnCountFigure(kind, label, amount string, negative, pressed, interactive bool, onClick ui.Handler) ui.Node {
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
		figure = Button(css.Class(buttonClass), Type("button"), Attr("data-testid", "txn-count-"+kind), Attr("aria-pressed", ariaBool(pressed)), OnClick(onClick),
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

func txnCategoryBreakdown(categories []txnCategoryTotal, selected string) ui.Node {
	total := int64(0)
	for _, category := range categories {
		total += category.Minor
	}
	segments := make([]ui.Node, 0, len(categories))
	items := make([]ui.Node, 0, len(categories))
	for _, category := range categories {
		category.Share = float64(category.Minor) * 100 / float64(total)
		segments = append(segments, ui.CreateElement(txnCategorySegment, txnCategorySegmentProps{Category: category, Selected: category.ID == selected}))
		items = append(items, ui.CreateElement(txnCategoryListItem, txnCategorySegmentProps{Category: category, Selected: category.ID == selected}))
	}
	return Div(css.Class("txn-category-breakdown"),
		Div(css.Class("txn-category-bar"), Attr("role", "group"), Attr("aria-label", uistate.T("uxt1.categoryBreakdown")), segments),
		Div(css.Class("txn-category-list"), items))
}

type txnCategorySegmentProps struct {
	Category txnCategoryTotal
	Selected bool
}

func txnCategorySegment(props txnCategorySegmentProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	onClick := ui.UseEvent(Prevent(func() { toggleTxnCategoryFilter(filterAtom, props.Category.ID) }))
	class := "txn-category-segment"
	if props.Selected {
		class += " is-selected"
	}
	return Button(css.Class(class), Type("button"), Attr("data-testid", "txn-category-segment"), Attr("data-category-id", props.Category.ID), Attr("aria-label", uistate.T("uxt1.categoryFilter", props.Category.Name, fmtMoney(money.New(props.Category.Minor, txnCountBaseCurrency())))), Attr("aria-pressed", ariaBool(props.Selected)),
		Style(map[string]string{"width": strconvTxnPercent(props.Category.Share)}), OnClick(onClick), Span(css.Class("sr-only"), props.Category.Name))
}

func txnCategoryListItem(props txnCategorySegmentProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	onClick := ui.UseEvent(Prevent(func() { toggleTxnCategoryFilter(filterAtom, props.Category.ID) }))
	class := "txn-category-item"
	if props.Selected {
		class += " is-selected"
	}
	return Button(css.Class(class), Type("button"), Attr("data-testid", "txn-category-item"), Attr("data-category-id", props.Category.ID), Attr("aria-pressed", ariaBool(props.Selected)), OnClick(onClick),
		Span(css.Class("txn-category-name"), props.Category.Name), Span(css.Class("txn-category-amount"), fmtMoney(money.New(props.Category.Minor, txnCountBaseCurrency()))))
}

func toggleTxnCategoryFilter(atom state.Atom[uistate.TxFilter], category string) {
	setTxFilterOn(atom, func(filter *uistate.TxFilter) {
		if filter.Category == category {
			filter.Category = ""
		} else {
			filter.Category = category
		}
	})
}

func txnCountCategories(filter txnfilter.Criteria, memberLens []string) []txnCategoryTotal {
	app := appstate.Default
	if app == nil {
		return nil
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
	base, rates := txnCountCurrency()
	criteria, _ := filter.WithOwnerLens(memberLens)
	criteria.Flow = ""
	criteria.Sort, criteria.Dir, criteria.Page, criteria.PageSize = "", "", 0, 0
	labels := txnfilter.Labels{Account: accountNames, Category: categoryNames, Payee: app.PayeeResolver().Resolve, BaseCurrency: base, ToBase: func(value money.Money) (money.Money, error) { return rates.Convert(value, base) }}
	visible := txnfilter.ApplyWithLabels(app.Transactions(), criteria, labels)
	totals := make(map[string]int64)
	for _, transaction := range visible {
		if transaction.TransferAccountID != "" || transaction.Amount.Amount >= 0 {
			continue
		}
		if len(transaction.Splits) == 0 {
			converted, err := rates.Convert(transaction.Amount, base)
			if err == nil {
				totals[transaction.CategoryID] += absTxnAmount(converted.Amount)
			}
			continue
		}
		for _, split := range transaction.Splits {
			if split.Amount.Amount >= 0 {
				continue
			}
			converted, err := rates.Convert(split.Amount, base)
			if err == nil {
				totals[split.CategoryID] += absTxnAmount(converted.Amount)
			}
		}
	}
	result := make([]txnCategoryTotal, 0, len(totals))
	for id, amount := range totals {
		if amount <= 0 {
			continue
		}
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
	if len(result) > 5 {
		result = result[:5]
	}
	return result
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

func txnCountShouldShowUpcoming(filter txnfilter.Criteria) bool {
	return strings.TrimSpace(filter.Text) == ""
}

func txnFutureTotal(items []bills.Bill, rates currency.Rates, base string) (money.Money, int) {
	total := money.New(0, base)
	unconverted := 0
	for _, item := range items {
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
	rev    int
	filter txnfilter.Criteria
	count  int
	valid  bool
}

func txnSearchAllDatesCount(allDates txnfilter.Criteria) int {
	app := appstate.Default
	if app == nil {
		return 0
	}
	rev := uistate.CurrentDataRevision()
	m := &txnSearchAllDatesMemo
	if m.valid && m.app == app && m.rev == rev && m.filter == allDates {
		return m.count
	}
	count := len(txnfilter.ApplyWithLabels(app.Transactions(), allDates, txnfilter.Labels{Payee: app.PayeeResolver().Resolve}))
	m.app, m.rev, m.filter, m.count, m.valid = app, rev, allDates, count, true
	return count
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
