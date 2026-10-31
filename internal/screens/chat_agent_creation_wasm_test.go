// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assistantCreationFixture(t *testing.T) (*appstate.App, []chatTool) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	return app, agToolsCreation(app, "EUR", currency.Rates{Base: "EUR"})
}
func assistantCreationTool(t *testing.T, tools []chatTool, name string) chatTool {
	t.Helper()
	for _, tool := range tools {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("missing creation tool %s", name)
	return chatTool{}
}
func assistantCreationApprove(tool chatTool, raw string) (string, string) {
	args := json.RawMessage(raw)
	return tool.preview(args), tool.run(args)
}
func assistantCreationRow(id, payee string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "bank", Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Payee: payee, Desc: "Market purchase", Amount: money.New(-100, "USD")}
}

func TestAssistantCreationRegisteredOnceAndApprovalRequired(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	counts := map[string]int{}
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		counts[tool.spec.Function.Name]++
	}
	for _, name := range []string{"add_transaction", "create_category", "categorize_transactions"} {
		if counts[name] != 1 {
			t.Errorf("%s registrations%d, want1", name, counts[name])
		}
		tool := assistantCreationTool(t, tools, name)
		if !tool.mutates || tool.preview == nil {
			t.Fatal("creation tool lost approval")
		}
		revision := app.Rev()
		if out := tool.run(json.RawMessage(`{}`)); !strings.Contains(out, "Review and approve") || app.Rev() != revision {
			t.Fatalf("unreviewed write: %s", out)
		}
	}
}

