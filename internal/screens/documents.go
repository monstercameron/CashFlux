//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Documents imports transactions from CSV (no AI needed). Paste rows with a
// header (date, payee/desc, amount, account, category, member), then Import —
// valid rows are added through the validated write path.
func Documents() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:documents", 0)
	csvText := ui.UseState("")
	msg := ui.UseState("")

	onCsv := ui.UseEvent(func(v string) { csvText.Set(v) })

	importCSV := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(csvText.Get())
		if data == "" {
			msg.Set("Paste some CSV first.")
			return
		}
		n, err := app.ImportTransactionsCSV([]byte(data))
		if err != nil {
			msg.Set("Couldn't read that CSV: " + err.Error())
			return
		}
		msg.Set(fmt.Sprintf("Imported %s.", plural(n, "transaction")))
		rev.Set(rev.Get() + 1)
	}))

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Import transactions from CSV"),
			P(Class("muted"), "Paste rows with a header line. Columns are matched by name (date, payee/desc, amount, account, category, member); extra columns are ignored. Amounts are decimal — negative for expenses."),
			Form(OnSubmit(importCSV),
				Textarea(Class("field field-wide"), Attr("rows", "8"),
					Placeholder("date,payee,amount,account\n2026-06-01,Salary,4200.00,Checking\n2026-06-02,Groceries,-86.40,Checking"),
					OnInput(onCsv),
				),
				Div(Style(map[string]string{"margin-top": "0.6rem"}),
					Button(Class("btn btn-primary"), Type("submit"), "Import"),
				),
			),
			If(msg.Get() != "", P(Class("muted"), msg.Get())),
		),
		Section(Class("card"),
			Div(Class("badge badge-soon"), "Planned · Phase 2"),
			P(Class("muted"), "AI document parsing (PDFs and receipt images → transactions) arrives with the OpenAI client; this CSV import works today, no key required."),
		),
	)
}
