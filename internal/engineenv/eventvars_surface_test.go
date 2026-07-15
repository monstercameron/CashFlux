// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEventVarBasesCollision(t *testing.T) {
	bases := EventVarBases([]EventDef{{Name: "Trip"}, {Name: "Trip"}})
	if len(bases) != 2 || bases[0].Prefix != "event_trip_" || bases[1].Prefix != "event_trip_2_" {
		t.Fatalf("bases=%+v", bases)
	}
}

func TestAddEventVars(t *testing.T) {
	d := Data{
		Now:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		Rates: currency.Rates{Base: "USD"},
		Transactions: []domain.Transaction{
			{ID: "a", Amount: money.New(-1000, "USD")},
			{ID: "b", Amount: money.New(-3000, "USD")},
			{ID: "c", Amount: money.New(500, "USD")},
			{ID: "x", Amount: money.New(-9999, "USD"), TransferAccountID: "acct2"},
		},
		Events: []EventDef{{Name: "Portugal Trip", TxnIDs: []string{"a", "b", "c", "x"}}},
	}
	out := Vars(d)
	if got := out["event_portugal_trip_total"]; got != -35.0 {
		t.Fatalf("total=%v want -35", got)
	}
	if got := out["event_portugal_trip_spend"]; got != 35.0 {
		t.Fatalf("spend=%v want 35", got)
	}
	if got := out["event_portugal_trip_count"]; got != 3 {
		t.Fatalf("count=%v want 3 (transfer excluded)", got)
	}
}
