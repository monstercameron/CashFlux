// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// The report tools are the assistant's whole view of the Reports surface, and
// the traces are what lets it act on a figure instead of only quoting one.
// These mount them against a seeded household and check the two things that
// matter: the figures match the pure report functions, and a traced row comes
// back as the exact transactions with ids that update_transaction accepts.

var agrDay = time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)

// agrApp seeds a month with salary, a side gig, three spending categories and
// one uncategorized row — enough shape for the flow diagram to have both
// columns and for a trace to have something to narrow to.
func agrApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	acc := domain.Account{ID: "a-check", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", BalanceAsOf: agrDay}
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	for _, c := range []domain.Category{
		{ID: "c-sal", Name: "Salary", Kind: domain.KindIncome},
		{ID: "c-side", Name: "Side work", Kind: domain.KindIncome},
		{ID: "c-gro", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "c-rent", Name: "Rent", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(c); err != nil {
			t.Fatalf("PutCategory %s: %v", c.ID, err)
		}
	}
	rows := []domain.Transaction{
		{ID: "aa11111111111111", AccountID: "a-check", Payee: "UKG", Desc: "Paycheck", CategoryID: "c-sal", Date: agrDay, Amount: money.New(400000, "USD")},
		{ID: "bb22222222222222", AccountID: "a-check", Payee: "Etsy", Desc: "Side gig", CategoryID: "c-side", Date: agrDay, Amount: money.New(50000, "USD")},
		{ID: "cc33333333333333", AccountID: "a-check", Payee: "Publix", Desc: "Groceries", CategoryID: "c-gro", Date: agrDay, Amount: money.New(-12000, "USD")},
		{ID: "dd44444444444444", AccountID: "a-check", Payee: "Publix", Desc: "Groceries", CategoryID: "c-gro", Date: agrDay.AddDate(0, 0, 1), Amount: money.New(-8000, "USD")},
		{ID: "ee55555555555555", AccountID: "a-check", Payee: "Landlord", Desc: "August rent", CategoryID: "c-rent", Date: agrDay, Amount: money.New(-150000, "USD")},
		{ID: "ff66666666666666", AccountID: "a-check", Payee: "Corner store", Desc: "Unknown", Date: agrDay, Amount: money.New(-2500, "USD")},
	}
	for _, tx := range rows {
		if err := app.PutTransaction(tx); err != nil {
			t.Fatalf("PutTransaction %s: %v", tx.ID, err)
		}
	}
	prev := appstate.Default
	appstate.Default = app
	uistate.BumpDataRevision()
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

// agrTool finds one of the report tools by name.
func agrTool(t *testing.T, app *appstate.App, name string, tier aicontext.ConversationTier) chatTool {
	t.Helper()
	for _, tool := range agToolsReports(app, "USD", currency.Rates{Base: "USD"}, tier) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not registered", name)
	return chatTool{}
}

// agrArgs is the window every case runs over: the seeded month.
const agrArgs = `"from":"2026-08-01","to":"2026-08-31"`

func TestAgReportSectionsAllRender(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "report_section", aicontext.TierFull)
	for _, sec := range agReportSections() {
		args := json.RawMessage(`{"section":"` + sec.ID + `",` + agrArgs + `}`)
		out := tool.run(args)
		if strings.TrimSpace(out) == "" {
			t.Errorf("section %s rendered nothing", sec.ID)
		}
		if strings.Contains(out, "No report section called") {
			t.Errorf("section %s did not resolve by its own id", sec.ID)
		}
	}
}

func TestAgReportSectionFiguresMatchTheReport(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "report_section", aicontext.TierFull)
	out := tool.run(json.RawMessage(`{"section":"overview",` + agrArgs + `}`))
	// Income 4,500.00; spending 1,725.00; net 2,775.00.
	for _, want := range []string{"$4,500.00", "$1,725.00", "$2,775.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %s:\n%s", want, out)
		}
	}
}

