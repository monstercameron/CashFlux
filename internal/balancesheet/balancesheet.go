// SPDX-License-Identifier: MIT

// Package balancesheet turns accounts and transactions into the two things a
// household balance sheet is actually read for: what the two SIDES are made of
// over time, and what the standard ratios mean in plain terms.
//
// It exists because "net worth" as a single line hides both stories. A number
// that rises identically whether you saved it or whether a condo was
// re-appraised is not an answer, and a bare "12% liquid" is a number the reader
// is left to judge alone. So this package returns composition — cash, invested,
// property on the asset side; credit, loans, mortgage on the liability side —
// as a series, and returns each ratio paired with a BAND the caller phrases.
//
// Conventions match the rest of the app exactly so no surface can disagree with
// another: a balance "as of" a cutoff counts transactions STRICTLY BEFORE it
// (ledger.NetWorthSeries), liabilities contribute the magnitude of their balance
// (ledger.NetWorth), archived accounts are excluded, and every figure is in
// base-currency minor units.
//
// Pure Go, no syscall/js; unit-tested on native Go.
package balancesheet

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Bucket is one composition band of the balance sheet. The asset buckets and
// the liability buckets are deliberately separate value sets so a caller can
// never accidentally stack a mortgage on top of cash.
type Bucket string

const (
	// BucketCash is everyday, spendable money: checking, debit, savings, cash.
	BucketCash Bucket = "cash"
	// BucketInvested is market-exposed holdings: investment, retirement, crypto.
	BucketInvested Bucket = "invested"
	// BucketProperty is property and vehicles — real, but not spendable.
	BucketProperty Bucket = "property"
	// BucketOtherAsset is every other asset type.
	BucketOtherAsset Bucket = "otherAsset"

	// BucketCredit is revolving debt: credit cards and lines of credit.
	BucketCredit Bucket = "credit"
	// BucketLoans is instalment debt: loans and personal loans.
	BucketLoans Bucket = "loans"
	// BucketMortgage is mortgage debt.
	BucketMortgage Bucket = "mortgage"
)

// AssetBuckets is the canonical stacking order of the asset side, most liquid
// first — so the mirrored chart reads outward from spendable to fixed.
var AssetBuckets = []Bucket{BucketCash, BucketInvested, BucketProperty, BucketOtherAsset}

// LiabilityBuckets is the canonical stacking order of the liability side, most
// urgent first.
var LiabilityBuckets = []Bucket{BucketCredit, BucketLoans, BucketMortgage}

// BucketOf classifies an account into its composition bucket. It is the SINGLE
// definition of these bands: the page, the chart and the networth_* engine
// variables all bucket through here, so a figure can never mean one thing in
// the chart and another in a formula.
func BucketOf(a domain.Account) Bucket {
	if a.Class == domain.ClassLiability {
		switch a.Type {
		case domain.TypeCreditCard, domain.TypeLineOfCredit:
			return BucketCredit
		case domain.TypeMortgage:
			return BucketMortgage
		default:
			return BucketLoans
		}
	}
	switch a.Type {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return BucketCash
	case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		return BucketInvested
	case domain.TypeProperty, domain.TypeVehicle:
		return BucketProperty
	}
	return BucketOtherAsset
}

// Point is the balance sheet as of one cutoff.
type Point struct {
	// At is the cutoff this point describes (transactions strictly before it).
	At time.Time
	// Assets / Liabilities hold every bucket in the corresponding canonical
	// order, including zeros, so a chart can rely on the shape. Liability
	// amounts are POSITIVE magnitudes ("what you owe"), never negative — the
	// mirrored chart draws them downward itself.
	Assets      map[Bucket]int64
	Liabilities map[Bucket]int64
	// AssetsMinor / LiabilitiesMinor are the side totals; NetMinor is
	// AssetsMinor - LiabilitiesMinor, matching ledger.NetWorth exactly.
	AssetsMinor, LiabilitiesMinor, NetMinor int64
}

