// SPDX-License-Identifier: MIT

// Package householdview prepares household owner, spending and income facts.
// It retains independent availability and never reads UI state or mutates input.
package householdview

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// ErrBaseCurrency means the caller did not provide a usable base identity.
var ErrBaseCurrency = errors.New("household: base currency is required")

// ErrPeriod means the spending/income half-open interval is empty or reversed.
var ErrPeriod = errors.New("household: period end must follow period start")

// ErrNoRecipients marks a non-applicable income total, not known zero income.
var ErrNoRecipients = errors.New("household: no income split recipients")

// SourceErrors distinguishes failed reads from successfully empty collections.
// Settings failures affect financial facts; member failures retain known totals.
type SourceErrors struct {
	Settings, Accounts, Transactions, Members error
}

// Input supplies one whole-household snapshot and its explicit reporting period.
// Worth uses all transactions; spending/income use [PeriodStart, PeriodEnd).
// Rates.Base must be supplied by the caller, including any configured fallback.
type Input struct {
	Accounts               []domain.Account
	Transactions           []domain.Transaction
	Members                []domain.Member
	Rates                  currency.Rates
	PeriodStart, PeriodEnd time.Time
	Errors                 SourceErrors
}

// AmountFact is known exactly when Err is nil, including known zero. An error's
// zero payload must not be formatted as an available amount.
type AmountFact struct {
	Value money.Money
	Err   error
}

// PercentFact is a prepared 0..100 bar width, known exactly when Err is nil.
type PercentFact struct {
	Value int
	Err   error
}

// WorthProjection retains each complete owner reading separately from its total.
// Err applies to every owner; Total.Err may instead fail only the checked sum.
type WorthProjection struct {
	Base    string
	ByOwner map[string]money.Money
	Err     error
	Total   AmountFact
}

// Owner returns known base zero for an absent owner only after a successful read.
func (p WorthProjection) Owner(id string) AmountFact {
	if p.Err != nil {
		return amount(p.Base, 0, p.Err)
	}
	if value, ok := p.ByOwner[id]; ok {
		return AmountFact{Value: value}
	}
	return amount(p.Base, 0, nil)
}

// SpendProjection retains canonical member and unattributed (empty ID) amounts.
// Individual rows remain available if only the combined total overflows.
type SpendProjection struct {
	Base     string
	ByMember map[string]money.Money
	Err      error
	Total    AmountFact
}

// Member returns an absent member's known zero only after successful preparation.
func (p SpendProjection) Member(id string) AmountFact {
	if p.Err != nil {
		return amount(p.Base, 0, p.Err)
	}
	if value, ok := p.ByMember[id]; ok {
		return AmountFact{Value: value}
	}
	return amount(p.Base, 0, nil)
}

// IncomeProjection distinguishes no recipients from an unavailable or zero split.
// Splits preserve the canonical lexical member-ID order and exact remainders.
type IncomeProjection struct {
	Applicable bool
	Splits     []allocate.MemberIncomeSplit
	Err        error
	Total      AmountFact
}

// RowKind distinguishes a named member from shared/unattributed analytics rows.
type RowKind string

const (
	// RowMember identifies one named roster member.
	RowMember RowKind = "member"
	// RowShared identifies shared worth or unattributed spending.
	RowShared RowKind = "shared"
)

// RankedRow contains unformatted facts and a stable key for one analytics row.
// MemberID is the actual entity ID; shared spending uses an empty MemberID.
type RankedRow struct {
	Key            string
	Kind           RowKind
	MemberID, Name string
	Amount         AmountFact
	Percent        PercentFact
}

// MemberRow keeps the input roster order and contains scalar display identity.
// WorthPercent compares roster members only, as the existing roster does.
type MemberRow struct {
	ID, Name     string
	Worth, Spent AmountFact
	WorthPercent PercentFact
}

// LeaderKind is a translation-independent takeaway decision.
type LeaderKind string

const (
	// LeaderNone means no positive magnitude/spending exists in this decision.
	LeaderNone LeaderKind = "none"
	// LeaderShared means the shared/unattributed bucket meets the current policy.
	LeaderShared LeaderKind = "shared"
	// LeaderMember identifies the first leading visible member in roster order.
	LeaderMember LeaderKind = "member"
	// LeaderUnrepresented means spending exists without a matching visible person.
	LeaderUnrepresented LeaderKind = "unrepresented"
)

// Leader names a prepared decision; Err means no leader claim can be made.
type Leader struct {
	Kind     LeaderKind
	MemberID string
	Err      error
}

