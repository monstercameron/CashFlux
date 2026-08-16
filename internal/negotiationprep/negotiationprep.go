// SPDX-License-Identifier: MIT

// Package negotiationprep assembles what a household actually has to bargain with
// before they call a provider (AG16).
//
// Rocket Money's pitch is that they will negotiate your bills for you, which means
// handing a stranger your account access and a cut of the savings. The honest
// local version is different and, for most bills, better: the leverage in these
// calls is almost entirely facts about YOU — how long you have paid, what you paid
// before the rise, how much of a rise it was — and the household already has all
// of it. What they lack is those facts arranged, at the moment of the call, in an
// order that survives being read out loud while nervous.
//
// So this builds the prep sheet: the leverage, in the order it should be used, and
// a script that sounds like a person rather than a form letter. It states plainly
// where a claim is NOT supported by the data instead of writing a confident
// sentence around a gap, because being caught overstating on that call costs the
// whole conversation.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package negotiationprep

import (
	"fmt"
	"strings"
	"time"
)

// Charge is one payment to this provider.
type Charge struct {
	At          time.Time
	AmountMinor int64
}

// Subscription is the recurring cost being prepared for.
type Subscription struct {
	// Name is the provider as the household knows it.
	Name string
	// Charges are every payment seen, in any order.
	Charges []Charge
	// MonthlyMinor is the current cost per month.
	MonthlyMinor int64
	// CompetitorNote is an optional externally-sourced comparison ("similar plans
	// run $8–12"). It is passed in rather than derived because the app has no way
	// to know it, and inventing one would be the single most damaging thing this
	// package could do — a made-up competitor price is a claim the provider can
	// disprove in one sentence.
	CompetitorNote string
}

// Leverage is one thing worth saying on the call.
type Leverage struct {
	// Point is the fact, in the household's own terms.
	Point string
	// Strong marks the points that carry a call. Tenure and a documented price
	// rise are strong; a bare monthly figure is not.
	Strong bool
}

// Prep is the whole sheet.
type Prep struct {
	Name string
	// Leverage is what to lead with, strongest first.
	Leverage []Leverage
	// Script is what to say, in order.
	Script []string
	// Gaps names what the data could NOT support, so nothing on the sheet is
	// asserted beyond it.
	Gaps []string
	// TenureMonths is how long they have been paying.
	TenureMonths int
	// RiseMinor is how much the price has gone up since the lowest charge seen,
	// and RisePercent is that as a percentage. Zero when it has not risen.
	RiseMinor   int64
	RisePercent int
	// AnnualMinor is what this costs per year — the figure that makes a small
	// monthly number worth ten minutes on the phone.
	AnnualMinor int64
}

// Worthwhile reports whether the call is likely to be worth making at all. A
// subscription with no tenure, no rise and no comparison gives the household
// nothing to say, and sending somebody into that call is worse than not
// suggesting it.
func (p Prep) Worthwhile() bool {
	for _, l := range p.Leverage {
		if l.Strong {
			return true
		}
	}
	return false
}

// minTenureMonths is how long a household must have paid before tenure is worth
// raising. Below half a year "I've been a customer a while" invites the answer
// "you've been with us four months".
const minTenureMonths = 6

// Build assembles the prep sheet. fmtMoney formats a minor-unit figure; it comes
// from the caller so this package stays free of currency handling.
func Build(s Subscription, now time.Time, fmtMoney func(int64) string) Prep {
	p := Prep{Name: strings.TrimSpace(s.Name), AnnualMinor: s.MonthlyMinor * 12}
	if p.Name == "" {
		p.Name = "this subscription"
	}

	first, lowest, ok := firstAndLowest(s.Charges)
	if ok {
		p.TenureMonths = wholeMonths(first, now)
		if s.MonthlyMinor > lowest {
			p.RiseMinor = s.MonthlyMinor - lowest
			p.RisePercent = int(p.RiseMinor * 100 / lowest)
		}
	}

	// Strongest first: a documented price rise is the fact a provider is most
	// often willing to move on, because it is theirs.
	if p.RiseMinor > 0 {
		p.Leverage = append(p.Leverage, Leverage{Strong: true, Point: fmt.Sprintf(
			"The price went up %s a month (%d%%) — you were paying %s.",
			fmtMoney(p.RiseMinor), p.RisePercent, fmtMoney(lowest))})
	}
	if p.TenureMonths >= minTenureMonths {
		p.Leverage = append(p.Leverage, Leverage{Strong: true, Point: fmt.Sprintf(
			"You've paid them for %s.", humanMonths(p.TenureMonths))})
	} else if ok {
		p.Gaps = append(p.Gaps, fmt.Sprintf(
			"You've only been with them %s, so don't lead with loyalty.", humanMonths(p.TenureMonths)))
	}
	if note := strings.TrimSpace(s.CompetitorNote); note != "" {
		p.Leverage = append(p.Leverage, Leverage{Strong: true, Point: note})
	} else {
		p.Gaps = append(p.Gaps, "No competitor pricing was looked up, so don't claim one.")
	}
	if s.MonthlyMinor > 0 {
		p.Leverage = append(p.Leverage, Leverage{Point: fmt.Sprintf(
			"It costs you %s a year.", fmtMoney(p.AnnualMinor))})
	}
	if !ok {
		p.Gaps = append(p.Gaps, "No past charges were found, so there's no price history to point at.")
	}

	p.Script = buildScript(p, s, fmtMoney)
	return p
}

