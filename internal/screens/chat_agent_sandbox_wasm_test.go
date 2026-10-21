// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func sandboxToolApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

func TestAssistantSandboxUsesExactAccountCurrencyAndIsDisposable(t *testing.T) {
	app := sandboxToolApp(t)
	if err := app.Store().PutSettings(store.Settings{BaseCurrency: "USD", DefaultAccountID: "yen"}); err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "dollars", Name: "Daily", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "yen", Name: "Travel cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "JPY"},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	tool := agToolsSandbox(app, "EUR", currency.Rates{})[0] // captured registration values must not win over current settings.
	before, err := app.Store().ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	got := tool.run(json.RawMessage(`{"label":"monthly train pass","monthly_change":-1234,"months":2}`))
	for _, want := range []string{"JPY -1234", "Travel cash", "Household base currency: USD", "2 months", "Nothing was saved"} {
		if !strings.Contains(got, want) {
			t.Errorf("scenario output %q does not include %q", got, want)
		}
	}
	after, err := app.Store().ListTransactions()
	if err != nil || len(after) != len(before) {
		t.Fatalf("sandbox changed real transactions: before=%d after=%d err=%v", len(before), len(after), err)
	}
}

func TestAssistantSandboxSelectorAndAmountValidation(t *testing.T) {
	accounts := []domain.Account{
		{ID: "one", Name: "Cash", Currency: "USD"},
		{ID: "two", Name: "Cash", Currency: "JPY"},
		{ID: "archived", Name: "Old", Currency: "USD", Archived: true},
	}
	if _, err := selectSandboxAccount("Cash", accounts, nil, ""); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate name selection error = %v", err)
	}
	if selected, err := selectSandboxAccount("two", accounts, nil, ""); err != nil || selected.ID != "two" {
		t.Fatalf("ID selection = %+v, %v", selected, err)
	}
	if _, err := selectSandboxAccount("Old", accounts, nil, ""); err == nil {
		t.Fatal("archived account was selectable")
	}
	for _, tc := range []struct {
		raw      string
		decimals int
		want     int64
		bad      bool
	}{
		{`"-12.34"`, 2, -1234, false},
		{`-12.34`, 2, -1234, false},
		{`1234`, 0, 1234, false},
		{`12.3`, 0, 0, true},
		{`1e2`, 2, 0, true},
		{`"1.001"`, 2, 0, true},
		{`null`, 2, 0, true},
	} {
		got, err := parseSandboxAmount(json.RawMessage(tc.raw), tc.decimals)
		if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Errorf("parseSandboxAmount(%s,%d)=(%d,%v), want %d bad=%v", tc.raw, tc.decimals, got, err, tc.want, tc.bad)
		}
	}
}

func TestAssistantSandboxRejectsInvalidHorizonAndUnknownFields(t *testing.T) {
	app := sandboxToolApp(t)
	tool := agToolsSandbox(app, "USD", currency.Rates{})[0]
	for _, raw := range []string{
		`{"label":"test","monthly_change":1,"months":0}`,
		`{"label":"test","monthly_change":1,"months":61}`,
		`{"label":"test","monthly_change":1,"months":null}`,
		`{"label":"test","monthly_change":1,"months":1.5}`,
		`{"label":"test","monthly_change":1,"months":"12"}`,
		`{"label":"test","monthly_change":1,"months":1,"surprise":true}`,
		`{"label":"test","monthly_change":1} {"label":"other","monthly_change":1}`,
	} {
		if got := tool.run(json.RawMessage(raw)); !strings.Contains(got, "no scenario was run") && !strings.Contains(got, "one JSON object") && !strings.Contains(got, "Couldn't read the scenario") {
			t.Errorf("invalid scenario accepted (%s): %s", raw, got)
		}
	}
}

func TestAssistantSandboxFormatsUnavailableVariables(t *testing.T) {
	if got := formatSandboxVar(0, true, false, true); !strings.Contains(got, "unavailable") {
		t.Fatalf("added variable's missing before value = %q", got)
	}
	if got := formatSandboxVar(0, false, true, false); !strings.Contains(got, "unavailable") {
		t.Fatalf("removed variable's missing after value = %q", got)
	}
	if got := formatSandboxVar(math.NaN(), false, false, false); !strings.Contains(got, "non-finite") {
		t.Fatalf("NaN variable = %q", got)
	}
}

func TestAssistantSandboxClampsRecurringDatesAndKeepsSubUnitChanges(t *testing.T) {
	start, err := dateutil.ParseDate("2024-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if got := dateutil.AddMonthsClamped(start, 1); dateutil.FormatDate(got) != "2024-02-29" {
		t.Fatalf("February anchor date = %s, want 2024-02-29", dateutil.FormatDate(got))
	}
	if got := dateutil.AddMonthsClamped(start, 2); dateutil.FormatDate(got) != "2024-03-31" {
		t.Fatalf("March anchor date = %s, want 2024-03-31", dateutil.FormatDate(got))
	}

	app := sandboxToolApp(t)
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	tool := agToolsSandbox(app, "USD", currency.Rates{})[0]
	got := tool.run(json.RawMessage(`{"label":"small change","monthly_change":"-0.50","months":1}`))
	if !strings.Contains(got, "Tracked changes") || !strings.Contains(got, "not a complete future forecast") {
		t.Fatalf("sub-unit scenario was hidden or presented as a forecast: %s", got)
	}
}
