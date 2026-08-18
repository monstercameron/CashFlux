// SPDX-License-Identifier: MIT

package reconcile

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func acct(class domain.AccountClass, openingMinor int64) domain.Account {
	return domain.Account{
		ID: "a1", Name: "Test", Class: class, Currency: "USD",
		OpeningBalance: money.New(openingMinor, "USD"),
	}
}

func TestConventionOf(t *testing.T) {
	tests := []struct {
		name        string
		acc         domain.Account
		bookedMinor int64
		want        Convention
		wantKnown   bool
	}{
		{"asset is always as-stated", acct(domain.ClassAsset, 100000), 250000, AsStated, true},
		{"asset with a negative balance is still as-stated", acct(domain.ClassAsset, 0), -5000, AsStated, true},
		{"debt opened positive stores positive-owed", acct(domain.ClassLiability, 50000), 50000, PositiveOwed, true},
		{"debt opened negative stores negative-owed", acct(domain.ClassLiability, -50000), -50000, NegativeOwed, true},
		// The opening balance records the choice the writer made. A booked balance
		// crosses zero when a card is overpaid, and inferring from it there would
		// report the opposite convention for the same account.
		{"an overpaid positive-owed card keeps its convention", acct(domain.ClassLiability, 50000), -2500, PositiveOwed, true},
		{"an overpaid negative-owed card keeps its convention", acct(domain.ClassLiability, -50000), 2500, NegativeOwed, true},
		// With no opening balance the booked sign is the only signal left, and it
		// is the very one that crosses zero — so it is used, and reported as a
		// guess rather than a reading.
		{"no opening falls back to the booked balance, unknown", acct(domain.ClassLiability, 0), 50000, PositiveOwed, false},
		{"no opening, negative booked, unknown", acct(domain.ClassLiability, 0), -50000, NegativeOwed, false},
		{"a settled debt with no opening is a guess", acct(domain.ClassLiability, 0), 0, PositiveOwed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := ConventionOf(tt.acc, tt.bookedMinor)
			if got != tt.want {
				t.Errorf("ConventionOf = %v, want %v", got, tt.want)
			}
			if known != tt.wantKnown {
				t.Errorf("known = %v, want %v", known, tt.wantKnown)
			}
		})
	}
}

// The case the adversarial pass found: a liability CREATED at zero (the add form
// allows it, and the sample data ships one) has no opening balance to read, so
// the convention comes from a booked balance that flips the moment the card goes
// into credit. The guess is still made — a balance has to be displayed — but it
// must be reported as a guess, so that nothing which ACCUSES the data of being
// wrong is built on it.
func TestZeroOpeningLiabilityIsNeverAConfidentReading(t *testing.T) {
	zeroOpened := acct(domain.ClassLiability, 0)

	// $1.00 sitting on the card. Under positive-owed that is a dollar owed; under
	// negative-owed it is a dollar of credit. Nothing here can tell which.
	got, known := ConventionOf(zeroOpened, 100)
	if known {
		t.Errorf("known = true, but the only signal is a balance that crosses zero")
	}
	if got != PositiveOwed {
		t.Errorf("guess = %v, want the likelier PositiveOwed", got)
	}

	// And the same account after the balance crosses.
	got, known = ConventionOf(zeroOpened, -100)
	if known {
		t.Errorf("known = true on the other side of zero")
	}
	if got != NegativeOwed {
		t.Errorf("guess = %v, want NegativeOwed", got)
	}
}

// The stated convention is the one a person reads: positive is money you have,
// negative is money you owe. Both storage conventions must land there.
func TestStatedNormalizesBothStorageConventions(t *testing.T) {
	const debt = 50000 // $500 owed

	if got := Stated(debt, PositiveOwed); got != -debt {
		t.Errorf("a $500 debt stored positive states as %d, want %d", got, -debt)
	}
	if got := Stated(-debt, NegativeOwed); got != -debt {
		t.Errorf("a $500 debt stored negative states as %d, want %d", got, -debt)
	}
	if got := Stated(debt, AsStated); got != debt {
		t.Errorf("an asset of $500 states as %d, want %d", got, debt)
	}
}

// An overpaid card is money the lender owes YOU. Taking -Abs — what the account
// card does today — turns that credit into more debt; the conversion must not.
func TestStatedKeepsACreditBalancePositive(t *testing.T) {
	const credit = 2500 // $25 overpaid

	if got := Stated(-credit, PositiveOwed); got != credit {
		t.Errorf("an overpaid positive-owed card states as %d, want +%d", got, credit)
	}
	if got := Stated(credit, NegativeOwed); got != credit {
		t.Errorf("an overpaid negative-owed card states as %d, want +%d", got, credit)
	}
}

func TestStoredIsTheInverseOfStated(t *testing.T) {
	for _, c := range []Convention{PositiveOwed, NegativeOwed, AsStated} {
		for _, v := range []int64{0, 1, -1, 50000, -50000, 123456789, -123456789} {
			if got := Stored(Stated(v, c), c); got != v {
				t.Errorf("convention %v: Stored(Stated(%d)) = %d, want %d", c, v, got, v)
			}
			if got := Stated(Stored(v, c), c); got != v {
				t.Errorf("convention %v: Stated(Stored(%d)) = %d, want %d", c, v, got, v)
			}
		}
	}
}

