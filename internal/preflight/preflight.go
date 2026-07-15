// SPDX-License-Identifier: MIT

// Package preflight assembles the payday pre-flight checklist (XC9): at each
// pay-cycle boundary it turns the bill-scheduling outputs (paydays, the bills
// due before the next payday, the projected low point, the keep floor) into a
// glanceable ritual model — what's due this cycle, whether the projected low
// dips below the floor, and which accounts are running thin.
//
// It is pure assembly: no clock, no store, no syscall/js. The wasm layer feeds
// it billsched outputs and account balances; it returns a Checklist the screen
// renders and the XC8 task layer turns into self-resolving to-dos.
package preflight

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// BillItem is one obligation due within the pay cycle. AmountMinor is a positive
// magnitude in base-currency minor units. Autopay marks bills the biller pulls
// automatically (the user only needs funds available, not to pay by hand).
type BillItem struct {
	ID          string
	Name        string
	AmountMinor int64
	Due         time.Time
	Autopay     bool
	AccountID   string
	Currency    string
}

// AccountBalance is one account's current liquid balance for the dip check.
type AccountBalance struct {
	ID           string
	Name         string
	BalanceMinor int64
}

// Input is everything the checklist assembly needs, all supplied by the caller
// (billsched + ledger). Now bounds "due this cycle" on the near side; NextPayday
// bounds it on the far side.
type Input struct {
	Now               time.Time
	CycleStart        time.Time // the current pay-cycle's payday
	NextPayday        time.Time // the boundary that closes this cycle
	Bills             []BillItem
	ProjectedLowMinor int64
	ProjectedLowDate  time.Time
	KeepFloorMinor    int64
	Accounts          []AccountBalance
	// Paid is the set of BillItem.IDs already settled — a bill whose expected
	// occurrence carries a durable bill-match link (TX9). Those bills are dropped
	// from the checklist: they are done, not "still due this cycle". Empty/nil
	// means nothing is known-paid (the pre-TX9 behavior). Their amount is also
	// excluded from TotalDueMinor.
	Paid map[string]bool
}

// BillRow is one checklist line: a bill due this cycle. It is checklist-quiet —
// name, amount, date, and an autopay marker.
type BillRow struct {
	ID          string
	Name        string
	AmountMinor int64
	Due         time.Time
	Autopay     bool
	AccountID   string
	Currency    string
}

// AccountDip is an account whose balance sits below the keep floor.
type AccountDip struct {
	ID             string
	Name           string
	BalanceMinor   int64
	ShortfallMinor int64 // floor − balance (positive)
}

// Checklist is the assembled pre-flight model for one pay cycle.
type Checklist struct {
	CycleStart    time.Time
	NextPayday    time.Time
	Bills         []BillRow
	TotalDueMinor int64

	// LowPointMinor / LowPointDate are the projected trough over the cycle — the
	// one emphasized figure on the surface.
	LowPointMinor int64
	LowPointDate  time.Time
	FloorMinor    int64
	// BelowFloor is true when the projected low dips beneath the keep floor.
	BelowFloor     bool
	ShortfallMinor int64 // floor − low when BelowFloor, else 0

	DippingAccounts []AccountDip
}

// HasItems reports whether the checklist has anything to show (bills, a floor
// breach, or a dipping account) — an empty checklist should not be surfaced.
func (c Checklist) HasItems() bool {
	return len(c.Bills) > 0 || c.BelowFloor || len(c.DippingAccounts) > 0
}

// Build assembles the checklist from the input. Bills are filtered to those due
// within [Now, NextPayday) — this cycle's obligations — and sorted by due date
// then name for a stable, readable order.
func Build(in Input) Checklist {
	c := Checklist{
		CycleStart:    day(in.CycleStart),
		NextPayday:    day(in.NextPayday),
		LowPointMinor: in.ProjectedLowMinor,
		LowPointDate:  in.ProjectedLowDate,
		FloorMinor:    in.KeepFloorMinor,
	}
	now := day(in.Now)
	for _, b := range in.Bills {
		d := day(b.Due)
		if d.Before(now) {
			continue
		}
		if !in.NextPayday.IsZero() && !d.Before(day(in.NextPayday)) {
			continue
		}
		if in.Paid[b.ID] {
			continue // already settled by a matched transaction (TX9)
		}
		c.Bills = append(c.Bills, BillRow{
			ID: b.ID, Name: b.Name, AmountMinor: b.AmountMinor, Due: d,
			Autopay: b.Autopay, AccountID: b.AccountID, Currency: b.Currency,
		})
		c.TotalDueMinor += b.AmountMinor
	}
	sort.SliceStable(c.Bills, func(i, j int) bool {
		if !c.Bills[i].Due.Equal(c.Bills[j].Due) {
			return c.Bills[i].Due.Before(c.Bills[j].Due)
		}
		if c.Bills[i].Name != c.Bills[j].Name {
			return c.Bills[i].Name < c.Bills[j].Name
		}
		return c.Bills[i].ID < c.Bills[j].ID
	})

	if in.KeepFloorMinor != 0 && in.ProjectedLowMinor < in.KeepFloorMinor {
		c.BelowFloor = true
		c.ShortfallMinor = in.KeepFloorMinor - in.ProjectedLowMinor
	}

	for _, a := range in.Accounts {
		if in.KeepFloorMinor != 0 && a.BalanceMinor < in.KeepFloorMinor {
			c.DippingAccounts = append(c.DippingAccounts, AccountDip{
				ID: a.ID, Name: a.Name, BalanceMinor: a.BalanceMinor,
				ShortfallMinor: in.KeepFloorMinor - a.BalanceMinor,
			})
		}
	}
	sort.SliceStable(c.DippingAccounts, func(i, j int) bool {
		if c.DippingAccounts[i].BalanceMinor != c.DippingAccounts[j].BalanceMinor {
			return c.DippingAccounts[i].BalanceMinor < c.DippingAccounts[j].BalanceMinor
		}
		return c.DippingAccounts[i].ID < c.DippingAccounts[j].ID
	})
	return c
}

// ResolveForBill builds the self-resolving rule (XC8) for a checklist bill: the
// to-do auto-checks when a matching payment posts — a transaction whose payee
// carries the bill's name and whose magnitude is within a small tolerance of the
// bill amount. Tolerance is 2% of the amount (minimum 1 unit) to absorb rounding
// and small variable-bill wobble.
func ResolveForBill(row BillRow) domain.TaskResolve {
	tol := row.AmountMinor / 50 // 2%
	if tol < 1 {
		tol = 1
	}
	return domain.TaskResolve{
		MatchPayee:          strings.TrimSpace(row.Name),
		MatchAmountMinor:    row.AmountMinor,
		MatchCurrency:       row.Currency,
		MatchToleranceMinor: tol,
	}
}

// day canonicalizes a time to its calendar day at UTC midnight so that mixed-zone
// due dates and clocks compare as "same day" rather than off-by-a-few-hours.
func day(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