func TestAssistantCreationExactAmountCompletePreviewAndLiveDuplicate(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	tool := assistantCreationTool(t, tools, "add_transaction")
	raw := `{"account_id":"bank","category_id":"food","amount":"-12.34","currency":"USD","payee":" Market ","date":"2026-10-19"}`
	preview, out := assistantCreationApprove(tool, raw)
	for _, want := range []string{"-12.34 USD", "ID bank", "ID food", "2026-10-19", `payee "Market"`, `description "Market"`, "source assistant", "cleared=false", "reviewed=false"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	if !strings.Contains(out, "Recorded transaction") {
		t.Fatal(out)
	}
	rows := app.Transactions()
	if len(rows) != 1 || rows[0].Amount != money.New(-1234, "USD") || rows[0].CategoryID != "food" || rows[0].Desc != "Market" || rows[0].Date.Day() != 19 || rows[0].Source != domain.TxnSourceAssistant {
		t.Fatalf("effective row differs: %+v", rows)
	}
	revision := app.Rev()
	if retry := tool.run(json.RawMessage(raw)); !strings.Contains(retry, "already processed") || app.Rev() != revision {
		t.Fatalf("approval retry wrote twice: %s", retry)
	}
	newTools := agToolsCreation(app, "USD", currency.Rates{Base: "USD"})
	preview, out = assistantCreationApprove(assistantCreationTool(t, newTools, "add_transaction"), raw)
	if !strings.Contains(preview, "Canonical duplicate ID "+rows[0].ID) || !strings.Contains(out, "already exists") || app.Rev() != revision {
		t.Fatalf("existing duplicate was written: %s / %s", preview, out)
	}
	_, out = assistantCreationApprove(tool, `{"account_id":"bank","amount":90071992547409.93,"description":"Exact large income","date":"2026-10-20"}`)
	if !strings.Contains(out, "Recorded transaction") {
		t.Fatal(out)
	}
	found := false
	for _, row := range app.Transactions() {
		if row.Desc == "Exact large income" {
			found = row.Amount.Amount == 9007199254740993
		}
	}
	if !found {
		t.Fatal("numeric JSON lost integer precision")
	}
}

func TestAssistantCreationRejectsMalformedDatesAmountsAndAmbiguousNames(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	tool := assistantCreationTool(t, tools, "add_transaction")
	for _, raw := range []string{`{"account":"Ban","amount":1}`, `{"account_id":"bank","amount":null}`, `{"account_id":"bank","amount":0}`, `{"account_id":"bank","amount":1.001}`, `{"account_id":"bank","amount":1e2}`, `{"account_id":"bank","amount":"NaN"}`, `{"account_id":"bank","amount":"92233720368547758.08"}`, `{"account_id":"bank","amount":1,"date":"2026-02-30"}`, `{"account_id":"bank","amount":1,"date":"yesterday"}`, `{"account_id":"bank","amount":1,"category":"Foo"}`, `{"account_id":"bank","amount":1,"currency":"EUR"}`, `{"account_id":"bank","amount":1,"transfer_account_id":"other"}`} {
		revision := app.Rev()
		preview, out := assistantCreationApprove(tool, raw)
		if !strings.Contains(preview, "Can't prepare") || !strings.Contains(out, "Can't make") || app.Rev() != revision {
			t.Fatalf("invalid write %s -> %s / %s", raw, preview, out)
		}
	}
	other := app.Accounts()[0]
	other.ID = "bank2"
	if err := app.PutAccount(other); err != nil {
		t.Fatal(err)
	}
	if preview := tool.preview(json.RawMessage(`{"account":"Bank","amount":1}`)); !strings.Contains(preview, "2 matches") {
		t.Fatalf("ambiguous account silently resolved: %s", preview)
	}
	if err := app.PutCategory(domain.Category{ID: "parent", Name: "Parent", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "child-food", Name: "Food", Kind: domain.KindExpense, ParentID: "parent"}); err != nil {
		t.Fatal(err)
	}
	if preview := tool.preview(json.RawMessage(`{"account_id":"bank","amount":1,"category":"Food"}`)); !strings.Contains(preview, "2 matches") {
		t.Fatalf("ambiguous category silently resolved: %s", preview)
	}
}

func TestAssistantCreationHonorsFixedLiabilityFlowSign(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	tool := assistantCreationTool(t, tools, "add_transaction")
	for _, tc := range []struct {
		id            string
		opening, want int64
		sign          domain.LiabilitySign
	}{{"positive", 10000, 1234, domain.LiabilityPositiveOwed}, {"negative", -10000, -1234, domain.LiabilityNegativeOwed}} {
		if err := app.PutAccount(domain.Account{ID: tc.id, Name: tc.id, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(tc.opening, "USD"), LiabilitySign: tc.sign}); err != nil {
			t.Fatal(err)
		}
		preview, out := assistantCreationApprove(tool, `{"account_id":"`+tc.id+`","amount":"-12.34","date":"2026-10-20"}`)
		if !strings.Contains(preview, "public flow -12.34 USD") || !strings.Contains(preview, "booked amount "+assistantContributionMoney(money.New(tc.want, "USD"))) || !strings.Contains(out, "Recorded transaction") {
			t.Fatalf("liability preview/execution wrong: %s / %s", preview, out)
		}
		found := false
		for _, row := range app.Transactions() {
			if row.AccountID == tc.id {
				found = row.Amount.Amount == tc.want && row.Desc == "Expense"
			}
		}
		if !found {
			t.Fatal("liability charge reversed or description wrong")
		}
	}
}

func TestAssistantCreationRejectsStaleAccountCategoryAndDuplicate(t *testing.T) {
	for _, kind := range []string{"account", "category", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			app, tools := assistantCreationFixture(t)
			tool := assistantCreationTool(t, tools, "add_transaction")
			raw := `{"account_id":"bank","category_id":"food","amount":1,"payee":"Test","date":"2026-10-20"}`
			if kind == "duplicate" {
				row := assistantCreationRow("dup", "Test")
				row.Amount = money.New(100, "USD")
				if err := app.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			}
			preview := tool.preview(json.RawMessage(raw))
			if strings.Contains(preview, "Can't prepare") {
				t.Fatal(preview)
			}
			switch kind {
			case "account":
				a := app.Accounts()[0]
				a.Name = "Changed"
				if err := app.PutAccount(a); err != nil {
					t.Fatal(err)
				}
			case "category":
				c := app.Categories()[0]
				c.Name = "Changed"
				if err := app.PutCategory(c); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				row := app.Transactions()[0]
				row.Desc = "Edited"
				if err := app.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			}
			revision, before := app.Rev(), app.Transactions()
			out := tool.run(json.RawMessage(raw))
			if !strings.Contains(out, "review") || app.Rev() != revision || !reflect.DeepEqual(app.Transactions(), before) {
				t.Fatalf("stale approval wrote: %s", out)
			}
		})
	}
}

