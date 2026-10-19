// SPDX-License-Identifier: MIT

package txnlink

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

// BestSubscription returns the detected subscription that matches the
// transaction's payee or description and whose typical charge is within 10% of
// the transaction amount. It returns an empty name when there is no confident
// match; detection order is stable, so ties remain deterministic.
func BestSubscription(txn domain.Transaction, detected []subscriptions.Subscription, rates currency.Rates) string {
	payee := strings.TrimSpace(txn.Payee)
	description := strings.TrimSpace(txn.Desc)
	if (payee == "" && description == "") || txn.Amount.Amount == 0 {
		return ""
	}
	amount, err := txn.Amount.AbsChecked()
	if err != nil {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(amount.Currency), strings.TrimSpace(rates.Base)) {
		converted, err := rates.Convert(amount, rates.Base)
		if err != nil {
			return ""
		}
		amount = converted
	}
	amount, err = amount.AbsChecked()
	if err != nil {
		return ""
	}
	want := amount.Amount
	bestName := ""
	bestDiff := int64(-1)
	for _, candidate := range detected {
		name := strings.TrimSpace(candidate.Name)
		if (!strings.EqualFold(name, payee) && !strings.EqualFold(name, description)) || candidate.Amount < 0 {
			continue
		}
		// Both magnitudes are nonnegative int64 values, so their difference fits.
		diff, _ := money.Sub(candidate.Amount, want)
		diff, _ = money.Abs(diff)
		if diff <= want/10 && (bestDiff < 0 || diff < bestDiff) {
			bestName, bestDiff = candidate.Name, diff
		}
	}
	return bestName
}

// SubscriptionNames returns the detected subscription kinds the picker
// can link to, in detector order, without duplicates or blank labels.
func SubscriptionNames(detected []subscriptions.Subscription) []string {
	seen := make(map[string]bool, len(detected))
	var names []string
	for _, candidate := range detected {
		name := strings.TrimSpace(candidate.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// FilterSubscriptionKinds keeps only real subscription commitments. The
// recurring-charge detector also finds bills and habitual spending, so its raw
// matches are not by themselves proof that a merchant is a subscription.
func FilterSubscriptionKinds(
	detected []subscriptions.Subscription,
	detectionTxns []domain.Transaction,
	txns []domain.Transaction,
	accounts []domain.Account,
	categoryNames map[string]string,
	confirmed map[string]bool,
	ignored []domain.SubscriptionIgnore,
) []subscriptions.Subscription {
	ignoredNames := make(map[string]bool, len(ignored))
	for _, item := range ignored {
		ignoredNames[strings.ToLower(strings.TrimSpace(item.SubName))] = true
	}
	categoryName := func(id string) string { return categoryNames[id] }
	filtered := make([]subscriptions.Subscription, 0, len(detected))
	for _, candidate := range detected {
		name := strings.TrimSpace(candidate.Name)
		if name == "" || ignoredNames[strings.ToLower(name)] || subscriptions.IsLiabilityPayment(candidate, txns, accounts) {
			continue
		}
		isSubscription := false
		for _, txn := range detectionTxns {
			if !strings.EqualFold(strings.TrimSpace(txn.Desc), name) {
				continue
			}
			if subscriptions.IsSubscriptionCommitment(name, categoryName(txn.CategoryID), confirmed) {
				isSubscription = true
				break
			}
		}
		if isSubscription {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}
