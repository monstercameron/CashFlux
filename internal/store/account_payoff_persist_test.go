package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// The per-account "include in payoff" choice rides the existing accounts JSON, so
// it must survive an export/import round-trip (including the explicit-false case,
// which omitempty must not drop because it's a non-nil pointer).
func TestAccountIncludeInPayoffRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	no := false
	acc := domain.Account{
		ID: "l1", Name: "Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(-100000, "USD"),
		IncludeInPayoff: &no,
	}
	if err := st.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}

	ds, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(imported.Accounts) != 1 {
		t.Fatalf("want 1 account, got %d", len(imported.Accounts))
	}
	got := imported.Accounts[0]
	if got.IncludeInPayoff == nil || *got.IncludeInPayoff != false {
		t.Fatalf("IncludeInPayoff lost in round-trip: %v", got.IncludeInPayoff)
	}
	if got.IncludedInPayoff() {
		t.Errorf("an explicitly-excluded card should report IncludedInPayoff() = false")
	}
}
