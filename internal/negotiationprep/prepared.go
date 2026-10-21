// SPDX-License-Identifier: MIT

package negotiationprep

import (
	"fmt"
	"sort"
	"strings"
	"time"

	recurringview "github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Input is a fresh household snapshot and an explicit review time.
type Input struct {
	Recurring    []domain.Recurring
	Transactions []domain.Transaction
	Rates        currency.Rates
	Now          time.Time
}

// Selection identifies a provider. RecurringID takes precedence over a name.
type Selection struct{ Name, RecurringID, CompetitorNote string }

// Prepared retains the currency and evidence basis alongside the native sheet input.
type Prepared struct {
	Subscription     Subscription
	Currency         string
	RecurringID      string
	Cadence          domain.RecurringCadence
	ChargeCount      int
	ConvertedHistory bool
}

// Prepare selects one active cost and gathers eligible historical gross charges.
// Historical transactions alone never establish a current billing cadence.
func Prepare(in Input, selection Selection) (Prepared, error) {
	if in.Now.IsZero() {
		return Prepared{}, fmt.Errorf("subscription review date is required")
	}
	want := strings.ToLower(strings.TrimSpace(selection.Name))
	id := strings.TrimSpace(selection.RecurringID)
	if want == "" && id == "" {
		return Prepared{}, fmt.Errorf("provide a subscription name or recurring_id")
	}
	var candidates, exact []domain.Recurring
	for _, r := range in.Recurring {
		if !r.Active() || r.Amount.Amount >= 0 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(r.Label))
		if id != "" {
			if r.ID == id {
				candidates = append(candidates, r)
			}
		} else if strings.Contains(label, want) {
			candidates = append(candidates, r)
			if label == want {
				exact = append(exact, r)
			}
		}
	}
	if len(exact) > 0 {
		candidates = exact
	}
	if len(candidates) > 1 {
		labels := make([]string, 0, len(candidates))
		for _, r := range candidates {
			labels = append(labels, fmt.Sprintf("%s (recurring_id %s)", r.Label, r.ID))
		}
		sort.Strings(labels)
		return Prepared{}, fmt.Errorf("ambiguous subscription; choose one: %s", strings.Join(labels, "; "))
	}
	if id != "" && len(candidates) == 0 {
		return Prepared{}, fmt.Errorf("recurring_id %q is not an active outgoing cost", id)
	}
	p := Prepared{Subscription: Subscription{CompetitorNote: selection.CompetitorNote}}
	if len(candidates) == 1 {
		r := candidates[0]
		p.RecurringID, p.Cadence, p.Currency = r.ID, r.Cadence, r.Amount.Currency
		p.Subscription.Name = r.Label
		// History must match the chosen provider, rather than a broader query.
		want = strings.ToLower(strings.TrimSpace(r.Label))
		annual, err := recurringview.AnnualEquivalent(r)
		if err != nil {
			return Prepared{}, fmt.Errorf("current subscription annual cost: %w", err)
		}
		monthly, err := recurringview.MonthlyEquivalent(r)
		if err != nil {
			return Prepared{}, fmt.Errorf("current subscription monthly equivalent: %w", err)
		}
		p.Subscription.AnnualMinor, err = money.Abs(annual)
		if err != nil {
			return Prepared{}, fmt.Errorf("current subscription annual magnitude: %w", err)
		}
		p.Subscription.MonthlyMinor, err = money.Abs(monthly)
		if err != nil {
			return Prepared{}, fmt.Errorf("current subscription monthly magnitude: %w", err)
		}
		p.Subscription.AnnualKnown = true
		p.Subscription.MonthlyIsEquivalent = r.Cadence != domain.CadenceMonthly
		if p.Subscription.MonthlyIsEquivalent {
			p.Subscription.ComparisonGap = "Price-rise comparison is unavailable: historical charge amounts cannot be compared with a nonmonthly schedule's monthly equivalent."
		}
	} else {
		p.Subscription.Name = strings.TrimSpace(selection.Name)
		p.Subscription.CostGap = "No active outgoing schedule establishes the current cost or cadence. Monthly and annual costs are unknown; a historical charge is not a monthly bill."
		p.Subscription.ComparisonGap = "Price-rise comparison is unavailable without a current monthly billing schedule."
	}
	var matched []domain.Transaction
	identities := map[string]bool{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.IsExpense() || !t.CountsInReports() || t.Date.After(in.Now) {
			continue
		}
		if !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), want) {
			continue
		}
		if t.Date.IsZero() {
			return Prepared{}, fmt.Errorf("matched subscription charge %q has no date", t.ID)
		}
		identity := strings.ToLower(strings.TrimSpace(t.Payee))
		if identity == "" {
			identity = strings.ToLower(strings.TrimSpace(t.Desc))
		}
		identities[identity] = true
		matched = append(matched, t)
	}
	if len(identities) > 1 {
		return Prepared{}, fmt.Errorf("ambiguous historical merchant matches; narrow the provider name before making tenure or price claims")
	}
	if len(matched) == 0 && p.RecurringID == "" {
		return Prepared{}, fmt.Errorf("no matching active outgoing schedule or eligible historical charges")
	}
	if p.Currency == "" && len(matched) > 0 {
		p.Currency = matched[0].Amount.Currency
	}
	if strings.TrimSpace(p.Currency) == "" {
		return Prepared{}, fmt.Errorf("subscription currency is unavailable")
	}
	for _, t := range matched {
		amount := t.Amount
		if amount.Currency != p.Currency {
			var err error
			amount, err = in.Rates.Convert(amount, p.Currency)
			if err != nil {
				return Prepared{}, fmt.Errorf("subscription historical charge %q FX: %w", t.ID, err)
			}
			p.ConvertedHistory = true
		}
		positive, err := money.Abs(amount.Amount)
		if err != nil {
			return Prepared{}, fmt.Errorf("subscription historical charge %q magnitude: %w", t.ID, err)
		}
		p.Subscription.Charges = append(p.Subscription.Charges, Charge{At: t.Date, AmountMinor: positive})
	}
	p.ChargeCount = len(p.Subscription.Charges)
	if p.ConvertedHistory {
		p.Subscription.ComparisonGap = "Price-rise comparison is unavailable: mixed-currency history uses current FX and does not establish a nominal provider price rise."
	}
	return p, nil
}

// Basis describes the evidence limits separately from the call script.
func (p Prepared) Basis(now time.Time) string {
	basis := fmt.Sprintf("Household subscription review at %s. Amounts in %s. %d eligible historical gross charges through the review time; future, transfer, income and report-excluded rows omitted. Refunds are not netted into gross charge evidence.", now.Format(time.RFC3339), p.Currency, p.ChargeCount)
	if p.RecurringID != "" {
		basis += fmt.Sprintf(" Current active schedule: %s; cadence %s. Annual cost is normalized directly from that schedule, not from rounded monthly equivalents.", p.RecurringID, p.Cadence)
	}
	if p.ConvertedHistory {
		basis += " Historical currency conversion uses the current household FX table, not historical rates."
	}
	return basis
}
