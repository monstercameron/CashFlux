// SPDX-License-Identifier: MIT

package transferpair

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(n int) time.Time {
	return time.Date(2026, 7, n, 0, 0, 0, 0, time.UTC)
}

func accounts() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "SCCU Checkings", Mask: "8945", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "sav", Name: "SCCU Savings", Mask: "6500", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"},
		{ID: "cns", Name: "CNS Loan", Mask: "1958", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD"},
		{ID: "card", Name: "Apple Card", Mask: "1677", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD"},
	}
}

func tx(id, acct, desc string, minor int64, d time.Time) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Desc: desc,
		Amount: money.New(minor, "USD"), Date: d,
	}
}

func asTransfer(t domain.Transaction, to string) domain.Transaction {
	t.TransferAccountID = to
	return t
}

// The case the package exists for: two legs of one sweep, each naming the other
// account. Amount and date alone would leave this merely "possible"; the
// descriptor settles it.
func TestDescriptorEvidenceVerifiesAPair(t *testing.T) {
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	in := tx("i", "sav", "Transfer from Checking *8945", 50000, day(14))

	m := For(out, []domain.Transaction{out, in}, accounts())
	if m.Confidence != Verified {
		t.Fatalf("confidence = %s, want verified (reasons %v)", m.Confidence, m.Reasons)
	}
	if m.Partner.ID != "i" {
		t.Errorf("partner = %q, want i", m.Partner.ID)
	}
	if !m.Has(ReasonDescriptorNames) || !m.Has(ReasonAmount) || !m.Has(ReasonSameDay) {
		t.Errorf("reasons = %v, want the descriptor, amount and same-day evidence", m.Reasons)
	}
}

// Two identical sweeps on one day. Every amount-and-date rule in the app calls
// these interchangeable; only the descriptors tell them apart, and where they
// cannot, the matcher must refuse rather than pick.
func TestTwoIdenticalSweepsAreNotGuessedAt(t *testing.T) {
	out := tx("o", "chk", "Online transfer", -75000, day(14))
	a := tx("a", "sav", "Online transfer", 75000, day(14))
	b := tx("b", "cns", "Online transfer", 75000, day(14))

	m := For(out, []domain.Transaction{out, a, b}, accounts())
	if m.Confidence != Possible {
		t.Fatalf("confidence = %s, want possible — two rows fit equally", m.Confidence)
	}
	if !m.Has(ReasonSeveralCandidates) {
		t.Errorf("reasons = %v, want several-candidates", m.Reasons)
	}
	if len(m.Others) != 1 {
		t.Errorf("Others = %d, want the alternative offered rather than hidden", len(m.Others))
	}
}

// ...and where the descriptor DOES name one of them, it decides.
func TestDescriptorPicksBetweenTwoIdenticalCandidates(t *testing.T) {
	out := tx("o", "chk", "Transfer to account ending 1958", -75000, day(14))
	a := tx("a", "sav", "Online transfer", 75000, day(14))
	b := tx("b", "cns", "Online transfer", 75000, day(14))

	m := For(out, []domain.Transaction{out, a, b}, accounts())
	if m.Confidence != Verified {
		t.Fatalf("confidence = %s, want verified (reasons %v)", m.Confidence, m.Reasons)
	}
	if m.Partner.ID != "b" {
		t.Errorf("partner = %q, want b — the descriptor names 1958", m.Partner.ID)
	}
}

// A descriptor naming an account the only candidate is NOT on argues against the
// pair. It must not be treated as an absence of evidence.
func TestDescriptorDisagreementDowngradesTheMatch(t *testing.T) {
	out := tx("o", "chk", "Transfer to account ending 1958", -75000, day(14))
	other := tx("s", "sav", "Deposit", 75000, day(14))

	m := For(out, []domain.Transaction{out, other}, accounts())
	if m.Confidence != Possible {
		t.Fatalf("confidence = %s, want possible", m.Confidence)
	}
	if !m.Has(ReasonDescriptorDisagrees) {
		t.Errorf("reasons = %v, want descriptor-disagrees", m.Reasons)
	}
}

// A transfer can leave on Friday and land on Monday.
func TestSettlementLagStillPairs(t *testing.T) {
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(10))
	in := tx("i", "sav", "Transfer from Checking *8945", 50000, day(13))

	m := For(out, []domain.Transaction{out, in}, accounts())
	if m.Confidence != Verified {
		t.Fatalf("confidence = %s, want verified across a weekend", m.Confidence)
	}
	if m.DaysApart != 3 {
		t.Errorf("DaysApart = %d, want 3", m.DaysApart)
	}
	if !m.Has(ReasonSettlementLag) {
		t.Errorf("reasons = %v, want settlement-lag", m.Reasons)
	}
}

