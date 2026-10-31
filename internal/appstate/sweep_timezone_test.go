// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/sweep"
)

// sweepCadenceCases lists the four real cadences with the number of days each spans.
var sweepCadenceCases = []struct {
	cadence domain.SweepCadence
	days    int
}{
	{domain.SweepWeekly, 7},
	{domain.SweepBiweekly, 14},
	{domain.SweepMonthly, 30},
	{domain.SweepQuarterly, 90},
}

// TestSweepWindowAgreesWithDueInNonUTCZones posts a sweep of each cadence in zones west and east of
// UTC at several offsets into the cadence window and times of day, then walks forward day by day. A
// replay on the day it posted is refused as already posted, the rule is not due on any of the next
// cadence-1 local days, and on the cadence's last day it is due, gets a fresh window and posts. An
// approval on a not-due day must fail with ErrSweepAlreadyPosted, because the rule's stamp names a
// sweep that posted, and must move no money.
func TestSweepWindowAgreesWithDueInNonUTCZones(t *testing.T) {
	for _, cc := range sweepCadenceCases {
		t.Run(string(cc.cadence), func(t *testing.T) { sweepWindowWalk(t, cc.cadence, cc.days) })
	}
}

func sweepWindowWalk(t *testing.T, cadence domain.SweepCadence, cadenceDays int) {
	zones := map[string]*time.Location{
		"Los Angeles summer": time.FixedZone("PDT", -7*3600),
		"Los Angeles winter": time.FixedZone("PST", -8*3600),
		"Kiritimati":         time.FixedZone("LINT", 14*3600),
		"UTC":                time.UTC,
	}
	next := sweep.Proposal{RuleID: "r1", SourceAccountID: "chk", DestAccountID: "sav", AmountMinor: 200000}
	for name, loc := range zones {
		t.Run(name, func(t *testing.T) {
			for startDay := 0; startDay < cadenceDays; startDay += max(1, cadenceDays/10) {
				for _, hour := range []int{0, 7, 17, 23} {
					first := time.Date(2026, 3, 1+startDay, hour, 30, 0, 0, loc)
					app := sweepFixture(t)
					if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Cadence: cadence, Enabled: true}); err != nil {
						t.Fatal(err)
					}
					props := app.SweepProposals(first)
					if len(props) != 1 {
						t.Fatalf("%v: proposals = %+v, want one", first, props)
					}
					if err := app.ApproveSweepProposal(props[0], "Sweep", first); err != nil {
						t.Fatalf("%v: first approval: %v", first, err)
					}
					if err := app.ApproveSweepProposal(props[0], "Sweep", first.Add(time.Minute)); !errors.Is(err, ErrSweepAlreadyPosted) {
						t.Fatalf("%v: same-day replay = %v, want ErrSweepAlreadyPosted", first, err)
					}

					// Top the source up so the next sweep has a surplus again.
					if err := app.PutTransaction(domain.Transaction{ID: "topup", AccountID: "chk", Desc: "Pay", Amount: money.New(200000, "USD"), Date: first}); err != nil {
						t.Fatal(err)
					}
					legs := len(app.Transactions())
					for day := 1; day < cadenceDays; day++ {
						at := first.AddDate(0, 0, day)
						if n := len(app.SweepProposals(at)); n != 0 {
							t.Fatalf("first %v, day +%d: %d proposals, want none before the cadence has elapsed", first, day, n)
						}
						if err := app.ApproveSweepProposal(next, "Sweep", at); !errors.Is(err, ErrSweepAlreadyPosted) {
							t.Fatalf("first %v, day +%d: not-due approval = %v, want ErrSweepAlreadyPosted", first, day, err)
						}
						if n := len(app.Transactions()); n != legs {
							t.Fatalf("first %v, day +%d: %d transactions, want %d: a not-due approval moved money", first, day, n, legs)
						}
					}
					at := first.AddDate(0, 0, cadenceDays)
					if n := len(app.SweepProposals(at)); n != 1 {
						t.Fatalf("first %v, day +%d: %d proposals, want one", first, cadenceDays, n)
					}
					if err := app.ApproveSweepProposal(next, "Sweep", at); err != nil {
						t.Fatalf("first %v, day +%d: the rule is due but approval = %v", first, cadenceDays, err)
					}
					if err := app.ApproveSweepProposal(next, "Sweep", at.Add(time.Minute)); !errors.Is(err, ErrSweepAlreadyPosted) {
						t.Fatalf("first %v, day +%d: replay = %v, want ErrSweepAlreadyPosted", first, cadenceDays, err)
					}
				}
			}
		})
	}
}

