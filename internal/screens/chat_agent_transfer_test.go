//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func transferTestAccounts() (domain.Account, domain.Account) {
	return domain.Account{ID: "eur", Name: "Euro account", Currency: "EUR"},
		domain.Account{ID: "usd", Name: "Dollar account", Currency: "USD"}
}

func TestAssistantTransferPreviewUsesTheParamsThatRunWrites(t *testing.T) {
	from, to := transferTestAccounts()
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	raw := json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":100,"received_amount":120,"operation_id":"move_123","date":"2026-10-18"}`)
	built, err := buildAssistantTransfer(raw, from, to, rates, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), "Transfer to Dollar account")
	if err != nil {
		t.Fatal(err)
	}
	preview := assistantTransferPreview(built)
	for _, want := range []string{"€100.00 (EUR)", "Euro account", "Dollar account (USD)", "$120.00"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview %q does not contain %q", preview, want)
		}
	}
	if built.params.OperationID != "move_123" || built.params.AmountMinor != 10000 || built.params.ReceivedMinor != 12000 {
		t.Fatalf("params written by run = %+v", built.params)
	}
	if !strings.Contains(preview, "differs from today's rate by 9%") {
		t.Errorf("preview does not warn about the material rate difference: %q", preview)
	}
}

func TestAssistantTransferUsesConvertedAmountWhenReceivedIsOmitted(t *testing.T) {
	from, to := transferTestAccounts()
	built, err := buildAssistantTransfer(json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":100}`), from, to,
		currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, time.Now(), "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if built.params.ReceivedMinor != 11000 {
		t.Fatalf("received minor = %d, want 11000", built.params.ReceivedMinor)
	}
	if !strings.Contains(assistantTransferPreview(built), "$110.00") {
		t.Fatal("preview does not show the converted destination amount")
	}
}

func TestAssistantTransferWithoutRateIsExplicitlyRefused(t *testing.T) {
	from, to := transferTestAccounts()
	built, err := buildAssistantTransfer(json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":100}`), from, to,
		currency.Rates{Base: "USD", Rates: map[string]float64{}}, time.Now(), "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if built.params.ReceivedMinor != 0 || built.hasConversion {
		t.Fatalf("unexpected unavailable-rate params: %+v, conversion=%v", built.params, built.hasConversion)
	}
	if preview := strings.ToLower(assistantTransferPreview(built)); !strings.Contains(preview, "no exchange rate") || !strings.Contains(preview, "will be refused") {
		t.Errorf("preview should explain refusal, got %q", preview)
	}
}

func TestAssistantTransferPreviewDoesNotReuseRatesFromEarlierBuild(t *testing.T) {
	from, to := transferTestAccounts()
	raw := json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":100}`)
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	withRate, err := buildAssistantTransfer(raw, from, to,
		currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, now, "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if preview := assistantTransferPreview(withRate); !strings.Contains(preview, "$110.00") {
		t.Fatalf("rate-backed preview does not show the converted amount: %q", preview)
	}
	withRateKey, _ := assistantTransferBuildCacheKey(raw, from, to, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, now, "Transfer")
	emptyRateKey, _ := assistantTransferBuildCacheKey(raw, from, to, currency.Rates{Base: "USD", Rates: map[string]float64{}}, now, "Transfer")
	if withRateKey == emptyRateKey {
		t.Fatal("transfer cache key does not include the exchange rates")
	}

	withoutRate, err := buildAssistantTransfer(raw, from, to, currency.Rates{Base: "USD", Rates: map[string]float64{}}, now, "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	preview := strings.ToLower(assistantTransferPreview(withoutRate))
	if withoutRate.params.ReceivedMinor != 0 || withoutRate.hasConversion {
		t.Fatalf("empty rates reused a prior conversion: params=%+v conversion=%v", withoutRate.params, withoutRate.hasConversion)
	}
	if strings.Contains(preview, "$110.00") || !strings.Contains(preview, "no exchange rate") || !strings.Contains(preview, "will be refused") {
		t.Errorf("empty-rate preview must refuse without showing an earlier amount, got %q", preview)
	}
}

func TestAssistantTransferAmountParsingRejectsInvalidValues(t *testing.T) {
	from, to := transferTestAccounts()
	for _, amount := range []string{"-1", "NaN", "Infinity", "1e999", "92233720368547759"} {
		raw := json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":` + amount + `}`)
		if _, err := buildAssistantTransfer(raw, from, to, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, time.Now(), "Transfer"); err == nil {
			t.Errorf("amount %s unexpectedly accepted", amount)
		}
	}
}

func TestAssistantTransferOperationIDValidationAndGeneration(t *testing.T) {
	from, to := transferTestAccounts()
	base := `{"from_account":"Euro account","to_account":"Dollar account","amount":1,"operation_id":%q}`
	for _, id := range []string{strings.Repeat("a", 65), "has space", "slash/", "é"} {
		raw := json.RawMessage(fmt.Sprintf(base, id))
		if _, err := buildAssistantTransfer(raw, from, to, currency.Rates{}, time.Now(), "Transfer"); err == nil {
			t.Errorf("operation id %q unexpectedly accepted", id)
		}
	}
	raw := json.RawMessage(`{"from_account":"Euro account","to_account":"Dollar account","amount":1}`)
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	built, err := buildAssistantTransfer(raw, from, to, currency.Rates{Base: "EUR"}, now, "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if built.params.OperationID == "" || !validTransferOperationID(built.params.OperationID) {
		t.Errorf("generated operation id is invalid: %q", built.params.OperationID)
	}
	rebuilt, err := buildAssistantTransfer(raw, from, to, currency.Rates{Base: "EUR"}, now, "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if built.params.OperationID != rebuilt.params.OperationID {
		t.Errorf("identical transfer inputs generated different operation IDs: %q != %q", built.params.OperationID, rebuilt.params.OperationID)
	}
}
