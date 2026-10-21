// SPDX-License-Identifier: MIT

package recurringview

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/billsched"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

const (
	agendaRewindCap                = 400
	agendaIncomeOccurrencesPerFlow = 24
)

// AgendaSpanInput contains the immutable domain snapshot and date window used
// to prepare calendar and forward-agenda occurrences.
type AgendaSpanInput struct {
	Accounts  []domain.Account
	Recurring []domain.Recurring
	Now       time.Time
	From      time.Time
	Until     time.Time
	Base      string
	Rates     currency.Rates
}

// AgendaOccurrence is one merged bill or paycheck occurrence. Amount is signed
// and expressed in Base when AmountAvailable is true. Bill is populated for
// outflows and retains the source-currency bill identity used by presentation
// adapters; it is zero for income occurrences.
type AgendaOccurrence struct {
	Bill            bills.Bill
	RecurringID     string
	Date            time.Time
	Name            string
	AccountID       string
	AnchorAccountID string
	Amount          money.Money
	AmountAvailable bool
	AmountError     error
	Income          bool
	Past            bool
	Paid            bool
	Missed          bool

	hasSchedule      bool
	scheduleAdvanced bool
}

// AgendaSpan is an immutable, date-sorted set of prepared agenda occurrences.
// Callers should treat Occurrences as read-only after construction.
type AgendaSpan struct {
	Base        string
	Occurrences []AgendaOccurrence
}

// AgendaSettlementEvidence carries durable paid and matched facts gathered by
// an application adapter. BillID is the key used for explicit paid records;
// RecurringID is used for transaction-match evidence.
type AgendaSettlementEvidence struct {
	BillID      string
	RecurringID string
	DueDate     time.Time
	Paid        bool
	Matched     bool
}

// BuildAgendaSpan prepares merged bill occurrences and positive paycheck
// events over [From, Until]. It rewinds schedule copies to the start boundary,
// preserves bills.OccurrencesWithin's obligation dedupe and caps, converts all
// amounts into Base, and records settlement facts derivable from each original
// schedule. Missing or invalid FX leaves the amount explicitly unavailable.
func BuildAgendaSpan(in AgendaSpanInput) AgendaSpan {
	base := in.Base
	if base == "" {
		base = in.Rates.Base
	}
	if base == "" {
		base = "USD"
	}
	rates := in.Rates
	rates.Base = base
	today := calendarDay(in.Now)
	start := calendarDay(in.From)

	originalByID := make(map[string]domain.Recurring, len(in.Recurring))
	for _, recurring := range in.Recurring {
		originalByID[recurring.ID] = recurring
	}
	rewound := rewindAgendaSchedules(in.Recurring, start)
	span := AgendaSpan{Base: base}

	// OccurrencesWithin owns recurring/liability merge and duplicate policy.
	billOccurrences := bills.OccurrencesWithin(in.Accounts, rewound, start, in.Until)
	span.Occurrences = make([]AgendaOccurrence, 0, len(billOccurrences)+len(in.Recurring))
	for _, bill := range billOccurrences {
		rid, hasSchedule := bills.RecurringIDFromAccount(bill.AccountID)
		fact := AgendaOccurrence{
			Bill:            bill,
			RecurringID:     rid,
			Date:            bill.DueDate,
			Name:            bill.Name,
			AccountID:       bill.AccountID,
			AnchorAccountID: bill.AnchorAccountID,
			Amount:          money.Zero(base),
			Past:            bill.DueDate.Before(today),
			hasSchedule:     hasSchedule,
		}
		if hasSchedule {
			original := originalByID[rid]
			fact.scheduleAdvanced = !original.NextDue.IsZero() && bill.DueDate.Before(original.NextDue)
			fact.Paid = fact.scheduleAdvanced
			fact.Missed = fact.Past && !fact.Paid
		}
		amount, err := sourceInBase(bill.Amount, base, rates)
		if err == nil {
			amount, err = amount.NegChecked()
		}
		if err != nil {
			fact.AmountError = fmt.Errorf("agenda bill %q amount: %w", bill.AccountID, err)
		} else {
			fact.Amount = amount
			fact.AmountAvailable = true
		}
		span.Occurrences = append(span.Occurrences, fact)
	}

	for _, recurring := range rewound {
		if !recurring.Active() || recurring.Amount.IsNegative() {
			continue
		}
		amount, err := sourceInBase(recurring.Amount, base, rates)
		date := recurring.NextDue
		for i := 0; i < agendaIncomeOccurrencesPerFlow && !date.After(in.Until); i++ {
			if !date.Before(start) {
				fact := AgendaOccurrence{
					RecurringID: recurring.ID,
					Date:        date,
					Name:        recurring.Label,
					Amount:      money.Zero(base),
					Income:      true,
					Past:        date.Before(today),
				}
				if err != nil {
					fact.AmountError = fmt.Errorf("agenda income %q amount: %w", recurring.ID, err)
				} else {
					fact.Amount = amount
					fact.AmountAvailable = true
				}
				span.Occurrences = append(span.Occurrences, fact)
			}
			next := recurring.Cadence.Next(date)
			if !next.After(date) {
				break
			}
			date = next
		}
	}

	sort.SliceStable(span.Occurrences, func(i, j int) bool {
		return span.Occurrences[i].Date.Before(span.Occurrences[j].Date)
	})
	return span
}

