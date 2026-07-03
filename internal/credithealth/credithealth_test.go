// SPDX-License-Identifier: MIT

package credithealth_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// baseNow is the fixed reference time used across tests.
var baseNow = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

// makeCard is a test helper to build a minimal credit-card account.
func makeCard(id, name string, limitCents, balanceAsOfMonthsAgo int, dueDayOfMonth int, balanceAsOfYears int) domain.Account {
	var balanceAsOf time.Time
	if balanceAsOfMonthsAgo >= 0 {
		balanceAsOf = baseNow.AddDate(0, -balanceAsOfMonthsAgo, 0)
	}
	var limit money.Money
	if limitCents > 0 {
		limit = money.New(int64(limitCents), "USD")
	}
	_ = balanceAsOfYears
	return domain.Account{
		ID:            id,
		Name:          name,
		Type:          domain.TypeCreditCard,
		Class:         domain.ClassLiability,
		Currency:      "USD",
		CreditLimit:   limit,
		DueDayOfMonth: dueDayOfMonth,
		BalanceAsOf:   balanceAsOf,
	}
}

// makeTxn is a test helper to build a transaction.
func makeTxn(accountID string, date time.Time, amountCents int64) domain.Transaction {
	return domain.Transaction{
		ID:        accountID + date.String(),
		AccountID: accountID,
		Date:      date,
		Amount:    money.New(amountCents, "USD"),
		Desc:      "payment",
	}
}

// ---- Utilization tests ----

func TestSingleCard25Pct(t *testing.T) {
	// $250 balance on a $1000 limit card → 25 % utilization.
	// Target30 = max(0, 250 − 300) = 0.
	// Target10 = max(0, 250 − 100) = 150.
	card := makeCard("c1", "Visa", 100000, 24, 15, 0) // limit $1000.00
	balances := map[string]int64{"c1": -25000}        // owe $250.00

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: balances,
		Now:      baseNow,
	})

	if len(res.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(res.Cards))
	}
	cu := res.Cards[0]

	if cu.UtilPct != 25 {
		t.Errorf("UtilPct: want 25, got %d", cu.UtilPct)
	}
	if cu.Target30Minor != 0 {
		t.Errorf("Target30Minor: want 0 (already under 30%%), got %d", cu.Target30Minor)
	}
	if cu.Target10Minor <= 0 {
		t.Errorf("Target10Minor: want >0, got %d", cu.Target10Minor)
	}
	// 250 − 100 = 150 (minor units, i.e. $1.50 when limit=$10)
	// limit=100000 cents ($1000), 10%=10000 cents ($100), owed=25000 cents ($250)
	// Target10 = 25000 − 10000 = 15000
	if cu.Target10Minor != 15000 {
		t.Errorf("Target10Minor: want 15000, got %d", cu.Target10Minor)
	}
}

func TestCardNoLimit(t *testing.T) {
	// Card with no CreditLimit → UtilPct == -1, counted in CardsMissingLimit.
	card := makeCard("c2", "Store Card", 0, -1, 0, 0) // no limit
	card.BalanceAsOf = time.Time{}                    // zero
	balances := map[string]int64{"c2": -5000}

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: balances,
		Now:      baseNow,
	})

	if len(res.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(res.Cards))
	}
	cu := res.Cards[0]
	if cu.UtilPct != -1 {
		t.Errorf("UtilPct: want -1 (no limit), got %d", cu.UtilPct)
	}
	if res.Agg.CardsMissingLimit != 1 {
		t.Errorf("CardsMissingLimit: want 1, got %d", res.Agg.CardsMissingLimit)
	}
}

// ---- On-time payment proxy tests ----

// helper: build 3 months of payments on a card (3/3 on time).
func paymentsOnTime(accountID string, dueDayOfMonth int, now time.Time) []domain.Transaction {
	var txns []domain.Transaction
	for i := 1; i <= 3; i++ {
		// Payment on the exact due date, i months back.
		date := time.Date(now.Year(), now.Month()-time.Month(i), dueDayOfMonth, 12, 0, 0, 0, now.Location())
		txns = append(txns, makeTxn(accountID, date, -15000))
	}
	return txns
}

func TestOnTime3of3(t *testing.T) {
	card := makeCard("c3", "Amex", 200000, 48, 15, 0)
	txns := paymentsOnTime("c3", 15, baseNow)

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts:     []domain.Account{card},
		Balances:     map[string]int64{"c3": -40000},
		Transactions: txns,
		Now:          baseNow,
	})

	if res.OnTimeScore != 100 {
		t.Errorf("OnTimeScore 3/3: want 100, got %d", res.OnTimeScore)
	}
}

