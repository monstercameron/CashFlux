package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// memberExpense builds a non-transfer spend by a member in USD.
func memberExpense(member string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{MemberID: member, Amount: money.New(-major*100, "USD"), Date: on}
}

func TestSpendingByMemberSortedAndExcludes(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		memberExpense("alice", 100, dt(2026, time.June, 5)),
		memberExpense("alice", 50, dt(2026, time.June, 20)),
		memberExpense("bob", 400, dt(2026, time.June, 10)),
		memberExpense("bob", 999, dt(2026, time.May, 31)),                                                        // out of range — excluded
		{MemberID: "alice", Amount: money.New(5000, "USD"), Date: dt(2026, time.June, 10)},                       // income — excluded
		{MemberID: "bob", Amount: money.New(-7000, "USD"), TransferAccountID: "x", Date: dt(2026, time.June, 9)}, // transfer — excluded
	}
	got, err := SpendingByMember(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d members, want 2: %+v", len(got), got)
	}
	// bob (400) sorts before alice (150); largest first.
	if got[0].MemberID != "bob" || got[0].Amount != 40000 {
		t.Errorf("first = %+v, want bob 40000", got[0])
	}
	if got[1].MemberID != "alice" || got[1].Amount != 15000 {
		t.Errorf("second = %+v, want alice 15000", got[1])
	}
}

func TestSpendingByMemberEmptyMemberID(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		memberExpense("", 30, dt(2026, time.June, 5)), // unattributed
		memberExpense("alice", 20, dt(2026, time.June, 6)),
	}
	got, err := SpendingByMember(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (alice + unattributed)", len(got))
	}
	// alice (3000... wait) — alice 20 → 2000, unattributed 30 → 3000; unattributed first.
	if got[0].MemberID != "" || got[0].Amount != 3000 {
		t.Errorf("first = %+v, want unattributed 3000", got[0])
	}
}

func TestSpendingByMemberEmpty(t *testing.T) {
	got, err := SpendingByMember(nil, dt(2026, time.June, 1), dt(2026, time.July, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input should yield no rows, got %+v", got)
	}
}
