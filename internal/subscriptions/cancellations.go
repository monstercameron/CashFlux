package subscriptions

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// LateCharge is a subscription charge that landed after the user explicitly
// cancelled the subscription — the core signal for the charged-after-cancel
// alert. Amount is a positive figure in the base currency's minor units.
type LateCharge struct {
	// SubName is the subscription description (matches Subscription.Name and
	// SubscriptionCancellation.SubName).
	SubName string
	// CancelledOn is the date the user marked the subscription as cancelled.
	CancelledOn time.Time
	// ChargeDate is the date the post-cancel charge was posted.
	ChargeDate time.Time
	// Amount is the charge in base-currency minor units (positive).
	Amount int64
}

// ChargedAfterCancel finds every transaction that was charged for a cancelled
// subscription. For each SubscriptionCancellation, it scans txns for expense
// transactions whose Desc matches SubName (case-insensitive exact match — the
// same identity the detection engine uses) and whose Date is strictly after
// CancelledOn. Each such transaction produces a LateCharge with the amount
// converted to base currency. Income and transfers are excluded.
//
// Results are sorted deterministically: ascending ChargeDate, then SubName.
func ChargedAfterCancel(
	txns []domain.Transaction,
	cancels []domain.SubscriptionCancellation,
	rates currency.Rates,
) ([]LateCharge, error) {
	if len(cancels) == 0 {
		return nil, nil
	}

	// Build a lookup: lower-case sub name → cancellation date.
	cancelMap := make(map[string]time.Time, len(cancels))
	for _, c := range cancels {
		cancelMap[strings.ToLower(strings.TrimSpace(c.SubName))] = c.CancelledOn
	}

	var out []LateCharge
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(t.Desc))
		cancelledOn, found := cancelMap[key]
		if !found {
			continue
		}
		if !t.Date.After(cancelledOn) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		out = append(out, LateCharge{
			SubName:     strings.TrimSpace(t.Desc),
			CancelledOn: cancelledOn,
			ChargeDate:  t.Date,
			Amount:      conv.Abs().Amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].ChargeDate.Equal(out[j].ChargeDate) {
			return out[i].ChargeDate.Before(out[j].ChargeDate)
		}
		return out[i].SubName < out[j].SubName
	})
	return out, nil
}
