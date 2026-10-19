// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSOLB2EnvelopeStartAndPast(t *testing.T) {
	b := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(10000), RolloverStart: mustDate("2026-09-01")}
	tx := []domain.Transaction{expense(3000, "USD", "food", "", "2026-10-10"), expense(20000, "USD", "food", "", "2020-01-10")}
	for _, tt := range []struct {
		date string
		want int64
	}{{"2026-09-15", 10000}, {"2026-10-15", 17000}} {
		got, err := EnvelopeAvailableAnchored(b, tx, mustDate(tt.date), time.Monday, time.Time{}, currency.Rates{Base: "USD"}, nil)
		if err != nil || got.Amount != tt.want {
			t.Errorf("%s: %v, %v; want %d", tt.date, got, err, tt.want)
		}
	}
}
func TestSOLB2SavingEnvelope(t *testing.T) {
	b := domain.Budget{CategoryID: "save", Period: domain.PeriodMonthly, Direction: domain.DirectionSave, Limit: usd(10000), RolloverStart: mustDate("2026-09-01")}
	tx := expense(3000, "USD", "save", "", "2026-09-10")
	tx.TransferAccountID = "pair"
	got, err := EnvelopeAvailableAnchored(b, []domain.Transaction{tx}, mustDate("2026-10-15"), time.Monday, time.Time{}, currency.Rates{Base: "USD"}, nil)
	if err != nil || got.Amount != 17000 {
		t.Fatalf("saving envelope: %v %v; want 17000", got, err)
	}
}
func TestSOLB2SignedSuggestionAndTracking(t *testing.T) {
	tx := expense(10000, "USD", "food", "", "2026-09-10")
	tx.Splits = []domain.CategorySplit{{CategoryID: "food", Amount: usd(-12000)}, {CategoryID: "food", Amount: usd(2000)}}
	got, err := SuggestLimit("food", []domain.Transaction{tx}, mustDate("2026-10-15"), 3, currency.Rates{Base: "USD"})
	if err != nil || got != 10000 {
		t.Errorf("suggestion=%d %v; want 10000", got, err)
	}
	meta := ComputeTrackingMetadata([]domain.Transaction{tx}, mustDate("2026-09-01"), mustDate("2026-10-01"), "USD", currency.Rates{Base: "USD"})
	if meta.CategoryTotal["food"] != 10000 {
		t.Errorf("metadata=%v; want food=10000", meta)
	}
}
func TestSOLB2TrueUpScope(t *testing.T) {
	for _, mode := range []string{"excluded", "other owner", "tag"} {
		t.Run(mode, func(t *testing.T) {
			b := domain.Budget{ID: "b", CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(10000), Scope: domain.ScopeIndividual, OwnerID: "alice"}
			tx := expense(90000, "USD", "food", "alice", "2026-09-10")
			want := 0
			switch mode {
			case "excluded":
				tx.ExcludeFromReports = true
			case "other owner":
				tx.MemberID = "bob"
			case "tag":
				b.CategoryID = "other"
				b.TrackedTags = []string{"trip"}
				tx.Tags = []string{"trip"}
				want = 1
			}
			got, err := SuggestTrueUps([]domain.Budget{b}, []domain.Transaction{tx}, nil, mustDate("2026-10-15"), currency.Rates{Base: "USD"})
			if err != nil || len(got) != want {
				t.Fatalf("trueup=%+v %v, want %d", got, err, want)
			}
		})
	}
}
func TestSOLB2UnbudgetedScopeAndOverflow(t *testing.T) {
	b := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "alice"}
	tx := expense(10000, "USD", "food", "bob", "2026-09-10")
	for _, tag := range []bool{false, true} {
		if tag {
			b.CategoryID = "other"
			b.TrackedTags = []string{"trip"}
			tx.Tags = []string{"trip"}
		}
		got, err := ComputeUnbudgeted([]domain.Budget{b}, nil, []domain.Transaction{tx}, mustDate("2026-09-01"), mustDate("2026-10-01"), currency.Rates{Base: "USD"})
		if err != nil || got.Total.Amount != 10000 {
			t.Errorf("tag=%v unbudgeted=%v %v", tag, got, err)
		}
	}
	tx.Amount = usd(-math.MaxInt64)
	_, err := ComputeUnbudgeted(nil, nil, []domain.Transaction{tx, tx}, mustDate("2026-09-01"), mustDate("2026-10-01"), currency.Rates{Base: "USD"})
	if err == nil {
		t.Error("unbudgeted overflow hidden")
	}
}
func TestSOLB2WallClockPace(t *testing.T) {
	start, end := mustDate("2026-10-01"), mustDate("2026-11-01")
	now := time.Date(2026, 10, 31, 21, 0, 0, 0, time.FixedZone("NY", -4*3600))
	p := ProjectPace(Status{Spent: usd(1000), Limit: usd(10000)}, start, end, now)
	if p.Elapsed >= 1 {
		t.Fatalf("local Oct 31 marked ended: %+v", p)
	}
}
func TestSOLB2AdjustedLimitPrecision(t *testing.T) {
	limit := int64(9007199254740993)
	delta, err := money.MulDiv(limit, 123, 10000, money.RoundHalfAwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	want, err := money.Add(limit, delta)
	if err != nil {
		t.Fatal(err)
	}
	if got := AdjustedLimit(limit, 1.23); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestSOLB2BoundedRolloverAndEnvelope(t *testing.T) {
	b := domain.Budget{CategoryID: "food", Period: domain.PeriodWeekly, Limit: usd(100), Rollover: true, RolloverStart: mustDate("0026-01-01")}
	in := ViewInput{Base: "USD", Rates: currency.Rates{Base: "USD"}, Period: ViewPeriod{Anchor: mustDate("2026-10-19"), Now: mustDate("2026-10-19"), WeekStart: time.Monday}, Transactions: []domain.Transaction{expense(1, "USD", "food", "", "0026-01-01")}}
	if _, err := EvaluateRolloverView(b, in, nil); err == nil {
		t.Error("unbounded rollover accepted")
	}
	if _, err := EnvelopeRolloverView(b, in, nil); err == nil {
		t.Error("unbounded envelope accepted")
	}
	v := EvaluateViewBudget(b, in)
	if v.Valid || v.Error == nil {
		t.Fatal("long history must make status incomplete")
	}
}
func TestSOLB2SweepUsesCardStatus(t *testing.T) {
	b := domain.Budget{ID: "food", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(30000), Rollover: true, RolloverStart: mustDate("2026-08-01"), OwnerID: domain.GroupOwnerID}
	b.PeriodBoosts = map[string]int64{"2026-09-01": 1000}
	annual := b
	annual.ID = "annual"
	annual.Period = domain.PeriodYearly
	saving := b
	saving.ID = "saving"
	saving.Direction = domain.DirectionSave
	ordinary := b
	ordinary.ID = "ordinary"
	ordinary.Rollover = false
	ordinary.PeriodBoosts = nil
	other := b
	other.ID = "other"
	other.OwnerID = "bob"
	other.Scope = domain.ScopeIndividual
	euro := b
	euro.ID = "eur"
	euro.Limit = money.New(10000, "EUR")
	euro.Rollover = false
	euro.PeriodBoosts = nil
	in := ViewInput{Base: "USD", Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, Period: ViewPeriod{Anchor: mustDate("2026-09-01"), Now: mustDate("2026-10-19"), WeekStart: time.Monday}, Categories: []domain.Category{{ID: "food"}, {ID: "child", ParentID: "food"}}, Transactions: []domain.Transaction{expense(28000, "USD", "child", "", "2026-08-10"), expense(28000, "USD", "child", "", "2026-09-10")}}
	// Sept cap = 30000 + 2000 Aug carry + 1000 boost; its remainder is not swept.
	cfg := SweepConfig{Enabled: true, BudgetIDs: []string{"food", "annual", "saving", "ordinary", "other", "eur"}}
	plan, err := EvaluateSweep([]domain.Budget{b, annual, saving, ordinary, other, euro}, in, cfg, "alice", nil)
	// Rollover budgets are excluded so a leftover cannot be both swept and carried.
	if err != nil || plan.Total.Amount != 2000 || plan.BudgetCount() != 1 || len(plan.Lines) != 1 || plan.Lines[0].BudgetID != "ordinary" {
		t.Fatalf("sweep=%+v %v", plan, err)
	}
	fx := Status{Budget: domain.Budget{ID: "eur", Period: domain.PeriodMonthly}, Remaining: money.New(100, "EUR")}
	plan, err = ComputeSweepWithRates([]Status{fx}, SweepConfig{Enabled: true, BudgetIDs: []string{"eur"}}, in.Rates, nil)
	if err != nil || plan.Total.Amount != 200 {
		t.Fatalf("FX sweep=%+v %v", plan, err)
	}
	_, err = ComputeSweepWithRates([]Status{fx}, cfg, currency.Rates{Base: "USD"}, nil)
	if err == nil {
		t.Error("missing FX hidden")
	}
	fx.Remaining = usd(math.MaxInt64)
	fx.Budget.ID = "food"
	_, err = ComputeSweepWithRates([]Status{fx, fx}, cfg, currency.Rates{Base: "USD"}, nil)
	if err == nil {
		t.Error("overflow hidden")
	}
}
func TestSOLB2AdjustOverflowAndBasisPoints(t *testing.T) {
	b := domain.Budget{ID: "b", Limit: usd(math.MaxInt64), Period: domain.PeriodMonthly}
	p := AdjustAllPreviewFor([]domain.Budget{b}, 10, AdjustEveryPeriod, AdjustOverlays{"b": 0})
	if p.Error == nil || len(p.Lines) != 0 {
		t.Fatalf("overflow preview=%+v", p)
	}
	got, err := AdjustedLimitBPS(9007199254740993, 123)
	delta, _ := money.MulDiv(9007199254740993, 123, 10000, money.RoundHalfAwayFromZero)
	if err != nil || got != 9007199254740993+delta {
		t.Fatalf("bps=%d %v", got, err)
	}
}
func TestSOLB2AutoBudgetNormalized(t *testing.T) {
	budgets := []domain.Budget{{ID: "annual", CategoryID: "travel", Period: domain.PeriodYearly, Limit: usd(120000)}, {ID: "food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(10000, "EUR")}}
	got, err := AutoBudgetImpactWithRates(budgets, 100000, map[string]int64{"food": 30000}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil || got.BudgetedBefore != 30000 || got.BudgetedAfter != 40000 {
		t.Fatalf("impact=%+v %v", got, err)
	}
	if _, err := AutoBudgetImpactWithRates(budgets, 100000, nil, currency.Rates{Base: "USD"}); err == nil {
		t.Fatal("impact missing FX hidden")
	}
}
func TestSOLB2CompositionEqualsSpent(t *testing.T) {
	b := domain.Budget{ID: "b", CategoryID: "food", TrackedTags: []string{"trip"}, Scope: domain.ScopeIndividual, OwnerID: "alice", Limit: usd(10000)}
	tx := expense(10000, "USD", "other", "bob", "2026-09-10")
	tx.Splits = []domain.CategorySplit{{CategoryID: "child", Amount: usd(-12000), MemberID: "alice"}, {CategoryID: "food", Amount: usd(2000), MemberID: "alice"}}
	tag := expense(1000, "USD", "hotel", "alice", "2026-09-12")
	tag.Tags = []string{"trip"}
	all := []domain.Transaction{tx, tag}
	covers := map[string]bool{"child": true}
	start, end := mustDate("2026-09-01"), mustDate("2026-10-01")
	spent, err := SpentRollup(b, all, start, end, currency.Rates{Base: "USD"}, covers)
	if err != nil {
		t.Fatal(err)
	}
	slices, err := CompositionSlices(b, all, start, end, currency.Rates{Base: "USD"}, covers)
	if err != nil {
		t.Fatal(err)
	}
	sum := money.Zero("USD")
	for _, slice := range slices {
		sum, err = sum.Add(slice.Amount)
		if err != nil {
			t.Fatal(err)
		}
	}
	if sum != spent {
		t.Fatalf("pie=%v spent=%v", sum, spent)
	}
}

func TestSOLB2SmoothingContributionScope(t *testing.T) {
	for _, mode := range []string{"excluded", "owner", "tag", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			b := cmtBudget(domain.PeriodMonthly, 10000)
			b.Scope = domain.ScopeIndividual
			b.OwnerID = "alice"
			tx := cmtTxn(-60000, mustDate("2026-06-15"))
			tx.MemberID = "alice"
			r := cmtRec("bill", -60000, domain.CadenceYearly, mustDate("2026-06-15"), true)
			r.SmoothIntoBudgets = true
			want := int64(0)
			switch mode {
			case "excluded":
				tx.ExcludeFromReports = true
			case "owner":
				tx.MemberID = "bob"
			case "tag":
				b.CategoryID = "other"
				b.TrackedTags = []string{"trip"}
				tx.Tags = []string{"trip"}
				want = 60000
			case "overflow":
				tx.Amount = usd(math.MinInt64)
			}
			got, _, err := SmoothingLandingItemsWithRates(b, []domain.Recurring{r}, []domain.Transaction{tx}, mustDate("2026-06-01"), mustDate("2026-07-01"), currency.Rates{Base: "USD"})
			if mode == "overflow" {
				if err == nil {
					t.Fatal("overflow hidden")
				}
				return
			}
			if err != nil || got != want {
				t.Fatalf("offset %d err %v want %d", got, err, want)
			}
		})
	}
}

func TestSOLB2ExplicitRefundLedgerIsolation(t *testing.T) {
	buy := expense(12000, "USD", "food", "", "2026-09-10")
	buy.ID = "buy"
	refund := buy
	refund.ID = "refund"
	refund.Amount = usd(4000)
	refund.Date = mustDate("2026-10-10")
	all := []domain.Transaction{buy, refund}
	links := []domain.TxnLink{{ID: "refund-link", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: usd(4000)}}
	b := domain.Budget{ID: "b", CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(20000), Rollover: true, RolloverStart: mustDate("2026-09-01")}
	for _, tt := range []struct {
		links []domain.TxnLink
		want  int64
	}{{links, 8000}, {nil, 12000}} {
		ledger := NewNettedLedgerWithLinks(all, tt.links)
		SetRefundLinks(links)
		defer SetRefundLinks(nil)
		in := ViewInput{Ledger: &ledger, Transactions: all, Base: "USD", Rates: currency.Rates{Base: "USD"}, Period: ViewPeriod{Anchor: mustDate("2026-09-01"), Now: mustDate("2026-10-19")}}
		v := EvaluateViewBudget(b, in)
		if !v.Valid || v.Status.Spent.Amount != tt.want || v.Rollover.Current.Spend.Amount != tt.want {
			t.Fatalf("explicit ledger %d: %+v", tt.want, v)
		}
	}
}

func TestSOLB2TrueUpRoundingOverflow(t *testing.T) {
	b := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(10000)}
	tx := expense(math.MaxInt64, "USD", "food", "", "2026-09-10")
	got, err := SuggestTrueUps([]domain.Budget{b}, []domain.Transaction{tx}, nil, mustDate("2026-10-19"), currency.Rates{Base: "USD"})
	if err != nil || len(got) != 1 || got[0].SuggestedMinor != math.MaxInt64-7 {
		t.Fatalf("large true-up=%+v %v", got, err)
	}
	if _, err := roundToChecked(math.MaxInt64, 10); err == nil {
		t.Fatal("unrepresentable rounded value accepted")
	}
}
func TestSOLB2ApplyAdjustChecksBoost(t *testing.T) {
	b := domain.Budget{ID: "b", Limit: usd(100), Period: domain.PeriodMonthly, PeriodBoosts: map[string]int64{"2026-10-01": math.MaxInt64}}
	p := AdjustPreview{Lines: []AdjustLine{{Budget: b, Before: 100, After: 200}}}
	if _, err := ApplyAdjustChecked(p, AdjustThisPeriod, func(domain.Budget) time.Time { return mustDate("2026-10-01") }); err == nil {
		t.Fatal("boost overflow hidden")
	}
	if b.PeriodBoosts["2026-10-01"] != math.MaxInt64 {
		t.Fatal("mutated input boost")
	}
}

func TestSOLB2HistoricalBaseBeforeFirstChange(t *testing.T) {
	b := domain.Budget{Limit: usd(20000), LimitHistory: map[string]int64{"2026-09-01": 10000, "2026-11-01": 20000}}
	got, err := BudgetBaseAt(b, mustDate("2026-08-01"))
	if err != nil || got.Amount != 10000 {
		t.Fatalf("past base=%v %v; want original 10000", got, err)
	}
}

func TestSOLB2SmoothingOffsetsActualScopedContribution(t *testing.T) {
	b := cmtBudget(domain.PeriodMonthly, 10000)
	b.Scope = domain.ScopeIndividual
	b.OwnerID = "alice"
	r := cmtRec("bill", -60000, domain.CadenceYearly, mustDate("2026-06-15"), true)
	for _, split := range []bool{false, true} {
		tx := cmtTxn(-59999, mustDate("2026-06-15"))
		tx.MemberID = "alice"
		want := int64(59999)
		if split {
			tx.Amount = usd(-60000)
			tx.Splits = []domain.CategorySplit{{CategoryID: "cat1", MemberID: "alice", Amount: usd(-20000)}, {CategoryID: "cat1", MemberID: "bob", Amount: usd(-40000)}}
			want = 20000
		}
		got, _, err := SmoothingLandingItemsWithRates(b, []domain.Recurring{r}, []domain.Transaction{tx}, mustDate("2026-06-01"), mustDate("2026-07-01"), currency.Rates{Base: "USD"})
		if err != nil || got != want {
			t.Errorf("split=%v offset=%d err=%v want=%d", split, got, err, want)
		}
	}
}
