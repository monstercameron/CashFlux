// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// monthlyCharges builds n monthly occurrences of a charge ending at the month
// before `ref` (so the series is "current" unless told otherwise).
func monthlyCharges(name string, amount int64, lastMonth time.Month, n int) []domain.Transaction {
	var txns []domain.Transaction
	for i := range n {
		m := time.Month(int(lastMonth) - (n - 1 - i))
		d := time.Date(2026, m, 8, 0, 0, 0, 0, time.UTC)
		txns = append(txns, domain.Transaction{
			ID: name + itoa64(int64(i)), AccountID: "x", Date: d, Amount: usd(amount), Desc: name,
		})
	}
	return txns
}

func TestSU4AnnualSavings(t *testing.T) {
	in := baseInput()
	in.Transactions = monthlyCharges("Spotify", -1000, time.June, 4) // $10/mo, $120/yr
	got := su4AnnualSavings(in)
	if len(got) != 1 {
		t.Fatalf("want 1 annual-savings nudge, got %d: %+v", len(got), got)
	}
	// 16% of $120 = ~$19.20.
	if got[0].Amount.Amount != 1920 {
		t.Errorf("saving = %d, want 1920", got[0].Amount.Amount)
	}
}

func TestSU4SkipsSmall(t *testing.T) {
	in := baseInput()
	in.Transactions = monthlyCharges("Tiny", -100, time.June, 4) // $1/mo → $12/yr < floor
	if got := su4AnnualSavings(in); len(got) != 0 {
		t.Errorf("below floor — want 0, got %d", len(got))
	}
}

func TestSU1CancelCandidateHighShare(t *testing.T) {
	in := baseInput()
	// One dominant sub + one small one → the big one is a high-share candidate.
	txns := monthlyCharges("Cable", -10000, time.June, 4) // $100/mo
	txns = append(txns, monthlyCharges("News", -500, time.June, 4)...)
	in.Transactions = txns
	got := su1CancelCandidates(in)
	if len(got) == 0 {
		t.Fatalf("want at least 1 cancel candidate, got 0")
	}
	var sawCable bool
	for _, i := range got {
		if i.Key == "SMART-SU1:cable" {
			sawCable = true
		}
	}
	if !sawCable {
		t.Errorf("expected Cable flagged as a high-share candidate: %+v", got)
	}
}

func TestSU1StaleCandidate(t *testing.T) {
	in := baseInput()
	// A monthly sub whose last charge was 4 months ago → stale (NeedsReview).
	in.Transactions = monthlyCharges("OldGym", -3000, time.February, 4)
	got := su1CancelCandidates(in)
	if len(got) != 1 {
		t.Fatalf("want 1 stale candidate, got %d: %+v", len(got), got)
	}
}

func TestSU14CancellationTally(t *testing.T) {
	in := baseInput()
	in.Subscriptions = []domain.SubscriptionCancellation{
		{ID: "1", SubName: "Hulu", CancelledOn: ref.AddDate(0, -1, 0)},
		{ID: "2", SubName: "Disney+", CancelledOn: ref.AddDate(0, -2, 0)},
	}
	got := su14CancellationTally(in)
	if len(got) != 1 {
		t.Fatalf("want 1 tally insight, got %d", len(got))
	}
	if got[0].Severity != smart.SeverityInfo {
		t.Errorf("tally should be info, got %v", got[0].Severity)
	}
}

func TestSU14EmptyNoInsight(t *testing.T) {
	if got := su14CancellationTally(baseInput()); len(got) != 0 {
		t.Errorf("no cancellations — want 0, got %d", len(got))
	}
}

// Priming the detected-subscription set once per run is a pure sharing of work:
// the engines must say exactly what they said when each detected the set for
// itself. This runs the whole subscriptions page both ways and compares the
// resulting insight keys, so a future engine that starts reading Input.Subs (or a
// mis-registered subsBacked entry) cannot quietly change what the page reports.
func TestSubsPrimingPreservesInsights(t *testing.T) {
	in := baseInput()
	in.Members = []domain.Member{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	in.Categories = []domain.Category{{ID: "c-fit", Name: "Fitness"}}
	in.Transactions = append(
		monthlyCharges("Spotify", -1000, time.June, 5),
		monthlyCharges("Gym", -4000, time.June, 5)...,
	)
	for i := range in.Transactions {
		if in.Transactions[i].Desc == "Gym" {
			in.Transactions[i].CategoryID = "c-fit"
		}
	}
	s := smart.Settings{}

	primed := RunPage(in, s, smart.PageSubscriptions)

	// The same run without the priming: dispatch each engine on the raw Input.
	var raw []smart.Insight
	for _, f := range s.EnabledFeaturesForPage(smart.PageSubscriptions) {
		if fn := engines[f.Code]; fn != nil {
			raw = append(raw, fn(in)...)
		}
	}
	raw = s.Active(raw)
	smart.SortInsights(raw)

	if len(primed) == 0 {
		t.Fatal("fixture produced no subscription insights — it cannot guard anything")
	}
	if len(primed) != len(raw) {
		t.Fatalf("primed run produced %d insights, unprimed %d", len(primed), len(raw))
	}
	for i := range primed {
		if primed[i].Key != raw[i].Key {
			t.Errorf("insight %d: primed %q, unprimed %q", i, primed[i].Key, raw[i].Key)
		}
	}
}