func TestBeyondTheWindowIsNotAPair(t *testing.T) {
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(1))
	in := tx("i", "sav", "Transfer from Checking *8945", 50000, day(20))

	if m := For(out, []domain.Transaction{out, in}, accounts()); m.Confidence != Unmatched {
		t.Errorf("confidence = %s, want unmatched nineteen days apart", m.Confidence)
	}
}

func TestNonPairableRowsAreRejected(t *testing.T) {
	base := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	cases := []struct {
		name  string
		other domain.Transaction
	}{
		{"same account", tx("i", "chk", "Transfer from Checking *8945", 50000, day(14))},
		{"different amount", tx("i", "sav", "Transfer from Checking *8945", 50001, day(14))},
		{"same sign", tx("i", "sav", "Transfer from Checking *8945", -50000, day(14))},
		{"already linked elsewhere", asTransfer(tx("i", "sav", "Transfer from Checking *8945", 50000, day(14)), "cns")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if m := For(base, []domain.Transaction{base, c.other}, accounts()); m.Confidence != Unmatched {
				t.Errorf("confidence = %s, want unmatched", m.Confidence)
			}
		})
	}
}

// Two amounts sharing a number in different currencies are not the same money.
func TestDifferentCurrenciesNeverPair(t *testing.T) {
	accs := append(accounts(), domain.Account{ID: "eur", Name: "EU", Mask: "3311", Class: domain.ClassAsset, Currency: "EUR"})
	out := tx("o", "chk", "Transfer to account ending 3311", -50000, day(14))
	in := domain.Transaction{ID: "i", AccountID: "eur", Desc: "Transfer from Checking *8945",
		Amount: money.New(50000, "EUR"), Date: day(14)}

	if m := For(out, []domain.Transaction{out, in}, accs); m.Confidence != Unmatched {
		t.Errorf("confidence = %s, want unmatched across currencies", m.Confidence)
	}
}

// An existing group is somebody's explicit decision and outranks every heuristic
// — including one that would otherwise find a closer-looking row.
func TestAnExistingGroupOutranksTheHeuristics(t *testing.T) {
	out := tx("o", "chk", "Transfer", -50000, day(14))
	out.TransferGroupID = "g1"
	linked := tx("i", "sav", "Transfer", 50000, day(16))
	linked.TransferGroupID = "g1"
	decoy := tx("d", "cns", "Transfer from Checking *8945", 50000, day(14))

	m := For(out, []domain.Transaction{out, linked, decoy}, accounts())
	if m.Confidence != Verified || m.Partner.ID != "i" {
		t.Errorf("match = %s/%s, want verified against the grouped leg", m.Confidence, m.Partner.ID)
	}
	if !m.Has(ReasonAlreadyLinked) {
		t.Errorf("reasons = %v, want already-linked", m.Reasons)
	}
}

// Without masks there is no descriptor evidence to be had, so nothing reaches
// Verified — the matcher must not quietly lower its bar.
func TestWithoutMasksNothingIsVerified(t *testing.T) {
	bare := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Currency: "USD"},
	}
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	in := tx("i", "sav", "Transfer from Checking *8945", 50000, day(14))

	m := For(out, []domain.Transaction{out, in}, bare)
	if m.Confidence != Possible {
		t.Errorf("confidence = %s, want possible with no masks configured", m.Confidence)
	}
	if !m.Has(ReasonNoDescriptorEvidence) && !m.Has(ReasonDescriptorDisagrees) {
		t.Errorf("reasons = %v, want the missing evidence named", m.Reasons)
	}
}

// Two accounts ending in the same four digits cannot be told apart, so the mask
// must stop being evidence rather than pointing at whichever was found first.
func TestDuplicateMasksAreNotEvidence(t *testing.T) {
	accs := []domain.Account{
		{ID: "chk", Name: "Checking", Mask: "8945", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav", Name: "Savings", Mask: "6500", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav2", Name: "Savings Two", Mask: "6500", Class: domain.ClassAsset, Currency: "USD"},
	}
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	a := tx("a", "sav", "Deposit", 50000, day(14))
	b := tx("b", "sav2", "Deposit", 50000, day(14))

	m := For(out, []domain.Transaction{out, a, b}, accs)
	if m.Confidence == Verified {
		t.Errorf("confidence = verified — two accounts share the mask 6500, so it settles nothing")
	}
}

func TestZeroAmountRowsNeverPair(t *testing.T) {
	a := tx("a", "chk", "Adjustment", 0, day(14))
	b := tx("b", "sav", "Adjustment", 0, day(14))
	if m := For(a, []domain.Transaction{a, b}, accounts()); m.Confidence != Unmatched {
		t.Errorf("confidence = %s, want unmatched for a zero amount", m.Confidence)
	}
}
