// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// monthly builds one expense txn on the 10th of the month `offset` months before base.
func monthlyTxn(id, cat string, base time.Time, offset int, minor int64) domain.Transaction {
	d := time.Date(base.Year(), base.Month(), 10, 0, 0, 0, 0, time.UTC).AddDate(0, -offset, 0)
	return domain.Transaction{ID: id, CategoryID: cat, Date: d, Amount: money.New(-minor, "USD")}
}

func TestSuggestTrueUpsTrailing(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cats := []domain.Category{{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense}}
	budget := domain.Budget{ID: "b1", Name: "Groceries", CategoryID: "groceries",
		Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}

	// 6 completed months at ~$480 against a $400 limit.
	var txns []domain.Transaction
	for k := 1; k <= 6; k++ {
		txns = append(txns, monthlyTxn(string(rune('a'+k)), "groceries", now, k, 48000))
	}

	ups, err := SuggestTrueUps([]domain.Budget{budget}, txns, cats, now, rates)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(ups) != 1 {
		t.Fatalf("want 1 true-up, got %d", len(ups))
	}
	u := ups[0]
	if u.Seasonal {
		t.Error("6 months of data should not be seasonal")
	}
	if u.BasisMonths != 6 {
		t.Errorf("basis months = %d, want 6", u.BasisMonths)
	}
	if u.LearnedMinor != 48000 {
		t.Errorf("learned = %d, want 48000", u.LearnedMinor)
	}
	if u.SuggestedMinor != 48000 {
		t.Errorf("suggested = %d, want 48000", u.SuggestedMinor)
	}
}

func TestSuggestTrueUpsWithinToleranceSkipped(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cats := []domain.Category{{ID: "groceries", Kind: domain.KindExpense}}
	budget := domain.Budget{ID: "b1", CategoryID: "groceries",
		Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}
	// Spending just 5% over — within tolerance, no flag.
	var txns []domain.Transaction
	for k := 1; k <= 6; k++ {
		txns = append(txns, monthlyTxn(string(rune('a'+k)), "groceries", now, k, 42000))
	}
	ups, err := SuggestTrueUps([]domain.Budget{budget}, txns, cats, now, rates)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(ups) != 0 {
		t.Errorf("within-tolerance drift should not flag, got %+v", ups)
	}
}

func TestSuggestTrueUpsSeasonal(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cats := []domain.Category{{ID: "gifts", Kind: domain.KindExpense}}
	budget := domain.Budget{ID: "b1", CategoryID: "gifts",
		Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}

	var txns []domain.Transaction
	// 24 months of low spend everywhere ($20/mo) to establish >=13 months history...
	for k := 1; k <= 24; k++ {
		txns = append(txns, monthlyTxn("low"+string(rune(k)), "gifts", now, k, 2000))
	}
	// ...but the past two Julys (12 and 24 months ago) spiked to $600.
	txns = append(txns, monthlyTxn("jul1", "gifts", now, 12, 60000))
	txns = append(txns, monthlyTxn("jul2", "gifts", now, 24, 60000))

	ups, err := SuggestTrueUps([]domain.Budget{budget}, txns, cats, now, rates)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(ups) != 1 {
		t.Fatalf("want 1 seasonal true-up, got %d: %+v", len(ups), ups)
	}
	u := ups[0]
	if !u.Seasonal {
		t.Error("with >=13 months, July should use a seasonal basis")
	}
	// Past Julys were 60000 + 62000 (the low July also present at k=12/24? No — those months
	// carry both the low and the spike). Each prior July = 2000 + 60000 = 62000; avg = 62000.
	if u.LearnedMinor != 62000 {
		t.Errorf("seasonal learned = %d, want 62000", u.LearnedMinor)
	}
}
