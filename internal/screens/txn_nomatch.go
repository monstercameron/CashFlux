// SPDX-License-Identifier: MIT

//go:build js && wasm

// The ledger's no-match state (C628).
//
// "No transactions match your filters." was true and inert: the only way back was
// an unlabelled ✕ in the toolbar the user had to go find, and every other control
// on the page still described a view with nothing in it. The escape belongs where
// the disappointment is.
//
// It clears the SEARCH only. The period, member and category filters are not what
// the user just typed, and a "clear" that silently widened the view to a year
// they were not looking at would be a worse surprise than the empty list.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnNoMatchProps carries the query that found nothing. Empty means the view was
// narrowed by something other than a search, and there is no search to clear.
type txnNoMatchProps struct {
	Search         string
	FuturePeriod   bool
	ScheduledCount int
	ScheduledItems []bills.Bill
	InTable        bool
}

// txnNoMatch is the empty state for a filtered ledger with no rows. Its own
// component so the clear button's click hook sits at a stable position — it is
// rendered from a switch arm that only sometimes runs.
func txnNoMatch(props txnNoMatchProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	pending := uistate.UseTxnSearchPending()
	prefs := uistate.UsePrefs().Get()
	if !props.FuturePeriod {
		props.FuturePeriod, props.ScheduledCount = txnFutureScheduleCount(filterAtom.Get(), time.Now())
	}
	if props.FuturePeriod && props.ScheduledItems == nil {
		props.ScheduledItems = txnFutureScheduledItems(filterAtom.Get(), time.Now())
	}
	clear := ui.UseEvent(func() {
		// Cancel the in-flight debounce FIRST. Without this the pending callback
		// fires ~400ms later and writes the query straight back — the clear would
		// work and then undo itself, which is a worse bug than the one it fixes.
		debounce.Flush("txn-filter:text")
		pending.Set(false)
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.Text = "" })
	})

	if props.FuturePeriod {
		return Div(css.Class("empty", tw.Flex, tw.FlexCol, tw.ItemsCenter, tw.Gap2),
			Attr("data-testid", "txn-nomatch"), Attr("role", "status"),
			P(css.Class("txn-future-empty-line"), uistate.T("fut2.futureEmptyLine")),
			If(!props.InTable && len(props.ScheduledItems) > 0, Div(
				Attr("aria-label", uistate.T("transactions.noMatchFutureListLabel")),
				Fragment(txnScheduledGhostRows(props.ScheduledItems, prefs.FormatDate)...))),
			If(props.Search != "", Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "txn-nomatch-clear"), OnClick(clear), uistate.T("action.clearSearch"))),
			A(css.Class("btn-link"), Href(uistate.RoutePath("/recurring")),
				Attr("data-testid", "txn-nomatch-recurring"), uistate.T("transactions.noMatchFutureLink")),
		)
	}
	if props.Search == "" {
		return P(css.Class("empty"), Attr("data-testid", "txn-nomatch"),
			Attr("role", "status"), uistate.T("transactions.noMatch"))
	}
	return Div(css.Class("empty", tw.Flex, tw.FlexCol, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "txn-nomatch"), Attr("role", "status"),
		P(uistate.T("transactions.noMatchSearch", props.Search)),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "txn-nomatch-clear"),
			OnClick(clear), uistate.T("action.clearSearch")),
		P(css.Class("t-caption", tw.TextDim), uistate.T("transactions.noMatchKeepsFilters")),
	)
}

func txnScheduledGhostRows(items []bills.Bill, formatDate func(time.Time) string) []any {
	rows := make([]any, 0, len(items))
	accounts := map[string]string{}
	categories := map[string]string{}
	if app := appstate.Default; app != nil {
		for _, account := range app.Accounts() {
			accounts[account.ID] = account.Name
		}
		for _, category := range app.Categories() {
			categories[category.ID] = category.Name
		}
	}
	for _, item := range items {
		account := accounts[item.AccountID]
		category := ""
		if app := appstate.Default; app != nil {
			category = categories[billCategoryID(app, item)]
		}
		amount, tone := fmtLedgerAmount(money.New(-item.Amount.Amount, item.Amount.Currency), false)
		rows = append(rows, ui.CreateElement(txnScheduledGhostRow, txnScheduledGhostRowProps{
			Date: formatDate(item.DueDate), Name: item.Name, Category: category,
			Account: account, Amount: amount, AmountTone: tone,
		}))
	}
	return rows
}

type txnScheduledGhostRowProps struct {
	Date, Name, Category, Account, Amount, AmountTone string
}

// txnScheduledGhostRow keeps a scheduled item inert as a ledger row while giving
// its overflow menu the one supported action: open the schedule that owns it.
func txnScheduledGhostRow(props txnScheduledGhostRowProps) ui.Node {
	return Div(css.Class("txn-row", "txn-scheduled-ghost"),
		Attr("data-testid", "txn-scheduled-ghost"), Attr("aria-disabled", "true"),
		Div(css.Class("td-date"), props.Date),
		Div(css.Class("td-desc"),
			Span(css.Class("txn-scheduled-name"), props.Name),
			Span(css.Class("txn-scheduled-pill"), uistate.T("transactions.scheduledStatus"))),
		Div(css.Class("td-cat"), props.Category),
		Div(css.Class("td-acct", "txn-scheduled-account"), Attr("title", props.Account), props.Account),
		Div(css.Class("td-amount", tw.ColorClass(props.AmountTone)), props.Amount),
		Div(css.Class("txn-scheduled-status"), ui.CreateElement(txnScheduledGhostMenu, struct{}{})),
	)
}

