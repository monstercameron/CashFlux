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

// The budgets /networth actually passes. Tested with the real values, because
// an axis that is only readable at a budget the app never uses is not tested.
const (
	nwsAxisWideMax   = 8
	nwsAxisNarrowMax = 5
)

func TestShares(t *testing.T) {
	sum := func(xs []int64) int64 {
		var t int64
		for _, x := range xs {
			t += x
		}
		return t
	}

	t.Run("an exhaustive split always sums to exactly 100", func(t *testing.T) {
		cases := [][]int64{
			// The reported defect: assets came to 99%, liabilities to 98%.
			{4929165, 3149000, 30400000},
			{1105118, 6684000, 15372000},
			{1, 1, 1},
			{1, 1, 1, 1, 1, 1, 1},
			{33, 33, 34},
			{999999, 1},
			{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		}
		for _, parts := range cases {
			got := Shares(parts, sum(parts))
			if s := sum(got); s != 100 {
				t.Fatalf("Shares(%v) = %v, sums to %d, want exactly 100", parts, got, s)
			}
			for i, g := range got {
				if g < 0 {
					t.Fatalf("Shares(%v) = %v: a negative share", parts, got)
				}
				if parts[i] == 0 && g != 0 {
					t.Fatalf("Shares(%v) = %v: a zero part took a share", parts, got)
				}
			}
		}
	})

	t.Run("each share stays within a point of its true fraction", func(t *testing.T) {
		parts := []int64{4929165, 3149000, 30400000}
		total := sum(parts)
		for i, g := range Shares(parts, total) {
			exact := float64(parts[i]) * 100 / float64(total)
			if d := float64(g) - exact; d > 1 || d < -1 {
				t.Fatalf("share %d = %d, true %.2f — rounding must never move a figure by more than a point", i, g, exact)
			}
		}
	})

	t.Run("zero parts and empty sets do not invent a total", func(t *testing.T) {
		if got := Shares(nil, 100); len(got) != 0 {
			t.Fatalf("Shares(nil) = %v, want empty", got)
		}
		if got := Shares([]int64{5, 5}, 0); sum(got) != 0 {
			t.Fatalf("Shares with no total = %v, want zeros", got)
		}
		if got := Shares([]int64{0, 0, 0}, 0); sum(got) != 0 {
			t.Fatalf("Shares of nothing = %v, want zeros", got)
		}
	})

	t.Run("a partial set is not normalised into a false whole", func(t *testing.T) {
		// Parts that are only half the total must read as ~50%, never be
		// stretched to 100 — that would be the opposite dishonesty.
		got := Shares([]int64{25, 25}, 100)
		if s := sum(got); s != 50 {
			t.Fatalf("Shares(half a set) = %v, sums to %d, want 50", got, s)
		}
	})

	t.Run("the result is stable across identical calls", func(t *testing.T) {
		parts := []int64{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 2}
		first := Shares(parts, sum(parts))
		for i := 0; i < 5; i++ {
			next := Shares(parts, sum(parts))
			for j := range first {
				if first[j] != next[j] {
					t.Fatalf("Shares is not deterministic: %v vs %v", first, next)
				}
			}
		}
	})
}

func TestBuildPace(t *testing.T) {
	// climb builds a monthly series rising by perMonth from start.
	climb := func(start, perMonth int64, n int) []Point {
		out := make([]Point, 0, n)
		at := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < n; i++ {
			out = append(out, Point{At: at.AddDate(0, i, 0), NetMinor: start + int64(i)*perMonth})
		}
		return out
	}

	t.Run("rungs carry the months each leg took", func(t *testing.T) {
		p := BuildPace(climb(-1600000, 280000, 61))
		if len(p.Rungs) < 3 {
			t.Fatalf("Pace = %+v, want several rungs across a five-year climb", p.Rungs)
		}
		if p.Rungs[0].Months != 0 {
			t.Fatalf("the first rung has no previous leg, got %d months", p.Rungs[0].Months)
		}
		for _, r := range p.Rungs[1:] {
			if r.Months <= 0 {
				t.Fatalf("rung %+v: a leg must carry the time it took", r)
			}
		}
		// Rungs ascend in both value and time — that is what makes the gaps
		// between them readable as pace.
		for i := 1; i < len(p.Rungs); i++ {
			if p.Rungs[i].ValueMinor <= p.Rungs[i-1].ValueMinor || p.Rungs[i].AtIndex <= p.Rungs[i-1].AtIndex {
				t.Fatalf("rungs out of order: %+v", p.Rungs)
			}
		}
	})

	t.Run("a rung is never a setback", func(t *testing.T) {
		pts := append(climb(0, 500000, 40), Point{
			At: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC), NetMinor: 100000,
		})
		for _, r := range BuildPace(pts).Rungs {
			if r.ValueMinor < 0 {
				t.Fatalf("rung %+v: a rung is a level reached, never a fall", r)
			}
		}
	})

	t.Run("the projection is a rung ahead, at the recent rate", func(t *testing.T) {
		p := BuildPace(climb(0, 500000, 61))
		if !p.Next.OK {
			t.Fatalf("Next = %+v, want a projection from a steady climb", p.Next)
		}
		last := p.Rungs[len(p.Rungs)-1]
		if p.Next.ValueMinor <= last.ValueMinor {
			t.Fatalf("Next %+v must be ahead of the last rung %+v", p.Next, last)
		}
		if p.Next.Months <= 0 || p.Next.Months > 60 {
			t.Fatalf("Next = %+v, want a months figure inside the honest horizon", p.Next)
		}
	})

	t.Run("the next target is the figure a person would name", func(t *testing.T) {
		// The forward target follows its OWN rule, not the historical ladder:
		// the next multiple of half the current decade. Reusing the record's
		// sparse 1/2.5/5 ladder answered "$250k" at $151k, skipping the $200k
		// anyone in that position would say out loud.
		for _, tc := range []struct{ now, want int64 }{
			{15117047, 20000000},   // $151,170 -> $200k, not $250k
			{4900000, 5000000},     // $49,000  -> $50k
			{9900000, 10000000},    // $99,000  -> $100k
			{1200000, 1500000},     // $12,000  -> $15k
			{25000000, 30000000},   // $250,000 -> $300k
			{151000000, 200000000}, // $1.51M -> $2M
		} {
			if got := nextRoundAbove(tc.now); got != tc.want {
				t.Fatalf("nextRoundAbove(%d) = %d, want %d", tc.now, got, tc.want)
			}
		}
		if got := nextRoundAbove(0); got != 0 {
			t.Fatalf("nextRoundAbove(0) = %d, want none", got)
		}
	})

	t.Run("the next target is always ahead and never absurdly far", func(t *testing.T) {
		// Every reachable position must get a target that is genuinely above it
		// and within reach of the scale it sits at — no rung 65% away.
		for now := int64(50000); now < 500000000; now = now * 3 / 2 {
			got := nextRoundAbove(now)
			if got <= now {
				t.Fatalf("nextRoundAbove(%d) = %d, must be ahead", now, got)
			}
			if float64(got-now) > float64(now)*0.55 {
				t.Fatalf("nextRoundAbove(%d) = %d is %.0f%% away — too far to be the next thing you'd name",
					now, got, 100*float64(got-now)/float64(now))
			}
		}
	})

	t.Run("a stalled household gets no invented date", func(t *testing.T) {
		flat := climb(5000000, 0, 40)
		if n := BuildPace(flat).Next; n.OK || !n.Stalled {
			t.Fatalf("Next = %+v, want stalled and no projection", n)
		}
		falling := climb(9000000, -50000, 40)
		if n := BuildPace(falling).Next; n.OK {
			t.Fatalf("Next = %+v, a falling trend has no arrival month", n)
		}
	})

	t.Run("a crawl declines to project rather than promising a decade", func(t *testing.T) {
		// $1/month against a $100k gap is arithmetic, not a forecast.
		if n := BuildPace(climb(9000000, 100, 40)).Next; n.OK {
			t.Fatalf("Next = %+v, want no claim beyond the honest horizon", n)
		}
	})

	t.Run("setbacks stay in the marks even though they are not rungs", func(t *testing.T) {
		pts := append(climb(0, 400000, 30), climb(11600000, -600000, 8)...)
		p := BuildPace(pts)
		setback := false
		for _, m := range p.Marks {
			if m.Kind == MilestoneKindReversal || (m.Kind == MilestoneKindThreshold && !m.Up) {
				setback = true
			}
		}
		if !setback {
			t.Fatal("Marks must keep the falls — the chart record is not a trophy cabinet")
		}
	})

	t.Run("too little history yields nothing rather than a guess", func(t *testing.T) {
		if p := BuildPace([]Point{{NetMinor: 100}}); len(p.Rungs) != 0 || p.Next.OK {
			t.Fatalf("Pace = %+v, want nothing from a single point", p)
		}
	})
}