// TestSweepOperationIDFollowsTheLocalCalendarDay checks that instants on one local calendar day share
// a window, even across midnight UTC, and that the day sweep.IsDue reports a posted rule due again
// falls in a later window than the sweep, for each cadence.
func TestSweepOperationIDFollowsTheLocalCalendarDay(t *testing.T) {
	for _, cc := range sweepCadenceCases {
		t.Run(string(cc.cadence), func(t *testing.T) { sweepOperationIDWalk(t, cc.cadence, cc.days) })
	}
}

func sweepOperationIDWalk(t *testing.T, cadence domain.SweepCadence, cadenceDays int) {
	r := domain.SweepRule{ID: "r1", Cadence: cadence}
	la := time.FixedZone("PDT", -7*3600)
	evening := time.Date(2026, 10, 30, 20, 0, 0, 0, la)
	morning := time.Date(2026, 10, 30, 8, 0, 0, 0, la)
	if sweepOperationID(r, evening) != sweepOperationID(r, morning) {
		t.Fatal("two instants on one local day resolved to different windows")
	}

	// A rule that posted on a local day is due again on the 30th day after it, which sweep.IsDue
	// decides on its own. For every posting day the first day IsDue reports the rule due must fall
	// in a different window than the sweep, whatever the zone or time of day.
	r.Enabled = true
	for _, loc := range []*time.Location{la, time.FixedZone("LINT", 14*3600), time.UTC} {
		for offset := 0; offset < 2*cadenceDays; offset++ {
			posted := time.Date(2026, 10, 1+offset, 20, 0, 0, 0, loc)
			stamped := r
			stamped.LastProposed = normalizeTransactionDate(posted).Format(time.RFC3339)
			dueDay := -1
			for ahead := 1; ahead <= 2*cadenceDays; ahead++ {
				if sweep.IsDue(stamped, time.Date(2026, 10, 1+offset+ahead, 8, 0, 0, 0, loc)) {
					dueDay = ahead
					break
				}
			}
			if dueDay != cadenceDays {
				t.Fatalf("%v: IsDue first reports the rule due on day +%d, want +%d", posted, dueDay, cadenceDays)
			}
			due := time.Date(2026, 10, 1+offset+dueDay, 0, 5, 0, 0, loc)
			if sweepOperationID(r, due) == sweepOperationID(r, posted) {
				t.Fatalf("%v: the day sweep.IsDue says the rule is due shares a window with the sweep that posted", posted)
			}
			// Late evenings west of UTC and early mornings east of it sit on the other side of midnight
			// UTC from local midday, yet stay in midday's window.
			for _, hour := range []int{0, 8, 20, 23} {
				at := time.Date(2026, 10, 1+offset, hour, 5, 0, 0, loc)
				noon := time.Date(2026, 10, 1+offset, 12, 0, 0, 0, loc)
				if sweepOperationID(r, at) != sweepOperationID(r, noon) {
					t.Fatalf("%v: an instant on the same local day as %v was assigned a different window", at, noon)
				}
			}
		}
	}
}

// sweepPostedFixture returns an app whose monthly rule r1 is approved once at the given instant, plus
// the proposal that was approved.
func sweepPostedFixture(t *testing.T, at time.Time) (*App, sweep.Proposal) {
	t.Helper()
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Cadence: domain.SweepMonthly, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(at)
	if len(props) != 1 {
		t.Fatalf("%v: proposals = %+v, want one", at, props)
	}
	if err := app.ApproveSweepProposal(props[0], "Sweep", at); err != nil {
		t.Fatalf("%v: first approval: %v", at, err)
	}
	return app, props[0]
}

