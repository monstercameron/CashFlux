// SPDX-License-Identifier: MIT

package taxlot

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// threeLots is a position built over three years at rising prices: 100 @ $10,
// 100 @ $20, 100 @ $30 (in cents).
func threeLots() []Lot {
	return []Lot{
		{ID: "a", Date: d(2023, time.March, 1), Shares: 100, CostBasisMinor: 100_000},
		{ID: "b", Date: d(2024, time.March, 1), Shares: 100, CostBasisMinor: 200_000},
		{ID: "c", Date: d(2026, time.March, 1), Shares: 100, CostBasisMinor: 300_000},
	}
}

func TestIsLongTermIsMoreThanAYearNotAtLeast(t *testing.T) {
	acq := d(2025, time.June, 10)
	cases := []struct {
		name string
		sold time.Time
		want bool
	}{
		{"a day short", d(2026, time.June, 9), false},
		// The anniversary itself is SHORT term. This off-by-one is the whole rule.
		{"the anniversary", d(2026, time.June, 10), false},
		{"the day after", d(2026, time.June, 11), true},
		{"years later", d(2030, time.January, 1), true},
	}
	for _, c := range cases {
		if got := IsLongTerm(acq, c.sold); got != c.want {
			t.Errorf("%s: IsLongTerm = %v, want %v", c.name, got, c.want)
		}
	}
	if IsLongTerm(time.Time{}, d(2030, time.January, 1)) {
		t.Error("an unknown acquisition date must not claim long-term treatment")
	}
}

