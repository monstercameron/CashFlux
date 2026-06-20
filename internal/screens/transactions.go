//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// txnPageSize is how many filtered transactions render per page before a
// "Show more" reveals the next batch (C39 — keeps long ledgers responsive).
const txnPageSize = 50

// Transactions is the global ledger: add income/expense, list newest first, delete.
func Transactions() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:transactions", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	accounts := app.Accounts()
	categories := app.Categories()

	accByID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	accName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		accName[a.ID] = a.Name
	}
	// Auto-categorization: the user's saved rules take priority (first match wins,
	// and they can also assign tags), then fall back to implicit rules that treat
	// each category name as a match — so typing "Groceries" suggests that category.
	desc := ui.UseState("")
	amountStr := ui.UseState("")
	kind := ui.UseState("Expense")
	defaultAcc := ""
	if len(accounts) > 0 {
		defaultAcc = accounts[0].ID
	}
	accID := ui.UseState(defaultAcc)
	catID := ui.UseState("")
	toAccID := ui.UseState("")
	tagsStr := ui.UseState("")
	dateStr := ui.UseState(time.Now().Format(dateutil.Layout))
	customVals := ui.UseState(map[string]string{})
	selected := ui.UseState(map[string]bool{})
	visN := ui.UseState(txnPageSize) // how many filtered rows to render (paginated, C39)
	bulkCat := ui.UseState("")
	errMsg := ui.UseState("")
	noticeAtom := uistate.UseNotice()
	notifyErr := func(text string) { noticeAtom.Set(noticeAtom.Get().With(text, true)) }
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	setFilter := func(mut func(*uistate.TxFilter)) {
		nf := filterAtom.Get()
		mut(&nf)
		nf = nf.Normalize()
		filterAtom.Set(nf)
		uistate.PersistTxFilter(nf)
	}

	onDesc := ui.UseEvent(func(v string) {
		desc.Set(v)
		// Auto-suggest from the description via the matching rule, but never override
		// a category or tags the user already entered.
		nextCat, nextTags := app.SuggestTransactionFields(v, catID.Get(), textutil.CommaFields(tagsStr.Get()))
		catID.Set(nextCat)
		if len(nextTags) > 0 && strings.TrimSpace(tagsStr.Get()) == "" {
			tagsStr.Set(strings.Join(nextTags, ", "))
		}
	})
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kind.Set(e.GetValue()) })
	onAcc := ui.UseEvent(func(e ui.Event) { accID.Set(e.GetValue()) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	onToAcc := ui.UseEvent(func(e ui.Event) { toAccID.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tagsStr.Set(v) })
	onFilterText := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.Text = v }) })
	onFilterAcc := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Account = e.GetValue() }) })
	onFilterCat := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Category = e.GetValue() }) })
	// Click a column header to sort by it; click the active column again to flip
	// direction. (Replaces the old Sort dropdown — C47.)
	sortBy := func(key string) {
		setFilter(func(x *uistate.TxFilter) {
			if x.Sort == key {
				if x.Dir == txnfilter.Asc {
					x.Dir = txnfilter.Desc
				} else {
					x.Dir = txnfilter.Asc
				}
			} else {
				x.Sort, x.Dir = key, txnfilter.DefaultDir(key)
			}
		})
	}
	onFilterMember := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Member = e.GetValue() }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })
	onFilterCleared := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Cleared = e.GetValue() }) })

	txnDefs := app.CustomFieldDefsFor("transaction")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}
	clearFilters := ui.UseEvent(Prevent(func() {
		cleared := uistate.TxFilter{}.Normalize()
		filterAtom.Set(cleared)
		uistate.PersistTxFilter(cleared)
	}))
	exportFiltered := ui.UseEvent(Prevent(func() {
		rows := txnfilter.Apply(app.Transactions(), filterAtom.Get())
		if len(rows) == 0 {
			errMsg.Set(uistate.T("transactions.noExport"))
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			errMsg.Set(err.Error())
			return
		}
		downloadBytes("transactions.csv", "text/csv", data)
	}))

	add := ui.UseEvent(Prevent(func() {
		acc, ok := accByID[accID.Get()]
		if !ok {
			errMsg.Set(uistate.T("transactions.chooseAccount"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr.Get()), currency.Decimals(acc.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("transactions.positiveAmount"))
			return
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr.Get()))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		memberFor := app.MemberForNewTransaction
		label := strings.TrimSpace(desc.Get())

		if kind.Get() == "Transfer" {
			toAcc, ok := accByID[toAccID.Get()]
			if !ok || toAcc.ID == acc.ID {
				errMsg.Set(uistate.T("transactions.diffDestination"))
				return
			}
			if toAcc.Currency != acc.Currency {
				errMsg.Set(uistate.T("transactions.transferCurrency"))
				return
			}
			if label == "" {
				label = uistate.T("transactions.transfer")
			}
			out := domain.Transaction{
				ID: id.New(), AccountID: acc.ID, Date: date, Desc: label,
				Amount: money.New(-amt, acc.Currency), TransferAccountID: toAcc.ID, MemberID: memberFor(acc),
			}
			in := domain.Transaction{
				ID: id.New(), AccountID: toAcc.ID, Date: date, Desc: label,
				Amount: money.New(amt, toAcc.Currency), TransferAccountID: acc.ID, MemberID: memberFor(toAcc),
			}
			if err := app.PutTransaction(out); err != nil {
				errMsg.Set(err.Error())
				return
			}
			if err := app.PutTransaction(in); err != nil {
				errMsg.Set(err.Error())
				return
			}
			desc.Set("")
			amountStr.Set("")
			errMsg.Set("")
			bump()
			return
		}

		if kind.Get() == "Expense" {
			amt = -amt
		}
		t := domain.Transaction{
			ID: id.New(), AccountID: acc.ID, Date: date, Desc: label,
			CategoryID: catID.Get(), Amount: money.New(amt, acc.Currency), MemberID: memberFor(acc),
			Tags: textutil.CommaFields(tagsStr.Get()), Custom: customValuesToMap(txnDefs, customVals.Get()),
		}
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		desc.Set("")
		amountStr.Set("")
		tagsStr.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		bump()
	}))

	duplicateTxn := func(t domain.Transaction) {
		cp := t
		cp.ID = id.New()
		cp.Date = time.Now()
		cp.TransferAccountID = "" // a duplicate is a standalone entry, not a transfer leg
		if len(t.Tags) > 0 {
			cp.Tags = append([]string(nil), t.Tags...)
		}
		if err := app.PutTransaction(cp); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	toggleCleared := func(t domain.Transaction) {
		t.Cleared = !t.Cleared
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	editTxn := func(orig domain.Transaction, newDesc, amountStr, catID, dateStr string) {
		acc := accByID[orig.AccountID]
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(acc.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("transactions.positiveAmount"))
			return
		}
		if orig.Amount.IsNegative() {
			amt = -amt // preserve the original income/expense sign
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		orig.Desc = strings.TrimSpace(newDesc)
		orig.Amount = money.New(amt, orig.Amount.Currency)
		orig.CategoryID = catID
		orig.Date = date
		if err := app.PutTransaction(orig); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	deleteTxn := func(txnID string) {
		if err := app.DeleteTransactionWithTransferPair(txnID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	toggleSelect := func(txnID string) {
		m := selected.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if nm[txnID] {
			delete(nm, txnID)
		} else {
			nm[txnID] = true
		}
		selected.Set(nm)
	}
	clearSelection := ui.UseEvent(Prevent(func() { selected.Set(map[string]bool{}) }))
	// Select the extra copies in each duplicate group (all but the first), so the
	// existing bulk-delete can clean them up in one go.
	selectDuplicates := ui.UseEvent(Prevent(func() {
		nm := map[string]bool{}
		for _, g := range dedupe.FindDuplicates(app.Transactions()) {
			for _, dupID := range g.IDs[1:] {
				nm[dupID] = true
			}
		}
		selected.Set(nm)
	}))
	bulkDelete := ui.UseEvent(Prevent(func() {
		for id := range selected.Get() {
			deleteTxn(id)
		}
		selected.Set(map[string]bool{})
	}))
	bulkSetCleared := func(val bool) {
		sel := selected.Get()
		for _, t := range app.Transactions() {
			if !sel[t.ID] || t.Cleared == val {
				continue
			}
			t.Cleared = val
			if err := app.PutTransaction(t); err != nil {
				notifyErr(uistate.T("transactions.bulkClearErr", err.Error()))
			}
		}
		selected.Set(map[string]bool{})
		bump()
	}
	bulkMarkCleared := ui.UseEvent(Prevent(func() { bulkSetCleared(true) }))
	bulkMarkUncleared := ui.UseEvent(Prevent(func() { bulkSetCleared(false) }))
	onBulkCat := ui.UseEvent(func(e ui.Event) { bulkCat.Set(e.GetValue()) })
	bulkRecategorize := ui.UseEvent(Prevent(func() {
		sel := selected.Get()
		cid := bulkCat.Get()
		for _, t := range app.Transactions() {
			if !sel[t.ID] || t.IsTransfer() {
				continue
			}
			t.CategoryID = cid
			if err := app.PutTransaction(t); err != nil {
				notifyErr(uistate.T("transactions.bulkRecatErr", err.Error()))
			}
		}
		selected.Set(map[string]bool{})
		bulkCat.Set("")
		bump()
	}))

	repeatLast := ui.UseEvent(func() {
		all := app.Transactions()
		if len(all) == 0 {
			return
		}
		newest := all[0]
		for _, t := range all[1:] {
			if t.Date.After(newest.Date) {
				newest = t
			}
		}
		desc.Set(newest.Desc)
		accID.Set(newest.AccountID)
		catID.Set(newest.CategoryID)
		switch {
		case newest.IsTransfer():
			kind.Set("Transfer")
			toAccID.Set(newest.TransferAccountID)
		case newest.Amount.IsNegative():
			kind.Set("Expense")
		default:
			kind.Set("Income")
		}
		amt := newest.Amount.Amount
		if amt < 0 {
			amt = -amt
		}
		amountStr.Set(money.FormatMinor(amt, currency.Decimals(accByID[newest.AccountID].Currency)))
	})

	var formCard ui.Node
	if len(accounts) == 0 {
		formCard = Section(Class("card"), P(Class("empty"), uistate.T("transactions.needAccount")))
	} else {
		isTransfer := kind.Get() == "Transfer"
		kindOptions := []ui.Node{
			Option(Value("Expense"), SelectedIf(kind.Get() == "Expense"), uistate.T("category.expense")),
			Option(Value("Income"), SelectedIf(kind.Get() == "Income"), uistate.T("category.income")),
			Option(Value("Transfer"), SelectedIf(isTransfer), uistate.T("transactions.transfer")),
		}
		accOptions := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			accOptions = append(accOptions, Option(Value(a.ID), SelectedIf(accID.Get() == a.ID), a.Name))
		}
		catOptions := []ui.Node{Option(Value(""), SelectedIf(catID.Get() == ""), uistate.T("transactions.noCategory"))}
		for _, c := range categories {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		toAccOptions := []ui.Node{Option(Value(""), SelectedIf(toAccID.Get() == ""), uistate.T("transactions.toAccountOpt"))}
		for _, a := range accounts {
			toAccOptions = append(toAccOptions, Option(Value(a.ID), SelectedIf(toAccID.Get() == a.ID), a.Name))
		}
		accLabel := uistate.T("transactions.account")
		if isTransfer {
			accLabel = uistate.T("transactions.fromAccount")
		}
		// Custom fields apply to income/expense entries, not transfer legs.
		formTxnDefs := txnDefs
		if isTransfer {
			formTxnDefs = nil
		}
		formCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("transactions.addTitle")),
			Form(Class("form-grid"), OnSubmit(add),
				Input(append([]any{Class("field"), Attr("id", "txn-add"), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), Value(desc.Get()), OnInput(onDesc)}, errAttrs("txn-err", errMsg.Get())...)...),
				Input(Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("transactions.amountPlaceholder")), Value(amountStr.Get()), Step("0.01"), OnInput(onAmount)),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.kindLabel")), OnChange(onKind), kindOptions),
				Select(Class("field"), Attr("aria-label", accLabel), Title(accLabel), OnChange(onAcc), accOptions),
				IfElse(isTransfer,
					Select(Class("field"), Attr("aria-label", uistate.T("transactions.toAccount")), Title(uistate.T("transactions.toAccount")), OnChange(onToAcc), toAccOptions),
					Select(Class("field"), Attr("aria-label", uistate.T("transactions.categoryLabel")), OnChange(onCat), catOptions),
				),
				If(!isTransfer, Input(Class("field"), Type("text"), Placeholder(uistate.T("transactions.tagsPlaceholder")), Value(tagsStr.Get()), OnInput(onTags))),
				Input(Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), Value(dateStr.Get()), OnInput(onDate)),
				MapKeyed(formTxnDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.repeatLastTitle")), OnClick(repeatLast), uistate.T("transactions.repeatLast")),
			),
			errText("txn-err", errMsg.Get()),
		)
	}

	txns := app.Transactions()
	shown := txnfilter.ApplyWithLabels(txns, f, txnfilter.Labels{Account: accName, Category: catName})

	// Heads-up for likely double entries (same date, amount, and description).
	dupCount := dedupe.Count(dedupe.FindDuplicates(txns))

	// Summary of the shown set: count + net total converted to the base currency.
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	var shownNet int64
	for _, t := range shown {
		if c, err := rates.Convert(t.Amount, base); err == nil {
			shownNet += c.Amount
		}
	}

	// Status text for the screen-reader live region: how many transactions match
	// the current filters, announced as the filters change. Mirrors the visible
	// summary, but also covers the zero-results case (the visible summary hides at
	// zero, so without this the "no matches" outcome would never be announced).
	filterStatus := ""
	switch {
	case len(txns) == 0:
		filterStatus = ""
	case len(shown) == 0:
		filterStatus = uistate.T("transactions.noMatch")
	default:
		filterStatus = uistate.T("transactions.summary", plural(len(shown), "transaction"), fmtMoney(money.New(shownNet, base)))
	}

	var listBody ui.Node
	switch {
	case len(txns) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("transactions.empty"), CTALabel: uistate.T("transactions.addFirst"), FocusID: "txn-add"})
	case len(shown) == 0:
		listBody = P(Class("empty"), uistate.T("transactions.noMatch"))
	default:
		// Paginate: render only the first visN rows so a long ledger stays fast,
		// with a "Show more" button to reveal the next page (C39).
		page := shown
		hidden := 0
		if n := visN.Get(); len(shown) > n {
			page = shown[:n]
			hidden = len(shown) - n
		}
		rows := MapKeyed(page,
			func(t domain.Transaction) any { return t.ID },
			func(t domain.Transaction) ui.Node {
				acc := accByID[t.AccountID]
				return ui.CreateElement(TransactionRow, transactionRowProps{
					Txn: t, Account: acc.Name, Category: catName[t.CategoryID], Categories: categories,
					Selected: selected.Get()[t.ID],
					OnDelete: deleteTxn, OnDuplicate: duplicateTxn, OnSave: editTxn, OnToggleSelect: toggleSelect, OnToggleCleared: toggleCleared,
				})
			},
		)
		head := Tr(
			Th(Attr("scope", "col"), Span(Class("sr-only"), "Select")),
			ui.CreateElement(sortTh, sortThProps{Key: "date", Label: "Date", Active: f.Sort, Dir: f.Dir, OnSort: sortBy}),
			ui.CreateElement(sortTh, sortThProps{Key: "payee", Label: "Description", Active: f.Sort, Dir: f.Dir, OnSort: sortBy}),
			ui.CreateElement(sortTh, sortThProps{Key: "category", Label: "Category", Active: f.Sort, Dir: f.Dir, OnSort: sortBy}),
			ui.CreateElement(sortTh, sortThProps{Key: "account", Label: "Account", Active: f.Sort, Dir: f.Dir, OnSort: sortBy}),
			Th(Attr("scope", "col"), "Tags"),
			ui.CreateElement(sortTh, sortThProps{Key: "amount", Label: "Amount", Active: f.Sort, Dir: f.Dir, OnSort: sortBy}),
			Th(Attr("scope", "col"), "Cleared"),
			Th(Attr("scope", "col"), "Actions"),
		)
		listBody = Div(
			Table(Class("txn-table"), Thead(head), Tbody(rows)),
			If(hidden > 0, Div(Class("flex justify-center py-1"),
				Button(Class("btn"), Type("button"), OnClick(func() { visN.Set(visN.Get() + txnPageSize) }),
					uistate.T("transactions.showMore", hidden)),
			)),
		)
	}

	filterAccOptions := []ui.Node{Option(Value(""), SelectedIf(f.Account == ""), uistate.T("transactions.allAccounts"))}
	for _, a := range accounts {
		filterAccOptions = append(filterAccOptions, Option(Value(a.ID), SelectedIf(f.Account == a.ID), a.Name))
	}
	filterCatOptions := []ui.Node{Option(Value(""), SelectedIf(f.Category == ""), uistate.T("transactions.allCategories"))}
	for _, c := range categories {
		filterCatOptions = append(filterCatOptions, Option(Value(c.ID), SelectedIf(f.Category == c.ID), c.Name))
	}
	filterMemberOptions := []ui.Node{Option(Value(""), SelectedIf(f.Member == ""), uistate.T("transactions.allMembers"))}
	for _, m := range app.Members() {
		filterMemberOptions = append(filterMemberOptions, Option(Value(m.ID), SelectedIf(f.Member == m.ID), m.Name))
	}
	bulkCatOptions := []ui.Node{Option(Value(""), SelectedIf(bulkCat.Get() == ""), uistate.T("transactions.bulkNoCategory"))}
	for _, c := range categories {
		bulkCatOptions = append(bulkCatOptions, Option(Value(c.ID), SelectedIf(bulkCat.Get() == c.ID), c.Name))
	}

	return Div(
		formCard,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("transactions.listTitle")),
			Form(Class("form-grid"), OnSubmit(clearFilters),
				Input(Class("field"), Type("search"), Attr("aria-label", uistate.T("transactions.searchPlaceholder")), Placeholder(uistate.T("transactions.searchPlaceholder")), Value(f.Text), OnInput(onFilterText)),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.filterAccount")), OnChange(onFilterAcc), filterAccOptions),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.filterCategory")), OnChange(onFilterCat), filterCatOptions),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.member")), Title(uistate.T("transactions.member")), OnChange(onFilterMember), filterMemberOptions),
				Input(Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.fromDate")), Title(uistate.T("transactions.fromDate")), Value(f.From), OnInput(onFilterFrom)),
				Input(Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.toDate")), Title(uistate.T("transactions.toDate")), Value(f.To), OnInput(onFilterTo)),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.clearedStatus")), Title(uistate.T("transactions.clearedStatus")), OnChange(onFilterCleared),
					Option(Value(""), SelectedIf(f.Cleared == ""), uistate.T("transactions.clearedAll")),
					Option(Value("no"), SelectedIf(f.Cleared == "no"), uistate.T("transactions.notCleared")),
					Option(Value("yes"), SelectedIf(f.Cleared == "yes"), uistate.T("transactions.cleared")),
				),
				Button(Class("btn"), Type("submit"), uistate.T("transactions.clear")),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.exportTitle")), OnClick(exportFiltered), uistate.T("transactions.exportCsv")),
			),
			If(len(selected.Get()) > 0, Div(Class("flex flex-wrap gap-2 items-center"), Style(map[string]string{"margin-bottom": "0.6rem"}),
				Span(Class("muted"), uistate.T("transactions.selected", plural(len(selected.Get()), "transaction"))),
				Select(Class("field"), Attr("aria-label", uistate.T("transactions.categoryToApply")), Title(uistate.T("transactions.categoryToApply")), OnChange(onBulkCat), bulkCatOptions),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.applyCategoryTitle")), OnClick(bulkRecategorize), uistate.T("transactions.applyCategory")),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.markClearedTitle")), OnClick(bulkMarkCleared), uistate.T("transactions.markCleared")),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.markUnclearedTitle")), OnClick(bulkMarkUncleared), uistate.T("transactions.markUncleared")),
				Button(Class("btn-del"), Type("button"), Title(uistate.T("transactions.deleteSelectedTitle")), OnClick(bulkDelete), uistate.T("transactions.deleteSelected")),
				Button(Class("btn"), Type("button"), OnClick(clearSelection), uistate.T("transactions.clearSelection")),
			)),
			If(len(shown) > 0, P(Class("muted"), Attr("aria-hidden", "true"), Text(uistate.T("transactions.summary", plural(len(shown), "transaction"), fmtMoney(money.New(shownNet, base)))))),
			// Screen-reader live region announcing the match count as filters change
			// (stays mounted across renders, so the zero-results case is announced too).
			P(Class("sr-only"), Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-atomic", "true"), Text(filterStatus)),
			If(dupCount > 0, Div(Class("flex flex-wrap items-center gap-2"), Style(map[string]string{"margin-bottom": "0.6rem"}),
				Span(Class("muted"), uistate.T("transactions.dupNotice", plural(dupCount, "possible duplicate"))),
				Button(Class("btn"), Type("button"), Title(uistate.T("transactions.selectDuplicatesTitle")), OnClick(selectDuplicates), uistate.T("transactions.selectDuplicates")),
			)),
			listBody,
		),
	)
}

