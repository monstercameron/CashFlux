// SPDX-License-Identifier: MIT

package attribution

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// This file extends the E1 attribution engine with the BRIDGE: the same window
// decomposition the dashboard's what-changed card uses, but split along the
// lines a balance sheet is actually read — money kept, market movement, debt
// paid down, new debt, revaluation — instead of the coarse flow/adjustment
// split. It shares Input, the half-open [Since, Until) window convention, the
// liability-magnitude convention, and the same absolute honesty rule: whatever
// the named legs fail to explain is reported as an explicit residual leg, never
// silently absorbed.

// LegKind names one leg of the net-worth bridge.
type LegKind string

const (
	// LegMoneyKept is ordinary cash flow through asset accounts: what came in
	// minus what went out, excluding balance-adjustment rows. This is the leg
	// the household actually controls week to week.
	LegMoneyKept LegKind = "moneyKept"
	// LegMarketMovement is the net effect of balance adjustments on investment,
	// retirement and crypto accounts — the market moving your holdings, not you
	// saving.
	LegMarketMovement LegKind = "marketMovement"
	// LegDebtPaidDown is the part of the window's liability activity that made
	// net worth go UP (payments against a balance owed).
	LegDebtPaidDown LegKind = "debtPaidDown"
	// LegNewDebt is the part of the window's liability activity that made net
	// worth go DOWN (new borrowing, interest, fresh card charges).
	LegNewDebt LegKind = "newDebt"
	// LegRevaluation is the net effect of balance adjustments on every other
	// asset account: a property re-appraisal, a vehicle depreciating, a cash
	// reconcile.
	LegRevaluation LegKind = "revaluation"
	// LegResidual is the exact remainder between the true net-worth movement and
	// the five named legs: report-excluded transactions, cross-currency transfer
	// asymmetry, a liability whose balance crossed zero mid-window, per-account
	// FX rounding. It is always reported, never folded into another leg.
	LegResidual LegKind = "residual"
)

// BridgeLegOrder is the canonical left-to-right order of the bridge's legs. A
// view should render them in this order so the waterfall always reads the same
// way: what you did, what happened to you, then the honest remainder.
var BridgeLegOrder = []LegKind{
	LegMoneyKept, LegMarketMovement, LegDebtPaidDown, LegNewDebt, LegRevaluation, LegResidual,
}

// Contributor is one account's share of a leg, in base-currency minor units.
// It is the next level of "why": a leg names WHAT moved net worth, and its
// contributors name WHICH accounts produced it.
type Contributor struct {
	AccountID   string
	AccountName string
	AmountMinor int64
}

// Leg is one signed step of the bridge, in base-currency minor units. A
// positive amount pushed net worth up over the window.
type Leg struct {
	Kind        LegKind
	AmountMinor int64
	// Contributors are the accounts behind this leg, largest magnitude first.
	// They are recorded on the way through the same single pass that computes
	// the leg, never recomputed, so a contributor list always sums to its leg.
	// LegResidual has none by definition: the residual is precisely what could
	// not be attributed to an account.
	Contributors []Contributor
}

// Bridge decomposes the movement of net worth across one window into signed
// legs that sum EXACTLY from StartMinor to EndMinor:
//
//	StartMinor + Σ Legs[i].AmountMinor == EndMinor
//
// That identity is the whole contract — it is asserted by the unit tests and is
// what lets a waterfall be drawn without an unexplained gap.
type Bridge struct {
	Since, Until time.Time
	Base         string
	// StartMinor / EndMinor are net worth as of each cutoff, using the canonical
	// "strictly before the cutoff" balance convention (ledger.NetWorthSeries).
	StartMinor, EndMinor int64
	// Legs holds every leg in BridgeLegOrder, INCLUDING zero-valued ones, so a
	// caller can rely on the shape and decide for itself what to draw.
	Legs []Leg
}

// DeltaMinor is the window's total net-worth movement (EndMinor - StartMinor).
func (b Bridge) DeltaMinor() int64 { return b.EndMinor - b.StartMinor }

// Contributors returns the accounts behind one leg, largest magnitude first.
func (b Bridge) Contributors(k LegKind) []Contributor {
	for _, l := range b.Legs {
		if l.Kind == k {
			return l.Contributors
		}
	}
	return nil
}

// Leg returns the signed amount of one leg (0 when absent).
func (b Bridge) Leg(k LegKind) int64 {
	for _, l := range b.Legs {
		if l.Kind == k {
			return l.AmountMinor
		}
	}
	return 0
}

// LegsSumMinor is the sum of every leg, residual included. It equals
// DeltaMinor by construction.
func (b Bridge) LegsSumMinor() int64 {
	var total int64
	for _, l := range b.Legs {
		total += l.AmountMinor
	}
	return total
}