// Series returns the composed balance sheet at each cutoff, in cutoff order.
// Every point's NetMinor agrees with ledger.NetWorthSeries for the same cutoff.
func Series(accounts []domain.Account, txns []domain.Transaction, cutoffs []time.Time, rates currency.Rates) ([]Point, error) {
	base := rates.Base
	out := make([]Point, 0, len(cutoffs))
	for _, at := range cutoffs {
		p := Point{At: at, Assets: map[Bucket]int64{}, Liabilities: map[Bucket]int64{}}
		for _, b := range AssetBuckets {
			p.Assets[b] = 0
		}
		for _, b := range LiabilityBuckets {
			p.Liabilities[b] = 0
		}
		for _, a := range accounts {
			if a.Archived {
				continue
			}
			bal := a.OpeningBalance.Amount
			for _, t := range txns {
				if t.AccountID == a.ID && t.Date.Before(at) {
					bal += t.Amount.Amount
				}
			}
			conv, err := rates.Convert(money.New(bal, a.Currency), base)
			if err != nil {
				return nil, fmt.Errorf("balancesheet: account %s: %w", a.ID, err)
			}
			bucket := BucketOf(a)
			if a.Class == domain.ClassLiability {
				mag := conv.Amount
				if mag < 0 {
					mag = -mag
				}
				p.Liabilities[bucket] += mag
				p.LiabilitiesMinor += mag
				continue
			}
			p.Assets[bucket] += conv.Amount
			p.AssetsMinor += conv.Amount
		}
		p.NetMinor = p.AssetsMinor - p.LiabilitiesMinor
		out = append(out, p)
	}
	return out, nil
}

// Band is the interpretation of a ratio: what the number MEANS, decided once
// here rather than re-guessed by each surface. The caller turns a band into a
// sentence; only BandAlarm ever earns an alarm tone, because debt on its own is
// structure, not an emergency.
type Band string

const (
	// BandStrong is comfortably healthy.
	BandStrong Band = "strong"
	// BandOK is normal and unremarkable.
	BandOK Band = "ok"
	// BandWatch is worth attention but not urgent.
	BandWatch Band = "watch"
	// BandAlarm is genuine trouble — the only band that may be painted red.
	BandAlarm Band = "alarm"
)

// Ratio is one balance-sheet ratio with everything needed to state it honestly:
// the value, whether it could be computed at all, and its band.
type Ratio struct {
	// Pct is the ratio as whole percent. Meaningful only when OK.
	Pct int64
	// OK is false when the ratio has no denominator (no assets, no expense
	// history) — the caller must then say so rather than print a fake 0%.
	OK   bool
	Band Band
}

// Health bundles the three ratios the net-worth page interprets.
type Health struct {
	// LiquidShare is spendable cash as a share of total assets.
	LiquidShare Ratio
	// DebtToAsset is what you owe as a share of what you own.
	DebtToAsset Ratio
	// RunwayTenths is how many months of typical spending the cash covers, in
	// TENTHS of a month (15 = 1.5 months), with its own band. Tenths keep the
	// figure integer-only end to end.
	RunwayTenths int64
	RunwayOK     bool
	RunwayBand   Band
	// NetNegative is true when liabilities exceed assets — the one balance-sheet
	// state that is unambiguously alarming.
	NetNegative bool
}

// Assess computes the interpreted ratios. cashMinor is the spendable (BucketCash)
// total; monthlyExpenseMinor is typical monthly spending (0 when unknown).
func Assess(assetsMinor, liabilitiesMinor, cashMinor, monthlyExpenseMinor int64) Health {
	var h Health
	h.NetNegative = liabilitiesMinor > assetsMinor

	if assetsMinor > 0 {
		pct := cashMinor * 100 / assetsMinor
		h.LiquidShare = Ratio{Pct: pct, OK: true, Band: bandForLiquid(pct)}

		dta := liabilitiesMinor * 100 / assetsMinor
		h.DebtToAsset = Ratio{Pct: dta, OK: true, Band: bandForDebt(dta)}
	}

	if monthlyExpenseMinor > 0 && cashMinor >= 0 {
		tenths := cashMinor * 10 / monthlyExpenseMinor
		h.RunwayTenths, h.RunwayOK, h.RunwayBand = tenths, true, bandForRunway(tenths)
	}
	return h
}

