// SPDX-License-Identifier: MIT

package aispend

import (
	"testing"
	"time"
)

func at(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func TestRecordAccumulatesByMonthAndFeature(t *testing.T) {
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 3), Tokens: 1000, CostUSD: 0.02, HasCost: true})
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 9), Tokens: 500, CostUSD: 0.01, HasCost: true})
	l.Record(Entry{Feature: "categorize", At: at(2026, time.August, 9), Tokens: 200, CostUSD: 0.001, HasCost: true})
	l.Record(Entry{Feature: "assistant", At: at(2026, time.July, 30), Tokens: 9000, CostUSD: 0.5, HasCost: true})

	aug := l.Month(at(2026, time.August, 17))
	if !aug.Recorded {
		t.Fatal("August reports nothing recorded")
	}
	if aug.Calls != 3 || aug.Tokens != 1700 {
		t.Fatalf("August totals = %d calls / %d tokens", aug.Calls, aug.Tokens)
	}
	if len(aug.ByFeature) != 2 || aug.ByFeature[0].Feature != "assistant" {
		t.Fatalf("split = %+v, want the biggest spender first", aug.ByFeature)
	}
	// July's spend must not leak into August.
	if aug.CostUSD > 0.04 {
		t.Fatalf("August cost = %v — another month's spend leaked in", aug.CostUSD)
	}
}

func TestAMonthWithNothingRecordedIsNotAZeroMonth(t *testing.T) {
	// "Nothing yet" and "$0.00 spent" are different statements, and showing the
	// second when the first is true reads as a broken meter.
	s := (&Ledger{}).Month(at(2026, time.August, 17))
	if s.Recorded {
		t.Fatal("an empty ledger reported a recorded month")
	}
	if s.Calls != 0 || s.CostUSD != 0 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestAnUnpricedCallStillCountsItsTokens(t *testing.T) {
	// Dropping the row would understate the total; recording it as $0 would
	// understate the cost. It counts, and the summary says the cost is a floor.
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 3), Tokens: 800})
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 4), Tokens: 200, CostUSD: 0.01, HasCost: true})
	s := l.Month(at(2026, time.August, 17))
	if s.Tokens != 1000 {
		t.Fatalf("tokens = %d, want the unpriced call counted", s.Tokens)
	}
	if s.UnpricedCalls != 1 {
		t.Fatalf("unpriced = %d", s.UnpricedCalls)
	}
	if s.Complete() {
		t.Fatal("a month containing an unpriced call reported as complete")
	}
}

func TestSpendWithNoFeatureNameIsStillCounted(t *testing.T) {
	l := &Ledger{}
	l.Record(Entry{At: at(2026, time.August, 3), Tokens: 100, CostUSD: 0.01, HasCost: true})
	s := l.Month(at(2026, time.August, 17))
	if s.Calls != 1 || len(s.ByFeature) != 1 || s.ByFeature[0].Feature != "other" {
		t.Fatalf("summary = %+v — unattributed spend must still appear", s)
	}
}

func TestTheSplitIsDeterministicOnTies(t *testing.T) {
	l := &Ledger{}
	for _, f := range []string{"zebra", "apple", "mango"} {
		l.Record(Entry{Feature: f, At: at(2026, time.August, 3), Tokens: 100, CostUSD: 0.01, HasCost: true})
	}
	first := l.Month(at(2026, time.August, 17)).ByFeature
	for i := 0; i < 5; i++ {
		again := l.Month(at(2026, time.August, 17)).ByFeature
		for j := range first {
			if again[j].Feature != first[j].Feature {
				t.Fatalf("order changed between calls: %v then %v", first[j].Feature, again[j].Feature)
			}
		}
	}
	if first[0].Feature != "apple" {
		t.Fatalf("ties should break alphabetically, got %q first", first[0].Feature)
	}
}

func TestTrimKeepsAYearAndDropsWhatIsOlder(t *testing.T) {
	l := &Ledger{}
	for i := 0; i < 20; i++ {
		l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1).AddDate(0, -i, 0), Tokens: 10, CostUSD: 0.01, HasCost: true})
	}
	l.Trim(at(2026, time.August, 17), 12)
	if got := len(l.Buckets()); got != 12 {
		t.Fatalf("kept %d months, want 12", got)
	}
	// The current month must survive its own trim.
	if !l.Month(at(2026, time.August, 17)).Recorded {
		t.Fatal("Trim removed the current month")
	}
}

func TestTrimIsANoOpWithoutALimit(t *testing.T) {
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2020, time.January, 1), Tokens: 10})
	l.Trim(at(2026, time.August, 17), 0)
	if len(l.Buckets()) != 1 {
		t.Fatal("Trim(0) dropped history")
	}
}

func TestProjectionRefusesToExtrapolateFromOneDay(t *testing.T) {
	// One day into a month, multiplying by 31 produces a number that is alarming
	// and meaningless in equal measure.
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: 1.0, HasCost: true})
	if _, ok := l.Month(at(2026, time.August, 1)).ProjectSpend(at(2026, time.August, 1)); ok {
		t.Fatal("projected a month from a single day")
	}
}

