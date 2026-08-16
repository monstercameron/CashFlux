// SPDX-License-Identifier: MIT

package negotiationprep

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var callNow = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func usd(v int64) string { return fmt.Sprintf("$%d.%02d", v/100, v%100) }

func charge(y int, m time.Month, minor int64) Charge {
	return Charge{At: time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), AmountMinor: minor}
}

func TestADocumentedPriceRiseLeadsTheSheet(t *testing.T) {
	// It is the fact a provider is most often willing to move on, because it is
	// theirs.
	p := Build(Subscription{
		Name:         "Streamly",
		MonthlyMinor: 1899,
		Charges: []Charge{
			charge(2024, time.March, 1299), charge(2025, time.March, 1499), charge(2026, time.August, 1899),
		},
	}, callNow, usd)
	if len(p.Leverage) == 0 || !strings.Contains(p.Leverage[0].Point, "went up") {
		t.Fatalf("first point = %+v, want the price rise", p.Leverage)
	}
	if p.RiseMinor != 600 {
		t.Fatalf("rise = %d, want 600 from the lowest charge seen", p.RiseMinor)
	}
	if p.RisePercent != 46 {
		t.Fatalf("rise percent = %d, want 46", p.RisePercent)
	}
}

func TestShortTenureIsAWarningNotALeveragePoint(t *testing.T) {
	// "I've been a customer a while" invites "you've been with us four months".
	p := Build(Subscription{
		Name: "Newish", MonthlyMinor: 999,
		Charges: []Charge{charge(2026, time.May, 999)},
	}, callNow, usd)
	for _, l := range p.Leverage {
		if strings.Contains(l.Point, "paid them for") {
			t.Fatalf("three months of tenure was offered as leverage: %q", l.Point)
		}
	}
	if !containsSubstring(p.Gaps, "don't lead with loyalty") {
		t.Fatalf("gaps = %v, want the tenure warning", p.Gaps)
	}
}

func TestLongTenureIsWorthSaying(t *testing.T) {
	p := Build(Subscription{
		Name: "Oldish", MonthlyMinor: 999,
		Charges: []Charge{charge(2023, time.January, 999)},
	}, callNow, usd)
	if !containsSubstring(pointsOf(p), "over 3 years") {
		t.Fatalf("leverage = %v, want the tenure stated the way somebody would say it", pointsOf(p))
	}
}

func TestACompetitorPriceIsNeverInvented(t *testing.T) {
	// A made-up competitor price is the single most damaging thing this could do:
	// the provider disproves it in one sentence and the call is over.
	p := Build(Subscription{Name: "Streamly", MonthlyMinor: 1899,
		Charges: []Charge{charge(2023, time.January, 1299)}}, callNow, usd)
	for _, l := range p.Leverage {
		if strings.Contains(strings.ToLower(l.Point), "competitor") || strings.Contains(l.Point, "similar plans") {
			t.Fatalf("a competitor claim appeared with nothing behind it: %q", l.Point)
		}
	}
	if !containsSubstring(p.Gaps, "don't claim one") {
		t.Fatalf("gaps = %v, want the missing-comparison warning", p.Gaps)
	}
	for _, line := range p.Script {
		if strings.Contains(line, "similar plans") {
			t.Fatalf("the script asserts a comparison that was never looked up: %q", line)
		}
	}
}

func TestASuppliedComparisonIsUsedVerbatim(t *testing.T) {
	p := Build(Subscription{
		Name: "Streamly", MonthlyMinor: 1899, CompetitorNote: "Similar plans run $8–12.",
		Charges: []Charge{charge(2023, time.January, 1299)},
	}, callNow, usd)
	if !containsSubstring(pointsOf(p), "Similar plans run $8–12.") {
		t.Fatalf("leverage = %v", pointsOf(p))
	}
	if !containsSubstring(p.Script, "Similar plans run $8–12.") {
		t.Fatalf("script = %v", p.Script)
	}
}

func TestTheAnnualFigureIsWhatMakesTheCallWorthMaking(t *testing.T) {
	p := Build(Subscription{Name: "Streamly", MonthlyMinor: 1899,
		Charges: []Charge{charge(2023, time.January, 1299)}}, callNow, usd)
	if p.AnnualMinor != 1899*12 {
		t.Fatalf("annual = %d", p.AnnualMinor)
	}
	if !containsSubstring(pointsOf(p), "$227.88 a year") {
		t.Fatalf("leverage = %v, want the annual figure", pointsOf(p))
	}
}

func TestTheScriptEndsWithTheAskThePauseAndTheEscalation(t *testing.T) {
	p := Build(Subscription{Name: "Streamly", MonthlyMinor: 1899,
		Charges: []Charge{charge(2023, time.January, 1299)}}, callNow, usd)
	joined := strings.Join(p.Script, "\n")
	for _, want := range []string{"better rate", "stop talking", "cancel"} {
		if !strings.Contains(joined, want) {
			t.Errorf("script is missing %q:\n%s", want, joined)
		}
	}
	// The pause is the actual technique, so it is written down rather than left
	// to nerve.
	if !strings.Contains(joined, "(Then stop talking") {
		t.Errorf("the script does not tell them to stop and wait:\n%s", joined)
	}
}

