// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func basicCreationApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

func TestAssistantCreationStrictJSONAndExactDecimals(t *testing.T) {
	for _, tc := range []struct {
		text     string
		decimals int
		want     int64
		bad      bool
	}{
		{"123.45", 2, 12345, false},
		{"1.2345e2", 2, 0, true},
		{"1e2", 2, 0, true},
		{"1e-3", 2, 0, true},
		{"92233720368547758.08", 2, 0, true},
		{"null", 2, 0, true},
	} {
		got, err := assistantDecimalMinor(tc.text, tc.decimals)
		if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Errorf("assistantDecimalMinor(%q)=(%d,%v), want %d bad=%v", tc.text, got, err, tc.want, tc.bad)
		}
	}
	var args assistantTaskArgs
	if err := assistantStrictJSON(json.RawMessage(`{"title":"x","surprise":true}`), &args); err == nil {
		t.Fatal("unknown JSON property was accepted")
	}
	if err := assistantStrictJSON(json.RawMessage(`{"title":"x"} {"title":"y"}`), &args); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
}

func TestAssistantTaskPreviewIsCompleteAndCreationRequiresFreshApproval(t *testing.T) {
	app := basicCreationApp(t)
	tool := agToolsBasicCreation(app)[0]
	raw := json.RawMessage(`{"title":"  Call bank  ","notes":"Bring the loan papers and account details.","priority":"high","due":"2026-10-30"}`)
	if out := tool.run(raw); !strings.Contains(out, "Review the task preview") || len(app.Tasks()) != 0 {
		t.Fatalf("run without preview wrote task: %s", out)
	}
	preview := tool.preview(raw)
	for _, want := range []string{`"Call bank"`, "Priority: high", "Due: 2026-10-30", "Notes: Bring the loan papers and account details."} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview %q does not include %q", preview, want)
		}
	}
	if len(app.Tasks()) != 0 {
		t.Fatal("preview changed task state")
	}
	preview = tool.preview(raw)
	if strings.Contains(preview, "Can't add") {
		t.Fatalf("valid task preview failed: %s", preview)
	}
	if err := app.Store().PutTask(domain.Task{ID: "changed", Title: "New task", Status: domain.StatusOpen, Priority: domain.PriorityLow}); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(raw); !strings.Contains(out, "changed since preview") || len(app.Tasks()) != 1 {
		t.Fatalf("stale approval created task: %s; tasks=%d", out, len(app.Tasks()))
	}
	for _, invalid := range []string{
		`{"title":"Bad date","due":"2026-02-30"}`,
		`{"title":"Bad priority","priority":"urgent"}`,
		`{"title":"Unknown","x":1}`,
	} {
		if got := tool.preview(json.RawMessage(invalid)); !strings.Contains(got, "Can't add") {
			t.Errorf("invalid task %s accepted: %s", invalid, got)
		}
	}
}

func TestAssistantTaskDuplicateIsNoop(t *testing.T) {
	app := basicCreationApp(t)
	if err := app.PutTask(domain.Task{ID: "existing", Title: "Call the bank", Status: domain.StatusOpen, Priority: domain.PriorityMedium}); err != nil {
		t.Fatal(err)
	}
	tool := agToolsBasicCreation(app)[0]
	before := app.Rev()
	raw := json.RawMessage(`{"title":"call the bank"}`)
	if got := tool.preview(raw); !strings.Contains(got, "already exists") {
		t.Fatalf("duplicate preview = %s", got)
	}
	if got := tool.run(raw); !strings.Contains(got, "already exists") || app.Rev() != before {
		t.Fatalf("duplicate run mutated state: %s", got)
	}
}