func TestProjectionScalesByHowMuchOfTheMonthHasElapsed(t *testing.T) {
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: 5.0, HasCost: true})
	// $5 over 10 days of a 31-day month projects to $15.50.
	got, ok := l.Month(at(2026, time.August, 10)).ProjectSpend(at(2026, time.August, 10))
	if !ok {
		t.Fatal("no projection")
	}
	if want := 5.0 * 31 / 10; got != want {
		t.Fatalf("projection = %v, want %v", got, want)
	}
}

func TestPaceAgainstACap(t *testing.T) {
	spend := func(usd float64) Summary {
		l := &Ledger{}
		l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: usd, HasCost: true})
		return l.Month(at(2026, time.August, 10))
	}
	now := at(2026, time.August, 10) // 10 of 31 days elapsed → ×3.1

	if got := spend(1.0).PaceAgainst(0, now); got != PaceNoCap {
		t.Fatalf("no cap = %v", got)
	}
	if got := spend(1.0).PaceAgainst(10, now); got != PaceComfortable {
		t.Fatalf("$3.10 projected against a $10 cap = %v, want comfortable", got)
	}
	if got := spend(1.0).PaceAgainst(3.3, now); got != PaceTight {
		t.Fatalf("$3.10 projected against a $3.30 cap = %v, want tight", got)
	}
	if got := spend(2.0).PaceAgainst(5, now); got != PaceOverPace {
		t.Fatalf("$6.20 projected against a $5 cap = %v, want over pace", got)
	}
	if got := spend(6.0).PaceAgainst(5, now); got != PaceExceeded {
		t.Fatalf("$6 spent against a $5 cap = %v, want exceeded", got)
	}
}

func TestAnAlreadyExceededCapIsReportedEvenEarlyInTheMonth(t *testing.T) {
	// The projection refuses to extrapolate from one day, but a cap that has
	// ALREADY been passed is a fact, not a forecast, and must still be reported.
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: 20, HasCost: true})
	if got := l.Month(at(2026, time.August, 1)).PaceAgainst(5, at(2026, time.August, 1)); got != PaceExceeded {
		t.Fatalf("pace = %v, want exceeded", got)
	}
}

func TestBucketsRoundTripThroughPersistence(t *testing.T) {
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 3), Tokens: 1000, CostUSD: 0.02, HasCost: true})
	l.Record(Entry{Feature: "categorize", At: at(2026, time.July, 3), Tokens: 40, CostUSD: 0.001, HasCost: true})

	restored := NewLedger(l.Buckets())
	aug := restored.Month(at(2026, time.August, 17))
	if aug.Tokens != 1000 || aug.CostUSD != 0.02 {
		t.Fatalf("August did not survive the round trip: %+v", aug)
	}
	// Newest month first keeps the stored order stable and a dataset diff readable.
	buckets := restored.Buckets()
	if !buckets[0].Month.After(buckets[1].Month) {
		t.Fatalf("stored order = %v then %v, want newest first", buckets[0].Month, buckets[1].Month)
	}
	// A restored ledger must keep accumulating into the same buckets, not shadow
	// them with new ones.
	restored.Record(Entry{Feature: "assistant", At: at(2026, time.August, 5), Tokens: 500, CostUSD: 0.01, HasCost: true})
	if got := len(restored.Buckets()); got != 2 {
		t.Fatalf("recording into a restored ledger made %d buckets, want 2", got)
	}
}

func TestMonthOfNormalisesToUTC(t *testing.T) {
	// A dataset that syncs across time zones must not split one month in two.
	tokyo := time.FixedZone("JST", 9*3600)
	late := time.Date(2026, time.August, 1, 2, 0, 0, 0, tokyo) // 31 July in UTC
	if got := MonthOf(late); !got.Equal(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("MonthOf = %v, want July in UTC", got)
	}
}

func TestAnUnpricedCallMakesTheVerdictUnknownNotComfortable(t *testing.T) {
	// The recorded cost is a floor when any call ran on a model with no known
	// price. A household could be well past its cap while the priced-only figure
	// still looks fine — calling that "comfortable" defeats the point of the cap.
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: 1.0, HasCost: true})
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 2), Tokens: 90000}) // unpriced model
	s := l.Month(at(2026, time.August, 10))
	if got := s.PaceAgainst(10, at(2026, time.August, 10)); got != PaceUnknown {
		t.Fatalf("pace = %v, want unknown while part of the spend cannot be priced", got)
	}
}

func TestAnAlreadyExceededCapIsStillReportedWhenSomeCallsAreUnpriced(t *testing.T) {
	// Past the cap on the priced calls ALONE is true whatever the rest cost, so
	// this verdict stays safe to give.
	l := &Ledger{}
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 1), Tokens: 100, CostUSD: 20, HasCost: true})
	l.Record(Entry{Feature: "assistant", At: at(2026, time.August, 2), Tokens: 50})
	if got := l.Month(at(2026, time.August, 10)).PaceAgainst(5, at(2026, time.August, 10)); got != PaceExceeded {
		t.Fatalf("pace = %v, want exceeded", got)
	}
}
