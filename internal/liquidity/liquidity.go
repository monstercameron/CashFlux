// SPDX-License-Identifier: MIT

// Package liquidity classifies asset accounts by how usable their money is
// right now: available cash, restricted (locked or retirement), investments,
// or held assets (property/vehicles/other illiquid holdings). The class is
// derived from the account's type and lock state, so it is always explainable
// — no per-account knob to fall out of date.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package liquidity

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Class is a coarse usability bucket for an asset account's balance.
type Class string

const (
	// Available is spendable cash: checking, debit, physical cash, savings.
	Available Class = "available"
	// Restricted is money that exists but can't be freely spent right now: any
	// account locked until a future date, plus retirement accounts.
	Restricted Class = "restricted"
	// Investments are market-priced holdings: brokerage and crypto.
	Investments Class = "investments"
	// Held are illiquid valuations: property, vehicles, and other assets.
	Held Class = "held"
)

// Of returns the liquidity class for an asset account as of now. A future
// LockUntil restricts any account regardless of type (a locked savings account
// is not available cash). Liabilities have no liquidity class; callers should
// filter them out — Of classifies them by type like any account.
func Of(a domain.Account, now time.Time) Class {
	if !a.LockUntil.IsZero() && now.Before(a.LockUntil) {
		return Restricted
	}
	switch a.Type {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeCash, domain.TypeSavings:
		return Available
	case domain.TypeRetirement:
		return Restricted
	case domain.TypeInvestment, domain.TypeCrypto:
		return Investments
	default:
		return Held
	}
}

// Totals sums base-converted asset balances per class. conv converts an
// account's booked balance into base minor units (the caller owns FX and
// balance folding); accounts where conv reports false are skipped. Archived
// accounts and liabilities are skipped.
func Totals(accounts []domain.Account, now time.Time, conv func(domain.Account) (int64, bool)) map[Class]int64 {
	out := map[Class]int64{}
	for _, a := range accounts {
		if a.Archived || a.Class == domain.ClassLiability {
			continue
		}
		v, ok := conv(a)
		if !ok {
			continue
		}
		out[Of(a, now)] += v
	}
	return out
}
