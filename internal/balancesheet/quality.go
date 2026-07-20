// SPDX-License-Identifier: MIT

package balancesheet

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Data quality for the balance sheet.
//
// A net-worth figure is only as trustworthy as the balances under it, and on a
// real household those are not equally trustworthy at all: a checking account
// reconciles itself from transactions every few days, while a property holding
// is a number somebody typed in once and may be worth 78% of the asset side. A
// single stale manual valuation, or a stale FX rate on a foreign-currency
// account, can therefore move the headline more than a year of saving — and the
// figure gives the reader no way to know.
//
// So this reports what the figure rests on: how many accounts are in it, which
// are overdue for an update, the oldest hand-entered valuation, whether one
// holding dominates, which currencies had to be converted, and what was left
// out. It does NOT define staleness — the caller passes the app's own notion in
// (freshness.IsStale), because a second, private idea of "stale" that disagreed
// with the rest of the app would be worse than none.

// SourceKind says where an account's current balance most recently came from,
// which is the difference between "counted" and "asserted".
type SourceKind string

const (
	// SourceTracked is a balance derived from transactions.
	SourceTracked SourceKind = "tracked"
	// SourceManual is a balance the user set directly (an update-balance
	// adjustment or an untouched opening balance) — an assertion, not a count.
	SourceManual SourceKind = "manual"
)

// QualityAccount is one account's contribution to the trustworthiness of the
// figure.
type QualityAccount struct {
	ID, Name string
	Currency string
	// AsOf is when the balance was last confirmed; zero means never.
	AsOf time.Time
	// DaysSince is whole days since confirmation, or -1 if never confirmed.
	DaysSince int
	Stale     bool
	Source    SourceKind
	// ShareOfSideBips is this account's share of its own side, in basis points
	// (10000 = 100%), so a caller can say which holdings actually matter.
	ShareOfSideBips int64
}

// Quality is the disclosure behind the headline figure.
type Quality struct {
	// AccountsIncluded counts the non-archived accounts in the figure.
	AccountsIncluded int
	// Stale lists the accounts overdue for an update, most overdue first.
	Stale []QualityAccount
	// OldestManual is the longest-unconfirmed hand-entered valuation — the one
	// most able to be quietly wrong. Valid only when HasOldestManual.
	OldestManual    QualityAccount
	HasOldestManual bool
	// Dominant is the single largest asset holding, with its share, when one
	// account is a large enough part of the total that the figure effectively
	// depends on it. Valid only when HasDominant.
	Dominant    QualityAccount
	HasDominant bool
	// BaseCurrency is what everything was converted to; Converted lists the
	// other currencies that had to pass through the FX table to get there.
	BaseCurrency string
	Converted    []string
	// ExcludedByChoice / ExcludedNoRate count accounts left out of the figure
	// deliberately and for want of an exchange rate.
	ExcludedByChoice, ExcludedNoRate int
}

// NeedsAttention reports whether anything here is worth the reader's time. When
// false a surface should stay quiet rather than displaying a clean bill of
// health as permanent furniture.
func (q Quality) NeedsAttention() bool {
	return len(q.Stale) > 0 || q.ExcludedByChoice > 0 || q.ExcludedNoRate > 0
}

// DominantShareBips is the threshold at which one holding is treated as
// carrying the figure: a third of everything owned. Below that, no single
// account's staleness can plausibly dominate the headline.
const DominantShareBips = 3333

// QualityInput configures one assessment.
type QualityInput struct {
	Accounts []domain.Account
	Txns     []domain.Transaction
	Rates    currency.Rates
	Now      time.Time
	// IsStale is the app's own staleness test (freshness.IsStale bound to the
	// household's windows). Nil means nothing is stale.
	IsStale func(domain.Account) bool
	// IsManual reports whether an account's balance was last set by hand rather
	// than derived from transactions (ledger.BalanceProvenance's adjusted /
	// opening kinds). Nil means nothing is manual.
	IsManual func(domain.Account) bool
	// ExcludedByChoice / ExcludedNoRate come from ledger.NetWorthExplained, so
	// the disclosure repeats the figure's own exclusions rather than
	// recomputing a second opinion about them.
	ExcludedByChoice, ExcludedNoRate int
}

