// SPDX-License-Identifier: MIT

package realizedrate

import (
	"math"
	"testing"
	"time"
)

var jan1 = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// monthly builds n monthly postings of minor units each, starting at jan1.
func monthly(n int, minor int64) []Posting {
	out := make([]Posting, 0, n)
	for i := range n {
		out = append(out, Posting{Date: jan1.AddDate(0, i, 0), Minor: minor})
	}
	return out
}

func TestItMeasuresWhatWasActuallyPaid(t *testing.T) {
	// $10,000 balance, $10 a month for a year = $120 = 1.2%.
	now := jan1.AddDate(0, 11, 0)
	r := Measure(monthly(12, 1_000), 1_000_000, now)
	if !r.Known {
		t.Fatal("expected a rate")
	}
	if math.Abs(r.AnnualPct-1.2) > 0.05 {
		t.Errorf("rate = %.3f%%, want about 1.2", r.AnnualPct)
	}
	if r.TotalMinor != 12_000 {
		t.Errorf("total = %d, want 12000 — the figure worth showing beside a percentage", r.TotalMinor)
	}
	if r.Postings != 12 {
		t.Errorf("postings = %d, want the evidence carried", r.Postings)
	}
}

// "We cannot tell" and "it paid nothing" would otherwise look identical, and the
// second is an accusation.
func TestThinEvidenceIsNotARateOfZero(t *testing.T) {
	now := jan1.AddDate(0, 6, 0)
	if r := Measure(monthly(2, 1_000), 1_000_000, now); r.Known {
		t.Errorf("two postings produced a rate: %+v", r)
	}
	// Three postings, but only six weeks between the first and now.
	short := []Posting{
		{Date: jan1, Minor: 1_000},
		{Date: jan1.AddDate(0, 0, 20), Minor: 1_000},
		{Date: jan1.AddDate(0, 0, 40), Minor: 1_000},
	}
	if r := Measure(short, 1_000_000, jan1.AddDate(0, 0, 42)); r.Known {
		t.Errorf("six weeks was annualised: %+v", r)
	}
	if r := Measure(monthly(12, 1_000), 0, now); r.Known {
		t.Error("a rate was reported on money that was not there")
	}
	if r := Measure(nil, 1_000_000, now); r.Known {
		t.Error("no postings produced a rate")
	}
}

// Interest that stopped three months ago is part of the story; measuring only up
// to the final payment would quietly hide an account that went quiet.
func TestASpanRunsToNowNotToTheLastPayment(t *testing.T) {
	postings := monthly(6, 1_000) // six months of interest, then silence
	stillPaying := Measure(postings, 1_000_000, jan1.AddDate(0, 5, 0))
	wentQuiet := Measure(postings, 1_000_000, jan1.AddDate(0, 11, 0))
	if !stillPaying.Known || !wentQuiet.Known {
		t.Fatal("expected both to be measurable")
	}
	if wentQuiet.AnnualPct >= stillPaying.AnnualPct {
		t.Errorf("an account that stopped paying reported %.2f%% against %.2f%% — the silence must count",
			wentQuiet.AnnualPct, stillPaying.AnnualPct)
	}
}

// A 5% account paying 4.6% is within the noise of when interest posts; a 5%
// account paying 1% is a different product from the one that was chosen.
func TestShortfallIsRelativeNotAFixedGap(t *testing.T) {
	now := jan1.AddDate(0, 11, 0)
	// Realized ~4.6% against a stated 5%: close enough.
	near := Measure(monthly(12, 3_833), 1_000_000, now)
	if _, under := Shortfall(near, 5.0); under {
		t.Errorf("a %.2f%% account against a stated 5%% was called short", near.AnnualPct)
	}
	// Realized ~1.2% against a stated 5%: a different product.
	far := Measure(monthly(12, 1_000), 1_000_000, now)
	gap, under := Shortfall(far, 5.0)
	if !under {
		t.Fatalf("a %.2f%% account against a stated 5%% was not flagged", far.AnnualPct)
	}
	if gap < 3.5 || gap > 4.0 {
		t.Errorf("gap = %.2f points, want about 3.8", gap)
	}
	// The relative rule must also catch a small stated rate that pays nearly
	// nothing — a fixed point-gap would stay silent here.
	tiny := Measure(monthly(12, 4), 1_000_000, now) // ~0.005%
	if _, under := Shortfall(tiny, 0.5); !under {
		t.Errorf("a 0.5%% account paying %.4f%% was not flagged", tiny.AnnualPct)
	}
}

func TestNothingToCompareAgainstIsNotAPass(t *testing.T) {
	now := jan1.AddDate(0, 11, 0)
	r := Measure(monthly(12, 1_000), 1_000_000, now)
	// A stated rate of zero (or none recorded) means there is no promise to
	// check, which is different from the account being fine.
	if gap, under := Shortfall(r, 0); under || gap != 0 {
		t.Errorf("compared against a rate nobody stated: %v, %v", gap, under)
	}
	if _, under := Shortfall(Result{}, 5); under {
		t.Error("an unmeasurable account was reported as under its rate")
	}
}

func TestPayingBetterThanPromisedIsNotAShortfall(t *testing.T) {
	now := jan1.AddDate(0, 11, 0)
	r := Measure(monthly(12, 5_000), 1_000_000, now) // ~6%
	if _, under := Shortfall(r, 5.0); under {
		t.Errorf("an account paying %.2f%% against a stated 5%% was called short", r.AnnualPct)
	}
}

func TestPostingOrderDoesNotMatter(t *testing.T) {
	now := jan1.AddDate(0, 11, 0)
	forward := monthly(12, 1_000)
	backward := make([]Posting, len(forward))
	for i, p := range forward {
		backward[len(forward)-1-i] = p
	}
	a, b := Measure(forward, 1_000_000, now), Measure(backward, 1_000_000, now)
	if a != b {
		t.Errorf("order changed the answer: %+v vs %+v", a, b)
	}
}

func TestRefundsAreCountedAsMagnitudes(t *testing.T) {
	now := jan1.AddDate(0, 11, 0)
	ps := monthly(12, 1_000)
	ps[0].Minor = -1_000 // stored with the opposite sign
	r := Measure(ps, 1_000_000, now)
	if r.TotalMinor != 12_000 {
		t.Errorf("total = %d, want 12000 — sign conventions are the caller's problem, not the rate's", r.TotalMinor)
	}
}
