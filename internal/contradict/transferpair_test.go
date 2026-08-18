// SPDX-License-Identifier: MIT

package contradict

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// C686: this check used to require both legs on the SAME CALENDAR DAY, while
// internal/integrity required an exactly opposite amount and ignored dates
// entirely. A transfer that left on a Friday and landed on a Monday was
// therefore a critical error on the Accounts page and perfectly fine on
// Settings → Data. Both now ask internal/transferpair.
func TestSettlementLagIsNoLongerAnError(t *testing.T) {
	friday := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	monday := friday.AddDate(0, 0, 3)

	d := Data{
		Accounts: []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, friday),
			leg("t2", "b", "a", 50000, monday),
		},
		Now: monday,
	}
	if n := kindsOf(Check(d))[KindOneSidedTransfer]; n != 0 {
		t.Errorf("%d one-sided findings for a Friday-to-Monday transfer, want none", n)
	}
}

// The window has an end, and a leg months from its supposed partner is a real
// orphan.
func TestAFarApartLegIsStillAnOrphan(t *testing.T) {
	july := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, july),
			leg("t2", "b", "a", 50000, july.AddDate(0, 2, 0)),
		},
		Now: july,
	}
	if n := kindsOf(Check(d))[KindOneSidedTransfer]; n != 1 {
		t.Errorf("%d one-sided findings two months apart, want 1", n)
	}
}

// A declared pair is honoured regardless of amount, so legs edited apart still
// report as disagreeing rather than as one side having vanished. Reporting
// "nothing arrived in the other one" when something did arrive, for a different
// amount, sends a person looking for a transaction that is right there.
func TestDeclaredLegsThatDisagreeAreNotCalledOneSided(t *testing.T) {
	day := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, day),
			leg("t2", "b", "a", 47000, day.AddDate(0, 0, 2)),
		},
		Now: day,
	}
	got := kindsOf(Check(d))
	if got[KindOneSidedTransfer] != 0 {
		t.Errorf("%d one-sided findings, want none — the other leg exists", got[KindOneSidedTransfer])
	}
	if got[KindTransferLegsDisagree] != 1 {
		t.Errorf("%d legs-disagree findings, want 1", got[KindTransferLegsDisagree])
	}
}