// buildScript writes what to say. It is short on purpose: a script somebody has to
// scroll is a script they abandon and improvise from, which is exactly the state
// this is meant to replace.
func buildScript(p Prep, s Subscription, fmtMoney func(int64) string) []string {
	script := []string{
		fmt.Sprintf("Hi — I'm calling about my %s account. I've been looking at what I'm paying and I'd like to see what can be done about the price.", p.Name),
	}
	if p.RiseMinor > 0 {
		script = append(script, fmt.Sprintf(
			"It's gone up to %s a month — I was paying %s. That's %s a year now.",
			fmtMoney(s.MonthlyMinor), fmtMoney(s.MonthlyMinor-p.RiseMinor), fmtMoney(p.AnnualMinor)))
	} else if s.MonthlyMinor > 0 {
		script = append(script, fmt.Sprintf(
			"I'm paying %s a month, which is %s a year.", fmtMoney(s.MonthlyMinor), fmtMoney(p.AnnualMinor)))
	}
	if p.TenureMonths >= minTenureMonths {
		script = append(script, fmt.Sprintf("I've been with you %s.", humanMonths(p.TenureMonths)))
	}
	if note := strings.TrimSpace(s.CompetitorNote); note != "" {
		script = append(script, note)
	}
	// The ask, then the silence. The pause is the actual technique — the first
	// person to fill it usually concedes — so it is written down as a step rather
	// than left to nerve.
	script = append(script,
		"Is there a better rate, a promotion, or a cheaper plan that would still cover what I use?",
		"(Then stop talking and let them answer.)",
		"If they say no: \"I understand. In that case I'd like to cancel — could you put me through?\" Retention teams can usually offer more than the first line can.",
	)
	return script
}

// firstAndLowest returns the earliest charge date and the smallest amount seen.
func firstAndLowest(charges []Charge) (first time.Time, lowest int64, ok bool) {
	for _, c := range charges {
		amount := c.AmountMinor
		if amount < 0 {
			amount = -amount
		}
		if amount == 0 {
			continue
		}
		if !ok || c.At.Before(first) {
			first = c.At
		}
		if !ok || amount < lowest {
			lowest = amount
		}
		ok = true
	}
	return first, lowest, ok
}

// wholeMonths counts whole months between two times, never negative.
func wholeMonths(from, to time.Time) int {
	if !to.After(from) {
		return 0
	}
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}

// humanMonths renders a month count the way somebody would say it on the phone —
// "two years", not "24 months".
func humanMonths(months int) string {
	switch {
	case months <= 1:
		return "a month"
	case months < 12:
		return fmt.Sprintf("%d months", months)
	case months < 24:
		return "over a year"
	default:
		return fmt.Sprintf("over %d years", months/12)
	}
}

// TaskNotes renders the prep sheet as the body of a to-do, so the call can be
// filed for later with everything needed already attached. A task that says only
// "call the provider" gets postponed indefinitely; one that already contains the
// script gets made.
func (p Prep) TaskNotes() string {
	var b strings.Builder
	if len(p.Leverage) > 0 {
		b.WriteString("What you've got:\n")
		for _, l := range p.Leverage {
			b.WriteString("• " + l.Point + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("What to say:\n")
	for _, line := range p.Script {
		b.WriteString("• " + line + "\n")
	}
	if len(p.Gaps) > 0 {
		b.WriteString("\nDon't claim:\n")
		for _, g := range p.Gaps {
			b.WriteString("• " + g + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
