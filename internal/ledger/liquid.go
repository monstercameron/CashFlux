package ledger

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// LiquidBalance sums the spendable cash across non-archived cash-type accounts
// (checking, debit, savings, cash), each converted to the base currency — the
// "how much could I actually spend right now" figure. Investments, liabilities,
// and "other" accounts are excluded. It's the canonical liquid total behind the
// cash-runway metric.
func LiquidBalance(accounts []domain.Account, all []domain.Transaction, rates currency.Rates) (money.Money, error) {
	total := money.Zero(rates.Base)
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		switch a.Type {
		case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		default:
			continue
		}
		bal, err := Balance(a, all)
		if err != nil {
			return money.Money{}, err
		}
		conv, err := rates.Convert(bal, rates.Base)
		if err != nil {
			return money.Money{}, err
		}
		if total, err = total.Add(conv); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}