// bandForLiquid reads the liquid share. A low share is not automatically bad —
// it is normal for a household whose wealth sits in a home — so it never
// reaches alarm on its own; the runway band is what says whether it hurts.
func bandForLiquid(pct int64) Band {
	switch {
	case pct >= 40:
		return BandStrong
	case pct >= 15:
		return BandOK
	default:
		return BandWatch
	}
}

// bandForDebt reads debt against assets. Borrowing to own a home is structure;
// only owing more than about four-fifths of what you own is real danger.
func bandForDebt(pct int64) Band {
	switch {
	case pct <= 30:
		return BandStrong
	case pct <= 60:
		return BandOK
	case pct <= 80:
		return BandWatch
	default:
		return BandAlarm
	}
}

// bandForRunway reads months of spending held in cash — the ratio that can
// genuinely hurt, so it is the one allowed to reach alarm.
func bandForRunway(tenths int64) Band {
	switch {
	case tenths >= 60:
		return BandStrong
	case tenths >= 30:
		return BandOK
	case tenths >= 10:
		return BandWatch
	default:
		return BandAlarm
	}
}

// AxisTicks returns "nice" round tick values spanning [loMinor, hiMinor], for a
// chart axis that must be READABLE rather than merely correct. want is the
// approximate number of ticks desired; the result may hold one or two fewer or
// more, because a round step matters more than an exact count — $220,000 is a
// number a person can read off an axis, $222,420.95 is not.
//
// It lives here, with the data it labels, because choosing a step is
// computation and view code does no computation. Values outside the range are
// never returned, so an axis can never claim to show a gridline it hasn't got.
func AxisTicks(loMinor, hiMinor int64, want int) []int64 {
	if hiMinor <= loMinor || want < 2 {
		return nil
	}
	span := float64(hiMinor - loMinor)
	raw := span / float64(want-1)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	// The multipliers a reader recognizes as round at any magnitude. Take the
	// LARGEST round step that still fits inside the ideal spacing: rounding the
	// step up instead would routinely overshoot the band and leave a chart with
	// one lonely gridline, which is worse than a slightly denser axis.
	step := mag
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if mag*m <= raw {
			step = mag * m
		}
	}
	if step <= 0 {
		return nil
	}
	first := math.Ceil(float64(loMinor)/step) * step
	out := make([]int64, 0, want+1)
	for v := first; v <= float64(hiMinor)+0.5; v += step {
		out = append(out, int64(math.Round(v)))
	}
	return out
}

// TimeTick is one x-axis label decision for the point at Index.
type TimeTick struct {
	// Index is the point this label belongs to.
	Index int
	// Label is the caption to draw.
	Label string
	// Major marks the labels that survive the NARROWEST layout, so a view can
	// drop the rest as the pane shrinks without recomputing the plan or the
	// spacing that depends on it.
	Major bool
}

// timeGrains is the ladder of calendar granularities, finest first. A date axis
// must thin on calendar boundaries, never on "every third point" — a reader
// looking for 2024 finds it under a year label, and finds nothing at all under
// a run of evenly spaced months starting from an arbitrary offset.
var timeGrains = []struct {
	// months is the spacing in calendar months; anchor is the month (1-12) the
	// spacing is measured from, so quarters land on Jan/Apr/Jul/Oct rather than
	// wherever the series happens to begin.
	months int
	// years is a multi-year spacing; when non-zero, months is ignored.
	years int
}{
	{months: 1}, {months: 3}, {months: 6}, {years: 1}, {years: 2}, {years: 5}, {years: 10},
}

