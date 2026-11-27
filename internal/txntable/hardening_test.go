// SPDX-License-Identifier: MIT

package txntable

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestDeriveRowHardening(t *testing.T) {
	d := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "cc", Name: "Credit Card", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD", OpeningBalance: money.New(50000, "USD")},
	}
	// Payment arriving on a positive-owed credit card is booked negative, but direction is INCOMING ("transferFrom").
	tx := domain.Transaction{
		ID: "in", AccountID: "cc", TransferAccountID: "chk", BillAccountID: "cc",
		Date: d, Amount: money.New(-10000, "USD"),
	}
	idx := BuildIndex([]domain.Transaction{tx}, accounts, nil, nil, nil, nil)
	ctx := RowContext{
		Index: idx, Now: d,
		Labels: Labels{
			T:      func(key string, args ...any) string { return key },
			Ledger: func(m money.Money, _ bool) (string, string) { return m.String(), "" },
			Money:  func(m money.Money) string { return m.String() },
		},
	}
	row := DeriveRow(ctx, RowInput{ID: "in", Date: d, Amount: tx.Amount, FollowUpTotal: 2, FollowUpOpen: 0})
	if row.CategoryTooltip != "r4c.transferFrom" {
		t.Fatalf("positive-owed incoming leg tooltip = %q, want r4c.transferFrom", row.CategoryTooltip)
	}
	if strings.Contains(row.InfoText, "uxt5.openFollowUp") {
		t.Fatalf("completed follow-ups emitted open follow-up text: %q", row.InfoText)
	}

	// VisibleTransfersWithAccounts collapses legacy positive-owed transfer pair to one row.
	out := domain.Transaction{ID: "out", AccountID: "chk", TransferAccountID: "cc", Date: d, Amount: money.New(-10000, "USD")}
	vis := VisibleTransfersWithAccounts([]domain.Transaction{out, tx}, accounts, txnfilter.Criteria{}, currency.Rates{Base: "USD"})
	if len(vis) != 1 || vis[0].ID != "out" {
		t.Fatalf("VisibleTransfersWithAccounts = %+v, want single outgoing row", vis)
	}
}
