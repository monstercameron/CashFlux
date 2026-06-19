package dedupe

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func txn(idStr, desc string, minor int64, on time.Time) domain.Transaction {
	return domain.Transaction{ID: idStr, Desc: desc, Amount: money.New(minor, "USD"), Date: on}
}

func TestFindDuplicates(t *testing.T) {
	txns := []domain.Transaction{
		txn("a", "Coffee", -500, day(2026, time.June, 1)),
		txn("b", " coffee ", -500, day(2026, time.June, 1)), // same date/amount/desc (normalized) → dup of a
		txn("c", "Coffee", -500, day(2026, time.June, 2)),   // different date → not a dup
		txn("d", "Rent", -100000, day(2026, time.June, 1)),
		txn("e", "Rent", -100000, day(2026, time.June, 1)), // dup of d
		txn("f", "Rent", -100001, day(2026, time.June, 1)), // 1 cent off → not a dup
		{ID: "x", Desc: "Move", Amount: money.New(-200, "USD"), TransferAccountID: "acc", Date: day(2026, time.June, 1)},
		{ID: "y", Desc: "Move", Amount: money.New(-200, "USD"), TransferAccountID: "acc", Date: day(2026, time.June, 1)}, // transfers excluded
	}
	groups := FindDuplicates(txns)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 (coffee, rent): %+v", len(groups), groups)
	}
	// 2 dup pairs → 2 removable.
	if c := Count(groups); c != 2 {
		t.Errorf("Count = %d, want 2", c)
	}
	// Find the coffee group; it should contain a and b only, sorted.
	var coffee *Group
	for i := range groups {
		if groups[i].Description == "Coffee" || groups[i].Description == "coffee" {
			coffee = &groups[i]
		}
	}
	if coffee == nil {
		t.Fatalf("no coffee group: %+v", groups)
	}
	if len(coffee.IDs) != 2 || coffee.IDs[0] != "a" || coffee.IDs[1] != "b" {
		t.Errorf("coffee ids = %v, want [a b]", coffee.IDs)
	}
}

func TestFindDuplicatesNone(t *testing.T) {
	txns := []domain.Transaction{
		txn("a", "Coffee", -500, day(2026, time.June, 1)),
		txn("b", "Tea", -500, day(2026, time.June, 1)),
	}
	if groups := FindDuplicates(txns); len(groups) != 0 {
		t.Errorf("distinct transactions should yield no groups, got %+v", groups)
	}
	if Count(nil) != 0 {
		t.Error("Count(nil) should be 0")
	}
}

func TestFindDuplicatesTriple(t *testing.T) {
	d := day(2026, time.June, 3)
	txns := []domain.Transaction{
		txn("a", "Lunch", -1500, d),
		txn("b", "Lunch", -1500, d),
		txn("c", "Lunch", -1500, d),
	}
	groups := FindDuplicates(txns)
	if len(groups) != 1 || len(groups[0].IDs) != 3 {
		t.Fatalf("triple should be one group of 3: %+v", groups)
	}
	if Count(groups) != 2 { // 3 entries → 2 removable
		t.Errorf("Count = %d, want 2", Count(groups))
	}
}