func TestOnTime1of3(t *testing.T) {
	card := makeCard("c4", "MC", 300000, 60, 20, 0)
	// Only one payment (2 months ago).
	date := time.Date(baseNow.Year(), baseNow.Month()-2, 20, 12, 0, 0, 0, baseNow.Location())
	txns := []domain.Transaction{makeTxn("c4", date, -20000)}

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts:     []domain.Account{card},
		Balances:     map[string]int64{"c4": -60000},
		Transactions: txns,
		Now:          baseNow,
	})

	// 1/3 → score = 1*100/3 = 33.
	want := 33
	if res.OnTimeScore != want {
		t.Errorf("OnTimeScore 1/3: want %d, got %d", want, res.OnTimeScore)
	}
}

func TestOnTimeDueDayZeroReturnsNegOne(t *testing.T) {
	// Card with DueDayOfMonth == 0 → no due-date info → OnTimeScore == -1.
	card := makeCard("c5", "Discover", 100000, 12, 0, 0) // DueDayOfMonth = 0

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: map[string]int64{"c5": -10000},
		Now:      baseNow,
	})

	if res.OnTimeScore != -1 {
		t.Errorf("OnTimeScore with DueDayOfMonth=0: want -1, got %d", res.OnTimeScore)
	}
}

// ---- Age proxy tests ----

func TestAgeProxy(t *testing.T) {
	// Card opened 42 months ago → score = 42*100/84 = 50.
	card := makeCard("c6", "Chase", 150000, 42, 10, 0)

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: map[string]int64{"c6": -30000},
		Now:      baseNow,
	})

	want := 50
	if res.AgeScore != want {
		t.Errorf("AgeScore 42 months: want %d, got %d", want, res.AgeScore)
	}
}

func TestAgeProxyZeroBalanceAsOf(t *testing.T) {
	// Card with zero BalanceAsOf → AgeScore == -1.
	card := makeCard("c7", "Citi", 80000, -1, 5, 0)
	card.BalanceAsOf = time.Time{} // zero it out

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: map[string]int64{"c7": -5000},
		Now:      baseNow,
	})

	if res.AgeScore != -1 {
		t.Errorf("AgeScore with zero BalanceAsOf: want -1, got %d", res.AgeScore)
	}
}

// ---- ProxyScore weight re-normalization tests ----

func TestProxyScoreAllThreeFactors(t *testing.T) {
	// util=20% → utilScore≈85, onTime=3/3→100, age=42mo→50.
	// weights: util=0.55, ontime=0.30, age=0.15 → already normalized.
	// proxy = (85*0.55 + 100*0.30 + 50*0.15) / 1.0 = 46.75+30+7.5 = 84.25 → 84.
	card := makeCard("c8", "AllFactors", 100000, 42, 15, 0)
	balances := map[string]int64{"c8": -20000}
	txns := paymentsOnTime("c8", 15, baseNow)

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts:     []domain.Account{card},
		Balances:     balances,
		Transactions: txns,
		Now:          baseNow,
	})

	// All three scores available; result must be in [0,100].
	if res.ProxyScore < 0 || res.ProxyScore > 100 {
		t.Errorf("ProxyScore out of range: %d", res.ProxyScore)
	}
	// With all three factors, ProxyScore must be > 0.
	if res.ProxyScore == 0 {
		t.Errorf("ProxyScore: expected non-zero with good data, got 0 (util=%d ontime=%d age=%d)",
			res.Agg.UtilPct, res.OnTimeScore, res.AgeScore)
	}
}

func TestProxyScoreUtilOnlyRenormalized(t *testing.T) {
	// No DueDayOfMonth (onTime=-1), no BalanceAsOf (age=-1).
	// Only util applies → weight renormalized to 1.0; proxy = utilScore only.
	card := makeCard("c9", "UtilOnly", 100000, -1, 0, 0)
	card.BalanceAsOf = time.Time{}
	balances := map[string]int64{"c9": -10000} // 10% util → utilScore=100

	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card},
		Balances: balances,
		Now:      baseNow,
	})

	if res.OnTimeScore != -1 {
		t.Errorf("OnTimeScore: want -1, got %d", res.OnTimeScore)
	}
	if res.AgeScore != -1 {
		t.Errorf("AgeScore: want -1, got %d", res.AgeScore)
	}
	// 10% util → utilScore=100; only util factor → proxy=100.
	if res.ProxyScore != 100 {
		t.Errorf("ProxyScore util-only at 10%%: want 100, got %d", res.ProxyScore)
	}
}

