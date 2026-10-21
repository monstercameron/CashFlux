// SPDX-License-Identifier: MIT

package goalview

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	// ErrAllocationUnavailable means an account cannot safely back an edited amount.
	ErrAllocationUnavailable = errors.New("goalview: allocation source unavailable")
	// ErrAllocationAmount means a selected row has no valid positive amount.
	ErrAllocationAmount = errors.New("goalview: allocation amount must be positive")
	// ErrAllocationHeadroom means an edited amount exceeds checked account headroom.
	ErrAllocationHeadroom = errors.New("goalview: allocation exceeds account headroom")
	// ErrAllocationCurrency means an amount was entered in an older target currency.
	ErrAllocationCurrency = errors.New("goalview: goal currency changed while editing")
)

// AllocationDraftError identifies the row that needs attention. Available uses
// the goal currency and is meaningful for ErrAllocationHeadroom.
type AllocationDraftError struct {
	AccountID string
	Available money.Money
	Err       error
}

func (e *AllocationDraftError) Error() string {
	return fmt.Sprintf("goalview: account %s: %v", e.AccountID, e.Err)
}

// Unwrap retains the underlying amount, FX, overflow or unavailable error.
func (e *AllocationDraftError) Unwrap() error { return e.Err }

// AllocationDraftRow represents an eligible account or an existing reservation
// on an unavailable source. Original records are immutable and keep their exact
// stored currencies/order, including legacy duplicate account reservations.
type AllocationDraftRow struct {
	AccountID               string
	Account                 domain.Account
	Exists, Selected        bool
	Original                []domain.GoalAllocation
	Amount                  money.Money
	AmountKnown             bool
	Available               money.Money
	AvailableKnown, CanEdit bool
	Issue                   error
}

// AllocationChange separates selecting a row from deliberately replacing its
// amount. A selected original row with AmountChanged=false stays byte-for-byte
// equivalent even if its display conversion or account is unavailable.
type AllocationChange struct {
	Selected      bool
	AmountText    string
	AmountChanged bool
	Currency      string
}

type allocationSource struct {
	balance        money.Money
	otherTarget    int64
	otherNonTarget money.Money
}

// AllocationDraft shares one indexed reservation/headroom picture across all
// editor rows. Rebuild it from current balances/rates immediately before Apply
// at submit time; unedited records are then taken from the latest stored Goal.
type AllocationDraft struct {
	Rows         []AllocationDraftRow
	Currency     string
	Free         money.Money
	FreeErr      error
	Incomplete   bool
	Prefill      money.Money
	PrefillKnown bool
	goal         domain.Goal
	rates        currency.Rates
	byID         map[string]int
	sources      map[string]allocationSource
}

