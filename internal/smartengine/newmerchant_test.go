// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// mtxn builds an expense transaction with a payee and date.
func mtxn(id, payee string, when time.Time, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "a", Date: when, Amount: usd(amount), Payee: payee}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestT19FlagsRecentFirstCharge(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Blue Bottle", day(2026, 6, 5), -650), // 10 days before ref
	}
	got := t19NewMerchant(in)
	if len(got) != 1 {
		t.Fatalf("want 1 new-merchant flag, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-T19:blue bottle" {
		t.Errorf("key = %q, want SMART-T19:blue bottle", got[0].Key)
	}
	if got[0].Amount.Amount != 650 {
		t.Errorf("amount = %d, want 650", got[0].Amount.Amount)
	}
}

func TestT19SkipsOldFirstCharge(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Blue Bottle", day(2026, 1, 5), -650), // months before ref
	}
	if got := t19NewMerchant(in); len(got) != 0 {
		t.Errorf("old first charge should not flag, got %d", len(got))
	}
}

func TestT19AliasResolutionCollapsesNoise(t *testing.T) {
	in := baseInput()
	// Two processor-noisy Amazon strings resolve to one merchant "Amazon" via the
	// built-in normalizer, so only the earliest counts as the first-ever charge.
	in.Transactions = []domain.Transaction{
		mtxn("1", "AMZN Mktp US*2K4RT0", day(2026, 6, 3), -1200),
		mtxn("2", "AMZN Mktp US*9ZZ11", day(2026, 6, 10), -800),
	}
	got := t19NewMerchant(in)
	if len(got) != 1 {
		t.Fatalf("want 1 flag for the resolved Amazon merchant, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-T19:amazon" {
		t.Errorf("key = %q, want SMART-T19:amazon", got[0].Key)
	}
}

func TestT20FlagsNewSubscription(t *testing.T) {
	in := baseInput()
	// Two similar charges ~30 days apart, the second recent.
	in.Transactions = []domain.Transaction{
		mtxn("1", "Streamly", day(2026, 5, 16), -1499),
		mtxn("2", "Streamly", day(2026, 6, 15), -1499),
	}
	got := t20NewSubscription(in)
	if len(got) != 1 {
		t.Fatalf("want 1 new-subscription nudge, got %d: %+v", len(got), got)
	}
	act := got[0].Action
	if act == nil || act.RecurringLabel != "Streamly" || act.RecurringAmount != -1499 {
		t.Errorf("action = %+v, want create-recurring Streamly -1499", act)
	}
	if act.RecurringCadence != string(domain.CadenceMonthly) {
		t.Errorf("cadence = %q, want monthly", act.RecurringCadence)
	}
}

func TestT20SkipsWrongGap(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Streamly", day(2026, 6, 5), -1499),
		mtxn("2", "Streamly", day(2026, 6, 12), -1499), // 7 days — too soon
	}
	if got := t20NewSubscription(in); len(got) != 0 {
		t.Errorf("7-day gap should not read as monthly sub, got %d", len(got))
	}
}

func TestT20SkipsDissimilarAmount(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Shop", day(2026, 5, 16), -500),
		mtxn("2", "Shop", day(2026, 6, 15), -5000), // 10x — not a subscription
	}
	if got := t20NewSubscription(in); len(got) != 0 {
		t.Errorf("dissimilar amounts should not flag, got %d", len(got))
	}
}

func TestT20SkipsWhenMoreThanTwoCharges(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Streamly", day(2026, 4, 16), -1499),
		mtxn("2", "Streamly", day(2026, 5, 16), -1499),
		mtxn("3", "Streamly", day(2026, 6, 15), -1499),
	}
	if got := t20NewSubscription(in); len(got) != 0 {
		t.Errorf("established (3+ charge) merchant should not flag as new, got %d", len(got))
	}
}

func TestT20SkipsAlreadyTracked(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		mtxn("1", "Streamly", day(2026, 5, 16), -1499),
		mtxn("2", "Streamly", day(2026, 6, 15), -1499),
	}
	in.Recurring = []domain.Recurring{{ID: "r1", Label: "Streamly", Amount: usd(-1499), Cadence: domain.CadenceMonthly}}
	if got := t20NewSubscription(in); len(got) != 0 {
		t.Errorf("already-tracked merchant should not re-flag, got %d", len(got))
	}
}

func TestT20LearnedAliasUnifies(t *testing.T) {
	in := baseInput()
	in.Aliases = []domain.PayeeAlias{{ID: "al1", RawPayee: "STREAMLY INC", Display: "Streamly"}}
	in.Transactions = []domain.Transaction{
		mtxn("1", "Streamly", day(2026, 5, 16), -1499),
		mtxn("2", "STREAMLY INC", day(2026, 6, 15), -1499),
	}
	got := t20NewSubscription(in)
	if len(got) != 1 {
		t.Fatalf("learned alias should unify the two charges, got %d", len(got))
	}
}