// Model shares one immutable preparation across hero, roster and analytics.
// Collections are newly owned by this result and must be treated as read-only.
type Model struct {
	Base                             string
	PeriodStart, PeriodEnd           time.Time
	RosterErr                        error
	Members                          []MemberRow
	Worth                            WorthProjection
	Spending                         SpendProjection
	Income                           IncomeProjection
	WorthRows, SpendRows, IncomeRows []RankedRow
	Holder, Spender                  Leader
}

// Prepare composes the canonical ledger, spending and income split services.
// A failing facet never suppresses an independently available facet, and failed
// checked totals never discard successfully prepared per-owner/member readings.
func Prepare(in Input) Model {
	base := strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	rates := currency.Rates{Base: base, Rates: in.Rates.Rates}
	var baseErr, periodErr error
	if base == "" {
		baseErr = ErrBaseCurrency
	}
	if !in.PeriodEnd.After(in.PeriodStart) {
		periodErr = ErrPeriod
	}
	out := Model{Base: base, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd, RosterErr: in.Errors.Members}
	out.Worth = WorthProjection{Base: base, Err: dependencies("worth inputs", baseErr, in.Errors.Settings, in.Errors.Accounts, in.Errors.Transactions)}
	if out.Worth.Err == nil {
		out.Worth.ByOwner, out.Worth.Err = ledger.NetByOwner(in.Accounts, in.Transactions, rates)
	}
	out.Worth.Total = sum(base, out.Worth.ByOwner, out.Worth.Err)
	out.Spending = SpendProjection{Base: base, Err: dependencies("spending inputs", baseErr, periodErr, in.Errors.Settings, in.Errors.Transactions)}
	if out.Spending.Err == nil {
		var rows []reports.MemberSpend
		rows, out.Spending.Err = reports.SpendingByMember(in.Transactions, in.PeriodStart, in.PeriodEnd, rates)
		if out.Spending.Err == nil {
			out.Spending.ByMember = make(map[string]money.Money, len(rows))
			for _, row := range rows {
				out.Spending.ByMember[row.MemberID] = money.New(row.Amount, base)
			}
		}
	}
	out.Spending.Total = sum(base, out.Spending.ByMember, out.Spending.Err)
	out.Income.Err = dependencies("income inputs", baseErr, periodErr, in.Errors.Settings, in.Errors.Transactions, in.Errors.Members)
	if out.Income.Err == nil {
		out.Income.Splits, out.Income.Err = allocate.SplitPeriodIncome(in.Transactions, in.Members, in.PeriodStart, in.PeriodEnd, base, rates)
		if out.Income.Err != nil {
			out.Income.Splits = nil
		} else {
			out.Income.Applicable = len(out.Income.Splits) > 0
		}
	}
	out.Income.Total = amount(base, 0, out.Income.Err)
	if out.Income.Err == nil {
		if !out.Income.Applicable {
			out.Income.Total.Err = ErrNoRecipients
		} else {
			for _, row := range out.Income.Splits {
				value, err := out.Income.Total.Value.Add(money.New(row.Amount, base))
				if err != nil {
					out.Income.Total = amount(base, 0, fmt.Errorf("household income total: %w", err))
					break
				}
				out.Income.Total.Value = value
			}
		}
	}
	prepareRows(&out, in.Members)
	out.Holder = holder(out)
	out.Spender = spender(out)
	return out
}

func amount(base string, value int64, err error) AmountFact {
	return AmountFact{Value: money.New(value, base), Err: err}
}

func dependencies(label string, errs ...error) error {
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("household %s: %w", label, err)
	}
	return nil
}

// sum folds sorted identities so checked partial-sum behavior is deterministic.
func sum(base string, values map[string]money.Money, err error) AmountFact {
	out := amount(base, 0, err)
	if err != nil {
		return out
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		next, err := out.Value.Add(values[key])
		if err != nil {
			return amount(base, 0, fmt.Errorf("household total at %s: %w", key, err))
		}
		out.Value = next
	}
	return out
}

func magnitude(value int64) uint64 {
	if value < 0 {
		return uint64(-(value + 1)) + 1
	}
	return uint64(value)
}

// width uses an exact unsigned product, including the magnitude of MinInt64.
// numerator <= denominator ensures bits.Div64's quotient is at most 100.
func width(value int64, maximum uint64) int {
	if maximum == 0 {
		return 0
	}
	numerator := magnitude(value)
	if numerator >= maximum {
		return 100
	}
	hi, lo := bits.Mul64(numerator, 100)
	quotient, _ := bits.Div64(hi, lo, maximum)
	return int(quotient)
}

