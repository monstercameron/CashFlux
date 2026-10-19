// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestLegacyLiabilityPairDirections(t *testing.T) {
	for _, tt := range []struct {
		name    string
		class   domain.AccountClass
		sign    domain.LiabilitySign
		out, in int64
		want    bool
	}{
		{"positive owed payment", domain.ClassLiability, domain.LiabilityPositiveOwed, -10000, -10000, true},
		{"positive owed advance", domain.ClassLiability, domain.LiabilityPositiveOwed, 10000, 10000, true},
		{"negative owed payment", domain.ClassLiability, domain.LiabilityNegativeOwed, -10000, 10000, true},
		{"asset same direction", domain.ClassAsset, "", -10000, -10000, false},
		{"wrong liability direction", domain.ClassLiability, domain.LiabilityPositiveOwed, -10000, 10000, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			accounts := []domain.Account{{ID: "cash", Class: domain.ClassAsset}, {ID: "card", Class: tt.class, LiabilitySign: tt.sign}}
			out := pairTxn("out", "cash", tt.out, 0)
			out.TransferAccountID = "card"
			in := pairTxn("in", "card", tt.in, 1)
			in.TransferAccountID = "cash"
			match, found := FindReciprocal(out, "card", []domain.Transaction{out, in}, accounts...)
			if found != tt.want || found && !match.Exact {
				t.Fatalf("match = %+v, found = %v, want %v", match, found, tt.want)
			}
			summary := SummarizeVisibleTransfersWithAccounts([]domain.Transaction{out, in}, accounts)
			wantUnpaired := 2
			if tt.want {
				wantUnpaired = 0
			}
			if summary.UnpairedRows != wantUnpaired {
				t.Fatalf("summary = %+v, want %d unpaired", summary, wantUnpaired)
			}
			if tt.want && summary.CountedRows[0] != (out.Amount.Amount < 0) {
				t.Fatalf("representative is not the outgoing money leg: %+v", summary)
			}
			in.ID = "second-candidate"
			if got := AmbiguousLeg(out, "card", []domain.Transaction{match.Txn, in}, accounts...); got != tt.want {
				t.Fatalf("ambiguous = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyBothNegativePairRequiresMatchingMagnitude(t *testing.T) {
	accounts := []domain.Account{{ID: "cash", Class: domain.ClassAsset}, {ID: "card", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed}}
	out := pairTxn("out", "cash", -10000, 0)
	out.TransferAccountID = "card"
	in := pairTxn("in", "card", -9000, 0)
	in.TransferAccountID = "cash"
	match, found := FindReciprocal(out, "card", []domain.Transaction{in}, accounts...)
	if !found || match.Exact {
		t.Fatalf("near match = %+v, found = %v", match, found)
	}
	summary := SummarizeVisibleTransfersWithAccounts([]domain.Transaction{out, in}, accounts)
	if summary.UnpairedRows != 2 {
		t.Fatalf("unequal amounts paired: %+v", summary)
	}
	in.Amount = money.New(-5000, "EUR")
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	summary = SummarizeVisibleTransfersWithAccounts([]domain.Transaction{out, in}, accounts, rates)
	counted := 0
	for _, count := range summary.CountedRows {
		if count {
			counted++
		}
	}
	if summary.UnpairedRows != 0 || counted != 1 {
		t.Fatalf("FX both-negative summary = %+v", summary)
	}
	if !summary.CountedRows[0] {
		t.Fatalf("FX payment did not count the cash source: %+v", summary)
	}
	out.TransferGroupID, in.TransferGroupID = "payment", "payment"
	summary = SummarizeVisibleTransfersWithAccounts([]domain.Transaction{in, out}, accounts, rates)
	if !summary.CountedRows[1] || summary.CountedRows[0] {
		t.Fatalf("grouped FX payment did not count the cash source: %+v", summary)
	}
}

func TestLiabilityPreviewFindsBothNegativePartner(t *testing.T) {
	out := pairTxn("out", "chk", -10000, 0)
	in := pairTxn("in", "card", -10000, 0)
	p := PreviewApply(out, "card", false, pairAccounts(), []domain.Transaction{out, in})
	if p.Err != nil || !p.HasPartner || !p.Partner.Exact {
		t.Fatalf("preview = %+v", p)
	}
}