// TestSweepReplayAcrossLocalMidnightIsAlreadyPosted posts a sweep at 23:50 on the last local day of
// its cadence window and replays it at 00:10 the next day, which names a new window in which the rule
// is not due. The money already moved, so the replay is ErrSweepAlreadyPosted and moves nothing.
func TestSweepReplayAcrossLocalMidnightIsAlreadyPosted(t *testing.T) {
	r := domain.SweepRule{ID: "r1", Cadence: domain.SweepMonthly}
	for name, loc := range map[string]*time.Location{
		"UTC-7":  time.FixedZone("PDT", -7*3600),
		"UTC+14": time.FixedZone("LINT", 14*3600),
		"UTC":    time.UTC,
	} {
		t.Run(name, func(t *testing.T) {
			lastDay := -1
			for day := 0; day < 40; day++ {
				noon := time.Date(2026, 3, 1+day, 12, 0, 0, 0, loc)
				if sweepOperationID(r, noon) != sweepOperationID(r, noon.AddDate(0, 0, 1)) {
					lastDay = day
					break
				}
			}
			if lastDay < 0 {
				t.Fatal("test setup: no window boundary in 40 days")
			}
			post := time.Date(2026, 3, 1+lastDay, 23, 50, 0, 0, loc)
			replay := time.Date(2026, 3, 2+lastDay, 0, 10, 0, 0, loc)
			app, prop := sweepPostedFixture(t, post)
			before, _, _ := app.Store().GetSweepRule("r1")
			legs := len(app.Transactions())

			if err := app.ApproveSweepProposal(prop, "Sweep", replay); !errors.Is(err, ErrSweepAlreadyPosted) {
				t.Fatalf("replay at %v = %v, want ErrSweepAlreadyPosted", replay, err)
			}
			after, _, _ := app.Store().GetSweepRule("r1")
			if after.LastProposed != before.LastProposed {
				t.Fatalf("LastProposed = %q after the replay, want %q", after.LastProposed, before.LastProposed)
			}
			if n := len(app.Transactions()); n != legs {
				t.Fatalf("%d transactions after the replay, want %d", n, legs)
			}
		})
	}
}

// TestSweepNotDueWithNothingPostedIsStale checks that a rule whose stamp names no posted sweep and is
// not due again refuses a proposal as out of date, not as already posted.
func TestSweepNotDueWithNothingPostedIsStale(t *testing.T) {
	loc := time.FixedZone("PDT", -7*3600)
	first := time.Date(2026, 3, 10, 9, 0, 0, 0, loc)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Cadence: domain.SweepMonthly, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(first)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if err := app.MarkSweepProposed("r1", first); err != nil {
		t.Fatal(err)
	}
	err := app.ApproveSweepProposal(props[0], "Sweep", first.AddDate(0, 0, 1))
	if !errors.Is(err, ErrSweepProposalStale) || errors.Is(err, ErrSweepAlreadyPosted) {
		t.Fatalf("approval = %v, want ErrSweepProposalStale", err)
	}
	if n := len(app.Transactions()); n != 0 {
		t.Fatalf("%d transactions, want none", n)
	}
}

// TestSweepReplayStampOnTheTransferDayIsNotRepaired checks the repair path compares calendar days.
// In UTC+14 a stamp for local day Mar 2 01:00 is Mar 1 11:00 UTC, an instant before the transfer's
// UTC-midnight date of Mar 2, yet it is the same calendar day, so the replay changes nothing.
func TestSweepReplayStampOnTheTransferDayIsNotRepaired(t *testing.T) {
	loc := time.FixedZone("LINT", 14*3600)
	first := time.Date(2026, 3, 2, 1, 0, 0, 0, loc)
	app, prop := sweepPostedFixture(t, first)
	if err := app.MarkSweepProposed("r1", first); err != nil {
		t.Fatal(err)
	}
	before, _, _ := app.Store().GetSweepRule("r1")
	if want := "2026-03-02T01:00:00+14:00"; before.LastProposed != want {
		t.Fatalf("test setup: stamp = %q, want %q", before.LastProposed, want)
	}
	legs := len(app.Transactions())

	if err := app.ApproveSweepProposal(prop, "Sweep", first.Add(time.Minute)); !errors.Is(err, ErrSweepAlreadyPosted) {
		t.Fatalf("replay = %v, want ErrSweepAlreadyPosted", err)
	}
	after, _, _ := app.Store().GetSweepRule("r1")
	if after.LastProposed != before.LastProposed {
		t.Fatalf("LastProposed = %q after the replay, want it left at %q", after.LastProposed, before.LastProposed)
	}
	if n := len(app.Transactions()); n != legs {
		t.Fatalf("%d transactions after the replay, want %d", n, legs)
	}
}
