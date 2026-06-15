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
	dateStr := ui.UseState(time.Now().Format(dateutil.Layout))
	errMsg := ui.UseState("")

	onDesc := ui.UseEvent(func(v string) { desc.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kind.Set(e.GetValue()) })
	onAcc := ui.UseEvent(func(e ui.Event) { accID.Set(e.GetValue()) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })

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
		if kind.Get() == "Expense" {
			amt = -amt
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr.Get()))
		if derr != nil {
			errMsg.Set("Enter a valid date (YYYY-MM-DD).")
			return
		}
		memberID := ""
		if acc.Scope == domain.ScopeIndividual {
			memberID = acc.OwnerID
		}
		t := domain.Transaction{
			ID: id.New(), AccountID: acc.ID, Date: date, Desc: strings.TrimSpace(desc.Get()),
			CategoryID: catID.Get(), Amount: money.New(amt, acc.Currency), MemberID: memberID,
		}
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		desc.Set("")
		amountStr.Set("")
		errMsg.Set("")
		bump()
	}))

	deleteTxn := func(txnID string) {
		if err := app.DeleteTransaction(txnID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	var formCard ui.Node
	if len(accounts) == 0 {
		formCard = Section(Class("card"), P(Class("empty"), "Add an account first, then you can record transactions."))
	} else {
		kindOptions := []ui.Node{
			Option(Value("Expense"), SelectedIf(kind.Get() == "Expense"), "Expense"),
			Option(Value("Income"), SelectedIf(kind.Get() == "Income"), "Income"),
		}
		accOptions := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			accOptions = append(accOptions, Option(Value(a.ID), SelectedIf(accID.Get() == a.ID), a.Name))
		}
		catOptions := []ui.Node{Option(Value(""), SelectedIf(catID.Get() == ""), "— No category —")}
		for _, c := range categories {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		formCard = Section(Class("card"),
			H2(Class("card-title"), "Add transaction"),
			Form(Class("form-grid"), OnSubmit(add),
				Input(Class("field"), Type("text"), Placeholder("Description"), Value(desc.Get()), OnInput(onDesc)),
				Input(Class("field"), Type("number"), Placeholder("Amount"), Value(amountStr.Get()), Step("0.01"), OnInput(onAmount)),
				Select(Class("field"), OnChange(onKind), kindOptions),
				Select(Class("field"), OnChange(onAcc), accOptions),
				Select(Class("field"), OnChange(onCat), catOptions),
				Input(Class("field"), Type("date"), Value(dateStr.Get()), OnInput(onDate)),
				Button(Class("btn btn-primary"), Type("submit"), "Add"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		)
	}

	txns := app.Transactions()
	sort.Slice(txns, func(i, j int) bool { return txns[i].Date.After(txns[j].Date) })

	var listBody ui.Node
	if len(txns) == 0 {
		listBody = P(Class("empty"), "No transactions yet.")
	} else {
		rows := MapKeyed(txns,
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

	return Div(
		formCard,
		Section(Class("card"),
			H2(Class("card-title"), "All transactions"),
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

// TransactionRow is a per-transaction row with a stable delete-handler hook.
func TransactionRow(props transactionRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Txn.ID) }))

	cat := props.Category
	if cat == "" {
		cat = "Uncategorized"
	}
	meta := cat + " · " + dateutil.FormatDate(props.Txn.Date)
	if props.Account != "" {
		meta += " · " + props.Account
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
