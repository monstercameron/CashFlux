// Package setup provides typed input preparation for the first-run wizard.
package setup

import (
	"errors"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	ErrNameRequired    = errors.New("setup: account name is required")
	ErrInvalidCurrency = errors.New("setup: currency is invalid")
	ErrNegativeAmount  = errors.New("setup: amount must be zero or greater")
	ErrInvalidType     = errors.New("setup: account type is invalid")
)

// IncomePlan is the prepared effect of the optional monthly-income step.
// Set is false for blank input, preserving the wizard's existing skip behavior.
type IncomePlan struct {
	Set                bool
	MonthlyIncomeMinor int64
}

// AccountPlan is a fully validated account draft. It contains no generated ID,
// timestamp, or persistence effect; callers apply it only after preparation succeeds.
type AccountPlan struct {
	Name           string
	Type           domain.AccountType
	Class          domain.AccountClass
	Currency       string
	OpeningBalance money.Money
}

// PrepareIncome parses optional monthly income exactly in the explicit currency.
// Blank input means skip; invalid, negative, or unrepresentable values return an
// empty plan and an error, so callers cannot accidentally apply partial values.
func PrepareIncome(raw, currencyCode string) (IncomePlan, error) {
	if strings.TrimSpace(raw) == "" {
		return IncomePlan{}, nil
	}
	decimals, err := inputCurrencyDecimals(currencyCode)
	if err != nil {
		return IncomePlan{}, err
	}
	minor, err := money.ParseMinor(raw, decimals)
	if err != nil {
		return IncomePlan{}, fmt.Errorf("setup: parse monthly income: %w", err)
	}
	if minor < 0 {
		return IncomePlan{}, ErrNegativeAmount
	}
	return IncomePlan{Set: true, MonthlyIncomeMinor: minor}, nil
}

// PrepareAccount validates the full first-account draft before returning any
// write-ready data. Blank balance keeps the wizard's existing zero default.
// Nonnegative liability inputs retain the established positive-owed convention.
func PrepareAccount(name, rawBalance, currencyCode string, accountType domain.AccountType) (AccountPlan, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AccountPlan{}, ErrNameRequired
	}
	if !accountType.Valid() {
		return AccountPlan{}, ErrInvalidType
	}
	decimals, err := inputCurrencyDecimals(currencyCode)
	if err != nil {
		return AccountPlan{}, err
	}
	minor := int64(0)
	if strings.TrimSpace(rawBalance) != "" {
		minor, err = money.ParseMinor(rawBalance, decimals)
		if err != nil {
			return AccountPlan{}, fmt.Errorf("setup: parse opening balance: %w", err)
		}
		if minor < 0 {
			return AccountPlan{}, ErrNegativeAmount
		}
	}
	return AccountPlan{
		Name: name, Type: accountType, Class: accountType.Class(),
		Currency:       strings.ToUpper(strings.TrimSpace(currencyCode)),
		OpeningBalance: money.New(minor, currencyCode),
	}, nil
}

func inputCurrencyDecimals(code string) (int, error) {
	if !currency.Valid(code) {
		return 0, ErrInvalidCurrency
	}
	return currency.Decimals(code), nil
}