// BuildAllocationDraft indexes other goals' reservations once, with checked
// aggregate conversion in each account's native currency. Balances are the
// caller's shared ledger snapshot; missing/invalid balances never become zero.
func BuildAllocationDraft(g domain.Goal, all []domain.Goal, accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) AllocationDraft {
	d := AllocationDraft{Currency: g.TargetAmount.Currency, goal: g, rates: rates, byID: map[string]int{}, sources: map[string]allocationSource{}}
	d.Free = money.Zero(d.Currency)
	original := map[string][]domain.GoalAllocation{}
	for _, a := range g.Allocations {
		original[a.AccountID] = append(original[a.AccountID], a)
	}
	other := map[string]map[string]int64{}
	issues := map[string]error{}
	for _, goal := range all {
		if goal.ID == g.ID {
			continue
		}
		for _, a := range goal.Allocations {
			if a.Amount.Amount == 0 {
				continue
			}
			if a.Amount.Amount < 0 {
				issues[a.AccountID] = fmt.Errorf("%w: negative reservation", ErrAllocationUnavailable)
				continue
			}
			code := money.Zero(a.Amount.Currency).Currency
			if other[a.AccountID] == nil {
				other[a.AccountID] = map[string]int64{}
			}
			total, err := money.Add(other[a.AccountID][code], a.Amount.Amount)
			if err != nil {
				issues[a.AccountID] = fmt.Errorf("%w: %w", ErrAllocationUnavailable, err)
				continue
			}
			other[a.AccountID][code] = total
		}
	}
	appendRow := func(account domain.Account, exists bool, id string) {
		if _, seen := d.byID[id]; seen {
			return
		}
		r := AllocationDraftRow{AccountID: id, Account: account, Exists: exists, Original: original[id], Selected: len(original[id]) > 0, Available: money.Zero(d.Currency), Amount: money.Zero(d.Currency)}
		r.AmountKnown = true
		if r.Selected {
			var err error
			r.Amount, err = goals.AllocationTotal(domain.Goal{ID: g.ID, Allocations: r.Original}, rates, d.Currency)
			r.AmountKnown = err == nil
		}
		source, available, err := allocationHeadroom(account, exists, balances[id], other[id], issues[id], rates, d.Currency)
		r.Available, r.Issue = available, err
		r.CanEdit, r.AvailableKnown = err == nil, err == nil
		if err == nil {
			d.sources[id] = source
			if d.FreeErr == nil {
				d.Free, d.FreeErr = d.Free.Add(available)
			}
		} else {
			d.Incomplete = true
		}
		if !r.AmountKnown {
			d.Incomplete = true
		}
		d.byID[id] = len(d.Rows)
		d.Rows = append(d.Rows, r)
	}
	// The same stable order drives picker, split, and new-record insertion.
	for _, liquid := range []bool{true, false} {
		for _, a := range accounts {
			if EarmarkSource(a) && LiquidSource(a.Type) == liquid {
				appendRow(a, true, a.ID)
			}
		}
	}
	byAccount := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		byAccount[a.ID] = a
	}
	for _, a := range g.Allocations {
		account, exists := byAccount[a.AccountID]
		appendRow(account, exists, a.AccountID)
	}
	coverage, _ := goals.EvaluateCoverage(g, rates)
	if coverage.SavedKnown {
		gap, err := g.TargetAmount.Sub(coverage.Saved)
		if err == nil {
			remaining := max(gap.Amount, int64(0))
			prefill := int64(0)
			for _, r := range d.Rows {
				if !r.AvailableKnown {
					continue
				}
				add := min(r.Available.Amount, remaining-prefill)
				prefill += add
			}
			d.Prefill, d.PrefillKnown = money.New(prefill, d.Currency), true
		}
	}
	return d
}