// ---- Disclaimer test ----

func TestDisclaimerNonEmpty(t *testing.T) {
	cases := []struct {
		name string
		in   credithealth.Inputs
	}{
		{
			name: "no accounts",
			in:   credithealth.Inputs{Now: baseNow},
		},
		{
			name: "single card no limit",
			in: credithealth.Inputs{
				Accounts: []domain.Account{makeCard("d1", "X", 0, -1, 0, 0)},
				Now:      baseNow,
			},
		},
		{
			name: "single card with limit",
			in: credithealth.Inputs{
				Accounts: []domain.Account{makeCard("d2", "Y", 100000, 12, 15, 0)},
				Balances: map[string]int64{"d2": -25000},
				Now:      baseNow,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := credithealth.Evaluate(tc.in)
			if res.Disclaimer == "" {
				t.Error("Disclaimer must be non-empty on every Result")
			}
		})
	}
}

// ---- Band tests ----

func TestBandMapping(t *testing.T) {
	// Excellent ≥ 75.
	card := makeCard("b1", "B1", 100000, 84, 15, 0)
	txns := paymentsOnTime("b1", 15, baseNow)
	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts:     []domain.Account{card},
		Balances:     map[string]int64{"b1": -5000}, // 5% util
		Transactions: txns,
		Now:          baseNow,
	})
	if res.Band != credithealth.BandExcellent && res.Band != credithealth.BandGood {
		t.Errorf("Expected Excellent or Good band for low-util + on-time, got %s (score=%d)", res.Band, res.ProxyScore)
	}

	// Poor: high util, no payments, no age.
	card2 := makeCard("b2", "B2", 100000, -1, 0, 0)
	card2.BalanceAsOf = time.Time{}
	res2 := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{card2},
		Balances: map[string]int64{"b2": -95000}, // 95% util
		Now:      baseNow,
	})
	if res2.Band != credithealth.BandPoor {
		t.Errorf("Expected Poor band for 95%% util-only, got %s (score=%d)", res2.Band, res2.ProxyScore)
	}
}

// ---- Demerits & advice tests ----

func findDemerit(ds []credithealth.Demerit, k credithealth.DemeritKind) (credithealth.Demerit, bool) {
	for _, d := range ds {
		if d.Kind == k {
			return d, true
		}
	}
	return credithealth.Demerit{}, false
}

func findAdvice(as []credithealth.Advice, k credithealth.AdviceKind) (credithealth.Advice, bool) {
	for _, a := range as {
		if a.Kind == k {
			return a, true
		}
	}
	return credithealth.Advice{}, false
}

func TestDemeritsAndAdvice(t *testing.T) {
	// A hot card (90% util, over 30 and over 50), a card with no limit, no on-time
	// payments (so on-time misses), aged ~5 years.
	hot := makeCard("hot", "Rewards", 1000000, 60, 15, 0) // $10,000 limit
	noLimit := makeCard("nl", "Store Card", 0, 60, 0, 0)  // no limit
	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts: []domain.Account{hot, noLimit},
		Balances: map[string]int64{"hot": -900000, "nl": -20000}, // owe $9,000 / $200
		Now:      baseNow,
	})

	// Demerits: aggregate utilization, the hot card, missing-limit, and on-time misses.
	if _, ok := findDemerit(res.Demerits, credithealth.DemeritUtilization); !ok {
		t.Error("expected a utilization demerit")
	}
	if d, ok := findDemerit(res.Demerits, credithealth.DemeritCardUtil); !ok || d.AccountID != "hot" {
		t.Errorf("expected a hot-card demerit for 'hot', got %+v (ok=%v)", d, ok)
	}
	if d, ok := findDemerit(res.Demerits, credithealth.DemeritMissingLimit); !ok || d.Pct != 1 {
		t.Errorf("expected a missing-limit demerit for 1 card, got %+v (ok=%v)", d, ok)
	}
	if _, ok := findDemerit(res.Demerits, credithealth.DemeritOnTime); !ok {
		t.Error("expected an on-time demerit (no payments near due date)")
	}
	// Demerits sorted by point cost, most costly first.
	for i := 1; i < len(res.Demerits); i++ {
		if res.Demerits[i-1].PointsLost < res.Demerits[i].PointsLost {
			t.Errorf("demerits not sorted by PointsLost desc: %+v", res.Demerits)
			break
		}
	}

	// Advice: pay the hot card to 30% ($9,000 - 30%*$10,000 = $6,000), build on-time
	// history, and add the missing limit.
	pay, ok := findAdvice(res.Advice, credithealth.AdviceLowerUtilization)
	if !ok {
		t.Fatal("expected a lower-utilization advice")
	}
	if pay.AccountID != "hot" || pay.PayMinor != 600000 || pay.TargetPct != 30 {
		t.Errorf("pay-down advice wrong: %+v (want hot / 600000 / 30)", pay)
	}
	if pay.ImpactPts <= 0 {
		t.Errorf("pay-down advice should have a positive impact, got %d", pay.ImpactPts)
	}
	if _, ok := findAdvice(res.Advice, credithealth.AdviceOnTime); !ok {
		t.Error("expected on-time advice")
	}
	al, ok := findAdvice(res.Advice, credithealth.AdviceAddLimit)
	if !ok || al.AccountID != "nl" {
		t.Errorf("expected add-limit advice for 'nl', got %+v (ok=%v)", al, ok)
	}
	// The highest-impact advice leads; add-limit (impact 0) is not first.
	if res.Advice[0].Kind == credithealth.AdviceAddLimit {
		t.Error("add-limit (0 impact) should not be the top advice")
	}
	for i := 1; i < len(res.Advice); i++ {
		if res.Advice[i-1].ImpactPts < res.Advice[i].ImpactPts {
			t.Errorf("advice not sorted by ImpactPts desc: %+v", res.Advice)
			break
		}
	}
}

