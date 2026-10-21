// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberallocation"
	"github.com/monstercameron/CashFlux/internal/money"
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
// Conversion, magnitude and per-member sum failures return an error with no
// partial rows; unavailable spending must not be interpreted as known zero.
func SpendingByMember(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([]MemberSpend, error) {
	totals := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, fmt.Errorf("spending by member: transaction %s conversion: %w", t.ID, err)
		}
		magnitude, err := conv.AbsChecked()
		if err != nil {
			return nil, fmt.Errorf("spending by member: transaction %s magnitude: %w", t.ID, err)
		}
		shares := memberallocation.SplitAmount(magnitude.Amount, t.Owners())
		if len(shares) == 0 {
			totals[""], err = money.Add(totals[""], magnitude.Amount)
			if err != nil {
				return nil, fmt.Errorf("spending by member: transaction %s unattributed total: %w", t.ID, err)
			}
		}
		for _, share := range shares {
			totals[share.MemberID], err = money.Add(totals[share.MemberID], share.Amount)
			if err != nil {
				return nil, fmt.Errorf("spending by member: transaction %s member %s total: %w", t.ID, share.MemberID, err)
			}
		}
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
