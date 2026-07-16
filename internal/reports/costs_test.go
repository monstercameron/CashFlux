// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func costTxn(desc, payee, cat string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Desc: desc, Payee: payee, CategoryID: cat, Amount: money.New(-major*100, "USD"), Date: on}
}

func TestCostOfMoney(t *testing.T) {
	start, end := dt(2026, time.January, 1), dt(2027, time.January, 1)
	cats := []domain.Category{{ID: "fees", Name: "Fees & Charges", Kind: domain.KindExpense}, {ID: "dining", Name: "Dining", Kind: domain.KindExpense}}
	txns := []domain.Transaction{
		costTxn("ATM fee", "NON-NETWORK ATM", "dining", 3, dt(2026, time.March, 5)),
		costTxn("Late payment fee", "Beacon Bank", "", 39, dt(2026, time.April, 2)),
		costTxn("Overdraft", "Beacon Bank", "", 35, dt(2026, time.May, 9)),
		costTxn("Card annual membership", "Beacon Bank", "fees", 95, dt(2026, time.June, 1)), // matches via the category name
		costTxn("Interest charge", "Beacon Bank", "", 62, dt(2026, time.July, 22)),
		costTxn("Coffee shop", "Brew Bros", "dining", 6, dt(2026, time.March, 6)),                 // "coffee" must NOT match "fee"
		costTxn("Pinterest Premium", "Pinterest", "", 5, dt(2026, time.March, 7)),                 // "Pinterest" must NOT match "interest"
		costTxn("ATM fee", "X", "", 99, dt(2025, time.December, 30)),                              // out of range
		{Desc: "Interest refund", Amount: money.New(1200, "USD"), Date: dt(2026, time.August, 1)}, // income — excluded
	}
	got, err := CostOfMoney(txns, cats, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FeeCount != 4 || got.FeeTotal != (3+39+35+95)*100 {
		t.Errorf("fees = %d x %d, want 4 x 17200", got.FeeTotal, got.FeeCount)
	}
	if got.InterestCount != 1 || got.InterestTotal != 6200 {
		t.Errorf("interest = %d x %d, want 1 x 6200", got.InterestTotal, got.InterestCount)
	}
	if len(got.Items) != 5 {
		t.Fatalf("items = %d, want 5: %+v", len(got.Items), got.Items)
	}
	// Largest first: the $95 annual fee leads.
	if got.Items[0].Amount != 9500 || got.Items[0].Interest {
		t.Errorf("items[0] = %+v, want the $95 fee", got.Items[0])
	}
}

func TestCostOfMoney_InterestWinsOverFee(t *testing.T) {
	start, end := dt(2026, time.January, 1), dt(2027, time.January, 1)
	txns := []domain.Transaction{costTxn("Interest charge fee", "Bank", "", 10, dt(2026, time.February, 1))}
	got, err := CostOfMoney(txns, nil, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InterestCount != 1 || got.FeeCount != 0 {
		t.Errorf("want the both-token charge classified as interest only: %+v", got)
	}
}