// TimeAxisTicks decides which of a dated series' points get an x-axis label,
// for wide and narrow layouts at once.
//
// It exists because tick density is COMPUTATION — it depends on the series
// length and on the pixels available — and the previous /networth axis did it
// by drawing one caption per point. Across an all-time window that is 37
// captions in an 900px gutter: each gets ~24px, every one truncates to its
// first letter, and the axis becomes a run of "J F M A M J J" that names no
// date at all. An unreadable label is worse than no label, because it still
// costs the space and now also costs the reader the attempt.
//
// wideMax and narrowMax are the label budgets for the widest and narrowest
// layouts. The finest calendar granularity that fits the budget wins: months
// while a year fits, then quarters, half-years, years, and multi-year steps for
// a decade of records. Ticks that also fit narrowMax are marked Major.
//
// The first and last points are ALWAYS labelled — a series whose ends are
// unnamed does not say what it covers — and a calendar tick immediately beside
// either end is dropped rather than allowed to collide with it.
func TimeAxisTicks(ats []time.Time, wideMax, narrowMax int) []TimeTick {
	if len(ats) == 0 || wideMax < 2 {
		return nil
	}
	if len(ats) == 1 {
		return []TimeTick{{Index: 0, Label: ats[0].Format("Jan 2006"), Major: true}}
	}
	multiYear := ats[0].Year() != ats[len(ats)-1].Year()
	// The budget is stated in DATED labels ("Jul 21", "2024"). A series inside
	// one year captions itself with the month alone, which is little more than
	// half as wide, so the same gutter holds half again as many — budgeting a
	// twelve-month window as though its labels were dated would thin an axis
	// that fits comfortably.
	wideMax, narrowMax = shortLabelBudget(wideMax, multiYear), shortLabelBudget(narrowMax, multiYear)
	wide := timeGrainFor(ats, wideMax)
	narrow := timeGrainFor(ats, narrowMax)

	major := map[int]bool{}
	for _, i := range narrow {
		major[i] = true
	}
	out := make([]TimeTick, 0, len(wide))
	for _, i := range wide {
		out = append(out, TimeTick{Index: i, Label: timeTickLabel(ats[i], multiYear), Major: major[i]})
	}
	return out
}

// shortLabelBudget widens a dated-label budget by half for a series captioned
// with bare month names.
func shortLabelBudget(max int, multiYear bool) int {
	if multiYear {
		return max
	}
	return max * 3 / 2
}

// timeGrainFor picks the finest calendar granularity whose tick count fits max,
// and returns the point indices it selects, ends included.
func timeGrainFor(ats []time.Time, max int) []int {
	last := len(ats) - 1
	if max < 2 {
		return []int{0, last}
	}
	// A label needs about the width the budget allots it, so two ticks closer
	// together than the series length divided by the budget cannot both be
	// drawn. Calendar boundaries are evenly spaced among themselves, but the
	// ENDS are not on the grid — a series beginning in July puts its first
	// January three points in — so the rule is enforced across every pair.
	minSep := (last + max - 1) / max
	for _, g := range timeGrains {
		idx := grainIndices(ats, g.months, g.years)
		// The grain must be coarse enough IN ITS OWN RIGHT. Thinning a fine
		// grain down to fit would put labels on arbitrary months ("Mar 22, Nov
		// 22") — evenly spaced and naming nothing a reader is looking for. A
		// grain either reads as a calendar or it is the wrong grain.
		if !spacedBy(idx, minSep) {
			continue
		}
		if idx = trimEnds(idx, minSep, last); len(idx) <= max {
			return idx
		}
	}
	return []int{0, last}
}

// spacedBy reports whether every INTERIOR pair is at least minSep apart. The
// ends sit off the calendar grid, so they are excluded here and handled by
// trimEnds.
func spacedBy(idx []int, minSep int) bool {
	for i := 2; i < len(idx)-1; i++ {
		if idx[i]-idx[i-1] < minSep {
			return false
		}
	}
	return true
}

// grainIndices selects the points landing on a calendar boundary of the given
// spacing, always including both ends.
func grainIndices(ats []time.Time, months, years int) []int {
	last := len(ats) - 1
	out := []int{0}
	for i := 1; i < last; i++ {
		if onGrain(ats[i], months, years) {
			out = append(out, i)
		}
	}
	return append(out, last)
}

// trimEnds drops calendar ticks that crowd either end. Both ends are kept
// whatever happens — a series whose ends are unnamed does not say what it
// covers — so a tick too close to one loses to it. A series beginning in July
// puts its first January three points in, which is exactly this case.
func trimEnds(idx []int, minSep, last int) []int {
	if minSep <= 1 || len(idx) < 3 {
		return idx
	}
	out := append([]int(nil), idx...)
	for len(out) > 2 && out[1]-out[0] < minSep {
		out = append(out[:1], out[2:]...)
	}
	for len(out) > 2 && last-out[len(out)-2] < minSep {
		out = append(out[:len(out)-2], last)
	}
	return out
}

