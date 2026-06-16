//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Transactions is the global ledger: add income/expense, list newest first, delete.
func Transactions() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
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
	// Implicit auto-categorization: treat each category name as a match rule, so
	// typing "Groceries" in the description suggests that category.
	implicitRules := make([]rules.Rule, 0, len(categories))
	for _, c := range categories {
		implicitRules = append(implicitRules, rules.Rule{Match: c.Name, SetCategoryID: c.ID})
	}

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
	errMsg := ui.UseState("")
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
		// Auto-suggest a category from the description, but never override a choice.
		if strings.TrimSpace(catID.Get()) == "" {
			if cid := rules.Category(implicitRules, "", v); cid != "" {
				catID.Set(cid)
			}
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
	onSortBy := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Sort = e.GetValue() }) })
	onFilterMember := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Member = e.GetValue() }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })

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

	add := ui.UseEvent(Prevent(func() {
		acc, ok := accByID[accID.Get()]
		if !ok {
			errMsg.Set("Choose an account.")
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr.Get()), currency.Decimals(acc.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set("Enter a positive amount.")
			return
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr.Get()))
		if derr != nil {
			errMsg.Set("Enter a valid date (YYYY-MM-DD).")
			return
		}
		memberFor := func(a domain.Account) string {
			if a.Scope == domain.ScopeIndividual {
				return a.OwnerID
			}
			return ""
		}
		label := strings.TrimSpace(desc.Get())

		if kind.Get() == "Transfer" {
			toAcc, ok := accByID[toAccID.Get()]
			if !ok || toAcc.ID == acc.ID {
				errMsg.Set("Choose a different destination account.")
				return
			}
			if toAcc.Currency != acc.Currency {
				errMsg.Set("Transfers between different currencies aren't supported yet.")
				return
			}
			if label == "" {
				label = "Transfer"
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
			Tags: parseTags(tagsStr.Get()), Custom: customValuesToMap(txnDefs, customVals.Get()),
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

	editTxn := func(orig domain.Transaction, newDesc, amountStr, catID, dateStr string) {
		acc := accByID[orig.AccountID]
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(acc.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set("Enter a positive amount.")
			return
		}
		if orig.Amount.IsNegative() {
			amt = -amt // preserve the original income/expense sign
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr))
		if derr != nil {
			errMsg.Set("Enter a valid date (YYYY-MM-DD).")
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
		all := app.Transactions()
		var target domain.Transaction
		found := false
		for _, t := range all {
			if t.ID == txnID {
				target, found = t, true
				break
			}
		}
		if err := app.DeleteTransaction(txnID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// A transfer is two paired legs; delete the reciprocal so balances stay
		// consistent. The pair has the accounts swapped, the amount negated, and
		// the same date.
		if found && target.IsTransfer() {
			for _, t := range all {
				if t.ID != txnID && t.IsTransfer() &&
					t.AccountID == target.TransferAccountID &&
					t.TransferAccountID == target.AccountID &&
					t.Amount.Amount == -target.Amount.Amount &&
					t.Date.Equal(target.Date) {
					_ = app.DeleteTransaction(t.ID)
					break
				}
			}
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
	bulkDelete := ui.UseEvent(Prevent(func() {
		for id := range selected.Get() {
			deleteTxn(id)
		}
		selected.Set(map[string]bool{})
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
		formCard = Section(Class("card"), P(Class("empty"), "Add an account first, then you can record transactions."))
	} else {
		isTransfer := kind.Get() == "Transfer"
		kindOptions := []ui.Node{
			Option(Value("Expense"), SelectedIf(kind.Get() == "Expense"), "Expense"),
			Option(Value("Income"), SelectedIf(kind.Get() == "Income"), "Income"),
			Option(Value("Transfer"), SelectedIf(isTransfer), "Transfer"),
		}
		accOptions := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			accOptions = append(accOptions, Option(Value(a.ID), SelectedIf(accID.Get() == a.ID), a.Name))
		}
		catOptions := []ui.Node{Option(Value(""), SelectedIf(catID.Get() == ""), "— No category —")}
		for _, c := range categories {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		toAccOptions := []ui.Node{Option(Value(""), SelectedIf(toAccID.Get() == ""), "— To account —")}
		for _, a := range accounts {
			toAccOptions = append(toAccOptions, Option(Value(a.ID), SelectedIf(toAccID.Get() == a.ID), a.Name))
		}
		accLabel := "Account"
		if isTransfer {
			accLabel = "From account"
		}
		// Custom fields apply to income/expense entries, not transfer legs.
		formTxnDefs := txnDefs
		if isTransfer {
			formTxnDefs = nil
		}
		formCard = Section(Class("card"),
			H2(Class("card-title"), "Add transaction"),
			Form(Class("form-grid"), OnSubmit(add),
				Input(Class("field"), Type("text"), Placeholder("Description"), Value(desc.Get()), OnInput(onDesc)),
				Input(Class("field"), Type("number"), Placeholder("Amount"), Value(amountStr.Get()), Step("0.01"), OnInput(onAmount)),
				Select(Class("field"), OnChange(onKind), kindOptions),
				Select(Class("field"), Title(accLabel), OnChange(onAcc), accOptions),
				IfElse(isTransfer,
					Select(Class("field"), Title("To account"), OnChange(onToAcc), toAccOptions),
					Select(Class("field"), OnChange(onCat), catOptions),
				),
				If(!isTransfer, Input(Class("field"), Type("text"), Placeholder("Tags (comma-separated)"), Value(tagsStr.Get()), OnInput(onTags))),
				Input(Class("field"), Type("date"), Value(dateStr.Get()), OnInput(onDate)),
				MapKeyed(formTxnDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(Class("btn btn-primary"), Type("submit"), "Add"),
				Button(Class("btn"), Type("button"), Title("Copy the most recent transaction"), OnClick(repeatLast), "Repeat last"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		)
	}

	txns := app.Transactions()
	sort.Slice(txns, func(i, j int) bool { return txns[i].Date.After(txns[j].Date) })

	ft := strings.ToLower(strings.TrimSpace(f.Text))
	fa := f.Account
	fc := f.Category
	fm := f.Member
	var fromT, toT time.Time
	if s := strings.TrimSpace(f.From); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			fromT = d
		}
	}
	if s := strings.TrimSpace(f.To); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			toT = d
		}
	}
	shown := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if fa != "" && t.AccountID != fa {
			continue
		}
		if fc != "" && t.CategoryID != fc {
			continue
		}
		if fm != "" && t.MemberID != fm {
			continue
		}
		if !fromT.IsZero() && t.Date.Before(fromT) {
			continue
		}
		if !toT.IsZero() && t.Date.After(toT) {
			continue
		}
		if ft != "" && !matchesText(t, ft) {
			continue
		}
		shown = append(shown, t)
	}

	switch f.Sort {
	case "amount":
		sort.Slice(shown, func(i, j int) bool { return absAmount(shown[i]) > absAmount(shown[j]) })
	case "payee":
		sort.Slice(shown, func(i, j int) bool { return strings.ToLower(shown[i].Desc) < strings.ToLower(shown[j].Desc) })
	}
	// "date" keeps the newest-first order already applied above.

	var listBody ui.Node
	switch {
	case len(txns) == 0:
		listBody = P(Class("empty"), "No transactions yet.")
	case len(shown) == 0:
		listBody = P(Class("empty"), "No matching transactions.")
	default:
		rows := MapKeyed(shown,
			func(t domain.Transaction) any { return t.ID },
			func(t domain.Transaction) ui.Node {
				acc := accByID[t.AccountID]
				return ui.CreateElement(TransactionRow, transactionRowProps{
					Txn: t, Account: acc.Name, Category: catName[t.CategoryID], Categories: categories,
					Selected: selected.Get()[t.ID],
					OnDelete: deleteTxn, OnDuplicate: duplicateTxn, OnSave: editTxn, OnToggleSelect: toggleSelect,
				})
			},
		)
		listBody = Div(Class("rows"), rows)
	}

	filterAccOptions := []ui.Node{Option(Value(""), SelectedIf(fa == ""), "— All accounts —")}
	for _, a := range accounts {
		filterAccOptions = append(filterAccOptions, Option(Value(a.ID), SelectedIf(fa == a.ID), a.Name))
	}
	filterCatOptions := []ui.Node{Option(Value(""), SelectedIf(fc == ""), "— All categories —")}
	for _, c := range categories {
		filterCatOptions = append(filterCatOptions, Option(Value(c.ID), SelectedIf(fc == c.ID), c.Name))
	}
	filterMemberOptions := []ui.Node{Option(Value(""), SelectedIf(fm == ""), "— All members —")}
	for _, m := range app.Members() {
		filterMemberOptions = append(filterMemberOptions, Option(Value(m.ID), SelectedIf(fm == m.ID), m.Name))
	}

	return Div(
		formCard,
		Section(Class("card"),
			H2(Class("card-title"), "All transactions"),
			Form(Class("form-grid"), OnSubmit(clearFilters),
				Input(Class("field"), Type("search"), Placeholder("Search description or tag"), Value(f.Text), OnInput(onFilterText)),
				Select(Class("field"), OnChange(onFilterAcc), filterAccOptions),
				Select(Class("field"), OnChange(onFilterCat), filterCatOptions),
				Select(Class("field"), Title("Member"), OnChange(onFilterMember), filterMemberOptions),
				Input(Class("field"), Type("date"), Title("From date"), Value(f.From), OnInput(onFilterFrom)),
				Input(Class("field"), Type("date"), Title("To date"), Value(f.To), OnInput(onFilterTo)),
				Select(Class("field"), Title("Sort by"), OnChange(onSortBy),
					Option(Value("date"), SelectedIf(f.Sort == "date"), "Newest first"),
					Option(Value("amount"), SelectedIf(f.Sort == "amount"), "Largest amount"),
					Option(Value("payee"), SelectedIf(f.Sort == "payee"), "Payee A–Z"),
				),
				Button(Class("btn"), Type("submit"), "Clear"),
			),
			If(len(selected.Get()) > 0, Div(Class("flex flex-wrap gap-2 items-center"), Style(map[string]string{"margin-bottom": "0.6rem"}),
				Span(Class("muted"), plural(len(selected.Get()), "transaction")+" selected"),
				Button(Class("btn-del"), Type("button"), Title("Delete the selected transactions"), OnClick(bulkDelete), "Delete selected"),
				Button(Class("btn"), Type("button"), OnClick(clearSelection), "Clear selection"),
			)),
			listBody,
		),
	)
}

type transactionRowProps struct {
	Txn            domain.Transaction
	Account        string
	Category       string
	Categories     []domain.Category // for the edit-mode category picker
	Selected       bool
	OnDelete       func(string)
	OnDuplicate    func(domain.Transaction)
	OnSave         func(orig domain.Transaction, desc, amount, categoryID, date string)
	OnToggleSelect func(string)
}

// absAmount returns the absolute minor-unit amount of a transaction (for sorting
// by size regardless of income/expense sign).
func absAmount(t domain.Transaction) int64 {
	a := t.Amount.Amount
	if a < 0 {
		return -a
	}
	return a
}

// matchesText reports whether the (already-lowercased) query appears in a
// transaction's description or any of its tags.
func matchesText(t domain.Transaction, q string) bool {
	if strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

// parseTags splits a comma-separated string into trimmed, non-empty tags.
func parseTags(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// TransactionRow is a per-transaction row. Income/expense rows can be edited
// inline (description, amount, category, date); transfers cannot. All hooks are
// declared unconditionally so the edit toggle never reorders them.
func TransactionRow(props transactionRowProps) ui.Node {
	t := props.Txn
	amountMajor := money.FormatMinor(absAmount(t), currency.Decimals(t.Amount.Currency))
	dateISO := dateutil.FormatDate(t.Date)

	del := ui.UseEvent(Prevent(func() { props.OnDelete(t.ID) }))
	dup := ui.UseEvent(Prevent(func() { props.OnDuplicate(t) }))
	sel := ui.UseEvent(Prevent(func() { props.OnToggleSelect(t.ID) }))
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

	if editing.Get() {
		catOptions := []ui.Node{Option(Value(""), SelectedIf(catS.Get() == ""), "— No category —")}
		for _, c := range props.Categories {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catS.Get() == c.ID), c.Name))
		}
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Description"), Value(descS.Get()), OnInput(onDesc)),
				Input(Class("field"), Type("number"), Placeholder("Amount"), Value(amountS.Get()), Step("0.01"), OnInput(onAmount)),
				Select(Class("field"), OnChange(onCat), catOptions),
				Input(Class("field"), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

	cat := props.Category
	switch {
	case props.Txn.IsTransfer():
		cat = "Transfer"
	case cat == "":
		cat = "Uncategorized"
	}
	meta := cat + " · " + pr.FormatDate(props.Txn.Date)
	if props.Account != "" {
		meta += " · " + props.Account
	}
	if len(props.Txn.Tags) > 0 {
		meta += " · #" + strings.Join(props.Txn.Tags, " #")
	}

	selectGlyph := "☐"
	if props.Selected {
		selectGlyph = "☑"
	}
	return Div(Class("row"),
		Button(Class("check"), Type("button"), Title("Select for bulk actions"), OnClick(sel), selectGlyph),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Txn.Desc),
			Span(Class("row-meta"), meta),
		),
		Span(Class(amountClass(props.Txn.Amount)), fmtMoney(props.Txn.Amount)),
		If(!props.Txn.IsTransfer(), Button(Class("btn"), Type("button"), Title("Edit this transaction"), OnClick(startEdit), "Edit")),
		If(!props.Txn.IsTransfer(), Button(Class("btn"), Type("button"), Title("Copy this transaction to today"), OnClick(dup), "Duplicate")),
		Button(Class("btn-del"), Type("button"), Title("Delete transaction"), OnClick(del), "✕"),
	)
}
