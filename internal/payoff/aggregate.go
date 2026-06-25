// SPDX-License-Identifier: MIT

package payoff

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// AggregateDebts builds a slice of Debt values ready for BuildPlan from a set of
// accounts and transactions, converting every balance and minimum payment to the
// given base currency using rates.
//
// Inclusion rule: an account must be a liability (Class == ClassLiability), must
// not be archived, and must pass Account.IncludedInPayoff() — which excludes
// mortgages by default and respects the user's explicit IncludeInPayoff flag.
//
// NOTE (R21 installment coordination): once loan-amortization term fields land,
// installment loans whose remaining balance is already tracked by the amortization
// engine may need to be excluded here to avoid double-counting. For now all
// non-mortgage liabilities are included.
//
// Currency handling: if a rate is missing for an account's currency, that account
// is silently skipped and its currency code is appended to missingRates (deduped).
// This is intentional — mixing unconverted foreign amounts with base-currency
// amounts would corrupt the payoff simulation. The caller should surface the
// missing-rates list to the user so they can add FX rates and re-run.
//
// The returned debts are in base currency, with positive Balance (amount owed,
// absolute value of the ledger balance) and non-negative MinPayment. They are
// ordered by the iteration order of accounts, which callers may sort by strategy
// before passing to BuildPlan.
func AggregateDebts(
	accounts []domain.Account,
	txns []domain.Transaction,
	base string,
	rates currency.Rates,
) (debts []Debt, missingRates []string) {
	seen := make(map[string]bool) // dedup missing currency codes

	for _, a := range accounts {
		// Only non-archived liabilities that the user wants in the plan.
		if a.Archived || a.Class != domain.ClassLiability || !a.IncludedInPayoff() {
			continue
		}

		// Compute the current ledger balance (opening + all transactions).
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			// Ledger errors are currency-mismatch data-integrity issues; skip safely.
			continue
		}

		// Liability balances are typically negative (money owed); take the absolute
		// value as the positive "amount owed" the payoff engine expects.
		owedMinor := bal.Amount
		if owedMinor < 0 {
			owedMinor = -owedMinor
		}
		// Nothing owed — skip; Debt.Balance must be positive for BuildPlan to count it.
		if owedMinor == 0 {
			continue
		}

		// Convert the owed balance from the account's currency to base.
		owedBase, err := currency.ConvertBetween(owedMinor, a.Currency, base, rates)
		if err != nil {
			if !seen[a.Currency] {
				seen[a.Currency] = true
				missingRates = append(missingRates, a.Currency)
			}
			continue
		}

		// Convert the minimum payment from the account's currency to base.
		// MinPayment is stored as money.Money; use its Amount (minor units).
		var minBase int64
		if a.MinPayment.Amount > 0 {
			minBase, err = currency.ConvertBetween(a.MinPayment.Amount, a.Currency, base, rates)
			if err != nil {
				// Same rate gap — already recorded above; skip the whole account.
				if !seen[a.Currency] {
					seen[a.Currency] = true
					missingRates = append(missingRates, a.Currency)
				}
				continue
			}
		}

		debts = append(debts, Debt{
			Name:       a.Name,
			Balance:    owedBase,
			AprPercent: a.InterestRateAPR,
			MinPayment: minBase,
		})
	}

	return debts, missingRates
}