func TestAssistantAccountPreviewAndCreationUseExactLiabilityMoney(t *testing.T) {
	app := basicCreationApp(t)
	tool := agToolsBasicCreation(app)[1]
	raw := json.RawMessage(`{"name":"Family loan","class":"liability","type":"loan","currency":"USD","balance":"1234.56","apr":"5.25","credit_limit":0,"min_payment":"75.00"}`)
	if out := tool.run(raw); !strings.Contains(out, "Review the account preview") || len(app.Accounts()) != 0 {
		t.Fatalf("account created without preview: %s", out)
	}
	preview := tool.preview(raw)
	for _, want := range []string{"liability loan", `"Family loan"`, "USD", "1,234.56 owed", "APR: 5.25%", "Credit limit", "Minimum payment", "Owner: household group", "Scope: shared", "Balance as of:", "owed balances are negative"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview %q does not include %q", preview, want)
		}
	}
	preview = tool.preview(raw)
	if strings.Contains(preview, "Can't create") {
		t.Fatalf("valid account preview failed: %s", preview)
	}
	if out := tool.run(raw); !strings.Contains(out, "Created liability account") {
		t.Fatalf("account create: %s", out)
	}
	accounts, err := app.Store().ListAccounts()
	if err != nil || len(accounts) != 1 {
		t.Fatalf("stored accounts=%d err=%v", len(accounts), err)
	}
	got := accounts[0]
	if got.OpeningBalance.Amount != -123456 || got.LiabilitySign != domain.LiabilityNegativeOwed || got.RateAPROrZero() != 5.25 || got.CreditLimit.Amount != 0 || got.MinPayment.Amount != 7500 || got.OwnerID != domain.GroupOwnerID || got.Scope != domain.ScopeShared {
		t.Fatalf("stored account lost effective fields: %+v", got)
	}
	if len(app.BalanceHistory(got.ID)) == 0 {
		t.Fatal("nonzero opening balance did not create balance history")
	}
}

func TestAssistantAccountRejectsMismatchedTypesInvalidMoneyAndStaleApproval(t *testing.T) {
	app := basicCreationApp(t)
	tool := agToolsBasicCreation(app)[1]
	for _, invalid := range []string{
		`{"name":"Wrong","class":"asset","type":"loan"}`,
		`{"name":"Fraction","class":"asset","balance":"1.001"}`,
		`{"name":"Negative debt","class":"liability","balance":-1}`,
		`{"name":"APR asset","class":"asset","apr":1}`,
		`{"name":"Unknown","class":"asset","extra":1}`,
	} {
		if got := tool.preview(json.RawMessage(invalid)); !strings.Contains(got, "Can't create") {
			t.Errorf("invalid account %s accepted: %s", invalid, got)
		}
	}
	raw := json.RawMessage(`{"name":"Checking","class":"asset","balance":100}`)
	if got := tool.preview(raw); strings.Contains(got, "Can't create") {
		t.Fatalf("valid account preview: %s", got)
	}
	if err := app.Store().PutSettings(storeSettingsForBasicCreation("EUR")); err != nil {
		t.Fatal(err)
	}
	if got := tool.run(raw); !strings.Contains(got, "base currency changed since preview") || len(app.Accounts()) != 0 {
		t.Fatalf("stale account approval wrote account: %s", got)
	}
	if got := tool.preview(raw); strings.Contains(got, "Can't create") {
		t.Fatalf("fresh account preview should use updated settings: %s", got)
	}
}

func TestAssistantAccountDuplicateIsNoop(t *testing.T) {
	app := basicCreationApp(t)
	account := domain.Account{ID: "existing", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	tool := agToolsBasicCreation(app)[1]
	before := app.Rev()
	raw := json.RawMessage(`{"name":"checking","class":"asset"}`)
	if got := tool.preview(raw); !strings.Contains(got, "already exists") {
		t.Fatalf("duplicate preview = %s", got)
	}
	if got := tool.run(raw); !strings.Contains(got, "already exists") || app.Rev() != before || len(app.Accounts()) != 1 {
		t.Fatalf("duplicate run mutated state: %s", got)
	}
}

func TestAssistantZeroLiabilityPreviewMatchesCanonicalSign(t *testing.T) {
	app := basicCreationApp(t)
	tool := agToolsBasicCreation(app)[1]
	got := tool.preview(json.RawMessage(`{"name":"Zero debt","class":"liability"}`))
	if !strings.Contains(got, "Opening balance: ") || !strings.Contains(got, "Stored liability convention: owed balances are positive") {
		t.Fatalf("zero-balance liability preview hides its stored convention: %s", got)
	}
}

func TestAssistantDecimalHandlesExponentAndRejectsLossyScale(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
		bad   bool
	}{{"1.234e2", 0, true}, {"12e-1", 0, true}, {"1e-3", 0, true}} {
		got, err := assistantMinorFromJSON(json.RawMessage(tc.input), 2)
		if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Errorf("minor(%s)=(%d,%v), want %d bad=%v", tc.input, got, err, tc.want, tc.bad)
		}
	}
}

func storeSettingsForBasicCreation(base string) store.Settings {
	return store.Settings{BaseCurrency: base}
}
