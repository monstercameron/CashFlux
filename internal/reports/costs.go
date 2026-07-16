// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// This file finds the year's "money that bought nothing": bank/card fees and
// interest charges, surfaced by the Annual Review's problem-spots section.
// Classification is deliberately word-based (whole tokens, not substrings) so
// "Late payment fee" and an "Interest charge" match while "Coffee shop" and
// "Pinterest" never do.

// CostCharge is one fee or interest charge found in the ledger.
type CostCharge struct {
	Desc     string
	Date     time.Time
	Amount   int64 // absolute minor units, base currency
	Interest bool  // true = interest charge, false = fee
}

// MoneyCosts is the period's cost-of-money summary.
type MoneyCosts struct {
	FeeTotal      int64
	FeeCount      int
	InterestTotal int64
	InterestCount int
	Items         []CostCharge // every matched charge, largest first
}

var feeTokens = map[string]struct{}{
	"fee": {}, "fees": {}, "nsf": {}, "overdraft": {}, "penalty": {}, "surcharge": {},
}

var interestTokens = map[string]struct{}{
	"interest": {}, "apr": {},
}

// hasToken reports whether s contains any of the given words as whole
// lowercase tokens (split on any non-letter/digit).
func hasToken(s string, words map[string]struct{}) bool {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, f := range fields {
		if _, ok := words[f]; ok {
			return true
		}
	}
	return false
}

// CostOfMoney scans expense transactions in [start, end) for fee and interest
// charges: a charge whose description, payee, or category name carries a fee
// token counts as a fee; an interest token counts as interest (interest wins
// when both match — "interest charge fee" is an interest line). Transfers,
// excluded-from-reports charges, and income are skipped; amounts convert to
// the base currency; refund-pair netting applies. Items sort largest first.
func CostOfMoney(txns []domain.Transaction, cats []domain.Category, start, end time.Time, rates currency.Rates) (MoneyCosts, error) {
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	var out MoneyCosts
	for _, t := range netted(txns) {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		hay := t.Desc + " " + t.Payee + " " + catName[t.CategoryID]
		interest := hasToken(hay, interestTokens)
		fee := !interest && hasToken(hay, feeTokens)
		if !interest && !fee {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return MoneyCosts{}, err
		}
		amt := conv.Abs().Amount
		desc := t.Desc
		if desc == "" {
			desc = t.Payee
		}
		out.Items = append(out.Items, CostCharge{Desc: desc, Date: t.Date, Amount: amt, Interest: interest})
		if interest {
			out.InterestTotal += amt
			out.InterestCount++
		} else {
			out.FeeTotal += amt
			out.FeeCount++
		}
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].Amount > out.Items[j].Amount })
	return out, nil
}
