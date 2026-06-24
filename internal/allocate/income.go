// SPDX-License-Identifier: MIT

// Package allocate — income helpers.
//
// PeriodIncome computes the unallocated income pool from a slice of transactions
// within a half-open date range [start, end). It is the pure-logic complement to
// the Allocate screen's "Allocate this month's income" entry-point: the screen calls
// this, formats the result, and pre-fills the amount input so the user can allocate
// their period income without manual data entry.
//
// Design notes:
//   - Only income (positive, non-transfer) transactions are counted; expenses and
//     transfers are excluded. This matches the ledger.PeriodTotals contract.
//   - Currency conversion: each transaction is converted to the requested base
//     currency at the provided rates. A missing rate returns an error.
//   - The result is always ≥ 0. If expenses exceed income (negative net) the function
//     returns 0, not a negative amount, because there is nothing to allocate.
package allocate

import "time"

// Transaction is the minimum interface PeriodIncome needs from a transaction
// record. It avoids importing the domain package (keeping this package free of
// UI / platform dependencies and making it easy to test with a lightweight stub).
type Transaction struct {
	// Amount is in minor currency units (positive = income, negative = expense).
	Amount int64
	// Currency is the ISO-4217 currency code for Amount.
	Currency string
	// IsIncome reports whether the transaction is an income record. Only income
	// transactions are counted; expense and transfer records are ignored.
	IsIncome bool
	// Date is used to check whether the transaction falls in [start, end).
	Date time.Time
}

// RateConverter converts an amount in one currency to the base currency.
// Returns the converted minor-unit amount or an error when the rate is missing.
type RateConverter func(amount int64, from, to string) (int64, error)

// NoConvert is a RateConverter that passes amounts through unchanged — useful
// when all transactions are already in the base currency (e.g. in unit tests).
func NoConvert(amount int64, _, _ string) (int64, error) { return amount, nil }

// PeriodIncome sums income for transactions in [start, end), converting each to
// baseCurrency via convert. It returns the total income in minor base-currency
// units (always ≥ 0) or an error if conversion fails for any transaction.
//
// Only transactions where IsIncome is true and Date ∈ [start, end) are counted.
func PeriodIncome(txns []Transaction, start, end time.Time, baseCurrency string, convert RateConverter) (int64, error) {
	var total int64
	for _, t := range txns {
		if !t.IsIncome {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		converted, err := convert(t.Amount, t.Currency, baseCurrency)
		if err != nil {
			return 0, err
		}
		total += converted
	}
	if total < 0 {
		return 0, nil
	}
	return total, nil
}