func TestTimeAxisTicks(t *testing.T) {
	// months builds n monthly cutoffs starting at the given year/month.
	months := func(y int, m time.Month, n int) []time.Time {
		out := make([]time.Time, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0))
		}
		return out
	}

	t.Run("a five-year run labels years, not months", func(t *testing.T) {
		ats := months(2021, time.July, 61)
		got := TimeAxisTicks(ats, nwsAxisWideMax, nwsAxisNarrowMax)
		if len(got) > 12 {
			t.Fatalf("TimeAxisTicks gave %d labels for 61 points, budget 12", len(got))
		}
		for _, tk := range got {
			if len(tk.Label) < 3 {
				t.Fatalf("tick %+v: a label this short is an initial, not a date", tk)
			}
		}
		// And it must thin to YEAR boundaries, which is what a reader scans a
		// multi-year axis for — not to every third month from an arbitrary
		// starting offset, which names nothing.
		years := 0
		for _, tk := range got {
			if len(tk.Label) == 4 && tk.Label[0] == '2' {
				years++
			}
		}
		if years < 3 {
			t.Fatalf("TimeAxisTicks = %+v, want year boundaries across a five-year run", got)
		}
	})

	t.Run("the narrow plan is a subset of the wide one", func(t *testing.T) {
		ats := months(2021, time.January, 61)
		got := TimeAxisTicks(ats, nwsAxisWideMax, nwsAxisNarrowMax)
		major := 0
		for _, tk := range got {
			if tk.Major {
				major++
			}
		}
		if major == 0 || major > 7 {
			t.Fatalf("TimeAxisTicks marked %d major of %d, want 1..7", major, len(got))
		}
		if major > len(got) {
			t.Fatalf("more major ticks (%d) than ticks (%d)", major, len(got))
		}
	})

	t.Run("both ends are always named", func(t *testing.T) {
		for _, n := range []int{2, 7, 13, 37, 61, 121} {
			ats := months(2019, time.March, n)
			got := TimeAxisTicks(ats, nwsAxisWideMax, nwsAxisNarrowMax)
			if len(got) < 2 || got[0].Index != 0 || got[len(got)-1].Index != n-1 {
				t.Fatalf("n=%d: TimeAxisTicks = %+v, want the first and last points labelled", n, got)
			}
			for _, tk := range got {
				if tk.Index < 0 || tk.Index >= n {
					t.Fatalf("n=%d: tick index %d out of range", n, tk.Index)
				}
			}
		}
	})

	t.Run("no tick crowds an end once the series outgrows the budget", func(t *testing.T) {
		ats := months(2021, time.January, 37)
		for _, tk := range TimeAxisTicks(ats, nwsAxisWideMax, nwsAxisNarrowMax) {
			if tk.Index == 1 || tk.Index == len(ats)-2 {
				t.Fatalf("tick at %d sits on top of an end label", tk.Index)
			}
		}
	})

	t.Run("a twelve-month window still labels every month", func(t *testing.T) {
		ats := months(2026, time.January, 12)
		if got := TimeAxisTicks(ats, nwsAxisWideMax, nwsAxisNarrowMax); len(got) != 12 {
			t.Fatalf("TimeAxisTicks = %d labels, want all 12 — a year fits", len(got))
		}
	})

	t.Run("a one-year window omits the year from each label", func(t *testing.T) {
		for _, tk := range TimeAxisTicks(months(2026, time.January, 12), nwsAxisWideMax, nwsAxisNarrowMax) {
			if len(tk.Label) != 3 {
				t.Fatalf("tick %+v: within one year the label is the month alone", tk)
			}
		}
	})

	t.Run("degenerate inputs do not panic or invent an axis", func(t *testing.T) {
		if got := TimeAxisTicks(nil, nwsAxisWideMax, nwsAxisNarrowMax); got != nil {
			t.Fatalf("TimeAxisTicks(nil) = %+v, want nil", got)
		}
		if got := TimeAxisTicks(months(2026, time.January, 3), 1, 1); got != nil {
			t.Fatalf("a budget below two ticks is not an axis, got %+v", got)
		}
		if got := TimeAxisTicks(months(2026, time.January, 1), nwsAxisWideMax, nwsAxisNarrowMax); len(got) != 1 {
			t.Fatalf("TimeAxisTicks of one point = %+v, want that point", got)
		}
	})
}