func allocationHeadroom(a domain.Account, exists bool, balance money.Money, others map[string]int64, issue error, rates currency.Rates, cur string) (allocationSource, money.Money, error) {
	s := allocationSource{balance: balance, otherNonTarget: money.Zero(a.Currency)}
	zero := money.Zero(cur)
	if !exists || !EarmarkSource(a) {
		return s, zero, ErrAllocationUnavailable
	}
	if issue != nil {
		return s, zero, issue
	}
	if balance.Currency != a.Currency || a.Currency == "" {
		return s, zero, fmt.Errorf("%w: invalid balance", ErrAllocationUnavailable)
	}
	codes := make([]string, 0, len(others))
	for code := range others {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	reserved := money.Zero(a.Currency)
	for _, code := range codes {
		converted, err := rates.Convert(money.New(others[code], code), a.Currency)
		if err == nil {
			reserved, err = reserved.Add(converted)
		}
		if err != nil {
			return s, zero, fmt.Errorf("%w: %w", ErrAllocationUnavailable, err)
		}
		if code == cur {
			s.otherTarget = others[code]
		} else {
			s.otherNonTarget, err = s.otherNonTarget.Add(converted)
			if err != nil {
				return s, zero, fmt.Errorf("%w: %w", ErrAllocationUnavailable, err)
			}
		}
	}
	free, err := balance.Sub(reserved)
	if err != nil {
		return s, zero, fmt.Errorf("%w: %w", ErrAllocationUnavailable, err)
	}
	free.Amount = max(free.Amount, int64(0))
	available, err := rates.Convert(free, cur)
	if err != nil {
		return s, zero, fmt.Errorf("%w: %w", ErrAllocationUnavailable, err)
	}
	if !s.fits(available, rates) {
		low, high := int64(0), available.Amount
		for low < high {
			step := high - low
			mid := low + step/2 + step%2
			if s.fits(money.New(mid, cur), rates) {
				low = mid
			} else {
				high = mid - 1
			}
		}
		available.Amount = low
	}
	return s, available, nil
}

func (s allocationSource) fits(candidate money.Money, rates currency.Rates) bool {
	amount, err := money.Add(s.otherTarget, candidate.Amount)
	if err != nil {
		return false
	}
	converted, err := rates.Convert(money.New(amount, candidate.Currency), s.balance.Currency)
	if err != nil {
		return false
	}
	total, err := s.otherNonTarget.Add(converted)
	return err == nil && total.Amount <= s.balance.Amount
}

// Row returns an immutable draft row by account ID.
func (d AllocationDraft) Row(id string) (AllocationDraftRow, bool) {
	i, ok := d.byID[id]
	if !ok {
		return AllocationDraftRow{}, false
	}
	return d.Rows[i], true
}

// CheckChange validates one row without scanning the household or allocating a
// replacement collection. It supports immediate row feedback while typing.
func (d AllocationDraft) CheckChange(id string, change AllocationChange) error {
	_, _, err := d.changeRecord(id, change)
	return err
}

func (d AllocationDraft) changeRecord(id string, change AllocationChange) (domain.GoalAllocation, bool, error) {
	var empty domain.GoalAllocation
	if !change.Selected {
		return empty, false, nil
	}
	r, ok := d.Row(id)
	if !ok {
		return empty, false, &AllocationDraftError{AccountID: id, Err: ErrAllocationUnavailable}
	}
	if len(r.Original) > 0 && !change.AmountChanged {
		return empty, false, nil
	}
	if change.Currency != "" && change.Currency != d.Currency {
		return empty, false, &AllocationDraftError{AccountID: id, Err: ErrAllocationCurrency}
	}
	if !r.CanEdit {
		return empty, false, &AllocationDraftError{AccountID: id, Err: r.Issue}
	}
	amount, err := money.ParseMinor(strings.TrimSpace(change.AmountText), currency.Decimals(d.Currency))
	if err != nil {
		return empty, false, &AllocationDraftError{AccountID: id, Err: fmt.Errorf("%w: %w", ErrAllocationAmount, err)}
	}
	if amount <= 0 {
		return empty, false, &AllocationDraftError{AccountID: id, Err: ErrAllocationAmount}
	}
	candidate := money.New(amount, d.Currency)
	if amount > r.Available.Amount || !d.sources[id].fits(candidate, d.rates) {
		return empty, false, &AllocationDraftError{AccountID: id, Available: r.Available, Err: ErrAllocationHeadroom}
	}
	return domain.GoalAllocation{AccountID: id, Amount: candidate}, true, nil
}

// Apply validates explicit changes against this snapshot and preserves all
// untouched original records, including unavailable/archived/deleted sources.
// Deselection always removes the row; only amount edits/new selections need FX.
func (d AllocationDraft) Apply(changes map[string]AllocationChange) ([]domain.GoalAllocation, error) {
	replacements := map[string]domain.GoalAllocation{}
	modifiedAmount := false
	ids := make([]string, 0, len(changes))
	for id := range changes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		change := changes[id]
		record, replace, err := d.changeRecord(id, change)
		if err != nil {
			return nil, err
		}
		if replace {
			replacements[id] = record
			modifiedAmount = true
		}
	}
	out := make([]domain.GoalAllocation, 0, len(d.goal.Allocations)+len(replacements))
	written := map[string]bool{}
	for _, original := range d.goal.Allocations {
		change, changed := changes[original.AccountID]
		if changed && !change.Selected {
			continue
		}
		if replacement, ok := replacements[original.AccountID]; ok {
			if !written[original.AccountID] {
				out = append(out, replacement)
				written[original.AccountID] = true
			}
			continue
		}
		out = append(out, original)
	}
	for _, r := range d.Rows {
		if replacement, ok := replacements[r.AccountID]; ok && !written[r.AccountID] {
			out = append(out, replacement)
			written[r.AccountID] = true
		}
	}
	if modifiedAmount {
		_, err := goals.AllocationTotal(domain.Goal{ID: d.goal.ID, Allocations: out}, d.rates, d.Currency)
		if errors.Is(err, money.ErrOverflow) {
			return nil, &AllocationDraftError{Err: err}
		}
	}
	return out, nil
}

