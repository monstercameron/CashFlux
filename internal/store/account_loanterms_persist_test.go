// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// C204/C206: a loan's term and origination date ride the accounts JSON, so they
// have to survive an export/import round-trip. Without them the app can compute
// a payoff PLAN (how long at $X a month) but not the actual amortization
// schedule, which is defined by the term the lender set — so losing them on
// import silently downgrades every loan to a projection.
func TestAccountLoanTermsRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	orig := time.Date(2019, time.September, 1, 0, 0, 0, 0, time.UTC)
	acc := domain.Account{
		ID: "l1", Name: "Mortgage", Currency: "USD", Type: domain.TypeMortgage,
		Class: domain.ClassLiability, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		OpeningBalance:  money.New(-24400000, "USD"),
		InterestRateAPR: domain.APR(4.1), TermMonths: 360, OriginationDate: orig,
	}
	if err := st.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}

	ds, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(imported.Accounts) != 1 {
		t.Fatalf("want 1 account, got %d", len(imported.Accounts))
	}
	got := imported.Accounts[0]
	if got.TermMonths != 360 {
		t.Errorf("TermMonths = %d, want 360", got.TermMonths)
	}
	if !got.OriginationDate.Equal(orig) {
		t.Errorf("OriginationDate = %v, want %v", got.OriginationDate, orig)
	}
	if !got.HasLoanTerms() {
		t.Error("the imported loan does not report having terms")
	}
}

// An account saved before these fields existed must import unchanged and simply
// report that its terms are unknown — never a zero-month loan, which would draw
// as already paid off.
func TestAnAccountWithoutLoanTermsImportsAsUnknown(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	acc := domain.Account{
		ID: "l2", Name: "Old Loan", Currency: "USD", Type: domain.TypeLoan,
		Class: domain.ClassLiability, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		OpeningBalance: money.New(-500000, "USD"),
	}
	if err := st.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	ds, _ := st.Snapshot()
	blob, _ := Export(ds)
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := imported.Accounts[0]
	if got.TermMonths != 0 {
		t.Errorf("TermMonths = %d, want 0", got.TermMonths)
	}
	if !got.IsInstallment() {
		t.Error("a loan stopped being installment")
	}
	if got.HasLoanTerms() {
		t.Error("a loan with no term claimed to have terms — it would draw an empty schedule")
	}
}

// The sample dataset's loans carry terms, so the amortization surfaces have
// something to show on a first run rather than four loans all asking for a
// number the user has not got to hand.
func TestSampleLoansCarryTheirTerms(t *testing.T) {
	ds := SampleDataset()
	var installment, withTerms int
	for _, a := range ds.Accounts {
		if !a.IsInstallment() {
			continue
		}
		installment++
		if a.HasLoanTerms() {
			withTerms++
		} else {
			t.Errorf("sample loan %q has no term", a.Name)
		}
		if a.OriginationDate.IsZero() {
			t.Errorf("sample loan %q has no origination date", a.Name)
		}
	}
	if installment == 0 {
		t.Fatal("the sample dataset has no installment loans to check")
	}
	if withTerms != installment {
		t.Errorf("%d of %d sample loans carry terms", withTerms, installment)
	}
	// A sample credit card must NOT gain a term — it has no schedule to draw.
	for _, a := range ds.Accounts {
		if a.Type == domain.TypeCreditCard && a.TermMonths != 0 {
			t.Errorf("sample credit card %q was given a %d-month term", a.Name, a.TermMonths)
		}
	}
}