func TestAssistantCreationCategorizationFullIDsAtomicAndLiveMatchSet(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	tool := assistantCreationTool(t, tools, "categorize_transactions")
	for _, id := range []string{"one", "two"} {
		if err := app.PutTransaction(assistantCreationRow(id, "Market")); err != nil {
			t.Fatal(err)
		}
	}
	other := app.Accounts()[0]
	other.ID = "other"
	other.Name = "Other"
	if err := app.PutAccount(other); err != nil {
		t.Fatal(err)
	}
	transfer := assistantCreationRow("move", "Market")
	transfer.TransferAccountID = other.ID
	if err := app.PutTransaction(transfer); err != nil {
		t.Fatal(err)
	}
	raw := `{"match":"Market","category_id":"food"}`
	preview := tool.preview(json.RawMessage(raw))
	for _, want := range []string{"exactly 2 transaction", "ID one", "ID two", "Excluded transfer IDs (1): move", "only_uncategorized=true"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview lacks %s: %s", want, preview)
		}
	}
	if err := app.PutTransaction(assistantCreationRow("late", "Market")); err != nil {
		t.Fatal(err)
	}
	revision, before := app.Rev(), app.Transactions()
	if out := tool.run(json.RawMessage(raw)); !strings.Contains(out, "changed after preview") || app.Rev() != revision || !reflect.DeepEqual(app.Transactions(), before) {
		t.Fatalf("changed match set retargeted: %s", out)
	}
	preview, out := assistantCreationApprove(tool, `{"transaction_ids":["one","two","move"],"category_id":"food"}`)
	if !strings.Contains(preview, "exactly 2") || !strings.Contains(out, "Categorized exactly 2") {
		t.Fatalf("batch failed: %s / %s", preview, out)
	}
	for _, row := range app.Transactions() {
		if row.ID == "one" || row.ID == "two" {
			if row.CategoryID != "food" {
				t.Fatal("batch missed approved row")
			}
		} else if row.CategoryID != "" {
			t.Fatalf("unapproved/transfer row changed: %+v", row)
		}
	}
	for _, raw := range []string{`{"match":" ","category_id":"food"}`, `{"transaction_ids":["missing"],"category_id":"food"}`, `{"transaction_ids":["one","one"],"category_id":"food"}`, `{"match":"Market","category_id":"food","unknown":1}`} {
		revision := app.Rev()
		preview, out := assistantCreationApprove(tool, raw)
		if !strings.Contains(preview, "Can't prepare") || !strings.Contains(out, "Can't make") || app.Rev() != revision {
			t.Fatalf("invalid categorization changed data: %s / %s", preview, out)
		}
	}
}

func TestAssistantCreationCategoryDefaultsDuplicateNoopAndFreshResolution(t *testing.T) {
	app, tools := assistantCreationFixture(t)
	tool := assistantCreationTool(t, tools, "create_category")
	for _, raw := range []string{`{"name":""}`, `{"name":"New","kind":"unsupported"}`, `{"name":"New","parent_id":"food"}`} {
		revision := app.Rev()
		preview, out := assistantCreationApprove(tool, raw)
		if !strings.Contains(preview, "Can't prepare") || !strings.Contains(out, "Can't make") || app.Rev() != revision {
			t.Fatalf("invalid category: %s / %s", preview, out)
		}
	}
	revision := app.Rev()
	preview, out := assistantCreationApprove(tool, `{"name":" FOOD ","kind":"income"}`)
	if !strings.Contains(preview, "Create nothing") || !strings.Contains(out, "already exists") || app.Rev() != revision {
		t.Fatalf("duplicate changed existing category: %s / %s", preview, out)
	}
	preview, out = assistantCreationApprove(tool, `{"name":"New category"}`)
	if !strings.Contains(preview, "kind expense") || !strings.Contains(out, "Created category") {
		t.Fatalf("default category failed: %s / %s", preview, out)
	}
	add := assistantCreationTool(t, tools, "add_transaction")
	_, out = assistantCreationApprove(add, `{"account_id":"bank","category":"New category","amount":-1,"date":"2026-10-20"}`)
	if !strings.Contains(out, "Recorded transaction") {
		t.Fatalf("new category unavailable in same tool group: %s", out)
	}
	_ = tool.preview(json.RawMessage(`{"name":"Food"}`))
	c, _, err := app.Store().GetCategory("food")
	if err != nil {
		t.Fatal(err)
	}
	c.Name = "Renamed"
	if err := app.PutCategory(c); err != nil {
		t.Fatal(err)
	}
	revision = app.Rev()
	if out := tool.run(json.RawMessage(`{"name":"Food"}`)); !strings.Contains(out, "changed after preview") || app.Rev() != revision {
		t.Fatalf("stale duplicate preview created replacement category: %s", out)
	}
}
