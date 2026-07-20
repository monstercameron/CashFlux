// SPDX-License-Identifier: MIT

package balancesheet

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func acct(id string, class domain.AccountClass, t domain.AccountType, openingMinor int64) domain.Account {
	return domain.Account{ID: id, Name: id, Class: class, Type: t, Currency: "USD", OpeningBalance: usd(openingMinor)}
}

func tx(id, account string, day int, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: account, Amount: usd(minor),
		Date: time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC),
	}
}

func day(d int) time.Time { return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC) }

func TestBucketOf(t *testing.T) {
	tests := []struct {
		name string
		a    domain.Account
		want Bucket
	}{
		{"checking is cash", acct("a", domain.ClassAsset, domain.TypeChecking, 0), BucketCash},
		{"savings is cash", acct("a", domain.ClassAsset, domain.TypeSavings, 0), BucketCash},
		{"brokerage is invested", acct("a", domain.ClassAsset, domain.TypeInvestment, 0), BucketInvested},
		{"crypto is invested", acct("a", domain.ClassAsset, domain.TypeCrypto, 0), BucketInvested},
		{"condo is property", acct("a", domain.ClassAsset, domain.TypeProperty, 0), BucketProperty},
		{"car is property", acct("a", domain.ClassAsset, domain.TypeVehicle, 0), BucketProperty},
		{"anything else is other", acct("a", domain.ClassAsset, domain.TypeOther, 0), BucketOtherAsset},
		{"card is credit", acct("a", domain.ClassLiability, domain.TypeCreditCard, 0), BucketCredit},
		{"line of credit is credit", acct("a", domain.ClassLiability, domain.TypeLineOfCredit, 0), BucketCredit},
		{"mortgage is mortgage", acct("a", domain.ClassLiability, domain.TypeMortgage, 0), BucketMortgage},
		{"loan is loans", acct("a", domain.ClassLiability, domain.TypeLoan, 0), BucketLoans},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BucketOf(tc.a); got != tc.want {
				t.Fatalf("BucketOf = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSeriesComposesBothSides(t *testing.T) {
	accounts := []domain.Account{
		acct("checking", domain.ClassAsset, domain.TypeChecking, 100000),
		acct("broker", domain.ClassAsset, domain.TypeInvestment, 5000000),
		acct("condo", domain.ClassAsset, domain.TypeProperty, 30400000),
		acct("visa", domain.ClassLiability, domain.TypeCreditCard, -200000),
		acct("mortgage", domain.ClassLiability, domain.TypeMortgage, -25000000),
	}
	txns := []domain.Transaction{
		tx("t1", "checking", 5, 50000),
		tx("t2", "broker", 5, 120000),
		tx("t3", "visa", 5, 30000),
		tx("t4", "checking", 20, 900000), // after the second cutoff
	}
	pts, err := Series(accounts, txns, []time.Time{day(1), day(10)}, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("got %d points, want 2", len(pts))
	}

	// Cutoff 1: opening balances only.
	p0 := pts[0]
	if p0.Assets[BucketCash] != 100000 || p0.Assets[BucketInvested] != 5000000 || p0.Assets[BucketProperty] != 30400000 {
		t.Fatalf("point 0 assets: %+v", p0.Assets)
	}
	if p0.Liabilities[BucketCredit] != 200000 || p0.Liabilities[BucketMortgage] != 25000000 {
		t.Fatalf("point 0 liabilities: %+v", p0.Liabilities)
	}
	if p0.NetMinor != p0.AssetsMinor-p0.LiabilitiesMinor {
		t.Fatalf("net %d != assets %d - liabilities %d", p0.NetMinor, p0.AssetsMinor, p0.LiabilitiesMinor)
	}

	// Cutoff 2: the day-5 rows count, the day-20 row does not.
	p1 := pts[1]
	if p1.Assets[BucketCash] != 150000 || p1.Assets[BucketInvested] != 5120000 {
		t.Fatalf("point 1 assets: %+v", p1.Assets)
	}
	if p1.Liabilities[BucketCredit] != 170000 {
		t.Fatalf("point 1 credit = %d, want 170000 (a payment reduces what is owed)", p1.Liabilities[BucketCredit])
	}

	// Liability magnitudes are always positive; the chart mirrors them itself.
	for _, b := range LiabilityBuckets {
		if p1.Liabilities[b] < 0 {
			t.Fatalf("liability bucket %s is negative (%d) — magnitudes only", b, p1.Liabilities[b])
		}
	}
	// Every canonical bucket is present even when empty, so a chart can rely on shape.
	for _, b := range AssetBuckets {
		if _, ok := p1.Assets[b]; !ok {
			t.Fatalf("asset bucket %s missing from the point", b)
		}
	}
}

// TestSeriesAgreesWithLedger is the no-contradiction contract: the composed net
// must equal the canonical net-worth series to the cent at every cutoff.
func TestSeriesAgreesWithLedger(t *testing.T) {
	accounts := []domain.Account{
		acct("checking", domain.ClassAsset, domain.TypeChecking, 123456),
		acct("condo", domain.ClassAsset, domain.TypeProperty, 30400000),
		acct("mortgage", domain.ClassLiability, domain.TypeMortgage, -25000000),
		acct("loan", domain.ClassLiability, domain.TypeLoan, 78900),
	}
	txns := []domain.Transaction{
		tx("t1", "checking", 3, -4321),
		tx("t2", "mortgage", 4, 90000),
		tx("t3", "loan", 6, -10000),
	}
	cutoffs := []time.Time{day(1), day(5), day(10)}
	rates := currency.Rates{Base: "USD"}

	pts, err := Series(accounts, txns, cutoffs, rates)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	if err != nil {
		t.Fatal(err)
	}
	for i := range pts {
		if pts[i].NetMinor != want[i].Amount {
			t.Fatalf("cutoff %d: composed net %d != ledger net %d", i, pts[i].NetMinor, want[i].Amount)
		}
	}
}

func TestSeriesExcludesArchived(t *testing.T) {
	live := acct("live", domain.ClassAsset, domain.TypeChecking, 100000)
	gone := acct("gone", domain.ClassAsset, domain.TypeChecking, 999999)
	gone.Archived = true
	pts, err := Series([]domain.Account{live, gone}, nil, []time.Time{day(10)}, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if pts[0].AssetsMinor != 100000 {
		t.Fatalf("assets = %d, want the archived account excluded", pts[0].AssetsMinor)
	}
}

func TestAssess(t *testing.T) {
	tests := []struct {
		name                                             string
		assets, liabilities, cash, monthlyExpense        int64
		wantLiquidPct, wantDebtPct, wantRunwayTenths     int64
		wantLiquidBand, wantDebtBand, wantRunwayBand     Band
		wantLiquidOK, wantDebtOK, wantRunwayOK, negative bool
	}{
		{
			name:   "property-heavy household: thin liquidity, structural debt",
			assets: 35500000, liabilities: 21300000, cash: 4260000, monthlyExpense: 300000,
			wantLiquidPct: 12, wantDebtPct: 60, wantRunwayTenths: 142,
			wantLiquidBand: BandWatch, wantDebtBand: BandOK, wantRunwayBand: BandStrong,
			wantLiquidOK: true, wantDebtOK: true, wantRunwayOK: true,
		},
		{
			name:   "cash-rich and nearly debt-free",
			assets: 1000000, liabilities: 100000, cash: 800000, monthlyExpense: 100000,
			wantLiquidPct: 80, wantDebtPct: 10, wantRunwayTenths: 80,
			wantLiquidBand: BandStrong, wantDebtBand: BandStrong, wantRunwayBand: BandStrong,
			wantLiquidOK: true, wantDebtOK: true, wantRunwayOK: true,
		},
		{
			name:   "underwater: debt exceeds assets",
			assets: 500000, liabilities: 900000, cash: 20000, monthlyExpense: 200000,
			wantLiquidPct: 4, wantDebtPct: 180, wantRunwayTenths: 1,
			wantLiquidBand: BandWatch, wantDebtBand: BandAlarm, wantRunwayBand: BandAlarm,
			wantLiquidOK: true, wantDebtOK: true, wantRunwayOK: true, negative: true,
		},
		{
			name:   "no assets and no spending history: nothing is claimed",
			assets: 0, liabilities: 0, cash: 0, monthlyExpense: 0,
			wantLiquidOK: false, wantDebtOK: false, wantRunwayOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := Assess(tc.assets, tc.liabilities, tc.cash, tc.monthlyExpense)
			if h.LiquidShare.OK != tc.wantLiquidOK || h.DebtToAsset.OK != tc.wantDebtOK || h.RunwayOK != tc.wantRunwayOK {
				t.Fatalf("OK flags = %v/%v/%v, want %v/%v/%v",
					h.LiquidShare.OK, h.DebtToAsset.OK, h.RunwayOK, tc.wantLiquidOK, tc.wantDebtOK, tc.wantRunwayOK)
			}
			if !tc.wantLiquidOK {
				return
			}
			if h.LiquidShare.Pct != tc.wantLiquidPct || h.LiquidShare.Band != tc.wantLiquidBand {
				t.Errorf("liquid = %d%%/%s, want %d%%/%s", h.LiquidShare.Pct, h.LiquidShare.Band, tc.wantLiquidPct, tc.wantLiquidBand)
			}
			if h.DebtToAsset.Pct != tc.wantDebtPct || h.DebtToAsset.Band != tc.wantDebtBand {
				t.Errorf("debt = %d%%/%s, want %d%%/%s", h.DebtToAsset.Pct, h.DebtToAsset.Band, tc.wantDebtPct, tc.wantDebtBand)
			}
			if h.RunwayTenths != tc.wantRunwayTenths || h.RunwayBand != tc.wantRunwayBand {
				t.Errorf("runway = %d tenths/%s, want %d/%s", h.RunwayTenths, h.RunwayBand, tc.wantRunwayTenths, tc.wantRunwayBand)
			}
			if h.NetNegative != tc.negative {
				t.Errorf("NetNegative = %v, want %v", h.NetNegative, tc.negative)
			}
		})
	}
}

func TestAxisTicks(t *testing.T) {
	tests := []struct {
		name       string
		lo, hi     int64
		want       int
		wantTicks  []int64
		wantNoneOK bool
	}{
		{
			// The real /networth case: a floored axis over a $222k–$394k band.
			// The reader must get round numbers, not the raw floor.
			name: "a large money band rounds to readable steps",
			lo:   22242095, hi: 39396600, want: 4,
			wantTicks: []int64{25000000, 30000000, 35000000},
		},
		{
			name: "a small band steps down a magnitude",
			lo:   0, hi: 1000, want: 3,
			wantTicks: []int64{0, 500, 1000},
		},
		{
			name: "an inverted range yields nothing rather than nonsense",
			lo:   500, hi: 100, want: 4,
			wantNoneOK: true,
		},
		{
			name: "a zero-width range yields nothing",
			lo:   100, hi: 100, want: 4,
			wantNoneOK: true,
		},
		{
			name: "fewer than two ticks is not an axis",
			lo:   0, hi: 100, want: 1,
			wantNoneOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AxisTicks(tc.lo, tc.hi, tc.want)
			if tc.wantNoneOK {
				if len(got) != 0 {
					t.Fatalf("AxisTicks = %v, want none", got)
				}
				return
			}
			if len(got) != len(tc.wantTicks) {
				t.Fatalf("AxisTicks = %v, want %v", got, tc.wantTicks)
			}
			for i := range got {
				if got[i] != tc.wantTicks[i] {
					t.Fatalf("AxisTicks = %v, want %v", got, tc.wantTicks)
				}
			}
			// A tick outside the range would be a gridline the chart cannot draw.
			for _, v := range got {
				if v < tc.lo || v > tc.hi {
					t.Fatalf("tick %d outside [%d, %d]", v, tc.lo, tc.hi)
				}
			}
		})
	}
}

func TestMilestones(t *testing.T) {
	pt := func(net int64) Point { return Point{NetMinor: net} }

	t.Run("a round figure crossed upward is reported", func(t *testing.T) {
		// The sample household: $131,837.65 -> $153,170.47 crosses $150,000.
		ms := Milestones([]Point{pt(13183765), pt(15317047)})
		found := false
		for _, m := range ms {
			if m.Kind == MilestoneKindThreshold && m.ValueMinor == 15000000 && m.Up {
				found = true
			}
		}
		if !found {
			t.Fatalf("Milestones = %+v, want the $150,000 crossing", ms)
		}
	})

	t.Run("becoming positive is its own milestone", func(t *testing.T) {
		ms := Milestones([]Point{pt(-5000), pt(12000)})
		found := false
		for _, m := range ms {
			if m.Kind == MilestoneKindPositive && m.Up {
				found = true
			}
		}
		if !found {
			t.Fatalf("Milestones = %+v, want a first-positive milestone", ms)
		}
	})

	t.Run("a crossing the other way is reported too, not hidden", func(t *testing.T) {
		ms := Milestones([]Point{pt(15317047), pt(13183765)})
		if len(ms) == 0 {
			t.Fatal("a downward crossing must still be reported — this is a record, not a trophy cabinet")
		}
		for _, m := range ms {
			if m.Up {
				t.Fatalf("Milestones = %+v, want every crossing marked downward", ms)
			}
		}
	})

	t.Run("a flat series has no milestones", func(t *testing.T) {
		if ms := Milestones([]Point{pt(15317047), pt(15317047)}); len(ms) != 0 {
			t.Fatalf("Milestones = %+v, want none", ms)
		}
	})

	t.Run("too short to have crossed anything", func(t *testing.T) {
		if ms := Milestones([]Point{pt(100)}); ms != nil {
			t.Fatalf("Milestones = %+v, want nil", ms)
		}
	})

	t.Run("no duplicate thresholds from overlapping ladder steps", func(t *testing.T) {
		ms := Milestones([]Point{pt(0), pt(100000000)})
		seen := map[int64]bool{}
		for _, m := range ms {
			if m.Kind != MilestoneKindThreshold {
				continue
			}
			if seen[m.ValueMinor] {
				t.Fatalf("threshold %d reported twice in %+v", m.ValueMinor, ms)
			}
			seen[m.ValueMinor] = true
		}
	})
}