func TestMilestones(t *testing.T) {
	pt := func(net int64) Point { return Point{NetMinor: net} }

	t.Run("a round figure crossed upward is reported", func(t *testing.T) {
		// The sample household topping out at $153,170.47: the ladder for that
		// peak is $10k / $25k / $50k / $100k, so climbing from $90,000 passes
		// $100,000. It does NOT report $150,000 — 1.5x is not a rung, and the
		// peak itself is stated exactly by the all-time-high milestone.
		ms := Milestones([]Point{pt(9000000), pt(15317047)})
		found := false
		for _, m := range ms {
			if m.Kind == MilestoneKindThreshold && m.ValueMinor == 10000000 && m.Up {
				found = true
			}
		}
		if !found {
			t.Fatalf("Milestones = %+v, want the $100,000 crossing", ms)
		}
	})

	t.Run("thresholds scale with the series peak, not with a fixed ladder", func(t *testing.T) {
		// The reviewer's case: a run from -$16,000 to $154,000. The old ladder
		// fired on $500 steps near zero and produced five rows for one month.
		ms := Milestones([]Point{pt(-1600000), pt(70000), pt(1200000), pt(15400000)})
		for _, m := range ms {
			if m.Kind != MilestoneKindThreshold {
				continue
			}
			if m.ValueMinor <= 0 {
				t.Fatalf("Milestones = %+v: a negative threshold (%d) is not an achievement", ms, m.ValueMinor)
			}
			if m.ValueMinor < 1000000 {
				t.Fatalf("Milestones = %+v: $%d is noise beside a $154,000 peak", ms, m.ValueMinor/100)
			}
		}
	})

	t.Run("no threshold rung at or below zero at any scale", func(t *testing.T) {
		for _, peak := range []int64{500, 90000, 1234567, 987654321} {
			for _, v := range milestoneLadder(peak) {
				if v <= 0 {
					t.Fatalf("milestoneLadder(%d) = %v, contains a non-positive rung", peak, milestoneLadder(peak))
				}
			}
		}
	})

	t.Run("turning negative is reported, not only the recovery", func(t *testing.T) {
		// The exact failure the reviewer caught: a page that narrates going
		// back up without ever saying it went down.
		ms := Milestones([]Point{pt(500000), pt(-200000), pt(300000)})
		var down, up bool
		for _, m := range ms {
			switch m.Kind {
			case MilestoneKindNegative:
				down = true
			case MilestoneKindPositive:
				up = true
			}
		}
		if !down || !up {
			t.Fatalf("Milestones = %+v, want BOTH the fall below zero and the recovery", ms)
		}
	})

	t.Run("the all-time high is reported once", func(t *testing.T) {
		ms := Milestones([]Point{pt(100000), pt(900000), pt(1500000), pt(1200000)})
		n, at := 0, -1
		for _, m := range ms {
			if m.Kind == MilestoneKindHigh {
				n, at = n+1, m.AtIndex
			}
		}
		if n != 1 || at != 2 {
			t.Fatalf("Milestones = %+v, want exactly one high at index 2, got %d at %d", ms, n, at)
		}
	})

	t.Run("a material fall from a high is one row at the trough", func(t *testing.T) {
		ms := Milestones([]Point{pt(10000000), pt(9800000), pt(8000000), pt(9000000)})
		n := 0
		for _, m := range ms {
			if m.Kind != MilestoneKindReversal {
				continue
			}
			n++
			if m.AtIndex != 2 || m.ValueMinor != 8000000 || m.FromMinor != 10000000 {
				t.Fatalf("reversal = %+v, want the trough (idx 2, $80,000) measured from $100,000", m)
			}
			if m.Up {
				t.Fatal("a reversal is not an achievement")
			}
		}
		if n != 1 {
			t.Fatalf("Milestones = %+v, want exactly one reversal, got %d", ms, n)
		}
	})

	t.Run("a small wobble is not a reversal", func(t *testing.T) {
		ms := Milestones([]Point{pt(10000000), pt(9800000), pt(10100000)})
		for _, m := range ms {
			if m.Kind == MilestoneKindReversal {
				t.Fatalf("Milestones = %+v: a 2%% dip is noise, not a milestone", ms)
			}
		}
	})

	t.Run("an all-time window stays a list a person would read", func(t *testing.T) {
		// Five years of monthly points climbing -$16,000 -> $154,000 with a
		// dip. The old generator produced 32 rows for this shape.
		pts := make([]Point, 0, 61)
		for i := 0; i < 61; i++ {
			v := int64(-1600000 + i*283333)
			if i >= 30 && i < 34 {
				v -= 1500000
			}
			pts = append(pts, pt(v))
		}
		ms := Milestones(pts)
		if len(ms) > 10 {
			t.Fatalf("Milestones produced %d rows for a 5-year series — that is an event log, not milestones", len(ms))
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
