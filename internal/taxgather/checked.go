// SPDX-License-Identifier: MIT

package taxgather

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/csvsafe"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// YearBounds defines the tool's household calendar-year window explicitly in UTC.
func YearBounds(year int) (time.Time, time.Time, error) {
	if year < 1 || year > 9999 {
		return time.Time{}, time.Time{}, fmt.Errorf("tax year must be an integer from 1 through 9999")
	}
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

// GatherWithLinks uses explicit workspace refund links, never the legacy global
// report adapter. Unsafe/ambiguous links are unavailable before any report is
// emitted. The canonical netting read model leaves the original ledger untouched.
func GatherWithLinks(txns []domain.Transaction, cats []domain.Category, links []domain.TxnLink, year int, start, end time.Time, rates currency.Rates) (Summary, error) {
	if err := validateRefunds(txns, links, start, end); err != nil {
		return Summary{}, err
	}
	return gatherLedger(budgeting.NewNettedLedger(txns, links), cats, year, start, end, rates)
}

func validateRefunds(txns []domain.Transaction, links []domain.TxnLink, start, end time.Time) error {
	byID := make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		byID[t.ID] = t
	}
	originalNets, refundNets := map[string]int64{}, map[string]int64{}
	for _, l := range links {
		if l.Kind != domain.TxnLinkRefundPair || len(l.TxnIDs) != 2 {
			continue
		}
		original, hasOriginal := byID[l.TxnIDs[0]]
		refund, hasRefund := byID[l.TxnIDs[1]]
		if !hasOriginal || !hasRefund {
			continue // canonical netting ignores unresolved references
		}
		if !(original.CountsInReports() && dateutil.InRange(original.Date, start, end)) && !(refund.CountsInReports() && dateutil.InRange(refund.Date, start, end)) {
			continue
		}
		// Preserve canonical cross-currency/legacy link policy: incompatible
		// native units never net implicitly, even if an FX rate happens to exist.
		if !sameCode(original.Amount.Currency, refund.Amount.Currency) || l.Amount.Amount != 0 && !sameCode(l.Amount.Currency, original.Amount.Currency) {
			continue
		}
		if !original.IsExpense() || !refund.IsIncome() {
			return fmt.Errorf("refund link %q has invalid purchase/refund directions (%s, %s)", l.ID, original.ID, refund.ID)
		}
		originalAbs, err := original.Amount.AbsChecked()
		if err != nil {
			return fmt.Errorf("refund purchase %q: %w", original.ID, err)
		}
		refundAbs, err := refund.Amount.AbsChecked()
		if err != nil {
			return fmt.Errorf("refund %q: %w", refund.ID, err)
		}
		if _, err := l.Amount.AbsChecked(); err != nil {
			return fmt.Errorf("refund link %q: %w", l.ID, err)
		}
		net := txnlinks.RefundNet(l, original, refund) // canonical selection/cap
		originalNets[original.ID], err = money.Add(originalNets[original.ID], net.Amount)
		if err != nil {
			return fmt.Errorf("refund purchase %q total: %w", original.ID, err)
		}
		refundNets[refund.ID], err = money.Add(refundNets[refund.ID], net.Amount)
		if err != nil {
			return fmt.Errorf("refund %q total: %w", refund.ID, err)
		}
		if originalNets[original.ID] > originalAbs.Amount || refundNets[refund.ID] > refundAbs.Amount {
			return fmt.Errorf("refund allocation exceeds purchase/refund amount (%s, %s)", original.ID, refund.ID)
		}
	}
	for id, net := range originalNets {
		original := byID[id]
		magnitude, err := original.Amount.AbsChecked()
		if err != nil {
			return err
		}
		if original.CountsInReports() && dateutil.InRange(original.Date, start, end) && original.HasSplits() && net > 0 && net < magnitude.Amount {
			return fmt.Errorf("tax records unavailable: partially refunded split transaction %q needs an explicit category allocation", id)
		}
	}
	return nil
}

func sameCode(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }

// GatherCSVExact labels the currency and renders integer minor units at its
// registered decimal precision (or the app's canonical two-decimal fallback for
// an unregistered code), preserving large values without float conversion.
func GatherCSVExact(s Summary, name func(string) string, code string) ([]byte, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("CSV currency is unavailable")
	}
	var formatErr error
	amount := func(v int64) string {
		formatted, err := money.FormatMinorChecked(v, currency.Decimals(code))
		if err != nil {
			formatErr = err
		}
		return formatted
	}
	body := GatherCSV(s, name, amount)
	if formatErr != nil {
		return nil, formatErr
	}
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	if err := w.Write([]string{"Currency", csvsafe.Guard(code)}); err != nil {
		return nil, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	out.Write(body)
	return out.Bytes(), nil
}