// onGrain reports whether t sits on a calendar boundary of the given spacing.
func onGrain(t time.Time, months, years int) bool {
	if years > 0 {
		return t.Month() == time.January && t.Year()%years == 0
	}
	if months <= 1 {
		return true
	}
	return (int(t.Month())-1)%months == 0
}

// timeTickLabel captions one tick: a bare year on a January boundary of a
// multi-year series (the label a reader is scanning for), otherwise the month,
// carrying its year only when the series spans more than one.
func timeTickLabel(t time.Time, multiYear bool) string {
	if multiYear && t.Month() == time.January {
		return t.Format("2006")
	}
	if multiYear {
		return t.Format("Jan 06")
	}
	return t.Format("Jan")
}

// Milestone is one genuinely notable event in the net-worth series.
type Milestone struct {
	// Kind is one of the MilestoneKind* constants.
	Kind string
	// AtIndex is the point at which the event was first observed.
	AtIndex int
	// ValueMinor is the figure the event is about: the threshold for a
	// threshold crossing, the peak for a high, the trough for a reversal, and
	// zero for a sign change.
	ValueMinor int64
	// FromMinor is the level the move started from. Meaningful for a reversal
	// (the high it fell from); zero otherwise.
	FromMinor int64
	// Up is false when the event is a setback. A setback is still a fact about
	// the window, and hiding it would make the list a trophy cabinet rather
	// than a record — so the sign changes are reported as a PAIR: a series that
	// reports turning positive must also report turning negative.
	Up bool
}

// The kinds of event the milestone list reports.
const (
	// MilestoneKindPositive is net worth crossing zero upward.
	MilestoneKindPositive = "positive"
	// MilestoneKindNegative is net worth crossing zero downward. It exists so
	// that a recovery can never be narrated without the fall that preceded it.
	MilestoneKindNegative = "negative"
	// MilestoneKindThreshold is a round figure passed, in either direction.
	MilestoneKindThreshold = "threshold"
	// MilestoneKindHigh is the highest net worth in the whole series.
	MilestoneKindHigh = "high"
	// MilestoneKindReversal is a material fall from a running high.
	MilestoneKindReversal = "reversal"
)

// reversalPctOfPeak is how far net worth must fall from a running high before
// the fall is an event rather than noise. A tenth of everything you have is a
// figure a household notices; a 2% wobble is not a milestone.
const reversalPctOfPeak = 10

// Milestones reports what actually happened to net worth across the series, in
// the order it happened: the first time it turned positive (and every time it
// turned negative), the round figures it passed, its all-time high, and every
// material fall from a high.
//
// The thresholds SCALE WITH MAGNITUDE. This is the whole difference between a
// milestone list and an event log. A fixed ladder applied across a range from
// -$16,000 to $154,000 fires on $500 steps down near zero, where $500 means
// nothing, and produces five rows for one afternoon — "turned positive",
// "passed $0", "passed $500" — while saying nothing a person would repeat out
// loud. So the ladder is derived from the series' own peak: round figures at 1,
// 2.5 and 5 times each of the top two decades of that peak, which for a
// household topping out near $154,000 is $10k, $25k, $50k and $100k.
//
// Negative thresholds are never reported. Climbing from -$1,600 to $700 is ONE
// story — getting out of debt — and it is already told, exactly once, by the
// first-positive milestone. "Passed -$1,000" is not an achievement, and a list
// that says so is not one a reader will trust.
func Milestones(pts []Point) []Milestone {
	if len(pts) < 2 {
		return nil
	}
	var peak int64
	for _, p := range pts {
		if p.NetMinor > peak {
			peak = p.NetMinor
		}
	}
	ladder := milestoneLadder(peak)

	var out []Milestone
	for i := 1; i < len(pts); i++ {
		prev, cur := pts[i-1].NetMinor, pts[i].NetMinor
		switch {
		case prev < 0 && cur >= 0:
			out = append(out, Milestone{Kind: MilestoneKindPositive, AtIndex: i, Up: true})
		case prev >= 0 && cur < 0:
			out = append(out, Milestone{Kind: MilestoneKindNegative, AtIndex: i})
		}
		lo, hi, up := prev, cur, true
		if lo > hi {
			lo, hi, up = hi, lo, false
		}
		for _, t := range ladder {
			if t > lo && t <= hi {
				out = append(out, Milestone{Kind: MilestoneKindThreshold, AtIndex: i, ValueMinor: t, Up: up})
			}
		}
	}
	out = append(out, milestoneReversals(pts)...)
	if hi, at := seriesPeak(pts); at > 0 && hi > 0 {
		out = append(out, Milestone{Kind: MilestoneKindHigh, AtIndex: at, ValueMinor: hi, Up: true})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AtIndex < out[j].AtIndex })
	return out
}