func TestHealthyCardNoDemerits(t *testing.T) {
	// A well-managed card: 8% utilization, on-time every month, aged ~7 years.
	card := makeCard("c1", "Visa", 1000000, 84, 15, 0) // $10,000 limit, 7yr old
	txns := []domain.Transaction{
		makeTxn("c1", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), -80000),
		makeTxn("c1", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), -80000),
		makeTxn("c1", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), -80000),
	}
	res := credithealth.Evaluate(credithealth.Inputs{
		Accounts:     []domain.Account{card},
		Balances:     map[string]int64{"c1": -80000}, // 8% util
		Transactions: txns,
		Now:          baseNow,
	})
	// No pay-down advice (already under 30%) and no on-time advice (paid every month).
	if _, ok := findAdvice(res.Advice, credithealth.AdviceLowerUtilization); ok {
		t.Error("healthy card should not get pay-down advice")
	}
	if _, ok := findAdvice(res.Advice, credithealth.AdviceOnTime); ok {
		t.Error("on-time card should not get on-time advice")
	}
}

// TestProxyWeightsFormulaIdentity guards the engine-surface contract: for ANY
// inputs, floor(utilScore×W.Util + onTime×W.OnTime + age×W.Age) clamped to
// 0–100 reproduces ProxyScore exactly — including the missing-factor cases
// (zero weights) and the no-limit case (utilization scored 0, still weighted).
func TestProxyWeightsFormulaIdentity(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC)
	card := func(id string, limit int64, due int, hasAsOf bool) domain.Account {
		a := domain.Account{ID: id, Type: domain.TypeCreditCard, Currency: "USD", DueDayOfMonth: due}
		if limit > 0 {
			a.CreditLimit = money.New(limit, "USD")
		}
		if hasAsOf {
			a.BalanceAsOf = asOf
		}
		return a
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}
	cases := []credithealth.Inputs{
		{Now: now}, // no cards at all
		{Now: now, Accounts: []domain.Account{card("a", 0, 0, false)}}, // card, no limit/due/age
		{Now: now, Accounts: []domain.Account{card("a", 100000, 15, true)}, Balances: map[string]int64{"a": -25000}},
		{Now: now, Accounts: []domain.Account{card("a", 100000, 0, true), card("b", 50000, 15, false)},
			Balances: map[string]int64{"a": -90000, "b": -10000}},
	}
	for i, in := range cases {
		r := credithealth.Evaluate(in)
		weighted := float64(credithealth.UtilScore(r.Agg.UtilPct)) * r.Weights.Util
		if r.OnTimeScore >= 0 {
			weighted += float64(r.OnTimeScore) * r.Weights.OnTime
		}
		if r.AgeScore >= 0 {
			weighted += float64(r.AgeScore) * r.Weights.Age
		}
		if got := clamp(int(weighted)); got != r.ProxyScore {
			t.Errorf("case %d: formula identity gives %d, ProxyScore is %d", i, got, r.ProxyScore)
		}
	}
}