// C683's acceptance case, in the arithmetic the dialog performs. July checking:
// opening $3,210.20, closing $4,493.95. Reconciling against the statement's own
// closing figure must land on zero, not on a gap.
func TestJulyCheckingReconcilesToZero(t *testing.T) {
	const (
		openingMinor = 321020
		closingMinor = 449395
		activity     = closingMinor - openingMinor // $1,283.75 of cleared movement
	)
	chk := acct(domain.ClassAsset, openingMinor)
	conv, _ := ConventionOf(chk, openingMinor+activity)

	clearedStated := Stated(openingMinor+activity, conv)
	if clearedStated != closingMinor {
		t.Fatalf("cleared states as %d, want %d", clearedStated, closingMinor)
	}
	if got := Diff(clearedStated, closingMinor); !got.Reconciled {
		t.Errorf("Diff = %+v, want reconciled with no difference", got)
	}
}

// The reported symptom: an $8,441.80 gap on a closing balance of $4,493.95. That
// is 4,493.95 + 3,947.85 — the statement diffed against a cleared figure carrying
// the opposite sign. Comparing in one stated convention removes it.
func TestOppositeSignedClearedProducesTheReportedGap(t *testing.T) {
	const (
		closingMinor = 449395
		clearedMinor = 394785
	)
	if got := Diff(-clearedMinor, closingMinor); got.DifferenceMinor != 844180 {
		t.Fatalf("sign-flipped diff = %d, want the reported 844180", got.DifferenceMinor)
	}
	// Stated on both sides, the same figures are merely out by the real amount.
	if got := Diff(clearedMinor, closingMinor); got.DifferenceMinor != closingMinor-clearedMinor {
		t.Errorf("diff = %d, want %d", got.DifferenceMinor, closingMinor-clearedMinor)
	}
}

// A debt reconciles the same way whichever convention it is stored in — the bug
// was that it did not.
func TestDebtReconcilesIdenticallyInBothStorageConventions(t *testing.T) {
	const owed = 50000 // the statement says you owe $500, so it states as -$500

	pos := acct(domain.ClassLiability, owed)
	neg := acct(domain.ClassLiability, -owed)

	posConv, _ := ConventionOf(pos, owed)
	posStated := Stated(owed, posConv)
	negConv, _ := ConventionOf(neg, -owed)
	negStated := Stated(-owed, negConv)
	if posStated != negStated {
		t.Fatalf("the same debt states as %d and %d", posStated, negStated)
	}
	for _, stated := range []int64{posStated, negStated} {
		if got := Diff(stated, -owed); !got.Reconciled {
			t.Errorf("Diff(%d, %d) = %+v, want reconciled", stated, -owed, got)
		}
	}
}

// The adjustment path end to end, as the dialog performs it: state the cleared
// balance, diff it against the typed statement figure, then post the difference
// back in the account's OWN convention. Posting a stated difference straight onto
// a positive-owed card moves the debt the wrong way by twice itself, so this
// asserts the account lands exactly ON the statement rather than anywhere near it.
func TestAdjustmentLandsOnTheStatementInBothConventions(t *testing.T) {
	cases := []struct {
		name           string
		acc            domain.Account
		storedMinor    int64 // the cleared balance as the account holds it
		statementMinor int64 // what the user types, in the stated convention
	}{
		{
			name: "positive-owed card, statement says more is owed",
			acc:  acct(domain.ClassLiability, 50000), storedMinor: 50000, statementMinor: -62500,
		},
		{
			name: "positive-owed card, statement says less is owed",
			acc:  acct(domain.ClassLiability, 50000), storedMinor: 50000, statementMinor: -25000,
		},
		{
			name: "negative-owed card, statement says more is owed",
			acc:  acct(domain.ClassLiability, -50000), storedMinor: -50000, statementMinor: -62500,
		},
		{
			name: "negative-owed card overpaid into credit",
			acc:  acct(domain.ClassLiability, -50000), storedMinor: -50000, statementMinor: 2500,
		},
		{
			name: "checking account, July",
			acc:  acct(domain.ClassAsset, 321020), storedMinor: 321020, statementMinor: 449395,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conv, _ := ConventionOf(c.acc, c.storedMinor)
			diff := Diff(Stated(c.storedMinor, conv), c.statementMinor)
			if diff.Reconciled {
				t.Fatalf("fixture should not already reconcile")
			}
			// What the dialog posts.
			adjustment := Stored(diff.DifferenceMinor, conv)
			after := c.storedMinor + adjustment

			if got := Stated(after, conv); got != c.statementMinor {
				t.Errorf("after adjusting, the account states as %d, want the statement's %d",
					got, c.statementMinor)
			}
			if got := Diff(Stated(after, conv), c.statementMinor); !got.Reconciled {
				t.Errorf("reconciling again = %+v, want a clean match", got)
			}
		})
	}
}
