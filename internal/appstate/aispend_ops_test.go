// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestWriteAISpendDoesNotWriteWhenTheSettingsReadFails(t *testing.T) {
	writes := 0
	readErr := errors.New("read settings: transient failure")
	err := writeAISpend(
		func() (store.Settings, error) { return store.Settings{}, readErr },
		func(store.Settings) error { writes++; return nil },
		aispend.Entry{Feature: "assistant", At: time.Now(), Tokens: 10},
	)
	if !errors.Is(err, readErr) {
		t.Fatalf("writeAISpend = %v, want the read error", err)
	}
	if writes != 0 {
		t.Fatalf("%d settings writes after a failed read, want none", writes)
	}
}

func TestWriteAISpendKeepsTheRestOfTheSettings(t *testing.T) {
	var saved store.Settings
	in := store.Settings{BaseCurrency: "CAD", FXRates: map[string]float64{"USD": 1.3}, AISpendCapUSD: 20}
	err := writeAISpend(
		func() (store.Settings, error) { return in, nil },
		func(s store.Settings) error { saved = s; return nil },
		aispend.Entry{Feature: "assistant", At: time.Now(), Tokens: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BaseCurrency != "CAD" || saved.FXRates["USD"] != 1.3 || saved.AISpendCapUSD != 20 {
		t.Fatalf("saved settings lost fields: %+v", saved)
	}
	if len(saved.AISpend) != 1 || saved.AISpend[0].Calls != 1 || saved.AISpend[0].Tokens != 10 {
		t.Fatalf("saved spend record = %+v, want one call of 10 tokens", saved.AISpend)
	}
}