// milestoneLadder is the set of round figures worth reporting for a series that
// peaked at peakMinor: 1, 2.5 and 5 times each of the top two decades. Two
// decades is what keeps the list proportionate — one decade alone would skip
// the $10,000 a household remembers on its way to $150,000, and three would
// drag $1,000 steps back into a six-figure story.
//
// Nothing at or below zero is ever a rung.
func milestoneLadder(peakMinor int64) []int64 {
	if peakMinor <= 0 {
		return nil
	}
	top := math.Floor(math.Log10(float64(peakMinor)))
	var out []int64
	for e := top - 1; e <= top; e++ {
		if e < 0 {
			continue
		}
		mag := math.Pow(10, e)
		for _, m := range []float64{1, 2.5, 5} {
			v := int64(math.Round(mag * m))
			if v > 0 && v <= peakMinor && !containsInt64(out, v) {
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// seriesPeak returns the highest net worth in the series and where it happened.
// A peak at the very first point is not an event that occurred inside the
// window, so it reports index 0 and the caller drops it.
func seriesPeak(pts []Point) (int64, int) {
	hi, at := pts[0].NetMinor, 0
	for i, p := range pts {
		if p.NetMinor > hi {
			hi, at = p.NetMinor, i
		}
	}
	return hi, at
}

// milestoneReversals finds each episode where net worth fell materially from a
// running high, and reports it once, at the bottom of the fall. Reporting every
// down month would restore the event log; reporting only the recoveries would
// restore the trophy cabinet. The trough is the honest single row: this is how
// far it fell, and from what.
func milestoneReversals(pts []Point) []Milestone {
	var out []Milestone
	peak, troughIdx := pts[0].NetMinor, -1
	trough := peak
	flush := func() {
		if troughIdx >= 0 && peak > 0 && (peak-trough)*100 >= peak*reversalPctOfPeak {
			out = append(out, Milestone{
				Kind: MilestoneKindReversal, AtIndex: troughIdx,
				ValueMinor: trough, FromMinor: peak,
			})
		}
		troughIdx = -1
	}
	for i := 1; i < len(pts); i++ {
		v := pts[i].NetMinor
		if v > peak {
			flush()
			peak, trough = v, v
			continue
		}
		if troughIdx < 0 || v < trough {
			trough, troughIdx = v, i
		}
	}
	flush()
	return out
}

// PaceRung is one round figure reached, and how long the climb to it took.
type PaceRung struct {
	// ValueMinor is the figure reached; AtIndex is where in the series.
	ValueMinor int64
	AtIndex    int
	// Months is the time since the previous rung — the number the whole
	// graphic exists to show. Zero on the first rung, which had no previous.
	Months int
}

// PaceNext is the extrapolation to the next round figure. It is a projection
// from recent pace and nothing more, which is why it carries its own OK and
// Stalled flags instead of always producing a date: a household whose recent
// trend is flat or falling has no honest arrival month, and inventing one would
// be the page's first promise.
type PaceNext struct {
	ValueMinor int64
	Months     int
	// OK is true only when a figure and a month count can both be stated.
	OK bool
	// Stalled is true when the recent trend is flat or downward, so the answer
	// is "not at this pace" rather than a date.
	Stalled bool
}

// Pace is the reading of the net-worth series as PROGRESS rather than as a list
// of events: which round figures were reached, how long each leg took, and what
// the next one would be at the recent rate.
//
// The insight a household wants is not "passed $50,000 in April 2024" — that is
// a receipt. It is that the last leg took eight months and the one before it
// took fourteen. So the legs, not the events, are the content.
type Pace struct {
	// Rungs are the figures reached, in order.
	Rungs []PaceRung
	// Next is the extrapolation past the last rung.
	Next PaceNext
	// Marks are everything worth marking on the chart, including the setbacks
	// that are not rungs — the record stays truthful about falls even though a
	// fall is not progress.
	Marks []Milestone
}

// paceRecentMonths is how much history the projection is drawn from. A year is
// long enough not to be moved by one unusual month and short enough to describe
// what the household is doing NOW rather than what it did in 2022.
const paceRecentMonths = 12

// paceMaxProjection caps how far ahead a projection will speak. Beyond a few
// years the extrapolation says more about the arithmetic than about the
// household, so it declines to answer instead.
const paceMaxProjection = 60

// BuildPace reads the series as progress. pts must be in cutoff order.
func BuildPace(pts []Point) Pace {
	var p Pace
	if len(pts) < 2 {
		return p
	}
	p.Marks = Milestones(pts)

	// Only levels REACHED are rungs: falling back below one is a setback, which
	// the marks already carry. Turning positive counts — it is the rung every
	// later one is measured from, and without it the rail would begin partway
	// up a climb with no account of how the household got there.
	var prev *PaceRung
	for _, m := range p.Marks {
		if m.Kind == MilestoneKindPositive && len(p.Rungs) == 0 {
			p.Rungs = append(p.Rungs, PaceRung{ValueMinor: 0, AtIndex: m.AtIndex})
			prev = &p.Rungs[0]
			continue
		}
		if m.Kind != MilestoneKindThreshold || !m.Up {
			continue
		}
		r := PaceRung{ValueMinor: m.ValueMinor, AtIndex: m.AtIndex}
		if prev != nil {
			r.Months = monthsBetween(pts[prev.AtIndex].At, pts[m.AtIndex].At)
		}
		p.Rungs = append(p.Rungs, r)
		prev = &p.Rungs[len(p.Rungs)-1]
	}
	p.Next = projectNext(pts)
	return p
}

// projectNext extrapolates the next rung from recent pace.
func projectNext(pts []Point) PaceNext {
	last := len(pts) - 1
	now := pts[last].NetMinor
	if now <= 0 {
		return PaceNext{}
	}
	// Recent pace: the change over the last year of the series, per month.
	from := 0
	for i := last; i >= 0; i-- {
		if monthsBetween(pts[i].At, pts[last].At) <= paceRecentMonths {
			from = i
			continue
		}
		break
	}
	months := monthsBetween(pts[from].At, pts[last].At)
	if months <= 0 {
		return PaceNext{}
	}
	perMonth := (now - pts[from].NetMinor) / int64(months)
	if perMonth <= 0 {
		return PaceNext{Stalled: true}
	}

	var peak int64
	for _, p := range pts {
		if p.NetMinor > peak {
			peak = p.NetMinor
		}
	}
	target := nextRungAbove(now, peak)
	if target <= 0 {
		return PaceNext{}
	}
	away := int((target-now+perMonth-1)/perMonth) // round up: the month it lands in
	if away > paceMaxProjection {
		return PaceNext{}
	}
	return PaceNext{ValueMinor: target, Months: away, OK: true}
}

// nextRungAbove is the first ladder rung strictly above now. The ladder is the
// milestone ladder, extended one decade so a household already standing at its
// own peak still has somewhere to be heading.
func nextRungAbove(now, peak int64) int64 {
	ref := peak
	if now > ref {
		ref = now
	}
	for _, v := range milestoneLadder(ref * 10) {
		if v > now {
			return v
		}
	}
	return 0
}

// monthsBetween counts whole calendar months from a to b, never negative.
func monthsBetween(a, b time.Time) int {
	m := (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if m < 0 {
		return 0
	}
	return m
}

func containsInt64(s []int64, v int64) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
