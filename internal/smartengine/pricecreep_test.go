// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func gymRecurring() domain.Recurring {
	return domain.Recurring{
		ID: "rec-gym", Label: "Gym", Amount: usd(-30_00),
		Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
}

func charge(id string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "chk", Date: when, Amount: usd(minor), Desc: "Gym"}
}

func TestDetectCreepFlagsTwoCycles(t *testing.T) {
	in := baseInput() // Now = 2026-06-15
	in.Recurring = []domain.Recurring{gymRecurring()}
	in.Transactions = []domain.Transaction{
		charge("t1", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), -35_00), // cycle [5-01,6-01)
		charge("t2", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), -35_00), // cycle [4-01,5-01)
		charge("t3", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), -30_00), // at expected
	}
	got := DetectCreep(in)
	if len(got) != 1 {
		t.Fatalf("want 1 creep, got %d: %+v", len(got), got)
	}
	if got[0].NewMinor != 35_00 {
		t.Fatalf("newest = %d, want 3500", got[0].NewMinor)
	}
	if got[0].Cycles < priceCreepCycles {
		t.Fatalf("cycles = %d, want >= %d", got[0].Cycles, priceCreepCycles)
	}
}

func TestDetectCreepIgnoresWithinTolerance(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{gymRecurring()}
	in.Transactions = []domain.Transaction{
		charge("t1", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), -30_10), // <1% over
		charge("t2", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), -30_10),
	}
	if got := DetectCreep(in); len(got) != 0 {
		t.Fatalf("within tolerance — want 0, got %d", len(got))
	}
}

func TestDetectCreepNeedsConsecutive(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{gymRecurring()}
	in.Transactions = []domain.Transaction{
		charge("t1", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), -35_00), // over
		charge("t2", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), -30_00), // back to expected
	}
	if got := DetectCreep(in); len(got) != 0 {
		t.Fatalf("only one cycle over — want 0, got %d", len(got))
	}
}

func TestBL16InsightKeyEncodesLevel(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{gymRecurring()}
	in.Transactions = []domain.Transaction{
		charge("t1", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), -35_00),
		charge("t2", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), -35_00),
	}
	ins := bl16PriceCreep(in)
	if len(ins) != 1 {
		t.Fatalf("want 1 insight, got %d", len(ins))
	}
	if !strings.HasSuffix(ins[0].Key, ":3500") {
		t.Fatalf("key should encode price level 3500, got %q", ins[0].Key)
	}
	if !ins[0].HasAmount || ins[0].Amount.Amount != 5_00 {
		t.Fatalf("delta amount = %v, want 500", ins[0].Amount)
	}
}
