// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"log/slog"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// backfillLiabilitySigns migrates additive account JSON on ingress without
// touching the caller's slice. No live balance or transaction order is consulted.
func backfillLiabilitySigns(ds Dataset) (Dataset, error) {
	if ds.Accounts == nil {
		return ds, nil
	}
	accounts := make([]domain.Account, len(ds.Accounts))
	for i, account := range ds.Accounts {
		if account.LiabilitySign != "" &&
			account.LiabilitySign != domain.LiabilityPositiveOwed && account.LiabilitySign != domain.LiabilityNegativeOwed {
			slog.Warn("unknown saved liability sign; using opening-balance default",
				"account_id", account.ID, "liability_sign", account.LiabilitySign)
			account.LiabilitySign = ""
		}
		accounts[i] = account.WithLiabilitySign()
		if err := validateLiabilitySign(accounts[i]); err != nil {
			return Dataset{}, err
		}
	}
	ds.Accounts = accounts
	return ds, nil
}

func validateLiabilitySign(a domain.Account) error {
	switch a.LiabilitySign {
	case "", domain.LiabilityPositiveOwed, domain.LiabilityNegativeOwed:
		return nil
	default:
		return fmt.Errorf("store: account %q: unknown liability sign %q", a.ID, a.LiabilitySign)
	}
}