type txnScheduledTableRowProps struct {
	Date, Name, Category, Account, Amount, AmountTone string
	Vis                                               uistate.TxnCols
	ShowUser, ShowBalance                             bool
}

func txnScheduledTableRow(props txnScheduledTableRowProps) ui.Node {
	return Tr(css.Class("txn-row", "txn-scheduled-ghost"), Attr("data-testid", "txn-scheduled-ghost"), Attr("aria-disabled", "true"),
		Td(ClassStr("td-select")),
		Td(ClassStr("td-date"), props.Date),
		Td(ClassStr("td-desc"), Span(css.Class("txn-scheduled-name"), props.Name), Span(css.Class("txn-scheduled-pill"), uistate.T("transactions.scheduledStatus"))),
		If(props.Vis.Category, Td(ClassStr("td-cat"), props.Category)),
		If(props.Vis.Account, Td(ClassStr("td-acct txn-scheduled-account"), Attr("title", props.Account), props.Account)),
		If(props.ShowUser, Td(ClassStr("td-user"))),
		If(props.Vis.Amount, Td(css.Class("td-amount", tw.ColorClass(props.AmountTone)), props.Amount)),
		If(props.ShowBalance, Td(ClassStr("td-amount"))),
		If(props.Vis.Source, Td(ClassStr("td-source"))),
		Td(ClassStr("td-actions txn-scheduled-status"), ui.CreateElement(txnScheduledGhostMenu, struct{}{})),
	)
}

func txnScheduledTableRows(items []bills.Bill, formatDate func(time.Time) string, vis uistate.TxnCols, showUser, showBalance bool) []any {
	rows := make([]any, 0, len(items))
	accounts, categories := map[string]string{}, map[string]string{}
	if app := appstate.Default; app != nil {
		for _, account := range app.Accounts() {
			accounts[account.ID] = account.Name
		}
		for _, category := range app.Categories() {
			categories[category.ID] = category.Name
		}
	}
	app := appstate.Default
	for _, item := range items {
		category := ""
		if app != nil {
			category = categories[billCategoryID(app, item)]
		}
		amount, tone := fmtLedgerAmount(money.New(-item.Amount.Amount, item.Amount.Currency), false)
		rows = append(rows, ui.CreateElement(txnScheduledTableRow, txnScheduledTableRowProps{
			Date: formatDate(item.DueDate), Name: item.Name, Category: category,
			Account: accounts[item.AccountID], Amount: amount, AmountTone: tone,
			Vis: vis, ShowUser: showUser, ShowBalance: showBalance,
		}))
	}
	return rows
}

func txnScheduledGhostMenu(struct{}) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	return Div(css.Class("txn-scheduled-menu"),
		Button(css.Class("btn btn-icon"), Type("button"),
			Attr("aria-label", uistate.T("fut2.scheduledMenuLabel")),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("data-testid", "txn-scheduled-menu-toggle"),
			OnClick(toggle), "⋯"),
		If(open.Get(), A(css.Class("txn-scheduled-menu-item"), Href(uistate.RoutePath("/recurring")),
			Attr("data-testid", "txn-scheduled-open"), uistate.T("fut2.scheduledMenuOpen"))),
	)
}

func txnFutureScheduledItems(filter uistate.TxFilter, now time.Time) []bills.Bill {
	start, err := time.ParseInLocation("2006-01-02", filter.From, now.Location())
	if err != nil || filter.From <= now.Format("2006-01-02") {
		return nil
	}
	var end time.Time
	if filter.To != "" {
		end, err = time.ParseInLocation("2006-01-02", filter.To, now.Location())
		if err != nil || end.Before(start) {
			return nil
		}
	}
	app := appstate.Default
	if app == nil {
		return nil
	}
	items := bills.UpcomingAll(app.Accounts(), app.Recurring(), now)
	filtered := make([]bills.Bill, 0, len(items))
	for _, item := range items {
		due := item.DueDate.Format("2006-01-02")
		if due < filter.From || (filter.To != "" && due > filter.To) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// txnFutureScheduleCount reports whether the selected date range starts after
// today and how many of the ledger's planned bill/recurring entries fall inside
// that range. It uses the same UpcomingAll source as the upcoming strip.
func txnFutureScheduleCount(filter uistate.TxFilter, now time.Time) (bool, int) {
	start, err := time.ParseInLocation("2006-01-02", filter.From, now.Location())
	if err != nil || filter.From <= now.Format("2006-01-02") {
		return false, 0
	}
	if filter.To != "" {
		end, parseErr := time.ParseInLocation("2006-01-02", filter.To, now.Location())
		if parseErr != nil || end.Before(start) {
			return false, 0
		}
	}
	app := appstate.Default
	if app == nil {
		return true, 0
	}
	count := 0
	for _, item := range bills.UpcomingAll(app.Accounts(), app.Recurring(), now) {
		due := item.DueDate.Format("2006-01-02")
		if due < filter.From || (filter.To != "" && due > filter.To) {
			continue
		}
		count++
	}
	return true, count
}
