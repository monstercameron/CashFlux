// SPDX-License-Identifier: MIT

package duedate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ref is the reference clock for every table below: Wednesday 12 August 2026.
var ref = time.Date(2026, time.August, 12, 14, 30, 0, 0, time.UTC)

// day builds an expected date in ref's location.
func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantTitle   string
		wantDue     time.Time
		wantCadence domain.RecurringCadence
	}{
		{
			name:      "the ticket's own example",
			in:        "pay rent friday",
			wantTitle: "pay rent",
			wantDue:   day(2026, time.August, 14),
		},
		{
			name:      "a dangling preposition goes with the date",
			in:        "pay rent on friday",
			wantTitle: "pay rent",
			wantDue:   day(2026, time.August, 14),
		},
		{
			name:      "today",
			in:        "call the bank today",
			wantTitle: "call the bank",
			wantDue:   day(2026, time.August, 12),
		},
		{
			name:      "tomorrow",
			in:        "move 200 to savings tomorrow",
			wantTitle: "move 200 to savings",
			wantDue:   day(2026, time.August, 13),
		},
		{
			name:      "in N days",
			in:        "chase the refund in 3 days",
			wantTitle: "chase the refund",
			wantDue:   day(2026, time.August, 15),
		},
		{
			name:      "in a week",
			in:        "review budgets in a week",
			wantTitle: "review budgets",
			wantDue:   day(2026, time.August, 19),
		},
		{
			name:      "next week",
			in:        "cancel the trial next week",
			wantTitle: "cancel the trial",
			wantDue:   day(2026, time.August, 19),
		},
		{
			name:      "next month",
			in:        "renegotiate insurance next month",
			wantTitle: "renegotiate insurance",
			wantDue:   day(2026, time.September, 12),
		},
		{
			name:      "next weekday skips the coming one",
			in:        "dentist next friday",
			wantTitle: "dentist",
			wantDue:   day(2026, time.August, 21),
		},
		{
			name:      "this weekday is the coming one",
			in:        "groceries this saturday",
			wantTitle: "groceries",
			wantDue:   day(2026, time.August, 15),
		},
		{
			name:      "a weekday that is today means today",
			in:        "file the receipts wednesday",
			wantTitle: "file the receipts",
			wantDue:   day(2026, time.August, 12),
		},
		{
			name:      "short weekday forms",
			in:        "gym mon",
			wantTitle: "gym",
			wantDue:   day(2026, time.August, 17),
		},
		{
			name:      "month and day",
			in:        "file taxes april 15",
			wantTitle: "file taxes",
			wantDue:   day(2027, time.April, 15), // already past this year → next
		},
		{
			name:      "month and ordinal day still ahead this year",
			in:        "renew passport dec 1st",
			wantTitle: "renew passport",
			wantDue:   day(2026, time.December, 1),
		},
		{
			name:      "reversed day and month",
			in:        "book flights 5 sept",
			wantTitle: "book flights",
			wantDue:   day(2026, time.September, 5),
		},
		{
			name:      "ISO date",
			in:        "close the account 2026-11-30",
			wantTitle: "close the account",
			wantDue:   day(2026, time.November, 30),
		},
		{
			name:      "slash date rolls to next year when past",
			in:        "audit fees 3/1",
			wantTitle: "audit fees",
			wantDue:   day(2027, time.March, 1),
		},
		{
			name:      "slash date with an explicit year",
			in:        "audit fees 3/1/2026",
			wantTitle: "audit fees",
			wantDue:   day(2026, time.March, 1),
		},
		{
			name:      "bare ordinal day of month",
			in:        "pay the card on the 20th",
			wantTitle: "pay the card",
			wantDue:   day(2026, time.August, 20),
		},
		{
			name:      "a past ordinal rolls to next month",
			in:        "pay the card on the 3rd",
			wantTitle: "pay the card",
			wantDue:   day(2026, time.September, 3),
		},
		{
			name:        "a bare cadence word",
			in:          "check the ledger weekly",
			wantTitle:   "check the ledger",
			wantCadence: domain.CadenceWeekly,
			wantDue:     day(2026, time.August, 19),
		},
		{
			name:        "every month with a start day",
			in:          "review subscriptions every month on the 1st",
			wantTitle:   "review subscriptions",
			wantCadence: domain.CadenceMonthly,
			wantDue:     day(2026, time.September, 1),
		},
		{
			name:        "every weekday is weekly and dates the first one",
			in:          "take out the trash every tuesday",
			wantTitle:   "take out the trash",
			wantCadence: domain.CadenceWeekly,
			wantDue:     day(2026, time.August, 18),
		},
		{
			name:        "every N units",
			in:          "rotate passwords every 3 months",
			wantTitle:   "rotate passwords",
			wantCadence: domain.CadenceMonthly,
			wantDue:     day(2026, time.September, 12),
		},
		{
			name:        "every other week is a fortnight",
			in:          "payday transfer every other week",
			wantTitle:   "payday transfer",
			wantCadence: domain.CadenceBiweekly,
			wantDue:     day(2026, time.August, 26),
		},
		{
			name:      "no date leaves the sentence alone",
			in:        "think about the emergency fund",
			wantTitle: "think about the emergency fund",
		},
		{
			name:      "a bare number is not a day of the month",
			in:        "transfer 15 to savings",
			wantTitle: "transfer 15 to savings",
		},
		{
			name:      "empty input",
			in:        "   ",
			wantTitle: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.in, ref)
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if !got.Due.Equal(tc.wantDue) {
				t.Errorf("Due = %v, want %v", got.Due, tc.wantDue)
			}
			if got.Cadence != tc.wantCadence {
				t.Errorf("Cadence = %q, want %q", got.Cadence, tc.wantCadence)
			}
		})
	}
}