// ForwardAgenda returns a copy containing occurrences on or after today's
// calendar date. Calendar views should use the complete span so past days remain
// visible; compact up-next views use this projection.
func ForwardAgenda(span AgendaSpan, now time.Time) AgendaSpan {
	today := calendarDay(now)
	forward := AgendaSpan{Base: span.Base, Occurrences: make([]AgendaOccurrence, 0, len(span.Occurrences))}
	for _, occurrence := range span.Occurrences {
		if !occurrence.Date.Before(today) {
			forward.Occurrences = append(forward.Occurrences, occurrence)
		}
	}
	return forward
}

// ProjectAgendaMonth projects a prepared span from the current balance through
// the displayed month using the existing billsched cash-flow policy. A future
// occurrence with an unavailable amount makes the projection unavailable.
func ProjectAgendaMonth(span AgendaSpan, startingBalance int64, month, now time.Time) (billsched.Projection, bool) {
	monthEnd := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
	if monthEnd.Before(now) {
		return billsched.Projection{}, false
	}
	items := make([]billsched.Item, 0, len(span.Occurrences))
	assign := make(map[string]time.Time, len(span.Occurrences))
	var paydays []time.Time
	var incomePerPayday int64
	for i, occurrence := range span.Occurrences {
		if occurrence.Date.Before(now) || !occurrence.Date.Before(monthEnd) {
			continue
		}
		if !occurrence.AmountAvailable {
			return billsched.Projection{}, false
		}
		if occurrence.Income {
			paydays = append(paydays, occurrence.Date)
			if occurrence.Amount.Amount > incomePerPayday {
				incomePerPayday = occurrence.Amount.Amount
			}
			continue
		}
		amount, err := money.Abs(occurrence.Amount.Amount)
		if err != nil {
			return billsched.Projection{}, false
		}
		id := occurrence.Name + strconv.Itoa(i)
		items = append(items, billsched.Item{ID: id, Amount: amount, Due: occurrence.Date})
		assign[id] = occurrence.Date
	}
	horizon := int(monthEnd.Sub(now).Hours() / 24)
	return billsched.Project(startingBalance, items, assign, paydays, incomePerPayday, now, horizon)
}

// WithSettlementEvidence returns a copy of span with persisted payment and
// match evidence applied. A past scheduled bill is missed only when no payment,
// match, or schedule-advanced fact settles it.
func WithSettlementEvidence(span AgendaSpan, evidence []AgendaSettlementEvidence) AgendaSpan {
	byBill := make(map[string]bool, len(evidence))
	byRecurring := make(map[string]bool, len(evidence))
	for _, item := range evidence {
		if item.Paid {
			byBill[agendaDateKey(item.BillID, item.DueDate)] = true
		}
		if item.Matched {
			byRecurring[agendaDateKey(item.RecurringID, item.DueDate)] = true
		}
	}

	settled := AgendaSpan{Base: span.Base, Occurrences: append([]AgendaOccurrence(nil), span.Occurrences...)}
	for i := range settled.Occurrences {
		occurrence := &settled.Occurrences[i]
		if occurrence.Income {
			continue
		}
		paid := byBill[agendaDateKey(occurrence.AccountID, occurrence.Date)]
		matched := byRecurring[agendaDateKey(occurrence.RecurringID, occurrence.Date)]
		occurrence.Paid = occurrenceSettled(occurrence.scheduleAdvanced, paid, matched)
		occurrence.Missed = occurrence.hasSchedule && occurrence.Past && !occurrence.Paid
	}
	return settled
}

// OccurrenceSettled reports whether durable payment/match evidence or the
// recurring schedule advancing past due establishes that an occurrence settled.
func OccurrenceSettled(recurring domain.Recurring, due time.Time, evidence AgendaSettlementEvidence) bool {
	advanced := !recurring.NextDue.IsZero() && due.Before(recurring.NextDue)
	return occurrenceSettled(advanced, evidence.Paid, evidence.Matched)
}

func occurrenceSettled(scheduleAdvanced, paid, matched bool) bool {
	return scheduleAdvanced || paid || matched
}

func sourceInBase(source money.Money, base string, rates currency.Rates) (money.Money, error) {
	if source.Currency == "" {
		source.Currency = base
	}
	return rates.Convert(source, base)
}

func rewindAgendaSchedules(recurring []domain.Recurring, from time.Time) []domain.Recurring {
	out := make([]domain.Recurring, len(recurring))
	copy(out, recurring)
	for i := range out {
		if out[i].NextDue.IsZero() {
			continue
		}
		date := out[i].NextDue
		for step := 0; step < agendaRewindCap && date.After(from); step++ {
			previous := out[i].Cadence.Prev(date)
			if !previous.Before(date) {
				break
			}
			date = previous
		}
		out[i].NextDue = date
	}
	return out
}

func calendarDay(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

func agendaDateKey(id string, date time.Time) string {
	return id + "|" + date.Format("2006-01-02")
}