func TestFIFORelievesTheOldestSharesFirst(t *testing.T) {
	// Sell 150 for $6,000. FIFO takes all of lot a (100 @ $10 = $1,000) and half
	// of lot b (50 @ $20 = $1,000): basis $2,000, gain $4,000.
	sale, rest, ok := Relieve(threeLots(), 150, 600_000, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	if sale.BasisMinor != 200_000 {
		t.Errorf("basis = %d, want 200000", sale.BasisMinor)
	}
	if sale.GainMinor != 400_000 {
		t.Errorf("gain = %d, want 400000", sale.GainMinor)
	}
	if len(sale.Pieces) != 2 {
		t.Fatalf("pieces = %d, want 2", len(sale.Pieces))
	}
	// Both relieved lots are over a year old, so all of it is long-term.
	if sale.ShortTermGainMinor != 0 || sale.LongTermGainMinor != 400_000 {
		t.Errorf("split short=%d long=%d, want 0/400000", sale.ShortTermGainMinor, sale.LongTermGainMinor)
	}
	// Lot a is gone, lot b is halved and keeps half its basis, lot c untouched.
	if len(rest) != 2 {
		t.Fatalf("remaining lots = %d, want 2", len(rest))
	}
	if rest[0].ID != "b" || rest[0].Shares != 50 || rest[0].CostBasisMinor != 100_000 {
		t.Errorf("remaining lot b = %+v, want 50 shares / 100000 basis", rest[0])
	}
}

func TestHighestCostReportsASmallerGainThanFIFO(t *testing.T) {
	// The point of offering the method at all: on the same sale it is worth real
	// money, and the two answers must genuinely differ.
	sold := d(2026, time.June, 1)
	fifo, _, ok1 := Relieve(threeLots(), 100, 400_000, sold, FIFO)
	hifo, _, ok2 := Relieve(threeLots(), 100, 400_000, sold, HighestCost)
	if !ok1 || !ok2 {
		t.Fatal("expected both sales to be covered")
	}
	if fifo.GainMinor <= hifo.GainMinor {
		t.Errorf("FIFO gain %d should exceed highest-cost gain %d", fifo.GainMinor, hifo.GainMinor)
	}
	// FIFO relieved the cheapest, oldest shares: $1,000 basis, and long-term.
	if fifo.BasisMinor != 100_000 || fifo.LongTermGainMinor != 300_000 {
		t.Errorf("fifo = %+v, want basis 100000 and a long-term gain of 300000", fifo)
	}
	// Highest-cost relieved the $30 lot bought this year: SHORT term, which is the
	// trade-off a smaller gain buys and the reason both numbers have to be shown.
	if hifo.BasisMinor != 300_000 || hifo.ShortTermGainMinor != 100_000 {
		t.Errorf("hifo = %+v, want basis 300000 and a short-term gain of 100000", hifo)
	}
}

func TestLIFORelievesTheNewestSharesFirst(t *testing.T) {
	sale, _, ok := Relieve(threeLots(), 100, 400_000, d(2026, time.June, 1), LIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	if sale.Pieces[0].LotID != "c" {
		t.Errorf("first piece from lot %q, want c", sale.Pieces[0].LotID)
	}
}

func TestOneSaleCanSpanBothHoldingPeriods(t *testing.T) {
	// The reason LongTerm lives on the piece and not on the sale.
	sale, _, ok := Relieve(threeLots(), 250, 1_000_000, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	if sale.ShortTermGainMinor == 0 || sale.LongTermGainMinor == 0 {
		t.Errorf("expected gain on both sides, got short=%d long=%d",
			sale.ShortTermGainMinor, sale.LongTermGainMinor)
	}
	if sale.ShortTermGainMinor+sale.LongTermGainMinor != sale.GainMinor {
		t.Errorf("the split must sum to the gain: %d + %d != %d",
			sale.ShortTermGainMinor, sale.LongTermGainMinor, sale.GainMinor)
	}
}

func TestUncoveredSaleRefusesRatherThanAssumingZeroBasis(t *testing.T) {
	// The headline safety property. A zero basis is not "unknown" — it is a claim
	// that every dollar received was profit, which is the largest possible tax
	// bill stated with the confidence of a correct one.
	_, rest, ok := Relieve(threeLots(), 500, 2_000_000, d(2026, time.June, 1), FIFO)
	if ok {
		t.Fatal("selling more shares than the lots account for must refuse")
	}
	if len(rest) != 3 {
		t.Errorf("a refused sale must leave the lots untouched, got %d", len(rest))
	}
}

func TestPiecesSumToExactlyTheProceeds(t *testing.T) {
	// An odd proceeds figure over three uneven lots is where per-piece rounding
	// leaves cents unallocated.
	lots := []Lot{
		{ID: "a", Date: d(2024, time.January, 1), Shares: 33, CostBasisMinor: 10_000},
		{ID: "b", Date: d(2024, time.February, 1), Shares: 33, CostBasisMinor: 10_000},
		{ID: "c", Date: d(2024, time.March, 1), Shares: 34, CostBasisMinor: 10_000},
	}
	const proceeds = 100_003
	sale, _, ok := Relieve(lots, 100, proceeds, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	var sum int64
	for _, p := range sale.Pieces {
		sum += p.ProceedsMinor
	}
	if sum != proceeds {
		t.Errorf("pieces sum to %d, want exactly %d", sum, proceeds)
	}
	if sale.GainMinor != proceeds-sale.BasisMinor {
		t.Errorf("gain %d != proceeds %d - basis %d", sale.GainMinor, proceeds, sale.BasisMinor)
	}
}

func TestPartialLotKeepsItsRemainingBasis(t *testing.T) {
	// Relieving a slice of a lot must leave behind exactly what was not sold —
	// basis that goes missing here is basis the household pays tax on twice.
	lots := []Lot{{ID: "a", Date: d(2024, time.January, 1), Shares: 3, CostBasisMinor: 1_000}}
	sale, rest, ok := Relieve(lots, 1, 500, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	if len(rest) != 1 {
		t.Fatalf("remaining lots = %d, want 1", len(rest))
	}
	if sale.BasisMinor+rest[0].CostBasisMinor != 1_000 {
		t.Errorf("basis leaked: sold %d + left %d != 1000", sale.BasisMinor, rest[0].CostBasisMinor)
	}
	if rest[0].Shares != 2 {
		t.Errorf("remaining shares = %v, want 2", rest[0].Shares)
	}
}

func TestSellingTheWholePositionLeavesNothing(t *testing.T) {
	sale, rest, ok := Relieve(threeLots(), 300, 1_200_000, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("expected the sale to be covered")
	}
	if len(rest) != 0 {
		t.Errorf("remaining lots = %d, want none", len(rest))
	}
	if sale.BasisMinor != 600_000 {
		t.Errorf("basis = %d, want the whole 600000", sale.BasisMinor)
	}
}

func TestFractionalSharesAreCoveredDespiteFloatDrift(t *testing.T) {
	// Dividend reinvestment produces these constantly, and ten additions of 0.1
	// do not sum to exactly 1.0 in binary floating point.
	var lots []Lot
	for i := range 10 {
		lots = append(lots, Lot{ID: string(rune('a' + i)), Date: d(2024, time.January, 1+i),
			Shares: 0.1, CostBasisMinor: 100})
	}
	if _, _, ok := Relieve(lots, 1.0, 2_000, d(2026, time.June, 1), FIFO); !ok {
		t.Error("ten lots of 0.1 shares must cover a 1.0-share sale")
	}
}

func TestRefusalsOnUnusableInput(t *testing.T) {
	sold := d(2026, time.June, 1)
	cases := []struct {
		name     string
		shares   float64
		proceeds int64
		when     time.Time
	}{
		{"zero shares", 0, 1_000, sold},
		{"negative shares", -5, 1_000, sold},
		{"negative proceeds", 10, -1, sold},
		{"no sale date", 10, 1_000, time.Time{}},
	}
	for _, c := range cases {
		if _, _, ok := Relieve(threeLots(), c.shares, c.proceeds, c.when, FIFO); ok {
			t.Errorf("%s: expected a refusal", c.name)
		}
	}
}

func TestZeroProceedsIsAValidTotalLoss(t *testing.T) {
	// A worthless position is a real disposal with a real deductible loss, and
	// refusing it would send the household looking for a workaround.
	sale, _, ok := Relieve(threeLots(), 100, 0, d(2026, time.June, 1), FIFO)
	if !ok {
		t.Fatal("a sale for nothing is still a sale")
	}
	if sale.GainMinor != -100_000 {
		t.Errorf("gain = %d, want the full basis as a loss (-100000)", sale.GainMinor)
	}
}

func TestUnknownMethodFallsBackToFIFO(t *testing.T) {
	sold := d(2026, time.June, 1)
	got, _, ok1 := Relieve(threeLots(), 100, 400_000, sold, Method("whatever"))
	want, _, ok2 := Relieve(threeLots(), 100, 400_000, sold, FIFO)
	if !ok1 || !ok2 {
		t.Fatal("expected both sales to be covered")
	}
	if got.BasisMinor != want.BasisMinor {
		t.Errorf("unknown method gave basis %d, want FIFO's %d", got.BasisMinor, want.BasisMinor)
	}
}

func TestCoversTellsTheSurfaceBeforeTheSale(t *testing.T) {
	lots := threeLots()
	if !Covers(lots, 300) {
		t.Error("300 lot shares must cover a 300-share position")
	}
	if Covers(lots, 500) {
		t.Error("300 lot shares must not claim to cover 500")
	}
	if TotalBasisMinor(lots) != 600_000 {
		t.Errorf("total basis = %d, want 600000", TotalBasisMinor(lots))
	}
}

func TestSameSaleAlwaysRelievesTheSameShares(t *testing.T) {
	// Lots sharing a date must still order deterministically, or the same
	// disposal reports different tax in two sessions.
	lots := []Lot{
		{ID: "z", Date: d(2024, time.January, 1), Shares: 10, CostBasisMinor: 5_000},
		{ID: "a", Date: d(2024, time.January, 1), Shares: 10, CostBasisMinor: 1_000},
	}
	for i := range 5 {
		sale, _, ok := Relieve(lots, 10, 20_000, d(2026, time.June, 1), FIFO)
		if !ok {
			t.Fatal("expected the sale to be covered")
		}
		if sale.Pieces[0].LotID != "a" {
			t.Fatalf("run %d relieved lot %q, want a every time", i, sale.Pieces[0].LotID)
		}
	}
}
