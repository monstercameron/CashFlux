// SPDX-License-Identifier: MIT

package coverplan

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Model holds the filtered rows and currency/identity decisions for one form.
type Model struct {
	Overs         []Over
	Sources       []Source
	Base          string
	Decimals      int
	Signature     string
	CurrencyError bool
	Empty         bool
}

// CanStart reports whether application/checkpoint state permits an apply event.
func (model Model) CanStart(appAvailable, saving bool) bool {
	return appAvailable && !saving && !model.CurrencyError
}

// Prepare filters a targeted form without modifying the caller's rows, and
// resolves the original whole-form mixed-currency guard (donors included).
func Prepare(overs []Over, sources []Source, base, targetID string) Model {
	if targetID != "" {
		filtered := make([]Over, 0, len(overs))
		for _, over := range overs {
			if over.ID == targetID {
				filtered = append(filtered, over)
			}
		}
		overs = filtered
	}
	currencies := map[string]bool{}
	var signature strings.Builder
	for _, over := range overs {
		currencies[over.Currency] = true
		signature.WriteString(over.ID)
		signature.WriteByte(';')
	}
	for _, source := range sources {
		currencies[source.Currency] = true
	}
	if len(currencies) == 1 {
		for code := range currencies {
			base = code
		}
	}
	return Model{Overs: overs, Sources: sources, Base: base, Decimals: currency.Decimals(base),
		Signature: signature.String(), CurrencyError: len(currencies) > 1, Empty: len(overs) == 0}
}

// Choose clones a selection before updating it, preserving the previous state.
func Choose(chosen map[string]string, overID, source string) map[string]string {
	next := make(map[string]string, len(chosen)+1)
	for id, value := range chosen {
		next[id] = value
	}
	next[overID] = source
	return next
}

// SeedChoices clears selections only when the displayed row identity changes.
// Revisions to the same rows retain explicit picks; new rows require new picks.
func SeedChoices(model Model, previousSignature string) (map[string]string, bool) {
	if previousSignature == model.Signature {
		return nil, false
	}
	return map[string]string{}, true
}

// HasDonor reports an explicit choice, including next-period borrowing.
func HasDonor(chosen map[string]string) bool {
	for _, source := range chosen {
		if source != "" {
			return true
		}
	}
	return false
}

// SelectedTotal returns the selected subtotal and count, or zeros on overflow.
// Call SelectedTotalChecked when the caller needs to display the failure.
func SelectedTotal(overs []Over, chosen map[string]string) (int64, int) {
	amount, count, _ := SelectedTotalChecked(overs, chosen)
	return amount, count
}

// SelectedTotalChecked sums explicit selections with checked minor-unit math.
// Currency compatibility is decided separately by Prepare and Submission.
func SelectedTotalChecked(overs []Over, chosen map[string]string) (int64, int, error) {
	var amount int64
	count := 0
	for _, over := range overs {
		if chosen[over.ID] == "" {
			continue
		}
		if over.Error != nil {
			return 0, 0, over.Error
		}
		var err error
		amount, err = money.Add(amount, over.Shortfall.Amount)
		if err != nil {
			return 0, 0, fmt.Errorf("sum selected cover amounts: %w", err)
		}
		count++
	}
	return amount, count, nil
}

// HasNextMonthSelection reports whether any selected value borrows next period.
func HasNextMonthSelection(chosen map[string]string) bool {
	for _, source := range chosen {
		if source == NextSource {
			return true
		}
	}
	return false
}

// Problem is an explainable validation failure. Err carries arithmetic failures;
// Key, Name and Amount carry the existing localized business messages.
type Problem struct {
	Key    string
	Name   string
	Amount money.Money
	Err    error
}

// Submission validates aggregate donor draws and builds planner requests. It
// never applies writes; budgeting.PlanCover rechecks live room at commit time.
func Submission(model Model, chosen map[string]string) ([]budgeting.CoverRequest, *Problem) {
	if _, _, err := SelectedTotalChecked(model.Overs, chosen); err != nil {
		return nil, &Problem{Err: err}
	}
	sources := sourceIndex(model.Sources)
	drawn := map[string]int64{}
	for _, over := range model.Overs {
		src := chosen[over.ID]
		if src == NextSource && over.Rollover {
			return nil, &Problem{Key: "coverAll.rolloverBorrowBlocked", Name: over.Name}
		}
		if src == "" || src == NextSource {
			continue
		}
		source, ok := sources[src]
		if !ok || source.Currency != over.Currency {
			return nil, &Problem{Key: "coverAll.currencyError"}
		}
		amount, err := money.Add(drawn[src], over.Shortfall.Amount)
		if err != nil {
			return nil, &Problem{Err: fmt.Errorf("sum cover source draws: %w", err)}
		}
		drawn[src] = amount
	}
	for id, amount := range drawn {
		source := sources[id]
		if amount <= source.Avail.Amount {
			continue
		}
		short, err := money.Sub(amount, source.Avail.Amount)
		if err != nil {
			return nil, &Problem{Err: fmt.Errorf("calculate cover source shortage: %w", err)}
		}
		return nil, &Problem{Key: "coverAll.sourceShort", Name: source.Name, Amount: money.New(short, source.Currency)}
	}
	return requests(model.Overs, chosen), nil
}

func sourceIndex(sources []Source) map[string]Source {
	byID := make(map[string]Source, len(sources))
	for _, source := range sources {
		byID[source.ID] = source
	}
	return byID
}

func requests(overs []Over, chosen map[string]string) []budgeting.CoverRequest {
	result := make([]budgeting.CoverRequest, 0, len(overs))
	for _, over := range overs {
		source := chosen[over.ID]
		if source == "" || !over.Shortfall.IsPositive() {
			continue
		}
		request := budgeting.CoverRequest{DestinationID: over.ID}
		if source == NextSource {
			request.BorrowNext = over.Shortfall
		} else {
			request.Shares = map[string]money.Money{source: over.Shortfall}
		}
		result = append(result, request)
	}
	return result
}
