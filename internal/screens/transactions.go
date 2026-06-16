//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
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
	errMsg := ui.UseState("")
	filterText := ui.UseState("")
	filterAcc := ui.UseState("")
	filterCat := ui.UseState("")

	onDesc := ui.UseEvent(func(v string) { desc.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kind.Set(e.GetValue()) })
	onAcc := ui.UseEvent(func(e ui.Event) { accID.Set(e.GetValue()) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	onToAcc := ui.UseEvent(func(e ui.Event) { toAccID.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tagsStr.Set(v) })
	onFilterText := ui.UseEvent(func(v string) { filterText.Set(v) })
	onFilterAcc := ui.UseEvent(func(e ui.Event) { filterAcc.Set(e.GetValue()) })
	onFilterCat := ui.UseEvent(func(e ui.Event) { filterCat.Set(e.GetValue()) })
	clearFilters := ui.UseEvent(Prevent(func() { filterText.Set(""); filterAcc.Set(""); filterCat.Set("") }))

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
			Tags: parseTags(tagsStr.Get()),
		}
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		desc.Set("")
		amountStr.Set("")
		tagsStr.Set("")
		errMsg.Set("")
		bump()
	}))

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
				Button(Class("btn btn-primary"), Type("submit"), "Add"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		)
	}

	txns := app.Transactions()
	sort.Slice(txns, func(i, j int) bool { return txns[i].Date.After(txns[j].Date) })

	ft := strings.ToLower(strings.TrimSpace(filterText.Get()))
	fa := filterAcc.Get()
	fc := filterCat.Get()
	shown := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if fa != "" && t.AccountID != fa {
			continue
		}
		if fc != "" && t.CategoryID != fc {
			continue
		}
		if ft != "" && !strings.Contains(strings.ToLower(t.Desc), ft) {
			continue
		}
		shown = append(shown, t)
	}

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
					Txn: t, Account: acc.Name, Category: catName[t.CategoryID], OnDelete: deleteTxn,
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

	return Div(
		formCard,
		Section(Class("card"),
			H2(Class("card-title"), "All transactions"),
			Form(Class("form-grid"), OnSubmit(clearFilters),
				Input(Class("field"), Type("search"), Placeholder("Search description"), Value(filterText.Get()), OnInput(onFilterText)),
				Select(Class("field"), OnChange(onFilterAcc), filterAccOptions),
				Select(Class("field"), OnChange(onFilterCat), filterCatOptions),
				Button(Class("btn"), Type("submit"), "Clear"),
			),
			listBody,
		),
	)
}

type transactionRowProps struct {
	Txn      domain.Transaction
	Account  string
	Category string
	OnDelete func(string)
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

// TransactionRow is a per-transaction row with a stable delete-handler hook.
func TransactionRow(props transactionRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Txn.ID) }))

	cat := props.Category
	switch {
	case props.Txn.IsTransfer():
		cat = "Transfer"
	case cat == "":
		cat = "Uncategorized"
	}
	meta := cat + " · " + dateutil.FormatDate(props.Txn.Date)
	if props.Account != "" {
		meta += " · " + props.Account
	}
	if len(props.Txn.Tags) > 0 {
		meta += " · #" + strings.Join(props.Txn.Tags, " #")
	}

	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Txn.Desc),
			Span(Class("row-meta"), meta),
		),
		Span(Class(amountClass(props.Txn.Amount)), fmtMoney(props.Txn.Amount)),
		Button(Class("btn-del"), Type("button"), Title("Delete transaction"), OnClick(del), "✕"),
	)
}
