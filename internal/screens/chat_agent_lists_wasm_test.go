// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
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

func assistantListsFixture(t *testing.T) *appstate.App {
	t.Helper()
	app := primaryTodoNotificationsApp(t)
	previous := assistantListsNow
	assistantListsNow = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { assistantListsNow = previous })
	return app
}

func assistantListsTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsLists(app) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatal("missing list tool " + name)
	return chatTool{}
}

func assistantListsPage(t *testing.T, tool chatTool, args any) assistantListResult {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	text := tool.run(raw)
	var fail struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &fail); err != nil || fail.Error != "" {
		t.Fatalf("list failed: %s (%v)", text, err)
	}
	var page assistantListResult
	if err := json.Unmarshal([]byte(text), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func assistantListsError(t *testing.T, tool chatTool, raw string) string {
	t.Helper()
	text := tool.run(json.RawMessage(raw))
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil || out.Error == "" {
		t.Fatalf("expected explicit error for %s; got %s", raw, text)
	}
	return out.Error
}

func TestAssistantListsRegistrationAndStrictArguments(t *testing.T) {
	app := assistantListsFixture(t)
	full, restricted := map[string]chatTool{}, map[string]bool{}
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		name := tool.spec.Function.Name
		if _, exists := full[name]; exists {
			t.Fatalf("duplicate registered tool %s", name)
		}
		full[name] = tool
	}
	// buildChatTools supplies the complete registry. Insights applies this
	// policy when constructing provider specs and again before dispatch.
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierAggregatesOnly) {
		if aicontext.ToolAllowed(tool.spec.Function.Name, aicontext.TierAggregatesOnly) {
			restricted[tool.spec.Function.Name] = true
		}
	}
	for _, name := range []string{"list_transactions", "list_uncategorized_transactions", "list_tasks", "list_members", "list_categories", "find_duplicate_transactions"} {
		tool, ok := full[name]
		if !ok || tool.mutates || !strings.Contains(string(tool.spec.Function.Parameters), `"snapshot"`) {
			t.Fatalf("%s registry did not wire the bounded read helper", name)
		}
		wantAllowed := name == "list_tasks" || name == "list_members" || name == "list_categories"
		if restricted[name] != wantAllowed {
			t.Fatalf("%s does not retain its existing privacy policy", name)
		}
		for _, raw := range []string{`null`, `[]`, `{"unknown":1}`, `{"limit":null}`, `{"Limit":1}`, `{"limit":"1"}`, `{"offset":-1}`, `{"offset":1}`, `{} {}`} {
			assistantListsError(t, tool, raw)
		}
	}
	for _, name := range []string{"list_transactions", "list_uncategorized_transactions", "find_duplicate_transactions"} {
		if restricted[name] {
			t.Fatalf("financial record tool %s exposed in aggregates-only", name)
		}
	}
}

