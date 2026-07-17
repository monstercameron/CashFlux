// SPDX-License-Identifier: MIT

package subscriptions

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAssessTiers(t *testing.T) {
	cases := []struct {
		name      string
		sub       Subscription
		confirmed map[string]bool
		want      Confidence
		wantWhy   string // substring that must appear in the reason line
	}{
		{
			name:      "confirmed by the user wins outright",
			sub:       Subscription{Name: "Netflix", Cadence: CadenceMonthly, Count: 2, AmountVarPct: 80, GapVarDays: 30},
			confirmed: map[string]bool{"netflix": true},
			want:      ConfidenceConfirmed,
			wantWhy:   "You confirmed",
		},
		{
			name:    "many identical steady charges are likely",
			sub:     Subscription{Name: "Spotify", Cadence: CadenceMonthly, Count: 6, AmountVarPct: 0, GapVarDays: 2},
			want:    ConfidenceLikely,
			wantWhy: "same amount every time",
		},
		{
			name:    "small amount drift within 10% still likely",
			sub:     Subscription{Name: "iCloud", Cadence: CadenceMonthly, Count: 4, AmountVarPct: 8, GapVarDays: 3},
			want:    ConfidenceLikely,
			wantWhy: "amounts within 8%",
		},
		{
			name:    "too few charges needs review",
			sub:     Subscription{Name: "New App", Cadence: CadenceMonthly, Count: 3, AmountVarPct: 0, GapVarDays: 1},
			want:    ConfidenceReview,
			wantWhy: "only 3 charges",
		},
		{
			name:    "wobbly gaps need review",
			sub:     Subscription{Name: "Gym", Cadence: CadenceMonthly, Count: 6, AmountVarPct: 0, GapVarDays: 9},
			want:    ConfidenceReview,
			wantWhy: "gaps vary by up to 9 days",
		},
		{
			name:    "variable amounts need review",
			sub:     Subscription{Name: "Corner Store", Cadence: CadenceWeekly, Count: 10, AmountVarPct: 45, GapVarDays: 1},
			want:    ConfidenceReview,
			wantWhy: "amounts vary by up to 45%",
		},
		{
			name:    "yearly tolerates wider gaps",
			sub:     Subscription{Name: "Domain", Cadence: CadenceYearly, Count: 4, AmountVarPct: 2, GapVarDays: 15},
			want:    ConfidenceLikely,
			wantWhy: "steady yearly cadence",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Assess(tc.sub, tc.confirmed)
			if got.Level != tc.want {
				t.Fatalf("Assess level = %s, want %s (reasons: %s)", got.Level, tc.want, got.ReasonLine())
			}
			if !strings.Contains(got.ReasonLine(), tc.wantWhy) {
				t.Errorf("reasons %q missing %q", got.ReasonLine(), tc.wantWhy)
			}
		})
	}
}

// TestDetectEvidenceSignals proves Detect fills the #52 evidence fields: equal
// steady charges → zero variance; a price wobble and an off-cycle charge show
// up in AmountVarPct / GapVarDays.
func TestDetectEvidenceSignals(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	mk := func(desc string, day int, month time.Month, cents int64) domain.Transaction {
		return domain.Transaction{
			ID: desc + month.String() + string(rune('0'+day%10)), AccountID: "a", Desc: desc,
			Date:   time.Date(2026, month, day, 0, 0, 0, 0, time.UTC),
			Amount: money.New(-cents, "USD"),
		}
	}
	txns := []domain.Transaction{
		// Steady: identical amount, identical spacing.
		mk("Steady", 1, time.January, 1000), mk("Steady", 1, time.February, 1000),
		mk("Steady", 1, time.March, 1000), mk("Steady", 1, time.April, 1000),
		// Wobbly: amounts spread ±25%, one gap 8 days late.
		mk("Wobbly", 1, time.January, 800), mk("Wobbly", 1, time.February, 1000),
		mk("Wobbly", 9, time.March, 1250), mk("Wobbly", 1, time.April, 1000),
	}
	subs, err := Detect(txns, rates, 3)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Subscription{}
	for _, s := range subs {
		byName[s.Name] = s
	}
	st, ok := byName["Steady"]
	if !ok {
		t.Fatal("Steady not detected")
	}
	// Calendar months are 28-31 days, so even a perfect 1st-of-the-month charge
	// drifts up to 3 days from the median gap — within the monthly tolerance (4).
	if st.AmountVarPct != 0 || st.GapVarDays > 3 {
		t.Errorf("Steady evidence = amountVar %d%%, gapVar %dd; want 0%%, <=3d", st.AmountVarPct, st.GapVarDays)
	}
	wb, ok := byName["Wobbly"]
	if !ok {
		t.Fatal("Wobbly not detected")
	}
	if wb.AmountVarPct < 15 {
		t.Errorf("Wobbly AmountVarPct = %d, want >= 15", wb.AmountVarPct)
	}
	if wb.GapVarDays < 5 {
		t.Errorf("Wobbly GapVarDays = %d, want >= 5", wb.GapVarDays)
	}
	if Assess(st, nil).Level != ConfidenceLikely {
		t.Errorf("Steady should assess Likely, got %s", Assess(st, nil).Level)
	}
	if Assess(wb, nil).Level != ConfidenceReview {
		t.Errorf("Wobbly should assess Review, got %s", Assess(wb, nil).Level)
	}
}