// A sentence that is nothing but a date must still yield a usable title rather
// than an empty task the caller has to reject.
func TestParse_DateOnlyKeepsTheWords(t *testing.T) {
	got := Parse("tomorrow", ref)
	if got.Title != "tomorrow" {
		t.Errorf("Title = %q, want %q", got.Title, "tomorrow")
	}
	if !got.Due.Equal(day(2026, time.August, 13)) {
		t.Errorf("Due = %v", got.Due)
	}
}

// Matched is the feature's receipt: the UI shows what was read instead of asking
// the user to trust it.
func TestParse_Matched(t *testing.T) {
	tests := []struct{ in, want string }{
		{"pay rent on friday", "on friday"},
		{"chase the refund in 3 days", "in 3 days"},
		{"review subscriptions every month on the 1st", "every month on the 1st"},
		{"think about the emergency fund", ""},
	}
	for _, tc := range tests {
		if got := Parse(tc.in, ref).Matched; got != tc.want {
			t.Errorf("Parse(%q).Matched = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDraft_Found(t *testing.T) {
	if Parse("think about it", ref).Found() {
		t.Error("a sentence with no date or repeat should report nothing found")
	}
	if !Parse("pay rent friday", ref).Found() {
		t.Error("a resolved date should report found")
	}
	if !Parse("check in weekly", ref).Found() {
		t.Error("a cadence should report found")
	}
}

// Punctuation stuck to a token must not stop it being recognized.
func TestParse_TrailingPunctuation(t *testing.T) {
	got := Parse("pay rent, friday.", ref)
	if !got.Due.Equal(day(2026, time.August, 14)) {
		t.Errorf("Due = %v, want 14 Aug", got.Due)
	}
	if got.Title != "pay rent" {
		t.Errorf("Title = %q, want %q", got.Title, "pay rent")
	}
}

// The ordinal roll-forward must clamp rather than skip a month: the 31st of a
// month whose successor is shorter lands on that successor's last day.
func TestAddMonth_ClampsShortMonths(t *testing.T) {
	got := addMonth(day(2026, time.January, 31))
	want := day(2026, time.February, 28)
	if !got.Equal(want) {
		t.Errorf("addMonth(31 Jan) = %v, want %v", got, want)
	}
}

func TestOrdinal(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"5", 5, true}, {"5th", 5, true}, {"1st", 1, true}, {"22nd", 22, true},
		{"3rd", 3, true}, {"31st", 31, true},
		{"", 0, false}, {"0", 0, false}, {"32", 0, false}, {"abc", 0, false},
		{"5x", 0, false},
	}
	for _, tc := range tests {
		got, ok := ordinal(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ordinal(%q) = %d, %v; want %d, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// The parse must not depend on the wall clock's time of day.
func TestParse_IgnoresClockTime(t *testing.T) {
	morning := time.Date(2026, time.August, 12, 0, 1, 0, 0, time.UTC)
	night := time.Date(2026, time.August, 12, 23, 59, 0, 0, time.UTC)
	if !Parse("pay rent friday", morning).Due.Equal(Parse("pay rent friday", night).Due) {
		t.Error("the same day at different clock times must resolve identically")
	}
}