func TestAssistantListsPagesReachAllTasksAndRejectChangedSelections(t *testing.T) {
	app := assistantListsFixture(t)
	for i := range 63 {
		if err := app.Store().PutTask(domain.Task{ID: fmt.Sprintf("task-%03d", i), Title: fmt.Sprintf("Task%d", i), Status: domain.StatusOpen, Priority: domain.PriorityMedium}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	offset := 0
	snapshot := ""
	last := ""
	for {
		page := assistantListsPage(t, assistantListsTool(t, app, "list_tasks"), assistantListArgs{Offset: offset, Limit: 17, Snapshot: snapshot})
		if snapshot == "" {
			snapshot = page.Snapshot
		}
		if page.Snapshot != snapshot || page.Total != 63 || page.FilteredTotal != 63 || page.Ordering != "id ascending" || page.SourceCollection != "tasks" {
			t.Fatalf("page context changed: %+v", page)
		}
		for _, row := range page.Records {
			var task assistantListTask
			if err := json.Unmarshal(row.Data, &task); err != nil {
				t.Fatal(err)
			}
			if row.ID != task.ID || seen[row.ID] || row.ID <= last {
				t.Fatalf("unstable/incomplete task IDs: %+v", row)
			}
			seen[row.ID] = true
			last = row.ID
		}
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if len(seen) != 63 {
		t.Fatalf("only%d tasks reachable", len(seen))
	}
	// Bypass App.Rev deliberately: the shared reader also fingerprints contents.
	if err := app.Store().PutTask(domain.Task{ID: "task-new", Title: "New task", Status: domain.StatusOpen, Priority: domain.PriorityMedium}); err != nil {
		t.Fatal(err)
	}
	if out := assistantListsError(t, assistantListsTool(t, app, "list_tasks"), fmt.Sprintf(`{"offset":17,"snapshot":%q}`, snapshot)); !strings.Contains(out, "snapshot changed") {
		t.Fatal(out)
	}
}

func TestAssistantListsTransactionsValidateCategoriesPeriodsAndDates(t *testing.T) {
	app := assistantListsFixture(t)
	for _, category := range []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "house", Name: "Household", Kind: domain.KindExpense}, {ID: "food-child", Name: "Food", ParentID: "house", Kind: domain.KindExpense}} {
		if err := app.Store().PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	for _, txn := range []domain.Transaction{
		{ID: "last", AccountID: "bank", Date: time.Date(2026, 9, 30, 23, 0, 0, 0, time.UTC), Desc: "Last month", Amount: money.New(-123, "JPY"), CategoryID: "food"},
		{ID: "current", AccountID: "bank", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Desc: "Current", Amount: money.New(-321, "EUR"), CategoryID: "food"},
		{ID: "other", AccountID: "bank", Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Desc: "Other", Amount: money.New(-500, "USD"), CategoryID: "food-child"},
	} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantListsTool(t, app, "list_transactions")
	for _, raw := range []string{`{"category":"missing"}`, `{"category":"Food"}`, `{"period":"last_year"}`, `{"from":"2026-02-30"}`, `{"from":"2026-10-21","through":"2026-10-20"}`, `{"period":"all","from":"2026-10-01"}`, `{"through":null}`, `{"limit":51}`} {
		assistantListsError(t, tool, raw)
	}
	page := assistantListsPage(t, tool, assistantListTransactionArgs{Category: "food"})
	if page.Total != 3 || page.FilteredTotal != 1 || len(page.Records) != 1 || page.Records[0].ID != "current" {
		t.Fatalf("exact ID/default month mismatch: %+v", page)
	}
	var txn assistantListTransaction
	if err := json.Unmarshal(page.Records[0].Data, &txn); err != nil {
		t.Fatal(err)
	}
	if txn.Amount != money.New(-321, "EUR") || txn.ID != "current" || txn.AccountID != "bank" {
		t.Fatalf("original money/identity lost: %+v", txn)
	}
	last := assistantListsPage(t, tool, assistantListTransactionArgs{Category: "food", Period: "last_month"})
	if len(last.Records) != 1 || last.Records[0].ID != "last" {
		t.Fatalf("last month incorrect: %+v", last)
	}
	bounded := assistantListsPage(t, tool, assistantListTransactionArgs{From: "2026-09-30", Through: "2026-10-01"})
	if bounded.FilteredTotal != 2 {
		t.Fatalf("inclusive calendar dates incorrect: %+v", bounded)
	}
	all := assistantListsPage(t, tool, assistantListTransactionArgs{Period: "all", Limit: 1})
	assistantListsError(t, tool, fmt.Sprintf(`{"period":"this_month","offset":1,"snapshot":%q}`, all.Snapshot))
	// A valid category with no rows must never return the unfiltered ledger.
	empty := assistantListsPage(t, tool, assistantListTransactionArgs{Category: "house", Period: "all"})
	if empty.FilteredTotal != 0 || len(empty.Records) != 0 {
		t.Fatalf("empty selection leaked ledger: %+v", empty)
	}
	// AddDate(-1 month) from March31 normalizes into March. The period must
	// instead anchor to month start so last_month remains February.
	assistantListsNow = func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) }
	if err := app.Store().PutTransaction(domain.Transaction{ID: "february", AccountID: "bank", Date: time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC), Desc: "Month-end", Amount: money.New(-100, "USD")}); err != nil {
		t.Fatal(err)
	}
	monthEnd := assistantListsPage(t, tool, assistantListTransactionArgs{Period: "last_month"})
	if len(monthEnd.Records) != 1 || monthEnd.Records[0].ID != "february" {
		t.Fatalf("last_month rollover did not select February: %+v", monthEnd)
	}
}

