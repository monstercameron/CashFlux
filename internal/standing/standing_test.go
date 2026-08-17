// SPDX-License-Identifier: MIT

package standing

import (
	"testing"
	"time"
)

var at = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

func keep(minor int64) Instruction {
	return Instruction{ID: "floor", Kind: KeepLiquid, AmountMinor: minor, At: at}
}

// The rule that shapes the package: an instruction limits what the app
// RECOMMENDS, never what somebody may do with their own money.
func TestItConstrainsAdviceAndNotTheHousehold(t *testing.T) {
	b := Book{}.Set(keep(1_500_000))
	// Below the floor there is nothing to propose...
	if got := b.SpendableMinor(1_200_000); got != 0 {
		t.Errorf("spendable = %d, want 0 below the floor", got)
	}
	// ...but the shortfall is reported rather than swallowed, because being told
	// is useful and being prevented would be wrong.
	short, breached := b.Breached(1_200_000)
	if !breached || short != 300_000 {
		t.Errorf("Breached = %d, %v — want a stated $3,000 shortfall", short, breached)
	}
	// And there is no Allowed/Blocked in the API at all: the only question this
	// package answers about an account is whether the app may SUGGEST drawing on
	// it.
	if !b.MayProposeDrawingFrom("any-account") {
		t.Error("an untouched account was reported as off-limits")
	}
}

func TestSpendableIsWhatSitsAboveTheFloor(t *testing.T) {
	b := Book{}.Set(keep(1_500_000))
	if got := b.SpendableMinor(2_000_000); got != 500_000 {
		t.Errorf("spendable = %d, want 500000", got)
	}
	if got := b.SpendableMinor(1_500_000); got != 0 {
		t.Errorf("spendable = %d, want 0 exactly at the floor", got)
	}
}

// Nothing said and "keep zero" are different statements, and an engine that
// confuses them either invents a constraint or ignores a real one.
func TestNoFloorIsNotAFloorOfZero(t *testing.T) {
	var none Book
	if _, ok := none.KeepLiquidMinor(); ok {
		t.Error("an empty book reported a floor")
	}
	if got := none.SpendableMinor(900_000); got != 900_000 {
		t.Errorf("spendable = %d, want everything when no floor was set", got)
	}
	if _, breached := none.Breached(-50_000); breached {
		t.Error("an overdrawn household breached a floor nobody set")
	}

	zero := Book{}.Set(keep(0))
	if v, ok := zero.KeepLiquidMinor(); !ok || v != 0 {
		t.Errorf("a deliberate zero floor read as %d, %v", v, ok)
	}
}

// Two floors is not a state a household can hold, and choosing between them
// silently would be picking a number nobody said.
func TestASecondFloorReplacesTheFirst(t *testing.T) {
	b := Book{}.Set(keep(1_000_000)).
		Set(Instruction{ID: "floor2", Kind: KeepLiquid, AmountMinor: 2_000_000, At: at})
	if b.Len() != 1 {
		t.Fatalf("instructions = %d, want the earlier floor replaced: %+v", b.Len(), b.Instructions)
	}
	if v, _ := b.KeepLiquidMinor(); v != 2_000_000 {
		t.Errorf("floor = %d, want the later one", v)
	}
}

func TestNeverDrawFromIsPerAccount(t *testing.T) {
	b := Book{}.Set(Instruction{ID: "r1", Kind: NeverDrawFrom, Subject: "acct-roth", At: at})
	if b.MayProposeDrawingFrom("acct-roth") {
		t.Error("the app would still suggest drawing on a protected account")
	}
	if !b.MayProposeDrawingFrom("acct-checking") {
		t.Error("protecting one account protected another")
	}
	// Saying it twice about the same account is one instruction, not two.
	b = b.Set(Instruction{ID: "r2", Kind: NeverDrawFrom, Subject: "acct-roth", At: at})
	if b.Len() != 1 {
		t.Errorf("instructions = %d, want one: %+v", b.Len(), b.Instructions)
	}
}

// A rule somebody set once and cannot lift becomes a reason to distrust the
// whole feature.
func TestEveryInstructionCanBeLifted(t *testing.T) {
	b := Book{}.Set(keep(1_500_000)).
		Set(Instruction{ID: "r1", Kind: NeverDrawFrom, Subject: "acct-roth", At: at})
	b = b.Forget("floor")
	if _, ok := b.KeepLiquidMinor(); ok {
		t.Error("the floor survived being forgotten")
	}
	b = b.Forget("r1")
	if !b.MayProposeDrawingFrom("acct-roth") {
		t.Error("the account protection survived being forgotten")
	}
	if b.Len() != 0 {
		t.Errorf("instructions = %d, want none", b.Len())
	}
}

func TestInstructionsThatConstrainNothingAreDropped(t *testing.T) {
	b := Load(`{"instructions":[
		{"id":"","kind":"keep_liquid","amountMinor":100},
		{"id":"a","kind":"invented"},
		{"id":"b","kind":"never_draw_from"},
		{"id":"c","kind":"keep_liquid","amountMinor":-5},
		{"id":"d","kind":"keep_liquid","amountMinor":1000}
	]}`)
	if b.Len() != 1 || b.Instructions[0].ID != "d" {
		t.Errorf("kept instructions that constrain nothing: %+v", b.Instructions)
	}
	// A "never draw from" naming no account constrains nothing at all.
	if !b.MayProposeDrawingFrom("") {
		t.Error("an instruction with no subject blocked an empty account id")
	}
}

func TestRoundTrip(t *testing.T) {
	b := Book{}.Set(keep(1_500_000)).
		Set(Instruction{ID: "r1", Kind: NeverDrawFrom, Subject: "acct-roth", Note: "retirement", At: at})
	got := Load(b.Marshal())
	if got.Len() != 2 {
		t.Fatalf("instructions = %d after round-trip", got.Len())
	}
	if v, ok := got.KeepLiquidMinor(); !ok || v != 1_500_000 {
		t.Errorf("floor = %d, %v after round-trip", v, ok)
	}
	if got.MayProposeDrawingFrom("acct-roth") {
		t.Error("the protection did not survive the round-trip")
	}
	for _, i := range got.Instructions {
		if i.Kind == NeverDrawFrom && i.Note != "retirement" {
			t.Errorf("the reason was lost: %+v", i)
		}
	}
}

// An unreadable book means advice the household has to correct again -
// noticeable and recoverable. Inventing a constraint silently withholds good
// advice with no symptom at all.
func TestAnUnreadableBookConstrainsNothing(t *testing.T) {
	for _, raw := range []string{"", "   ", "{{{", "null"} {
		b := Load(raw)
		if b.Len() != 0 {
			t.Errorf("Load(%q) invented instructions: %+v", raw, b.Instructions)
		}
		if got := b.SpendableMinor(500_000); got != 500_000 {
			t.Errorf("Load(%q) invented a floor (spendable %d)", raw, got)
		}
	}
}