type transactionRowProps struct {
	Txn             domain.Transaction
	Account         string
	Category        string
	Categories      []domain.Category // for the edit-mode category picker
	Selected        bool
	OnDelete        func(string)
	OnDuplicate     func(domain.Transaction)
	OnSave          func(orig domain.Transaction, desc, amount, categoryID, date string)
	OnToggleSelect  func(string)
	OnToggleCleared func(domain.Transaction)
}

// (Transaction filtering/sorting now lives in the pure, tested internal/txnfilter
// package; see txnfilter.Apply and txnfilter.AbsAmount.)

// parseTags splits a comma-separated string into trimmed, non-empty tags.
// TransactionRow is a per-transaction row. Income/expense rows can be edited
// inline (description, amount, category, date); transfers cannot. All hooks are
// declared unconditionally so the edit toggle never reorders them.
func TransactionRow(props transactionRowProps) ui.Node {
	t := props.Txn
	amountMajor := money.FormatMinor(txnfilter.AbsAmount(t), currency.Decimals(t.Amount.Currency))
	dateISO := dateutil.FormatDate(t.Date)

	del := ui.UseEvent(Prevent(func() { props.OnDelete(t.ID) }))
	dup := ui.UseEvent(Prevent(func() { props.OnDuplicate(t) }))
	sel := ui.UseEvent(Prevent(func() { props.OnToggleSelect(t.ID) }))
	clr := ui.UseEvent(Prevent(func() { props.OnToggleCleared(t) }))
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	descS := ui.UseState(t.Desc)
	amountS := ui.UseState(amountMajor)
	catS := ui.UseState(t.CategoryID)
	dateS := ui.UseState(dateISO)
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catS.Set(e.GetValue()) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		descS.Set(t.Desc)
		amountS.Set(amountMajor)
		catS.Set(t.CategoryID)
		dateS.Set(dateISO)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(t, descS.Get(), amountS.Get(), catS.Get(), dateS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("txn-edit-" + t.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		catOptions := []ui.Node{Option(Value(""), SelectedIf(catS.Get() == ""), uistate.T("transactions.noCategory"))}
		for _, c := range props.Categories {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catS.Get() == c.ID), c.Name))
		}
		return Tr(Class("row-edit"),
			Td(Attr("colspan", "9"),
				Form(Class("form-grid"), OnSubmit(saveEdit),
					Input(Class("field"), Attr("id", "txn-edit-"+t.ID), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), Value(descS.Get()), OnInput(onDesc)),
					Input(Class("field"), Type("number"), Placeholder(uistate.T("transactions.amountPlaceholder")), Value(amountS.Get()), Step("0.01"), OnInput(onAmount)),
					Select(Class("field"), Attr("aria-label", uistate.T("transactions.categoryLabel")), OnChange(onCat), catOptions),
					Input(Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), Value(dateS.Get()), OnInput(onDate)),
					Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
					Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
				),
			),
		)
	}

	cat := props.Category
	switch {
	case props.Txn.IsTransfer():
		cat = uistate.T("transactions.transfer")
	case cat == "":
		cat = uistate.T("transactions.uncategorized")
	}
	tagsText := ""
	if len(props.Txn.Tags) > 0 {
		tagsText = "#" + strings.Join(props.Txn.Tags, " #")
	}

	selectGlyph := "☐"
	if props.Selected {
		selectGlyph = "☑"
	}
	clearedLabel := uistate.T("transactions.markCleared")
	if t.Cleared {
		clearedLabel = uistate.T("transactions.clearedCheck")
	}
	rowClass := "row"
	if props.Selected {
		rowClass += " selected"
	}
	return Tr(Class(rowClass),
		Td(Class("td-select"), Button(Class("check"), Type("button"), Title(uistate.T("transactions.selectTitle")), OnClick(sel), selectGlyph)),
		Td(Class("td-date fig"), pr.FormatDate(props.Txn.Date)),
		Td(Class("row-desc"), props.Txn.Desc),
		Td(Class("td-cat"), cat),
		Td(Class("td-acct"), props.Account),
		Td(Class("td-tags"), tagsText),
		Td(Class("td-amount fig "+amountClass(props.Txn.Amount)), fmtMoney(props.Txn.Amount)),
		Td(Class("td-cleared"), Button(Class("btn"), Type("button"), Title(uistate.T("transactions.toggleClearedTitle")), OnClick(clr), clearedLabel)),
		Td(Class("td-actions"),
			If(!props.Txn.IsTransfer(), Button(Class("btn inline-flex items-center gap-1.5"), Type("button"), Title(uistate.T("transactions.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, Class("w-4 h-4 shrink-0")), Span(uistate.T("action.edit")))),
			If(!props.Txn.IsTransfer(), Button(Class("btn"), Type("button"), Title(uistate.T("transactions.duplicateTitle")), OnClick(dup), uistate.T("transactions.duplicate"))),
			Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("transactions.deleteTitle")), Title(uistate.T("transactions.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
		),
	)
}

// sortThProps configures one sortable column header.
type sortThProps struct {
	Key, Label, Active, Dir string
	OnSort                  func(string)
}

// sortTh renders a sortable column header: a real <button> that sorts by Key (and
// flips direction when already active), with a caret and aria-sort on the active
// column. Its own component so the click hook stays stable across the header row.
func sortTh(props sortThProps) ui.Node {
	ariaSort, caret := "none", ""
	if props.Active == props.Key {
		if props.Dir == txnfilter.Asc {
			ariaSort, caret = "ascending", " ▲"
		} else {
			ariaSort, caret = "descending", " ▼"
		}
	}
	key := props.Key
	on := ui.UseEvent(Prevent(func() { props.OnSort(key) }))
	return Th(Attr("scope", "col"), Attr("aria-sort", ariaSort),
		Button(Class("th-sort"), Type("button"), OnClick(on), props.Label+caret),
	)
}
