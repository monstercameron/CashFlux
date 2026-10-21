// SPDX-License-Identifier: MIT

package subscriptions

import (
	"fmt"
	"math"

	"github.com/monstercameron/CashFlux/internal/money"
)

func subscriptionMagnitude(value int64) (int64, error) {
	if value == math.MinInt64 {
		return 0, money.ErrOverflow
	}
	if value < 0 {
		value = -value
	}
	return value, nil
}

func subscriptionMultiply(value, multiplier int64) (int64, error) {
	if value < 0 || multiplier < 0 || multiplier != 0 && value > math.MaxInt64/multiplier {
		return 0, money.ErrOverflow
	}
	return value * multiplier, nil
}

// MonthlyAmountChecked returns a subscription's normalized monthly amount.
// Weekly normalization preserves the historical truncation toward zero.
func (s Subscription) MonthlyAmountChecked() (int64, error) {
	amount, err := subscriptionMagnitude(s.Amount)
	if err != nil {
		return 0, fmt.Errorf("subscription %q amount: %w", s.Name, err)
	}
	switch s.Cadence {
	case CadenceYearly:
		return amount / 12, nil
	case CadenceWeekly:
		// 52/12 reduces to 13/3. Dividing first preserves truncation and
		// keeps the intermediate within int64 for ordinary subscription sizes.
		whole, remainder := amount/3, amount%3
		monthly, err := subscriptionMultiply(whole, 13)
		if err != nil {
			return 0, fmt.Errorf("subscription %q monthly amount: %w", s.Name, err)
		}
		tail := remainder * 13 / 3
		if monthly > math.MaxInt64-tail {
			return 0, fmt.Errorf("subscription %q monthly amount: %w", s.Name, money.ErrOverflow)
		}
		return monthly + tail, nil
	default:
		return amount, nil
	}
}

// AnnualAmountChecked returns the subscription's annual cost using checked
// wide-intermediate arithmetic.
func (s Subscription) AnnualAmountChecked() (int64, error) {
	amount, err := subscriptionMagnitude(s.Amount)
	if err != nil {
		return 0, fmt.Errorf("subscription %q amount: %w", s.Name, err)
	}
	multiplier := int64(12)
	if s.Cadence == CadenceYearly {
		return amount, nil
	}
	if s.Cadence == CadenceWeekly {
		multiplier = 52
	}
	annual, err := subscriptionMultiply(amount, multiplier)
	if err != nil {
		return 0, fmt.Errorf("subscription %q annual amount: %w", s.Name, err)
	}
	return annual, nil
}

// MonthlyTotalChecked sums normalized subscription costs without wrapping.
func MonthlyTotalChecked(subs []Subscription) (int64, error) {
	var total int64
	for _, sub := range subs {
		amount, err := sub.MonthlyAmountChecked()
		if err != nil {
			return 0, err
		}
		total, err = money.Add(total, amount)
		if err != nil {
			return 0, fmt.Errorf("subscription monthly total: %w", err)
		}
	}
	return total, nil
}

// AnnualTotalChecked sums annualized subscription costs without wrapping.
func AnnualTotalChecked(subs []Subscription) (int64, error) {
	var total int64
	for _, sub := range subs {
		amount, err := sub.AnnualAmountChecked()
		if err != nil {
			return 0, err
		}
		total, err = money.Add(total, amount)
		if err != nil {
			return 0, fmt.Errorf("subscription annual total: %w", err)
		}
	}
	return total, nil
}

// AnnualSavingsChecked sums annual costs selected by exact subscription name.
func AnnualSavingsChecked(subs []Subscription, selected map[string]bool) (int64, error) {
	var total int64
	for _, sub := range subs {
		if !selected[sub.Name] {
			continue
		}
		amount, err := sub.AnnualAmountChecked()
		if err != nil {
			return 0, err
		}
		total, err = money.Add(total, amount)
		if err != nil {
			return 0, fmt.Errorf("selected subscription annual total: %w", err)
		}
	}
	return total, nil
}

