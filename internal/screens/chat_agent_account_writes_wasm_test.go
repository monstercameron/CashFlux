// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func accountWriteTestApp(t *testing.T, accounts ...domain.Account) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for _, account := range accounts {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func accountWriteTestTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsAccountWrites(app) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return chatTool{}
}

func TestAccountWriteArgsRequireExactStringsDatesAndNames(t *testing.T) {
	for _, raw := range []string{
		`{"account_id":"a","balance":12.34}`, `{"account_id":"a","balance":"12.34","unknown":true}`,
		`{"account_id":"a","balance":"12.34","as_of":"2026-2-03"}`,
		`{"account_id":"a","balance":"12.34","as_of":"2026-02-30"}`,
		`{"account_id":"a","balance":"12.34"} {}`,
	} {
		if _, err := parseAccountBalanceWriteArgs(json.RawMessage(raw)); err == nil {
			t.Errorf("parseAccountBalanceWriteArgs(%s) unexpectedly succeeded", raw)
		}
	}
	for _, raw := range []string{
		`{"from_account_id":"a","to_account_id":"b","amount":10}`,
		`{"from_account_id":"a","to_account_id":"b","amount":"1.00","date":"2026-02-30"}`,
		`{"from_account_id":"a","to_account_id":"b","amount":"1.00","operation_id":"bad id"}`,
		`{"from_account_id":"a","to_account_id":"b","amount":"1.00"} {}`,
	} {
		if _, err := parseAccountTransferWriteArgs(json.RawMessage(raw)); err == nil {
			t.Errorf("parseAccountTransferWriteArgs(%s) unexpectedly succeeded", raw)
		}
	}
	if _, err := parseOptionalTransferAmount("0", "EUR", false); err == nil {
		t.Fatal("zero received amount was accepted as a conversion override")
	}
	if got, err := parseOptionalTransferAmount("0", "USD", true); err != nil || got != 0 {
		t.Fatalf("zero fee parse = %d, %v", got, err)
	}
}

func TestBalanceToolRequiresPreviewAndUsesCheckpointForPositiveOwedInput(t *testing.T) {
	account := domain.Account{ID: "card", Name: "Everyday card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(50000, "USD")}
	app := accountWriteTestApp(t, account)
	tool := accountWriteTestTool(t, app, "update_account_balance")
	raw := json.RawMessage(`{"account_id":"card","balance":"650.00","as_of":"2026-10-17"}`)
	if out := tool.run(raw); !strings.Contains(out, "no matching approval preview") || len(app.Transactions()) != 0 {
		t.Fatalf("run without preview = %q; rows=%d", out, len(app.Transactions()))
	}
	preview := tool.preview(raw)
	for _, want := range []string{"Everyday card", "($500.00)", "($650.00)", "($150.00)", "2026-10-17", "excluded from income and spending reports", "opening balance stays unchanged"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "checkpoint") || len(app.Transactions()) != 1 {
		t.Fatalf("approved balance update = %q; rows=%d", out, len(app.Transactions()))
	}
	saved, _, _ := app.Store().GetAccount(account.ID)
	if saved.OpeningBalance.Amount != account.OpeningBalance.Amount {
		t.Fatalf("opening balance rewritten: %d", saved.OpeningBalance.Amount)
	}
	checkpoint := app.Transactions()[0]
	if checkpoint.Amount.Amount != 15000 || !checkpoint.ExcludeFromReports || checkpoint.BalanceCheckpointAt.IsZero() {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	revision := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already processed") || app.Rev() != revision {
		t.Fatalf("repeated approval was not idempotent: %q", out)
	}
}

func TestBalanceToolRefusesStalePreviewAndZeroDeltaIsNoOp(t *testing.T) {
	account := domain.Account{ID: "cash", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
	app := accountWriteTestApp(t, account)
	tool := accountWriteTestTool(t, app, "update_account_balance")
	raw := json.RawMessage(`{"account_id":"cash","balance":"150.00"}`)
	tool.preview(raw)
	if err := app.Store().PutTransaction(domain.Transaction{ID: "intervening", AccountID: account.ID, Amount: money.New(100, "USD"),
		Date: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), Desc: "New activity"}); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(raw); !strings.Contains(out, "ledger changed since preview") || len(app.Transactions()) != 1 {
		t.Fatalf("stale reconciliation = %q; rows=%d", out, len(app.Transactions()))
	}

	noOp := json.RawMessage(`{"account_id":"cash","balance":"101.00"}`)
	tool.preview(noOp)
	revision := app.Rev()
	if out := tool.run(noOp); !strings.Contains(out, "already matches") || app.Rev() != revision || len(app.Transactions()) != 1 {
		t.Fatalf("zero adjustment result = %q; rev=%d/%d rows=%d", out, app.Rev(), revision, len(app.Transactions()))
	}
}

func TestTransferToolShowsBothCurrenciesAndRefusesChangedRates(t *testing.T) {
	from := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(100000, "USD")}
	to := domain.Account{ID: "euro", Name: "Euro savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "EUR", OpeningBalance: money.New(20000, "EUR")}
	app := accountWriteTestApp(t, from, to)
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	tool := accountWriteTestTool(t, app, "add_transfer")
	raw := json.RawMessage(`{"from_account_id":"checking","to_account_id":"euro","amount":"100.00","date":"2026-10-21","operation_id":"assistant_test_transfer_1"}`)
	if out := tool.run(raw); !strings.Contains(out, "no matching approval preview") || len(app.Transactions()) != 0 {
		t.Fatalf("run without preview = %q; rows=%d", out, len(app.Transactions()))
	}
	preview := tool.preview(raw)
	for _, want := range []string{"Checking", "Euro savings", "$100.00", "€50.00", "$1,000.00", "€200.00", "2026-10-21"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	changedRates := app.Settings()
	changedRates.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(changedRates); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(raw); !strings.Contains(out, "currency settings changed since preview") || len(app.Transactions()) != 0 {
		t.Fatalf("stale transfer = %q; rows=%d", out, len(app.Transactions()))
	}
	changedRates.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(changedRates); err != nil {
		t.Fatal(err)
	}
	tool.preview(raw)
	if out := tool.run(raw); !strings.Contains(out, "Recorded the approved transfer") || len(app.Transactions()) != 2 {
		t.Fatalf("approved transfer = %q; rows=%d", out, len(app.Transactions()))
	}
	for _, row := range app.Transactions() {
		if row.Source != domain.TxnSourceAssistant {
			t.Errorf("transfer row source = %q, want assistant", row.Source)
		}
	}
}

func TestAccountNameResolutionRequiresOneExactMatch(t *testing.T) {
	first := domain.Account{ID: "first", Name: "Joint cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD"}
	second := first
	second.ID = "second"
	app := accountWriteTestApp(t, first, second)
	tool := accountWriteTestTool(t, app, "update_account_balance")
	raw := json.RawMessage(`{"account":"Joint cash","balance":"10.00"}`)
	if out := tool.preview(raw); !strings.Contains(out, "ambiguous") {
		t.Fatalf("ambiguous exact name preview = %q", out)
	}
	if out := tool.run(raw); !strings.Contains(out, "ambiguous") || len(app.Transactions()) != 0 {
		t.Fatalf("ambiguous name changed data: %q", out)
	}
}