func prepareRows(out *Model, members []domain.Member) {
	var maxRosterWorth, maxWorth, maxSpend, maxIncome uint64
	if out.RosterErr == nil {
		out.Members = make([]MemberRow, 0, len(members))
		for _, member := range members {
			worth, spent := out.Worth.Owner(member.ID), out.Spending.Member(member.ID)
			out.Members = append(out.Members, MemberRow{ID: member.ID, Name: member.Name, Worth: worth, Spent: spent})
			if worth.Err == nil {
				maxRosterWorth = max(maxRosterWorth, magnitude(worth.Value.Amount))
			}
			if spent.Err == nil {
				maxSpend = max(maxSpend, magnitude(spent.Value.Amount))
			}
		}
	}
	shared := out.Worth.Owner(domain.GroupOwnerID)
	maxWorth = maxRosterWorth
	if shared.Err == nil {
		maxWorth = max(maxWorth, magnitude(shared.Value.Amount))
	}
	unattributed := out.Spending.Member("")
	if unattributed.Err == nil {
		maxSpend = max(maxSpend, magnitude(unattributed.Value.Amount))
	}
	for i := range out.Members {
		member := &out.Members[i]
		member.WorthPercent = PercentFact{Value: width(member.Worth.Value.Amount, maxRosterWorth), Err: member.Worth.Err}
		out.WorthRows = append(out.WorthRows, RankedRow{Key: "worth/member/" + member.ID, Kind: RowMember, MemberID: member.ID, Name: member.Name, Amount: member.Worth, Percent: PercentFact{Value: width(member.Worth.Value.Amount, maxWorth), Err: member.Worth.Err}})
		out.SpendRows = append(out.SpendRows, RankedRow{Key: "spend/member/" + member.ID, Kind: RowMember, MemberID: member.ID, Name: member.Name, Amount: member.Spent, Percent: PercentFact{Value: width(member.Spent.Value.Amount, maxSpend), Err: member.Spent.Err}})
	}
	if shared.Err == nil && shared.Value.Amount != 0 || len(out.Members) > 0 {
		out.WorthRows = append(out.WorthRows, RankedRow{Key: "worth/shared", Kind: RowShared, MemberID: domain.GroupOwnerID, Amount: shared, Percent: PercentFact{Value: width(shared.Value.Amount, maxWorth), Err: dependencies("worth scale", shared.Err, out.RosterErr)}})
	}
	if unattributed.Err == nil && unattributed.Value.Amount > 0 {
		out.SpendRows = append(out.SpendRows, RankedRow{Key: "spend/shared", Kind: RowShared, Amount: unattributed, Percent: PercentFact{Value: width(unattributed.Value.Amount, maxSpend), Err: out.RosterErr}})
	}
	for _, row := range out.Income.Splits {
		maxIncome = max(maxIncome, magnitude(row.Amount))
	}
	for _, row := range out.Income.Splits {
		out.IncomeRows = append(out.IncomeRows, RankedRow{Key: "income/member/" + row.MemberID, Kind: RowMember, MemberID: row.MemberID, Name: row.Name, Amount: amount(out.Base, row.Amount, nil), Percent: PercentFact{Value: width(row.Amount, maxIncome)}})
	}
}

func holder(out Model) Leader {
	if err := dependencies("holder", out.Worth.Err, out.RosterErr); err != nil {
		return Leader{Err: err}
	}
	shared := magnitude(out.Worth.Owner(domain.GroupOwnerID).Value.Amount)
	var total, top uint64
	exceeded := false
	leader := Leader{Kind: LeaderNone}
	for _, member := range out.Members {
		value := magnitude(member.Worth.Value.Amount)
		if value > top {
			top = value
			leader = Leader{Kind: LeaderMember, MemberID: member.ID}
		}
		// Only the comparison with shared is needed. Saturating at that bound
		// avoids overflowing a sum of many valid signed owner magnitudes.
		if !exceeded {
			if value > shared-total {
				exceeded = true
			} else {
				total += value
			}
		}
	}
	if shared > 0 && !exceeded {
		return Leader{Kind: LeaderShared}
	}
	return leader
}

func spender(out Model) Leader {
	if err := dependencies("spender", out.Spending.Total.Err, out.RosterErr); err != nil {
		return Leader{Err: err}
	}
	if out.Spending.Total.Value.Amount <= 0 {
		return Leader{Kind: LeaderNone}
	}
	unattributed := out.Spending.Member("").Value.Amount
	other, err := money.Sub(out.Spending.Total.Value.Amount, unattributed)
	if err != nil {
		return Leader{Err: err}
	}
	if unattributed >= other {
		return Leader{Kind: LeaderShared}
	}
	leader := Leader{Kind: LeaderUnrepresented}
	var top int64
	for _, member := range out.Members {
		if member.Spent.Value.Amount > top {
			top = member.Spent.Value.Amount
			leader = Leader{Kind: LeaderMember, MemberID: member.ID}
		}
	}
	return leader
}