// SharePercentChecked returns floor(part*100/total), rejecting negative
// financial inputs and safely handling the wide product.
func SharePercentChecked(part, total int64) (int64, error) {
	if part < 0 || total < 0 {
		return 0, fmt.Errorf("subscription share: amounts must be nonnegative")
	}
	if total == 0 {
		return 0, fmt.Errorf("subscription share: denominator is zero")
	}
	if part <= math.MaxInt64/100 {
		return part * 100 / total, nil
	}
	return money.MulDiv(part, 100, total, money.RoundTowardZero)
}

// SpendingSharePercentChecked compares monthly recurring cost with average
// spending across trailing full months. trailingExpense is their pooled total.
func SpendingSharePercentChecked(monthly, trailingExpense int64, trailingMonths int) (int64, error) {
	if monthly < 0 || trailingExpense < 0 || trailingMonths < 0 {
		return 0, fmt.Errorf("subscription spending share: amounts and month count must be nonnegative")
	}
	if trailingMonths == 0 || trailingExpense == 0 {
		return 0, fmt.Errorf("subscription spending share: denominator is unavailable")
	}
	if int64(trailingMonths) > math.MaxInt64/100 {
		return 0, fmt.Errorf("subscription spending share: month count is too large")
	}
	factor := int64(trailingMonths) * 100
	if monthly <= math.MaxInt64/factor {
		return monthly * factor / trailingExpense, nil
	}
	return money.MulDiv(monthly, factor, trailingExpense, money.RoundTowardZero)
}

// FinancialSummary is the immutable checked totals model prepared before the
// subscriptions screen renders rows and summary facts.
type FinancialSummary struct {
	ActiveMonthly, CountedMonthly, CountedAnnual, SelectedAnnual int64
	MonthlyByName                                                map[string]int64
	SharePercentByName                                           map[string]int64
	SpendingSharePercent                                         int64
	HasSpendingShare                                             bool
}

// BuildFinancialSummary prepares all subscription money facts in one checked
// pass. active includes review-tier rows for row proportions; counted excludes
// those rows from headline monthly and annual totals, matching the screen policy.
func BuildFinancialSummary(active, counted []Subscription, selected map[string]bool, trailingExpense int64, trailingMonths int) (FinancialSummary, error) {
	var out FinancialSummary
	var err error
	out.ActiveMonthly, err = MonthlyTotalChecked(active)
	if err != nil {
		return FinancialSummary{}, err
	}
	out.CountedMonthly, err = MonthlyTotalChecked(counted)
	if err != nil {
		return FinancialSummary{}, err
	}
	out.CountedAnnual, err = AnnualTotalChecked(counted)
	if err != nil {
		return FinancialSummary{}, err
	}
	out.SelectedAnnual, err = AnnualSavingsChecked(active, selected)
	if err != nil {
		return FinancialSummary{}, err
	}
	out.MonthlyByName = make(map[string]int64, len(active))
	out.SharePercentByName = make(map[string]int64, len(active))
	for _, sub := range active {
		monthly, err := sub.MonthlyAmountChecked()
		if err != nil {
			return FinancialSummary{}, err
		}
		out.MonthlyByName[sub.Name] = monthly
		if out.ActiveMonthly > 0 {
			share, err := SharePercentChecked(monthly, out.ActiveMonthly)
			if err != nil {
				return FinancialSummary{}, err
			}
			if share > 100 {
				share = 100
			}
			out.SharePercentByName[sub.Name] = share
		}
	}
	if trailingMonths > 0 && trailingExpense > 0 {
		out.SpendingSharePercent, err = SpendingSharePercentChecked(out.CountedMonthly, trailingExpense, trailingMonths)
		if err != nil {
			return FinancialSummary{}, err
		}
		out.HasSpendingShare = true
	}
	return out, nil
}