// Split returns explicit replacement edits for selected sources. With no rows
// selected it chooses available liquid accounts only. Unavailable selected
// reservations must be preserved or removed explicitly, never silently omitted
// from a split advertised as a total.
func (d AllocationDraft) Split(text string, changes map[string]AllocationChange, mode goals.SplitMode) (map[string]AllocationChange, error) {
	total, err := money.ParseMinor(strings.TrimSpace(text), currency.Decimals(d.Currency))
	if err != nil {
		return nil, &AllocationDraftError{Err: fmt.Errorf("%w: %w", ErrAllocationAmount, err)}
	}
	if total <= 0 {
		return nil, &AllocationDraftError{Err: ErrAllocationAmount}
	}
	for id, change := range changes {
		if _, ok := d.Row(id); !ok && change.Selected {
			return nil, &AllocationDraftError{AccountID: id, Err: ErrAllocationUnavailable}
		}
	}
	var targets []AllocationDraftRow
	selected := false
	for _, r := range d.Rows {
		picked := r.Selected
		if change, ok := changes[r.AccountID]; ok {
			picked = change.Selected
		}
		if !picked {
			continue
		}
		selected = true
		if !r.CanEdit {
			return nil, &AllocationDraftError{AccountID: r.AccountID, Err: r.Issue}
		}
		targets = append(targets, r)
	}
	if !selected {
		for _, r := range d.Rows {
			if r.CanEdit && LiquidSource(r.Account.Type) {
				targets = append(targets, r)
			}
		}
	}
	avails := make([]int64, len(targets))
	hasCapacity := false
	for i, r := range targets {
		avails[i] = r.Available.Amount
		hasCapacity = hasCapacity || avails[i] > 0
	}
	if !hasCapacity {
		return nil, &AllocationDraftError{Err: ErrAllocationHeadroom, Available: money.Zero(d.Currency)}
	}
	parts := goals.SplitEarmark(total, avails, mode)
	out := make(map[string]AllocationChange, len(changes)+len(targets))
	for id, change := range changes {
		out[id] = change
	}
	for i, r := range targets {
		out[r.AccountID] = AllocationChange{Selected: parts[i] > 0, AmountChanged: true, AmountText: money.FormatMinor(parts[i], currency.Decimals(d.Currency)), Currency: d.Currency}
	}
	return out, nil
}

// AllocationPreview contains checked totals for the current explicit edits.
// Unknown preserved legacy amounts may be saved unchanged, but never displayed
// as a known target-currency total or coverage percentage.
type AllocationPreview struct {
	Total      money.Money
	Evaluation goals.Evaluation
	Err        error
}

// Preview reuses Apply and canonical goal evaluation without mutating the Goal.
func (d AllocationDraft) Preview(changes map[string]AllocationChange, now time.Time) AllocationPreview {
	allocations, err := d.Apply(changes)
	if err != nil {
		return AllocationPreview{Err: err}
	}
	g := d.goal
	g.Allocations = allocations
	total, err := goals.AllocationTotal(g, d.rates, d.Currency)
	e := goals.EvaluateGoal(g, goals.TaskSummary{}, d.rates, now)
	if err == nil {
		err = e.Err
	}
	return AllocationPreview{Total: total, Evaluation: e, Err: err}
}