// isMarketAsset reports whether an asset account's balance adjustments should
// read as MARKET movement rather than a revaluation or a reconcile.
func isMarketAsset(t domain.AccountType) bool {
	switch t {
	case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		return true
	}
	return false
}

// BuildBridge decomposes the net-worth movement over in.Since..in.Until into
// the five named legs plus an exact residual.
//
// Method, per non-archived account: balances are accumulated in the ACCOUNT's
// own currency at both cutoffs and per leg, then converted to base once each —
// converting bucket sums rather than individual transactions keeps FX rounding
// from smearing across the legs, and whatever rounding remains lands in the
// residual where it can be seen.
//
// Liability activity is split by the SIGN of its net-worth effect rather than
// by adjustment-ness, because that is the split a reader means: a payment that
// reduces what you owe is "debt paid down" whether it was posted as a payment
// or as a balance update, and a new charge is new debt either way.
func BuildBridge(in Input) (Bridge, error) {
	base := in.Rates.Base
	b := Bridge{Since: in.Since, Until: in.Until, Base: base}
	isAdj := in.IsAdjustment
	if isAdj == nil {
		isAdj = func(domain.Transaction) bool { return false }
	}

	legs := map[LegKind]int64{}
	contribs := map[LegKind][]Contributor{}
	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		// Account-currency sums: balances at each cutoff and the window's activity
		// bucketed by leg.
		balSince, balUntil := a.OpeningBalance.Amount, a.OpeningBalance.Amount
		acct := map[LegKind]int64{}
		for _, t := range in.Txns {
			if t.AccountID != a.ID {
				continue
			}
			if t.Date.Before(in.Since) {
				balSince += t.Amount.Amount
			}
			if t.Date.Before(in.Until) {
				balUntil += t.Amount.Amount
			}
			if !dateutil.InRange(t.Date, in.Since, in.Until) {
				continue
			}
			if a.Class == domain.ClassLiability {
				// Net-worth effect of one liability row: +1 when debt is stored
				// negative (|b| = -b, so -Δ|b| = Δb), -1 when stored as a positive
				// "amount you owe". A balance that crosses zero mid-window makes this
				// linear factor approximate for the SPLIT only; the total stays exact
				// and the difference surfaces in the residual.
				factor := int64(-1)
				if balSince < 0 || (balSince == 0 && balUntil < 0) {
					factor = 1
				}
				effect := factor * t.Amount.Amount
				if effect >= 0 {
					acct[LegDebtPaidDown] += effect
				} else {
					acct[LegNewDebt] += effect
				}
				continue
			}
			switch {
			case !isAdj(t):
				acct[LegMoneyKept] += t.Amount.Amount
			case isMarketAsset(a.Type):
				acct[LegMarketMovement] += t.Amount.Amount
			default:
				acct[LegRevaluation] += t.Amount.Amount
			}
		}

		conv := func(minor int64) (int64, error) {
			c, err := in.Rates.Convert(money.New(minor, a.Currency), base)
			if err != nil {
				return 0, fmt.Errorf("attribution: bridge: account %s: %w", a.ID, err)
			}
			return c.Amount, nil
		}
		startContrib, endContrib := balSince, balUntil
		if a.Class == domain.ClassLiability {
			startContrib, endContrib = -abs64(balSince), -abs64(balUntil)
		}
		cs, err := conv(startContrib)
		if err != nil {
			return Bridge{}, err
		}
		ce, err := conv(endContrib)
		if err != nil {
			return Bridge{}, err
		}
		b.StartMinor += cs
		b.EndMinor += ce
		for _, k := range BridgeLegOrder {
			if k == LegResidual || acct[k] == 0 {
				continue
			}
			v, err := conv(acct[k])
			if err != nil {
				return Bridge{}, err
			}
			legs[k] += v
			if v != 0 {
				contribs[k] = append(contribs[k], Contributor{
					AccountID: a.ID, AccountName: a.Name, AmountMinor: v,
				})
			}
		}
	}

	// The residual is defined, not measured: it is exactly what the named legs
	// left unexplained. Reporting it is the only way the waterfall can be read as
	// arithmetic rather than as an illustration.
	var named int64
	for _, k := range BridgeLegOrder {
		if k != LegResidual {
			named += legs[k]
		}
	}
	legs[LegResidual] = (b.EndMinor - b.StartMinor) - named

	b.Legs = make([]Leg, 0, len(BridgeLegOrder))
	for _, k := range BridgeLegOrder {
		c := contribs[k]
		sort.SliceStable(c, func(i, j int) bool {
			if d := abs64(c[i].AmountMinor) - abs64(c[j].AmountMinor); d != 0 {
				return d > 0
			}
			return c[i].AccountID < c[j].AccountID
		})
		b.Legs = append(b.Legs, Leg{Kind: k, AmountMinor: legs[k], Contributors: c})
	}
	return b, nil
}