func TestAgMoneyFlowNamesBothSidesAndTheSurplus(t *testing.T) {
	app := agrApp(t)
	out := agrTool(t, app, "report_section", aicontext.TierFull).
		run(json.RawMessage(`{"section":"money_flow",` + agrArgs + `}`))
	for _, want := range []string{"Salary", "Side work", "Groceries", "Rent", "Savings"} {
		if !strings.Contains(out, want) {
			t.Errorf("money flow missing the %s node:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("a surplus month should read as kept, not overspent:\n%s", out)
	}
}

func TestAgTraceMoneyFlowReturnsTheRowsBehindARibbon(t *testing.T) {
	app := agrApp(t)
	out := agrTool(t, app, "trace_report_row", aicontext.TierFull).
		run(json.RawMessage(`{"section":"money_flow","row":"Groceries",` + agrArgs + `}`))
	// Both grocery rows, and neither the rent nor the salary.
	for _, want := range []string{"cc333333", "dd444444", "$200.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("trace missing %s:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"ee555555", "aa111111"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("trace leaked an unrelated row %s:\n%s", unwanted, out)
		}
	}
}

func TestAgTraceSavingsRefusesRatherThanGuessing(t *testing.T) {
	// Savings is income minus expense, not a group of records. Returning a
	// plausible set of rows for it would be worse than saying so.
	app := agrApp(t)
	out := agrTool(t, app, "trace_report_row", aicontext.TierFull).
		run(json.RawMessage(`{"section":"money_flow","row":"Savings",` + agrArgs + `}`))
	if !strings.Contains(out, "Couldn't find a row") {
		t.Errorf("expected a refusal for the savings node, got:\n%s", out)
	}
}

func TestAgTraceCategoryAndPayee(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "trace_report_row", aicontext.TierFull)
	byCat := tool.run(json.RawMessage(`{"section":"spending_by_category","row":"Rent",` + agrArgs + `}`))
	if !strings.Contains(byCat, "ee555555") || !strings.Contains(byCat, "$1,500.00") {
		t.Errorf("category trace:\n%s", byCat)
	}
	// The payee sections group by DESCRIPTION, so the row label the model reads
	// off a section ("Groceries") must be the row it can open. Matching the
	// payee field first would fail to open the very line just printed.
	byRow := tool.run(json.RawMessage(`{"section":"top_payees","row":"Groceries",` + agrArgs + `}`))
	if !strings.Contains(byRow, "cc333333") || !strings.Contains(byRow, "dd444444") {
		t.Errorf("payee trace by the printed row label:\n%s", byRow)
	}
	// A person naming the merchant instead still gets the same rows.
	byMerchant := tool.run(json.RawMessage(`{"section":"top_payees","row":"Publix",` + agrArgs + `}`))
	if !strings.Contains(byMerchant, "cc333333") || !strings.Contains(byMerchant, "dd444444") {
		t.Errorf("payee trace by merchant name:\n%s", byMerchant)
	}
}

func TestAgTraceUnknownRowSaysWhatToPass(t *testing.T) {
	app := agrApp(t)
	out := agrTool(t, app, "trace_report_row", aicontext.TierFull).
		run(json.RawMessage(`{"section":"spending_by_category","row":"Yacht upkeep",` + agrArgs + `}`))
	if !strings.Contains(out, "a category name") {
		t.Errorf("an unresolvable row should say what to pass instead:\n%s", out)
	}
}

func TestAgFindTransactionsReachesUncategorized(t *testing.T) {
	app := agrApp(t)
	out := agrTool(t, app, "find_transactions", aicontext.TierFull).
		run(json.RawMessage(`{"uncategorized":true,` + agrArgs + `}`))
	if !strings.Contains(out, "ff666666") {
		t.Errorf("uncategorized search missed the row:\n%s", out)
	}
	if strings.Contains(out, "cc333333") {
		t.Errorf("uncategorized search returned a categorized row:\n%s", out)
	}
}

func TestAgUpdateTransactionEditsExactlyOneRow(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "update_transaction", aicontext.TierFull)
	if !tool.mutates {
		t.Fatal("update_transaction must require approval")
	}
	args := json.RawMessage(`{"id":"ff666666","category":"Groceries","payee":"Corner Store"}`)
	if prev := tool.preview(args); !strings.Contains(prev, "Groceries") {
		t.Errorf("preview should state the new category: %s", prev)
	}
	if out := tool.run(args); !strings.Contains(out, "Updated") {
		t.Fatalf("update failed: %s", out)
	}
	var got domain.Transaction
	for _, tx := range app.Transactions() {
		if tx.ID == "ff666666666666666" || strings.HasPrefix(tx.ID, "ff666666") {
			got = tx
		}
	}
	if got.CategoryID != "c-gro" {
		t.Errorf("category = %q, want c-gro", got.CategoryID)
	}
	if got.Payee != "Corner Store" {
		t.Errorf("payee = %q", got.Payee)
	}
	// Untouched fields stay untouched: an edit that mentions two fields must
	// not quietly rewrite a third.
	if got.Amount.Amount != -2500 {
		t.Errorf("amount changed to %d", got.Amount.Amount)
	}
	// And nothing else moved.
	for _, tx := range app.Transactions() {
		if tx.ID == "cc33333333333333" && tx.Payee != "Publix" {
			t.Errorf("an unrelated row was edited: %+v", tx)
		}
	}
}

