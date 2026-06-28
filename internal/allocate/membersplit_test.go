// SPDX-License-Identifier: MIT

package allocate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// makeRates builds a minimal currency.Rates table for tests.
func makeRates(base string, pairs map[string]float64) currency.Rates {
	return currency.Rates{Base: base, Rates: pairs}
}

// incomeTxn builds a domain.Transaction marked as income (positive, non-transfer).
func incomeTxn(amount int64, cur, date string) domain.Transaction {
	t, _ := time.Parse("2006-01-02", date)
	return domain.Transaction{
		Amount: money.New(amount, cur),
		Date:   t,
		// IsIncome() returns true when Amount is positive and IsTransfer() is false.
	}
}

// expenseTxn builds a domain.Transaction marked as expense (negative, non-transfer).
func expenseTxn(amount int64, cur, date string) domain.Transaction {
	t, _ := time.Parse("2006-01-02", date)
	return domain.Transaction{
		Amount: money.New(-amount, cur),
		Date:   t,
	}
}

// member builds a simple domain.Member fixture.
func member(id, name string) domain.Member {
	return domain.Member{ID: id, Name: name}
}

func TestSplitPeriodIncome(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-06-01")
	end, _ := time.Parse("2006-01-02", "2026-07-01")
	rates := makeRates("USD", nil)

	t.Run("empty members returns nil", func(t *testing.T) {
		txns := []domain.Transaction{incomeTxn(100000, "USD", "2026-06-15")}
		got, err := SplitPeriodIncome(txns, nil, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("only group owner member returns nil", func(t *testing.T) {
		txns := []domain.Transaction{incomeTxn(100000, "USD", "2026-06-15")}
		grpMember := domain.Member{ID: domain.GroupOwnerID, Name: "Group"}
		got, err := SplitPeriodIncome(txns, []domain.Member{grpMember}, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("single member gets all income", func(t *testing.T) {
		txns := []domain.Transaction{
			incomeTxn(150000, "USD", "2026-06-10"), // $1500
			expenseTxn(30000, "USD", "2026-06-12"), // excluded
		}
		members := []domain.Member{member("alice", "Alice")}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 result, got %d", len(got))
		}
		if got[0].MemberID != "alice" {
			t.Errorf("MemberID = %q, want %q", got[0].MemberID, "alice")
		}
		if got[0].Amount != 150000 {
			t.Errorf("Amount = %d, want 150000", got[0].Amount)
		}
	})

	t.Run("two members split equally", func(t *testing.T) {
		txns := []domain.Transaction{
			incomeTxn(100000, "USD", "2026-06-05"), // $1000
			incomeTxn(100000, "USD", "2026-06-20"), // $1000 → total $2000
		}
		members := []domain.Member{
			member("alice", "Alice"),
			member("bob", "Bob"),
		}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 results, got %d", len(got))
		}
		// Verify exact sum invariant.
		var sum int64
		for _, s := range got {
			sum += s.Amount
		}
		if sum != 200000 {
			t.Errorf("sum = %d, want 200000", sum)
		}
		// Each member should get 100000.
		for _, s := range got {
			if s.Amount != 100000 {
				t.Errorf("member %s: Amount = %d, want 100000", s.MemberID, s.Amount)
			}
		}
	})

	t.Run("indivisible total distributed by Hamilton", func(t *testing.T) {
		// 10 cents among 3 members = 3+4+3 or 4+3+3 depending on remainder sort.
		txns := []domain.Transaction{incomeTxn(10, "USD", "2026-06-15")}
		members := []domain.Member{
			member("a", "A"),
			member("b", "B"),
			member("c", "C"),
		}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var sum int64
		for _, s := range got {
			sum += s.Amount
		}
		if sum != 10 {
			t.Errorf("parts sum = %d, want 10 (exact-sum invariant)", sum)
		}
	})

	t.Run("zero income yields zero splits", func(t *testing.T) {
		// Only expenses — no income.
		txns := []domain.Transaction{
			expenseTxn(50000, "USD", "2026-06-10"),
		}
		members := []domain.Member{
			member("alice", "Alice"),
			member("bob", "Bob"),
		}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 results, got %d", len(got))
		}
		for _, s := range got {
			if s.Amount != 0 {
				t.Errorf("member %s: Amount = %d, want 0 when no income", s.MemberID, s.Amount)
			}
		}
	})

	t.Run("FX path converts to base currency", func(t *testing.T) {
		// EUR income at 1.1 USD/EUR → 11000 USD-equivalent.
		fxRates := makeRates("USD", map[string]float64{"EUR": 1.1})
		txns := []domain.Transaction{
			incomeTxn(10000, "EUR", "2026-06-15"), // 10000 EUR-cents = €100 → $110 = 11000 USD-cents
			incomeTxn(9000, "USD", "2026-06-20"),  // 9000 USD-cents = $90
		}
		members := []domain.Member{
			member("alice", "Alice"),
			member("bob", "Bob"),
		}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", fxRates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var sum int64
		for _, s := range got {
			sum += s.Amount
		}
		// total = 11000 + 9000 = 20000; split 2-ways = 10000 each.
		wantTotal := int64(20000)
		if sum != wantTotal {
			t.Errorf("sum = %d, want %d", sum, wantTotal)
		}
		for _, s := range got {
			if s.Amount != 10000 {
				t.Errorf("member %s: Amount = %d, want 10000", s.MemberID, s.Amount)
			}
		}
	})

	t.Run("FX error propagates", func(t *testing.T) {
		// No rate for XYZ → conversion error should surface.
		noRates := makeRates("USD", nil)
		txns := []domain.Transaction{incomeTxn(100, "XYZ", "2026-06-15")}
		members := []domain.Member{member("alice", "Alice")}
		_, err := SplitPeriodIncome(txns, members, start, end, "USD", noRates)
		if err == nil {
			t.Error("expected error for unknown FX rate, got nil")
		}
		if !errors.Is(err, currency.ErrUnknownRate) {
			t.Errorf("want ErrUnknownRate, got %v", err)
		}
	})

	t.Run("transfers excluded", func(t *testing.T) {
		// A transfer should not count as income.
		transfer := domain.Transaction{
			Amount:            money.New(50000, "USD"),
			Date:              func() time.Time { t, _ := time.Parse("2006-01-02", "2026-06-10"); return t }(),
			TransferAccountID: "acc-other", // marks it as a transfer
		}
		txns := []domain.Transaction{
			transfer,
			incomeTxn(30000, "USD", "2026-06-15"),
		}
		members := []domain.Member{member("alice", "Alice")}
		got, err := SplitPeriodIncome(txns, members, start, end, "USD", rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].Amount != 30000 {
			t.Errorf("Amount = %d, want 30000 (transfer excluded)", got[0].Amount)
		}
	})
}
