package debtpay

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

// ErrInvalidPeriod means the period has no ID, a missing boundary, or an end
// that does not follow its start.
var ErrInvalidPeriod = errors.New("invalid debt payment period")

// ErrInvalidAllocation means a classified payment's components do not preserve
// its cash amount.
var ErrInvalidAllocation = errors.New("debt payment allocation does not preserve cash amount")

// Aggregate reports cash paid, principal, interest, and fees for each supplied
// period and currency. Period end dates are exclusive. Escrow is deliberately
// not folded into fees or interest; this phase's analytics contract reports the
// four measures requested by the Debt payments view.
func Aggregate(payments []ClassifiedPayment, periods []Period) ([]PeriodTotals, error) {
	for _, period := range periods {
		if period.ID == "" || period.Start.IsZero() || period.End.IsZero() || !period.Start.Before(period.End) {
			return nil, ErrInvalidPeriod
		}
	}
	var totals []PeriodTotals
	for _, period := range periods {
		byCurrency := make(map[string]*PeriodTotals)
		for _, payment := range payments {
			if payment.Currency == "" || payment.Date.IsZero() || payment.Date.Before(period.Start) || !payment.Date.Before(period.End) {
				continue
			}
			allocation := payment.Allocation
			if allocation.PaymentMinor < 0 || allocation.PrincipalMinor < 0 || allocation.InterestMinor < 0 || allocation.FeesMinor < 0 {
				return nil, fmt.Errorf("transaction %q: negative debt payment total", payment.TxnID)
			}
			components := new(big.Int)
			for _, amount := range []int64{allocation.PrincipalMinor, allocation.InterestMinor, allocation.FeesMinor, allocation.EscrowMinor} {
				if amount < 0 {
					return nil, fmt.Errorf("transaction %q: %w", payment.TxnID, ErrInvalidAllocation)
				}
				components.Add(components, big.NewInt(amount))
			}
			if components.Cmp(big.NewInt(allocation.PaymentMinor)) != 0 {
				return nil, fmt.Errorf("transaction %q: %w", payment.TxnID, ErrInvalidAllocation)
			}
			row := byCurrency[payment.Currency]
			if row == nil {
				row = &PeriodTotals{PeriodID: period.ID, Currency: payment.Currency}
				byCurrency[payment.Currency] = row
			}
			if err := add(&row.CashPaidMinor, allocation.PaymentMinor); err != nil {
				return nil, fmt.Errorf("period %q cash paid: %w", period.ID, err)
			}
			if err := add(&row.PrincipalMinor, allocation.PrincipalMinor); err != nil {
				return nil, fmt.Errorf("period %q principal: %w", period.ID, err)
			}
			if err := add(&row.InterestMinor, allocation.InterestMinor); err != nil {
				return nil, fmt.Errorf("period %q interest: %w", period.ID, err)
			}
			if err := add(&row.FeesMinor, allocation.FeesMinor); err != nil {
				return nil, fmt.Errorf("period %q fees: %w", period.ID, err)
			}
		}
		currencies := make([]string, 0, len(byCurrency))
		for currency := range byCurrency {
			currencies = append(currencies, currency)
		}
		sortStrings(currencies)
		for _, currency := range currencies {
			totals = append(totals, *byCurrency[currency])
		}
	}
	return totals, nil
}

func add(target *int64, amount int64) error {
	if amount > 0 && *target > math.MaxInt64-amount {
		return errors.New("amount overflow")
	}
	*target += amount
	return nil
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
