// SPDX-License-Identifier: MIT

package txnmove

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestMovePreservesEconomicDirectionAndSplits(t *testing.T) {
	accounts := []domain.Account{
		{ID: "cash", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "positive", Class: domain.ClassLiability, Currency: "USD", LiabilitySign: domain.LiabilityPositiveOwed},
		{ID: "negative", Class: domain.ClassLiability, Currency: "USD", LiabilitySign: domain.LiabilityNegativeOwed},
	}
	for _, tt := range []struct {
		from, to     string
		amount, want int64
	}{
		{"cash", "positive", -10000, 10000},
		{"positive", "cash", 10000, -10000},
		{"cash", "negative", -10000, -10000},
		{"positive", "negative", 10000, -10000},
		{"negative", "positive", -10000, 10000},
		{"cash", "positive", 10000, -10000},
	} {
		t.Run(tt.from+" to "+tt.to+money.New(tt.amount, "USD").String(), func(t *testing.T) {
			in := tx("purchase", tt.from, tt.amount)
			in.Splits = []domain.CategorySplit{{CategoryID: "a", Amount: money.New(tt.amount/4, "USD")}, {CategoryID: "b", Amount: money.New(tt.amount*3/4, "USD")}}
			got, _, found, err := Apply(in, tt.to, accounts, nil)
			if err != nil {
				t.Fatal(err)
			}
			if found || got.Amount.Amount != tt.want || got.Splits[0].Amount.Amount != tt.want/4 || got.Splits[1].Amount.Amount != tt.want*3/4 {
				t.Fatalf("moved = %+v", got)
			}
			if in.Splits[0].Amount.Amount != tt.amount/4 {
				t.Fatal("move mutated original splits")
			}
			back, _, _, err := Apply(got, tt.from, accounts, nil)
			if err != nil || back.Amount != in.Amount {
				t.Fatalf("move round trip = %+v, %v", back, err)
			}
		})
	}
}

func TestMoveLegacyLiabilityPairKeepsPartnerSign(t *testing.T) {
	accounts := append(accts(), domain.Account{ID: "card", Class: domain.ClassLiability, Currency: "USD", LiabilitySign: domain.LiabilityPositiveOwed})
	out := tx("out", "chk", -10000)
	out.TransferAccountID = "card"
	in := tx("in", "card", -10000)
	in.TransferAccountID = "chk"
	got, partner, found, err := Apply(out, "sav", accounts, []domain.Transaction{out, in})
	if err != nil || !found || got.AccountID != "sav" || partner.TransferAccountID != "sav" || partner.Amount != in.Amount {
		t.Fatalf("moved = %+v, partner = %+v, found = %v, err = %v", got, partner, found, err)
	}
	// Moving the payment leg to an asset changes its booked sign, and only
	// updates the other leg's pointer; the bank's other amount stays untouched.
	got, partner, found, err = Apply(in, "sav", accounts, []domain.Transaction{out, in})
	if err != nil || !found || got.Amount.Amount != 10000 || partner.Amount != out.Amount {
		t.Fatalf("moved = %+v, partner = %+v, found = %v, err = %v", got, partner, found, err)
	}
}

func TestMoveSignOverflowReturnsError(t *testing.T) {
	accounts := append(accts(), domain.Account{ID: "card", Class: domain.ClassLiability, Currency: "USD"})
	if _, _, _, err := Apply(tx("overflow", "chk", math.MinInt64), "card", accounts, nil); err == nil {
		t.Fatal("move silently overflowed")
	}
	row := tx("split-overflow", "chk", -100)
	row.Splits = []domain.CategorySplit{{Amount: money.New(math.MinInt64, "USD")}}
	if _, _, _, err := Apply(row, "card", accounts, nil); err == nil {
		t.Fatal("move silently overflowed a split")
	}
	if row.Splits[0].Amount.Amount != math.MinInt64 {
		t.Fatal("failed move changed the original split")
	}
}
