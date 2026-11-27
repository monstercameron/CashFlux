// SPDX-License-Identifier: MIT

package transferpair

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTransferPairHardening(t *testing.T) {
	d := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Mask: "1111", Class: domain.ClassAsset, Currency: "USD"},
		{
			ID: "cc", Name: "Card", Mask: "2222", Class: domain.ClassLiability,
			LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD",
			OpeningBalance: money.New(50000, "USD"),
		},
		{ID: "sav", Name: "Savings", Mask: "6500", Class: domain.ClassAsset, Currency: "USD"},
	}

	// Positive-owed credit card payment has two negative legs and must pair!
	out := domain.Transaction{ID: "out", AccountID: "chk", TransferAccountID: "cc", Date: d, Desc: "Pay *2222", Amount: money.New(-10000, "USD")}
	in := domain.Transaction{ID: "in", AccountID: "cc", TransferAccountID: "chk", Date: d, Desc: "Payment", Amount: money.New(-10000, "USD")}
	m := For(out, []domain.Transaction{out, in}, accounts)
	if m.Confidence != Verified || m.Partner.ID != "in" {
		t.Fatalf("positive-owed liability pair = %+v, want Verified with in", m)
	}

	// Descriptor disagreement must NOT be upgraded to Verified just because candidate mentions legMask.
	leg := domain.Transaction{ID: "l1", AccountID: "chk", Date: d, Desc: "Transfer to *9999", Amount: money.New(-5000, "USD")}
	other := domain.Transaction{ID: "l2", AccountID: "sav", Date: d, Desc: "From *1111", Amount: money.New(5000, "USD")}
	dm := For(leg, []domain.Transaction{leg, other}, accounts)
	if dm.Confidence == Verified || !dm.Has(ReasonDescriptorDisagrees) {
		t.Fatalf("descriptor disagreement upgraded: confidence=%v reasons=%v", dm.Confidence, dm.Reasons)
	}

	// MinInt64 must not self-pair or report balanced in NetStated.
	minA := domain.Transaction{ID: "ma", AccountID: "chk", Date: d, Amount: money.New(math.MinInt64, "USD")}
	minB := domain.Transaction{ID: "mb", AccountID: "sav", Date: d, Amount: money.New(math.MinInt64, "USD")}
	if pm := For(minA, []domain.Transaction{minA, minB}, accounts); pm.Confidence != Unmatched {
		t.Fatalf("MinInt64 paired falsely: %+v", pm)
	}
	if _, bal, known := NetStated(minA, minB, accounts); bal || known {
		t.Fatalf("NetStated(MinInt64, MinInt64) = bal=%v known=%v, want false,false", bal, known)
	}
}
