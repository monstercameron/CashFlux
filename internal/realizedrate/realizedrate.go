// SPDX-License-Identifier: MIT

// Package realizedrate works out what an account ACTUALLY paid, from its own
// postings (EC-6).
//
// A rate you were promised and a rate you received are different facts, and only
// the second is evidence. A "high-yield" account whose posted interest works out
// at 0.4% is the single most common quiet loss in a household's finances,
// because the number on the marketing page is the one everybody remembers and
// nothing in the app ever checks it.
//
// # What this package will not do
//
// It does not decide which postings are interest. That is a judgement about the
// household's own categories, and a package that guessed — by payee text, or by
// assuming all income into a savings account is interest — would produce a
// confident annual percentage from a birthday cheque. The caller identifies the
// postings and says so in the copy; this package does the arithmetic and refuses
// when the arithmetic would not mean anything.
package realizedrate

import "time"

// MinPostings is how many interest payments are needed before a rate can be
// worked out.
//
// Three. Two payments can be a stub period and a full one, or a correction
// following an error, and annualising either produces a headline number from an
// accident of timing.
const MinPostings = 3

// MinDays is the shortest span worth annualising.
//
// Ninety days. Below a quarter, one month's rounding is a large share of the
// total, and multiplying it by four to reach an annual figure multiplies the
// error with it.
const MinDays = 90

// ShortfallPct is how far under its stated rate an account must fall before it
// is worth saying.
//
// Twenty-five percent of the stated rate, rather than a fixed number of points:
// a 5% account paying 4.6% is within the noise of when interest posts, while a
// 5% account paying 1% is a different product from the one that was chosen. A
// fixed point-gap would shout about every high-rate account and stay silent
// about a 0.5% "savings" account paying 0.05%.
const ShortfallPct = 25.0

// Posting is one interest payment the caller has identified.
type Posting struct {
	Date  time.Time
	Minor int64 // magnitude
}

// Result is what the account actually paid.
type Result struct {
	// Known is false when there was not enough to work anything out. It is
	// reported rather than a rate of zero, because "we cannot tell" and "it paid
	// nothing" would otherwise look identical — and the second is an accusation.
	Known bool
	// AnnualPct is the realized rate, annualised from the observed span.
	AnnualPct float64
	// TotalMinor is what was actually paid over the span, which is the figure
	// worth showing next to the percentage: a rate is an abstraction and "$4.20 in
	// six months" is not.
	TotalMinor int64
	// Postings and Days are the evidence the rate rests on.
	Postings int
	Days     int
}

// Measure works out the realized annual rate from interest postings against the
// balance the money actually sat at.
//
// avgBalanceMinor is the average balance over the span — the caller's figure,
// because only it knows how the balance moved. A zero or negative average
// balance yields Known=false: there is no rate on money that was not there.
func Measure(postings []Posting, avgBalanceMinor int64, now time.Time) Result {
	var r Result
	if len(postings) < MinPostings || avgBalanceMinor <= 0 {
		return r
	}
	first, last := postings[0].Date, postings[0].Date
	for _, p := range postings {
		if p.Date.Before(first) {
			first = p.Date
		}
		if p.Date.After(last) {
			last = p.Date
		}
		r.TotalMinor += abs64(p.Minor)
	}
	// The span runs from the first posting to NOW rather than to the last one:
	// interest that stopped three months ago is part of the story, and measuring
	// only up to the final payment would quietly hide an account that went quiet.
	end := now
	if end.Before(last) {
		end = last
	}
	// A payment covers the period BEFORE it, so the span has to include the first
	// payment's own accrual period. Without it, twelve monthly payments look like
	// a year's interest earned in eleven months, and every account reads as
	// paying more than it does — 1.31% for what any reader would call 1.2%.
	//
	// The accrual period is taken as the average gap between postings, which is
	// what the account's own rhythm says it is.
	observed := end.Sub(first)
	gap := last.Sub(first) / time.Duration(len(postings)-1)
	days := int((observed + gap).Hours() / 24)
	if days < MinDays {
		return Result{}
	}
	r.Known = true
	r.Postings = len(postings)
	r.Days = days
	r.AnnualPct = float64(r.TotalMinor) / float64(avgBalanceMinor) * (365.0 / float64(days)) * 100
	return r
}

// Shortfall compares what an account paid against what it was supposed to.
//
// statedPct is the rate on the account. A stated rate of zero or less means
// there is nothing to compare against — which is not the same as the account
// being fine, and is why this returns a second value rather than false.
func Shortfall(r Result, statedPct float64) (shortPct float64, under bool) {
	if !r.Known || statedPct <= 0 {
		return 0, false
	}
	gap := statedPct - r.AnnualPct
	if gap <= 0 {
		return 0, false // it paid what it said, or better
	}
	if gap/statedPct*100 < ShortfallPct {
		return 0, false // within the noise of when interest posts
	}
	return gap, true
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