func TestAssistantListsUncategorizedCurrencyTotalsAndLegacyLimit(t *testing.T) {
	app := assistantListsFixture(t)
	for i := range 56 {
		txn := domain.Transaction{ID: fmt.Sprintf("txn-%03d", i), AccountID: "bank", Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Payee: "Merchant", Desc: "Purchase", Amount: money.New(-12345, "JPY")}
		if i == 54 {
			txn.CategoryID = "food"
		}
		if i == 55 {
			txn.TransferAccountID = "other"
		}
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantListsTool(t, app, "list_uncategorized_transactions")
	first := assistantListsPage(t, tool, assistantListArgs{Limit: 60})
	if first.Total != 56 || first.FilteredTotal != 54 || first.RequestedLimit != 60 || first.EffectiveLimit != 50 || len(first.Records) != 50 || first.NextOffset == nil || *first.NextOffset != 50 {
		t.Fatalf("legacy request or actual totals lost: %+v", first)
	}
	for _, record := range first.Records {
		var txn assistantListTransaction
		if err := json.Unmarshal(record.Data, &txn); err != nil {
			t.Fatal(err)
		}
		if txn.ID != record.ID || txn.Amount != money.New(-12345, "JPY") || !txn.Uncategorized {
			t.Fatalf("uncategorized identity/currency changed: %+v", txn)
		}
	}
	second := assistantListsPage(t, assistantListsTool(t, app, "list_uncategorized_transactions"), assistantListArgs{Limit: 60, Offset: 50, Snapshot: first.Snapshot})
	if len(second.Records) != 4 || second.NextOffset != nil {
		t.Fatalf("uncategorized tail inaccessible: %+v", second)
	}
	assistantListsError(t, tool, `{"limit":61}`)
}

func TestAssistantListsFreshReadsAndClosedStoreAreNotEmptySuccess(t *testing.T) {
	app := assistantListsFixture(t)
	tools := agToolsLists(app)
	// Retain these exact handler closures before mutations: no captured slices.
	if err := app.Store().PutTask(domain.Task{ID: "fresh-task", Title: "Created after helper", Status: domain.StatusOpen, Priority: domain.PriorityMedium}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutMember(domain.Member{ID: "fresh-member", Name: "Fresh member"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutCategory(domain.Category{ID: "fresh-category", Name: "Fresh category", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutTransaction(domain.Transaction{ID: "fresh-transaction", AccountID: "bank", Date: assistantListsNow(), Desc: "Fresh transaction", Amount: money.New(-100, "CAD")}); err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		page := assistantListsPage(t, tool, assistantListArgs{})
		if tool.spec.Function.Name != "find_duplicate_transactions" && len(page.Records) == 0 {
			t.Fatalf("%s captured stale data", tool.spec.Function.Name)
		}
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if out := assistantListsError(t, tool, `{}`); !strings.Contains(out, "read ") {
			t.Fatalf("%s silently treated store failure as empty: %s", tool.spec.Function.Name, out)
		}
	}
}

func TestAssistantListsDuplicatesPreserveCanonicalCompleteGroups(t *testing.T) {
	app := assistantListsFixture(t)
	day := assistantListsNow()
	txns := []domain.Transaction{
		{ID: "a", AccountID: "bank", Date: day, Payee: " Merchant ", Desc: "matching review note", Amount: money.New(-100, "EUR")},
		{ID: "b", AccountID: "bank", Date: day, Payee: "merchant", Desc: "different note", Amount: money.New(-100, "EUR")},
		{ID: "other-account", AccountID: "card", Date: day, Payee: "merchant", Desc: "matching review note", Amount: money.New(-100, "EUR")},
		{ID: "other-currency", AccountID: "bank", Date: day, Payee: "merchant", Desc: "matching review note", Amount: money.New(-100, "USD")},
		{ID: "transfer", AccountID: "bank", Date: day, Payee: "merchant", Desc: "matching review note", Amount: money.New(-100, "EUR"), TransferAccountID: "other"},
		{ID: "c", AccountID: "bank", Date: day, Payee: "Another", Amount: money.New(200, "USD")},
		{ID: "d", AccountID: "bank", Date: day, Payee: "Another", Amount: money.New(200, "USD")},
	}
	for _, txn := range txns {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantListsTool(t, app, "find_duplicate_transactions")
	page := assistantListsPage(t, tool, assistantListDuplicateArgs{Match: "matching review note"})
	if page.Total != 2 || page.FilteredTotal != 1 || len(page.Records) != 1 {
		t.Fatalf("canonical grouping/filter count changed: %+v", page)
	}
	var group assistantListDuplicate
	if err := json.Unmarshal(page.Records[0].Data, &group); err != nil {
		t.Fatal(err)
	}
	if group.ID != page.Records[0].ID || group.Amount != money.New(-100, "EUR") || group.AccountID != "bank" || group.MemberCount != 2 || !reflect.DeepEqual(group.TransactionIDs, []string{"a", "b"}) {
		t.Fatalf("duplicate members/currency/account split or changed: %+v", group)
	}
	empty := assistantListsPage(t, tool, assistantListDuplicateArgs{Match: "unknown phrase"})
	if empty.FilteredTotal != 0 || len(empty.Records) != 0 {
		t.Fatalf("no-match filter returned all groups: %+v", empty)
	}
	first := assistantListsPage(t, tool, assistantListDuplicateArgs{Limit: 1})
	second := assistantListsPage(t, tool, assistantListDuplicateArgs{Limit: 1, Offset: 1, Snapshot: first.Snapshot})
	if first.Records[0].ID == second.Records[0].ID || second.NextOffset != nil {
		t.Fatalf("group continuation unstable: %+v", second)
	}
	assistantListsError(t, tool, fmt.Sprintf(`{"match":"merchant","offset":1,"snapshot":%q}`, first.Snapshot))
	// Even without an App revision, changed group membership invalidates a page.
	extra := txns[0]
	extra.ID = "new-duplicate"
	if err := app.Store().PutTransaction(extra); err != nil {
		t.Fatal(err)
	}
	assistantListsError(t, tool, fmt.Sprintf(`{"offset":1,"snapshot":%q}`, first.Snapshot))
}

func TestAssistantListsOversizedDuplicateNeverReturnsPartialMembership(t *testing.T) {
	app := assistantListsFixture(t)
	// Large persisted IDs are legal imported records. Membership alone is too
	// large for the shared response budget, so no truncated group may escape.
	for i := range 26 {
		txn := domain.Transaction{ID: fmt.Sprintf("%02d-", i) + strings.Repeat("x", 1000), AccountID: "bank", Date: assistantListsNow(), Payee: "Merchant", Desc: "Same charge", Amount: money.New(-100, "USD")}
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantListsTool(t, app, "find_duplicate_transactions")
	out := tool.run(json.RawMessage(`{}`))
	if !strings.Contains(out, "no partial membership") || !strings.Contains(out, "query_data") || strings.Contains(out, `"records"`) {
		t.Fatalf("oversized group silently cut/unreadable: %s", out)
	}
	var recovery struct {
		MemberCount   int `json:"memberCount"`
		RecoveryQuery struct {
			Collection string                     `json:"collection"`
			From       string                     `json:"from"`
			Through    string                     `json:"through"`
			Equals     map[string]json.RawMessage `json:"equals"`
		} `json:"recoveryQuery"`
	}
	if err := json.Unmarshal([]byte(out), &recovery); err != nil {
		t.Fatal(err)
	}
	if recovery.MemberCount != 26 || recovery.RecoveryQuery.Collection != "transactions" || recovery.RecoveryQuery.From != "2026-10-20" || recovery.RecoveryQuery.Through != "2026-10-20" || string(recovery.RecoveryQuery.Equals["amount.Currency"]) != `"USD"` {
		t.Fatalf("oversized group lacks a usable typed recovery: %s", out)
	}
}