func TestAgUpdateTransactionFlipsDirectionWithoutTouchingMagnitude(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "update_transaction", aicontext.TierFull)
	out := tool.run(json.RawMessage(`{"id":"ff666666","direction":"income"}`))
	if !strings.Contains(out, "Updated") {
		t.Fatalf("direction flip failed: %s", out)
	}
	for _, tx := range app.Transactions() {
		if strings.HasPrefix(tx.ID, "ff666666") && tx.Amount.Amount != 2500 {
			t.Errorf("amount = %d, want +2500 (same magnitude, money in)", tx.Amount.Amount)
		}
	}
}

func TestAgUpdateTransactionRefusesAnAmbiguousID(t *testing.T) {
	app := agrApp(t)
	tool := agrTool(t, app, "update_transaction", aicontext.TierFull)
	// Every seeded id starts with a distinct pair, so a single character is
	// unambiguous; "" is not an id at all and must be refused, not applied to
	// whatever comes first.
	if out := tool.run(json.RawMessage(`{"id":"","category":"Rent"}`)); !strings.Contains(out, "give the transaction id") {
		t.Errorf("an empty id should be refused, got: %s", out)
	}
	if out := tool.run(json.RawMessage(`{"id":"zz999999","category":"Rent"}`)); !strings.Contains(out, "no transaction with id") {
		t.Errorf("an unknown id should be refused, got: %s", out)
	}
}

func TestAgUpdateTransactionRejectsAnUnknownCategory(t *testing.T) {
	app := agrApp(t)
	out := agrTool(t, app, "update_transaction", aicontext.TierFull).
		run(json.RawMessage(`{"id":"ff666666","category":"Yacht upkeep"}`))
	if !strings.Contains(out, "create_category") {
		t.Errorf("should point at create_category rather than inventing one: %s", out)
	}
}

func TestAgAggregatesOnlyWithholdsTheMerchantSections(t *testing.T) {
	app := agrApp(t)
	listed := agrTool(t, app, "list_report_sections", aicontext.TierAggregatesOnly).run(nil)
	if strings.Contains(listed, "top_payees") || strings.Contains(listed, "largest_expenses") {
		t.Errorf("aggregates-only listed a merchant-naming section:\n%s", listed)
	}
	if !strings.Contains(listed, "spending_by_category") {
		t.Errorf("aggregates-only should still offer the category sections:\n%s", listed)
	}
	out := agrTool(t, app, "report_section", aicontext.TierAggregatesOnly).
		run(json.RawMessage(`{"section":"top_payees",` + agrArgs + `}`))
	// The payee section groups by description, so "August rent" is the line a
	// merchant-naming section prints — that is what must not appear here.
	if strings.Contains(out, "August rent") {
		t.Errorf("aggregates-only leaked a merchant line:\n%s", out)
	}
	// And the full tier still reads it.
	full := agrTool(t, app, "report_section", aicontext.TierFull).
		run(json.RawMessage(`{"section":"top_payees",` + agrArgs + `}`))
	if !strings.Contains(full, "August rent") {
		t.Errorf("full detail should read the payee section:\n%s", full)
	}
}

func TestAgTraceToolsAreGatedByThePrivacyTier(t *testing.T) {
	// The tier filter in insights.go keys off this map, so the tracers being in
	// it is what makes the aggregates-only promise hold for tool results.
	for _, name := range []string{"trace_report_row", "find_transactions"} {
		if aicontext.ToolAllowed(name, aicontext.TierAggregatesOnly) {
			t.Errorf("%s is offered under aggregates-only", name)
		}
		if !aicontext.ToolAllowed(name, aicontext.TierFull) {
			t.Errorf("%s is withheld at full detail", name)
		}
	}
}
