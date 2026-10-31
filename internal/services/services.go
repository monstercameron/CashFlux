// SPDX-License-Identifier: MIT

// Package services lists the recurring bills the household shows on the Accounts page as services.
// A service is an ordinary recurring bill with its Service flag set. It is not an account, so it
// stays out of the asset, liability and net worth totals. Services are also excluded from budgets,
// reports, forecasts and auto-posting: the app filters them out of those views, and they appear only
// on the Accounts page. The package has no browser or component-framework dependencies.
package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Of returns the recurring bills marked as services, in their stored order.
func Of(recs []domain.Recurring) []domain.Recurring {
	var out []domain.Recurring
	for _, r := range recs {
		if r.Service {
			out = append(out, r)
		}
	}
	return out
}

// MonthlyTotal returns the monthly cost of the active services, as positive base-currency minor
// units. Paused services are skipped, as scheduling skips them. A service in a currency with no
// exchange rate to base makes the whole total an error.
func MonthlyTotal(recs []domain.Recurring, base string, rates currency.Rates) (int64, error) {
	var total int64
	for _, r := range Of(recs) {
		if !r.Active() {
			continue
		}
		monthly, err := recurring.MonthlyBase(r, base, rates)
		if err != nil {
			return 0, fmt.Errorf("service %q: %w", r.Label, err)
		}
		total, err = money.Add(total, monthly)
		if err != nil {
			return 0, fmt.Errorf("service total: %w", err)
		}
	}
	return total, nil
}

// Draft is what the add form collects for a monthly service.
type Draft struct {
	// ID is the record id. ParseDraft leaves it empty; the caller assigns one before
	// saving, because the store rejects a recurring bill without an id.
	ID           string
	Name         string
	Provider     string
	Currency     string
	MonthlyMinor int64 // the monthly cost in minor units of Currency
	NextDue      time.Time
}

// NewMonthly builds the stored record for a service. The cost is kept as a negative amount, the sign
// every recurring bill uses for spending, whatever the sign of d.MonthlyMinor: a service can never be
// stored as income. A cost too large to negate is stored as -math.MaxInt64. It is marked as a
// service, so the app keeps it out of budgets, reports and forecasts. NewMonthlyChecked rejects a
// non-positive cost instead.
func NewMonthly(d Draft) domain.Recurring {
	cost := d.MonthlyMinor
	switch {
	case cost == math.MinInt64:
		cost = math.MaxInt64
	case cost < 0:
		cost = -cost
	}
	return domain.Recurring{
		ID:       d.ID,
		Label:    d.Name,
		Provider: d.Provider,
		Service:  true,
		Cadence:  domain.CadenceMonthly,
		Amount:   money.New(-cost, d.Currency),
		NextDue:  d.NextDue,
	}
}

// NewMonthlyChecked is NewMonthly for a draft whose MonthlyMinor must be a positive amount. It returns
// ErrAmountInvalid, with the offending value, for a zero or negative cost.
func NewMonthlyChecked(d Draft) (domain.Recurring, error) {
	if d.MonthlyMinor <= 0 {
		return domain.Recurring{}, fmt.Errorf("%w: %d", ErrAmountInvalid, d.MonthlyMinor)
	}
	return NewMonthly(d), nil
}

// The ways ParseDraft can reject the add form's text.
var (
	// ErrNameRequired means the service name is blank.
	ErrNameRequired = errors.New("services: service name is required")
	// ErrAmountInvalid means the monthly cost is not a positive amount in the currency.
	ErrAmountInvalid = errors.New("services: service monthly cost must be a positive amount")
	// ErrCurrencyInvalid means the currency code is not three capital letters.
	ErrCurrencyInvalid = errors.New("services: currency must be a three-letter code")
)

// ParseDraft reads the add form's text into a Draft. The name is trimmed and must not be blank. The
// currency code is trimmed, upper-cased and must be three letters. The amount is read in the
// currency's minor units and must be positive. The first bill is due on the next 1st (today when it
// is the 1st). It returns ErrNameRequired, ErrCurrencyInvalid or ErrAmountInvalid, wrapped with context, so the caller can say which input to
// fix with errors.Is; an unreadable amount also wraps the parse error. The returned Draft has no ID.
func ParseDraft(name, provider, amount, cur string, now time.Time) (Draft, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return Draft{}, fmt.Errorf("services: parse draft: %w", ErrNameRequired)
	}
	cur = strings.ToUpper(strings.TrimSpace(cur))
	if !validCurrencyShape(cur) {
		return Draft{}, fmt.Errorf("%w: %q", ErrCurrencyInvalid, cur)
	}
	amount = strings.TrimSpace(amount)
	minor, err := money.ParseMinor(amount, currency.Decimals(cur))
	if err != nil {
		return Draft{}, fmt.Errorf("%w: %q: %w", ErrAmountInvalid, amount, err)
	}
	if minor <= 0 {
		return Draft{}, fmt.Errorf("%w: %q", ErrAmountInvalid, amount)
	}
	return Draft{
		Name: n, Provider: strings.TrimSpace(provider), Currency: cur, MonthlyMinor: minor,
		NextDue: dateutil.NextMonthlyDue(now, 1),
	}, nil
}

// validCurrencyShape reports whether code is exactly three capital ASCII letters.
func validCurrencyShape(code string) bool {
	if len(code) != 3 {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < 'A' || code[i] > 'Z' {
			return false
		}
	}
	return true
}
