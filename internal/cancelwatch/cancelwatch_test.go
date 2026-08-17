// SPDX-License-Identifier: MIT

package cancelwatch

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cancellation(name string, on time.Time) domain.SubscriptionCancellation {
	return domain.SubscriptionCancellation{ID: "c-" + name, SubName: name, CancelledOn: on}
}

func charge(payee string, on time.Time, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: payee + on.Format("0102"), Payee: payee, Desc: payee,
		Date: on, Amount: money.New(-minor, "USD"),
	}
}

// Declaring success the day after somebody cancels is how a monitor becomes
// reassurance rather than information.
func TestSilenceTooSoonIsNotSuccess(t *testing.T) {
	cancelled := day(2026, time.August, 1)
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		nil, day(2026, time.August, 10))
	if len(got) != 1 {
		t.Fatalf("statuses = %d, want 1", len(got))
	}
	if got[0].Verdict != VerdictTooSoon {
		t.Errorf("verdict = %q, want %q nine days after cancelling", got[0].Verdict, VerdictTooSoon)
	}
}

func TestSilenceLongEnoughMeansStopped(t *testing.T) {
	cancelled := day(2026, time.May, 1)
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		nil, day(2026, time.August, 1))
	if got[0].Verdict != VerdictStopped {
		t.Errorf("verdict = %q, want %q after three quiet months", got[0].Verdict, VerdictStopped)
	}
}

// One last charge for the period already used is the expected ending, not a
// problem. Flagging it would cry wolf on the most common outcome.
func TestOneChargeInsideTheGraceWindowIsTheExpectedEnding(t *testing.T) {
	cancelled := day(2026, time.August, 1)
	txns := []domain.Transaction{charge("Streamly", day(2026, time.August, 12), 1_499)}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.August, 20))
	if got[0].Verdict != VerdictFinalCharge {
		t.Errorf("verdict = %q, want %q", got[0].Verdict, VerdictFinalCharge)
	}
	if got[0].ChargesSince != 1 || got[0].TotalSinceMinor != 1_499 {
		t.Errorf("counted %d charges totalling %d, want 1 / 1499", got[0].ChargesSince, got[0].TotalSinceMinor)
	}
}

func TestAFinalChargeFollowedBySilenceBecomesStopped(t *testing.T) {
	// The final charge is not a permanent verdict — once two quiet cycles pass
	// after it, the subscription has demonstrably stopped.
	cancelled := day(2026, time.April, 1)
	txns := []domain.Transaction{charge("Streamly", day(2026, time.April, 10), 1_499)}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.August, 1))
	if got[0].Verdict != VerdictStopped {
		t.Errorf("verdict = %q, want %q long after a single final charge", got[0].Verdict, VerdictStopped)
	}
}

func TestChargesAfterTheGraceWindowAreStillCharging(t *testing.T) {
	cancelled := day(2026, time.April, 1)
	txns := []domain.Transaction{
		charge("Streamly", day(2026, time.April, 10), 1_499),
		charge("Streamly", day(2026, time.May, 10), 1_499),
		charge("Streamly", day(2026, time.June, 10), 1_499),
	}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.June, 20))
	if got[0].Verdict != VerdictStillCharging {
		t.Errorf("verdict = %q, want %q", got[0].Verdict, VerdictStillCharging)
	}
	if !got[0].Acting() {
		t.Error("three charges after cancelling needs the household to do something")
	}
	if got[0].TotalSinceMinor != 4_497 {
		t.Errorf("total = %d, want 4497 — the number that makes it worth acting on", got[0].TotalSinceMinor)
	}
	if !got[0].LastChargeOn.Equal(day(2026, time.June, 10)) {
		t.Errorf("last charge = %v, want the most recent one", got[0].LastChargeOn)
	}
}

func TestChargesBeforeTheCancellationAreNotCounted(t *testing.T) {
	cancelled := day(2026, time.June, 1)
	txns := []domain.Transaction{
		charge("Streamly", day(2026, time.March, 10), 1_499),
		charge("Streamly", day(2026, time.April, 10), 1_499),
	}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.August, 20))
	if got[0].ChargesSince != 0 {
		t.Errorf("counted %d charges from BEFORE the cancellation", got[0].ChargesSince)
	}
	if got[0].Verdict != VerdictStopped {
		t.Errorf("verdict = %q, want %q", got[0].Verdict, VerdictStopped)
	}
}