// AssessQuality builds the disclosure. It never changes a figure; it only says
// what the figure rests on.
func AssessQuality(in QualityInput) Quality {
	q := Quality{
		BaseCurrency:     in.Rates.Base,
		ExcludedByChoice: in.ExcludedByChoice,
		ExcludedNoRate:   in.ExcludedNoRate,
	}
	isStale := in.IsStale
	if isStale == nil {
		isStale = func(domain.Account) bool { return false }
	}
	isManual := in.IsManual
	if isManual == nil {
		isManual = func(domain.Account) bool { return false }
	}

	seenCur := map[string]bool{}
	var assetTotal int64
	type sized struct {
		qa    QualityAccount
		minor int64
	}
	var assets []sized

	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		q.AccountsIncluded++

		qa := QualityAccount{
			ID: a.ID, Name: a.Name, Currency: a.Currency,
			AsOf: a.BalanceAsOf, DaysSince: -1,
			Stale: isStale(a), Source: SourceTracked,
		}
		if isManual(a) {
			qa.Source = SourceManual
		}
		if !a.BalanceAsOf.IsZero() {
			qa.DaysSince = daysBetween(a.BalanceAsOf, in.Now)
		}
		if a.Currency != "" && a.Currency != in.Rates.Base && !seenCur[a.Currency] {
			seenCur[a.Currency] = true
			q.Converted = append(q.Converted, a.Currency)
		}
		if qa.Stale {
			q.Stale = append(q.Stale, qa)
		}
		// The oldest hand-entered valuation: never-confirmed counts as oldest of
		// all, because "we have no idea how old this is" is the weakest case.
		if qa.Source == SourceManual {
			if !q.HasOldestManual || olderThan(qa, q.OldestManual) {
				q.OldestManual, q.HasOldestManual = qa, true
			}
		}
		if a.Class == domain.ClassAsset {
			if bal, err := accountBase(a, in.Txns, in.Rates); err == nil && bal > 0 {
				assetTotal += bal
				assets = append(assets, sized{qa: qa, minor: bal})
			}
		}
	}

	sort.SliceStable(q.Stale, func(i, j int) bool {
		if q.Stale[i].DaysSince != q.Stale[j].DaysSince {
			// Never-confirmed (-1) is the most overdue, not the least.
			if q.Stale[i].DaysSince < 0 {
				return true
			}
			if q.Stale[j].DaysSince < 0 {
				return false
			}
			return q.Stale[i].DaysSince > q.Stale[j].DaysSince
		}
		return q.Stale[i].ID < q.Stale[j].ID
	})
	sort.Strings(q.Converted)

	if assetTotal > 0 {
		sort.SliceStable(assets, func(i, j int) bool { return assets[i].minor > assets[j].minor })
		top := assets[0]
		top.qa.ShareOfSideBips = top.minor * 10000 / assetTotal
		// Dominance needs something to dominate. With a single asset account it
		// is trivially 100%, and telling the reader that one holding is all of
		// what they own is noise, not insight.
		if len(assets) > 1 && top.qa.ShareOfSideBips >= DominantShareBips {
			q.Dominant, q.HasDominant = top.qa, true
		}
		if q.HasOldestManual {
			for _, s := range assets {
				if s.qa.ID == q.OldestManual.ID {
					q.OldestManual.ShareOfSideBips = s.minor * 10000 / assetTotal
					break
				}
			}
		}
	}
	return q
}

// olderThan reports whether a is a weaker (older, or entirely unconfirmed)
// valuation than b.
func olderThan(a, b QualityAccount) bool {
	if a.DaysSince < 0 || b.DaysSince < 0 {
		return a.DaysSince < 0 && b.DaysSince >= 0
	}
	if a.DaysSince != b.DaysSince {
		return a.DaysSince > b.DaysSince
	}
	return a.ID < b.ID
}

// accountBase is an account's current balance converted to the base currency.
func accountBase(a domain.Account, txns []domain.Transaction, rates currency.Rates) (int64, error) {
	bal := a.OpeningBalance.Amount
	for _, t := range txns {
		if t.AccountID == a.ID {
			bal += t.Amount.Amount
		}
	}
	conv, err := rates.Convert(money.New(bal, a.Currency), rates.Base)
	if err != nil {
		return 0, err
	}
	return conv.Amount, nil
}

// daysBetween is whole days from a to b (never negative).
func daysBetween(a, b time.Time) int {
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}