func TestACallWithNothingToSayIsNotWorthMaking(t *testing.T) {
	// Sending somebody into a call with no leverage is worse than not suggesting
	// it: they lose ten minutes and conclude the feature is useless.
	p := Build(Subscription{Name: "Brand new", MonthlyMinor: 500,
		Charges: []Charge{charge(2026, time.July, 500)}}, callNow, usd)
	if p.Worthwhile() {
		t.Fatalf("a four-week-old subscription with no rise was called worthwhile: %+v", p.Leverage)
	}
}

func TestASubscriptionWithARiseIsWorthTheCall(t *testing.T) {
	// This used to pass with a SINGLE past charge, which is exactly the shape the
	// review showed to be unsafe: one charge cannot establish a rise, and treating
	// it as one is how an unrelated purchase becomes a quoted old price. A rise now
	// needs a history that looks like a subscription.
	p := Build(Subscription{Name: "Streamly", MonthlyMinor: 1899,
		Charges: []Charge{
			charge(2026, time.May, 1299), charge(2026, time.June, 1499), charge(2026, time.July, 1899),
		}}, callNow, usd)
	if !p.Worthwhile() {
		t.Fatalf("a documented rise was not counted as leverage: %+v", p.Leverage)
	}
}

func TestNoChargeHistoryIsStatedRatherThanPapersOver(t *testing.T) {
	p := Build(Subscription{Name: "Mystery", MonthlyMinor: 999}, callNow, usd)
	if !containsSubstring(p.Gaps, "no price history") {
		t.Fatalf("gaps = %v", p.Gaps)
	}
	if p.Worthwhile() {
		t.Fatal("a subscription with no history at all was called worthwhile")
	}
}

func TestTaskNotesCarryEverythingNeededToMakeTheCall(t *testing.T) {
	// A task saying only "call the provider" gets postponed forever; one holding
	// the script gets made.
	p := Build(Subscription{Name: "Streamly", MonthlyMinor: 1899, CompetitorNote: "Similar plans run $8–12.",
		Charges: []Charge{charge(2023, time.January, 1299)}}, callNow, usd)
	notes := p.TaskNotes()
	for _, want := range []string{"What you've got:", "What to say:", "better rate"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes missing %q:\n%s", want, notes)
		}
	}
	if strings.HasSuffix(notes, "\n") {
		t.Error("notes end with a stray newline")
	}
}

func TestAnUnnamedSubscriptionStillReadsAsASentence(t *testing.T) {
	p := Build(Subscription{MonthlyMinor: 999, Charges: []Charge{charge(2023, time.January, 999)}}, callNow, usd)
	if strings.Contains(p.Script[0], "my  account") {
		t.Fatalf("first line has a hole in it: %q", p.Script[0])
	}
	if !strings.Contains(p.Script[0], "this subscription") {
		t.Fatalf("first line = %q", p.Script[0])
	}
}

func pointsOf(p Prep) []string {
	out := make([]string, 0, len(p.Leverage))
	for _, l := range p.Leverage {
		out = append(out, l.Point)
	}
	return out
}

func containsSubstring(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestAScatteredChargeHistoryIsNotQuotedAsAPriceRise(t *testing.T) {
	// Amazon hosts a $9 subscription and a $340 purchase under the same payee.
	// Taking the low one as "what you used to pay" produces a confident, specific,
	// wrong sentence — read out loud to a rep who corrects it, ending the call.
	p := Build(Subscription{
		Name: "Amazon", MonthlyMinor: 1499,
		Charges: []Charge{
			charge(2025, time.March, 899),   // the subscription
			charge(2025, time.April, 34000), // a television
			charge(2026, time.August, 1499),
		},
	}, callNow, usd)
	for _, l := range p.Leverage {
		if strings.Contains(l.Point, "went up") {
			t.Fatalf("a price rise was claimed from scattered charges: %q", l.Point)
		}
	}
	if !containsSubstring(p.Gaps, "vary too much") {
		t.Fatalf("gaps = %v, want the warning about mixed charges", p.Gaps)
	}
	for _, line := range p.Script {
		if strings.Contains(line, "I was paying") {
			t.Fatalf("the script quotes an old price it cannot support: %q", line)
		}
	}
}

func TestAConsistentSubscriptionHistoryStillQuotesTheRise(t *testing.T) {
	// The guard must not refuse a genuine rise: three charges within a normal
	// spread is a subscription that went up.
	p := Build(Subscription{
		Name: "Streamly", MonthlyMinor: 1899,
		Charges: []Charge{
			charge(2024, time.March, 1299), charge(2025, time.March, 1499), charge(2026, time.August, 1899),
		},
	}, callNow, usd)
	if !containsSubstring(pointsOf(p), "went up") {
		t.Fatalf("a genuine rise was refused: %v", pointsOf(p))
	}
}

func TestTwoChargesAreNotEnoughToCallItARise(t *testing.T) {
	// A line through two points fits anything.
	p := Build(Subscription{
		Name: "Newish", MonthlyMinor: 1899,
		Charges: []Charge{charge(2026, time.July, 1299), charge(2026, time.August, 1899)},
	}, callNow, usd)
	for _, l := range p.Leverage {
		if strings.Contains(l.Point, "went up") {
			t.Fatalf("a rise was claimed from two charges: %q", l.Point)
		}
	}
}