func TestFutureDatedChargesAreNotCounted(t *testing.T) {
	// A charge that has not arrived is not evidence of anything.
	cancelled := day(2026, time.August, 1)
	txns := []domain.Transaction{charge("Streamly", day(2026, time.December, 1), 1_499)}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.August, 10))
	if got[0].ChargesSince != 0 {
		t.Errorf("counted a future-dated charge: %+v", got[0])
	}
}

func TestADifferentMerchantIsNotMatched(t *testing.T) {
	// The worst possible output here is telling somebody a cancellation failed
	// when it did not — it sends them to argue with a company that complied.
	cancelled := day(2026, time.April, 1)
	txns := []domain.Transaction{charge("Groceries Ltd", day(2026, time.June, 10), 5_000)}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.August, 1))
	if got[0].ChargesSince != 0 || got[0].Verdict != VerdictStopped {
		t.Errorf("an unrelated merchant was matched: %+v", got[0])
	}
}

func TestMatchingIsCaseInsensitiveAndPartial(t *testing.T) {
	cancelled := day(2026, time.April, 1)
	txns := []domain.Transaction{charge("STREAMLY PREMIUM *UK", day(2026, time.June, 10), 1_499)}
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		txns, day(2026, time.June, 20))
	if got[0].ChargesSince != 1 {
		t.Errorf("did not match a real-world statement descriptor: %+v", got[0])
	}
}

func TestIncomeAndExcludedRowsAreNotCharges(t *testing.T) {
	cancelled := day(2026, time.April, 1)
	refund := domain.Transaction{ID: "r", Payee: "Streamly", Date: day(2026, time.June, 10),
		Amount: money.New(1_499, "USD")}
	excluded := charge("Streamly", day(2026, time.June, 11), 1_499)
	excluded.ExcludeFromReports = true
	got := Check([]domain.SubscriptionCancellation{cancellation("Streamly", cancelled)},
		[]domain.Transaction{refund, excluded}, day(2026, time.August, 1))
	if got[0].ChargesSince != 0 {
		t.Errorf("a refund or an excluded row counted as a charge: %+v", got[0])
	}
}

func TestRecordsWithoutANameOrDateAreSkippedNotReassured(t *testing.T) {
	// Reporting "stopped" for a record nobody can check is an unearned
	// reassurance.
	got := Check([]domain.SubscriptionCancellation{
		{ID: "a", SubName: "", CancelledOn: day(2026, time.April, 1)},
		{ID: "b", SubName: "Streamly"},
	}, nil, day(2026, time.August, 1))
	if len(got) != 0 {
		t.Errorf("statuses = %d, want none — neither record can be checked", len(got))
	}
}

func TestOrderIsNewestFirstAndStable(t *testing.T) {
	now := day(2026, time.August, 1)
	cs := []domain.SubscriptionCancellation{
		cancellation("Zeta", day(2026, time.March, 1)),
		cancellation("Alpha", day(2026, time.July, 1)),
		cancellation("Beta", day(2026, time.July, 1)),
	}
	for range 3 {
		got := Check(cs, nil, now)
		if got[0].Name != "Alpha" || got[1].Name != "Beta" || got[2].Name != "Zeta" {
			t.Fatalf("order = %s/%s/%s, want Alpha/Beta/Zeta", got[0].Name, got[1].Name, got[2].Name)
		}
	}
}

func TestStillChargingFiltersToWhatNeedsAction(t *testing.T) {
	now := day(2026, time.August, 1)
	cs := []domain.SubscriptionCancellation{
		cancellation("Quiet", day(2026, time.April, 1)),
		cancellation("Noisy", day(2026, time.April, 1)),
	}
	txns := []domain.Transaction{
		charge("Noisy", day(2026, time.June, 10), 900),
		charge("Noisy", day(2026, time.July, 10), 900),
	}
	acting := StillCharging(Check(cs, txns, now))
	if len(acting) != 1 || acting[0].Name != "Noisy" {
		t.Errorf("acting = %+v, want just Noisy", acting)
	}
}
