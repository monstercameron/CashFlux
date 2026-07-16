// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// MemberSpend is one household member's total expense over the reporting period,
// in base-currency minor units. MemberID is empty for spend not attributed to a
// member; the caller resolves names and labels the empty id.
type MemberSpend struct {
	MemberID string
	Amount   int64
}

// SpendingByMember totals expenses by member over the half-open period
// [start, end) in the base currency, largest first (ties broken by member id for
// determinism). Transfers and income are excluded (IsExpense), matching the rest
// of the reporting core. It answers the household question "who spent what?".
func SpendingByMember(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([]MemberSpend, error) {
	totals := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		totals[t.MemberID] += conv.Abs().Amount
	}

	out := make([]MemberSpend, 0, len(totals))
	for id, amt := range totals {
		out = append(out, MemberSpend{MemberID: id, Amount: amt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].MemberID < out[j].MemberID
	})
	return out, nil
}
