// SPDX-License-Identifier: MIT

package capgains

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func year2026() (time.Time, time.Time) { return d(2026, time.January, 1), d(2027, time.January, 1) }

func sale(id string, when time.Time, short, long, proceeds, basis int64, method string) domain.RealizedSale {
	return domain.RealizedSale{
		ID: id, Date: when, ShortTermGainMinor: short, LongTermGainMinor: long,
		GainMinor: short + long, ProceedsMinor: proceeds, BasisMinor: basis, Method: method,
	}
}

func TestShortAndLongAreKeptApart(t *testing.T) {
	// They are taxed at different rates. Netting them into one figure loses the
	// fact that decides what is owed.
	sales := []domain.RealizedSale{
		sale("a", d(2026, time.March, 1), 100_000, 0, 500_000, 400_000, "fifo"),
		sale("b", d(2026, time.June, 1), 0, 300_000, 800_000, 500_000, "fifo"),
	}
	from, to := year2026()
	s := Gather(sales, from, to)
	if s.ShortTermMinor != 100_000 || s.LongTermMinor != 300_000 {
		t.Errorf("split = %d/%d, want 100000/300000", s.ShortTermMinor, s.LongTermMinor)
	}
	if s.NetMinor != 400_000 {
		t.Errorf("net = %d, want 400000", s.NetMinor)
	}
	if s.ProceedsMinor != 1_300_000 || s.BasisMinor != 900_000 {
		t.Errorf("proceeds/basis = %d/%d, want 1300000/900000", s.ProceedsMinor, s.BasisMinor)
	}
}

func TestLossesAreRealOutcomesNotErrorsToClamp(t *testing.T) {
	sales := []domain.RealizedSale{sale("a", d(2026, time.March, 1), -250_000, 0, 100_000, 350_000, "fifo")}
	from, to := year2026()
	s := Gather(sales, from, to)
	if s.NetMinor != -250_000 {
		t.Errorf("net = %d, want -250000", s.NetMinor)
	}
}

func TestSalesComeOutNewestFirstAndDeterministically(t *testing.T) {
	sales := []domain.RealizedSale{
		sale("b", d(2026, time.March, 1), 0, 0, 0, 0, ""),
		sale("a", d(2026, time.March, 1), 0, 0, 0, 0, ""),
		sale("c", d(2026, time.June, 1), 0, 0, 0, 0, ""),
	}
	from, to := year2026()
	for i := range 5 {
		s := Gather(sales, from, to)
		got := s.Sales[0].ID + s.Sales[1].ID + s.Sales[2].ID
		if got != "cab" {
			t.Fatalf("run %d ordered %q, want \"cab\" every time", i, got)
		}
	}
}

func TestOutOfWindowSalesAreIgnored(t *testing.T) {
	sales := []domain.RealizedSale{
		sale("old", d(2025, time.December, 31), 999, 0, 0, 0, ""),
		sale("in", d(2026, time.January, 1), 100, 0, 0, 0, ""),
		sale("next", d(2027, time.January, 1), 999, 0, 0, 0, ""),
	}
	from, to := year2026()
	s := Gather(sales, from, to)
	if len(s.Sales) != 1 || s.ShortTermMinor != 100 {
		t.Errorf("summary = %+v, want only the sale inside the year", s)
	}
}

func TestDeductibleLossIsCappedAndTheRestCarriesForward(t *testing.T) {
	sales := []domain.RealizedSale{sale("a", d(2026, time.March, 1), -1_000_000, 0, 0, 0, "fifo")}
	from, to := year2026()
	deduct, carry, ok := Gather(sales, from, to).DeductibleLossMinor()
	if !ok {
		t.Fatal("a net loss must report a deductible figure")
	}
	if deduct != MaxLossOffsetMinor {
		t.Errorf("deductible = %d, want the %d cap", deduct, MaxLossOffsetMinor)
	}
	if carry != 1_000_000-MaxLossOffsetMinor {
		t.Errorf("carried forward = %d, want %d", carry, 1_000_000-MaxLossOffsetMinor)
	}
}

func TestASmallLossIsFullyDeductibleWithNothingCarried(t *testing.T) {
	sales := []domain.RealizedSale{sale("a", d(2026, time.March, 1), -50_000, 0, 0, 0, "fifo")}
	from, to := year2026()
	deduct, carry, ok := Gather(sales, from, to).DeductibleLossMinor()
	if !ok || deduct != 50_000 || carry != 0 {
		t.Errorf("got %d deductible / %d carried (ok=%v), want 50000 / 0", deduct, carry, ok)
	}
}

func TestAProfitableYearReportsNoLossRatherThanTwoZeroes(t *testing.T) {
	// "0 deductible, 0 carried forward" on a profitable year reads as a failed
	// calculation, not as good news.
	sales := []domain.RealizedSale{sale("a", d(2026, time.March, 1), 0, 400_000, 0, 0, "fifo")}
	from, to := year2026()
	if _, _, ok := Gather(sales, from, to).DeductibleLossMinor(); ok {
		t.Error("a gain must not report a deductible loss")
	}
}

func TestMixedBasisMethodsAreVisible(t *testing.T) {
	// Legitimate, but it means the figures are not reproducible from one rule,
	// and a preparer asking "how did you pick the shares" deserves the truth.
	sales := []domain.RealizedSale{
		sale("a", d(2026, time.March, 1), 0, 0, 0, 0, "fifo"),
		sale("b", d(2026, time.April, 1), 0, 0, 0, 0, "hifo"),
	}
	from, to := year2026()
	s := Gather(sales, from, to)
	if !s.MixedMethods() {
		t.Error("two methods in one year must be reported as mixed")
	}
	if len(s.Methods) != 2 || s.Methods[0] != "fifo" || s.Methods[1] != "hifo" {
		t.Errorf("methods = %v, want a sorted fifo,hifo", s.Methods)
	}
}

func TestOneMethodIsNotMixed(t *testing.T) {
	sales := []domain.RealizedSale{
		sale("a", d(2026, time.March, 1), 0, 0, 0, 0, "fifo"),
		sale("b", d(2026, time.April, 1), 0, 0, 0, 0, "fifo"),
	}
	from, to := year2026()
	if Gather(sales, from, to).MixedMethods() {
		t.Error("one method used twice is not mixed")
	}
}

func TestAnEmptyYearIsAnEmptySummary(t *testing.T) {
	from, to := year2026()
	s := Gather(nil, from, to)
	if len(s.Sales) != 0 || s.NetMinor != 0 || s.MixedMethods() {
		t.Errorf("empty summary = %+v, want zeroes", s)
	}
}
