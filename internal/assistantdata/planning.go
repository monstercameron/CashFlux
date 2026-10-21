// SPDX-License-Identifier: MIT

// Package assistantdata prepares checked, platform-independent Assistant facts.
package assistantdata

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// Amount preserves every integer minor unit when JSON is consumed by JavaScript.
// Decimals explains how to read Minor; monetary domain logic never uses floats.
type Amount struct {
	Minor    string `json:"minor"`
	Currency string `json:"currency"`
	Decimals int    `json:"decimals"`
}

// Value distinguishes a valid zero from an unavailable monetary calculation.
type Value struct {
	Known  bool    `json:"known"`
	Amount *Amount `json:"amount"`
	Error  string  `json:"error,omitempty"`
}

func amount(m money.Money) Amount {
	return Amount{Minor: strconv.FormatInt(m.Amount, 10), Currency: m.Currency, Decimals: currency.Decimals(m.Currency)}
}
func value(m money.Money, err error) Value {
	if err != nil {
		return Value{Error: err.Error()}
	}
	a := amount(m)
	return Value{Known: true, Amount: &a}
}

// PlanningInput is one call's current application snapshot. Callers must not
// retain a Planning across tool executions or mutate its input collections.
type PlanningInput struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Goals        []domain.Goal
	Tasks        []domain.Task
	Recurring    []domain.Recurring
	Rates        currency.Rates
	Now          time.Time
}

// Planning shares checked balances, task counts and reservations within one call.
type Planning struct {
	in           PlanningInput
	balances     map[string]money.Money
	reservations *goals.ReservationIndex
	tasks        map[string]goals.TaskSummary
}

// NewPlanning creates a read-only snapshot; expensive indexes are prepared lazily.
func NewPlanning(in PlanningInput) *Planning {
	in.Rates.Base = strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	if in.Rates.Base == "" {
		in.Rates.Base = "USD"
	}
	return &Planning{in: in}
}

func (p *Planning) base() string {
	if p.in.Rates.Base == "" {
		return "USD"
	}
	return p.in.Rates.Base
}
func (p *Planning) positions() goals.ReservationIndex {
	if p.reservations == nil {
		p.balances, _ = ledger.Balances(p.in.Accounts, p.in.Transactions)
		v := goals.BuildAccountReservations(p.in.Goals, p.in.Accounts, p.balances, p.in.Rates)
		p.reservations = &v
	}
	return *p.reservations
}
func (p *Planning) account(id string) (domain.Account, error) {
	for _, a := range p.in.Accounts {
		if a.ID == id && id != "" {
			return a, nil
		}
	}
	return domain.Account{}, fmt.Errorf("no account with ID %q", id)
}

// ReservationFact reports native-currency headroom including retained claims.
type ReservationFact struct {
	AccountID   string                  `json:"account_id"`
	AccountName string                  `json:"account_name"`
	Exists      bool                    `json:"exists"`
	Archived    bool                    `json:"archived"`
	Known       bool                    `json:"known"`
	Balance     Value                   `json:"balance"`
	Reserved    Value                   `json:"reserved"`
	Free        Value                   `json:"free"`
	Over        Value                   `json:"over"`
	Overbooked  bool                    `json:"overbooked"`
	Goals       []goals.ReservationGoal `json:"goals"`
	Error       string                  `json:"error,omitempty"`
}

func reservation(v goals.AccountReservation) ReservationFact {
	f := ReservationFact{AccountID: v.Account.ID, AccountName: v.Account.Name, Exists: v.Exists, Archived: v.Account.Archived, Known: v.Known, Overbooked: v.Overbooked, Goals: v.GoalRefs}
	if v.Err != nil {
		f.Error = v.Err.Error()
	}
	f.Balance = value(v.Balance, nil)
	f.Balance.Known = v.BalanceKnown
	if !v.BalanceKnown {
		f.Balance = Value{Error: f.Error}
	}
	f.Reserved = value(v.Reserved, nil)
	if !v.ReservedKnown {
		f.Reserved = Value{Error: f.Error}
	}
	if v.Known {
		f.Free, f.Over = value(v.Free, nil), value(v.Over, nil)
	} else {
		f.Free, f.Over = Value{Error: f.Error}, Value{Error: f.Error}
	}
	return f
}

// Reservations returns deterministic account positions, including missing or
// archived sources referenced by any goal. No funding builder is needed here.
func (p *Planning) Reservations(accountID string) ([]ReservationFact, error) {
	index := p.positions()
	if accountID != "" {
		v, ok := index.ByAccount[accountID]
		if !ok {
			return nil, fmt.Errorf("no reservation source with account ID %q", accountID)
		}
		return []ReservationFact{reservation(v)}, nil
	}
	out := make([]ReservationFact, 0, len(index.ByAccount))
	for _, v := range index.ByAccount {
		out = append(out, reservation(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out, nil
}

// AccountFact contains useful profile details and independently checked values.
type AccountFact struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	OwnerID            string               `json:"owner_id"`
	Scope              domain.Scope         `json:"scope"`
	Class              domain.AccountClass  `json:"class"`
	Type               domain.AccountType   `json:"type"`
	Currency           string               `json:"currency"`
	Institution        string               `json:"institution"`
	Archived           bool                 `json:"archived"`
	IncludedInNetWorth bool                 `json:"included_in_net_worth"`
	LiabilitySign      domain.LiabilitySign `json:"liability_sign,omitempty"`
	Liquidity          liquidity.Class      `json:"liquidity,omitempty"`
	LockUntil          time.Time            `json:"lock_until"`
	BalanceAsOf        time.Time            `json:"balance_as_of"`
	ObservedAt         time.Time            `json:"observed_at"`
	Balance            Value                `json:"balance"`
	BaseBalance        Value                `json:"base_balance"`
	CreditLimit        Amount               `json:"credit_limit"`
	MinimumPayment     Amount               `json:"minimum_payment"`
	MonthlySavings     Amount               `json:"monthly_savings"`
	InterestRateAPR    *float64             `json:"interest_rate_apr"`
	APY                float64              `json:"apy"`
	Reservation        ReservationFact      `json:"reservation"`
}

// Account reads one exact ID. Unknown FX only blocks BaseBalance, not the native balance.
func (p *Planning) Account(id string) (AccountFact, error) {
	a, err := p.account(id)
	if err != nil {
		return AccountFact{}, err
	}
	bal, err := ledger.Balance(a, p.in.Transactions)
	f := AccountFact{ID: a.ID, Name: a.Name, OwnerID: a.OwnerID, Scope: a.Scope, Class: a.Class, Type: a.Type, Currency: a.Currency, Institution: a.Institution, Archived: a.Archived, IncludedInNetWorth: a.CountsInNetWorth(), LiabilitySign: a.WithLiabilitySign().LiabilitySign, LockUntil: a.LockUntil, BalanceAsOf: a.BalanceAsOf, ObservedAt: p.in.Now, Balance: value(bal, err), CreditLimit: amount(a.CreditLimit), MinimumPayment: amount(a.MinPayment), MonthlySavings: amount(a.MonthlySavings), InterestRateAPR: a.InterestRateAPR, APY: a.APY}
	if !a.IsLiability() {
		f.Liquidity = liquidity.Of(a, p.in.Now)
	}
	if err == nil {
		converted, fxerr := p.in.Rates.Convert(bal, p.base())
		f.BaseBalance = value(converted, fxerr)
	} else {
		f.BaseBalance = value(money.Money{}, err)
	}
	f.Reservation = reservation(p.positions().ByAccount[id])
	return f, nil
}

// DriverFact is one canonical account projection cash flow.
type DriverFact struct {
	Label  string    `json:"label"`
	Date   time.Time `json:"date"`
	Amount Amount    `json:"amount"`
}

// ProjectionFact explicitly identifies the current balance and exact inclusive
// occurrence window. This is a forecast from the current ledger, not a historical balance.
type ProjectionFact struct {
	AccountID   string       `json:"account_id"`
	AsOf        time.Time    `json:"as_of"`
	Through     time.Time    `json:"through"`
	ObservedAt  time.Time    `json:"observed_at"`
	HorizonDays int          `json:"horizon_days"`
	Known       bool         `json:"known"`
	Start       Value        `json:"start"`
	Low         Value        `json:"low"`
	End         Value        `json:"end"`
	LowDate     time.Time    `json:"low_date"`
	Drivers     []DriverFact `json:"drivers"`
	Error       string       `json:"error,omitempty"`
}

// Project calls the shared checked projection helper and excludes paused rules.
// Neither an invalid balance nor arithmetic overflow is reported as a flat zero.
func (p *Planning) Project(id string, asOf time.Time, days int) (ProjectionFact, error) {
	f := ProjectionFact{AccountID: id, AsOf: asOf, ObservedAt: p.in.Now, HorizonDays: days, Drivers: []DriverFact{}}
	if days < 1 || days > 366 {
		return f, fmt.Errorf("horizon_days must be between 1 and 366")
	}
	f.Through = asOf.AddDate(0, 0, days)
	a, err := p.account(id)
	if err != nil {
		return f, err
	}
	bal, err := ledger.Balance(a, p.in.Transactions)
	var projected acctproject.Projection
	if err == nil {
		active := make([]domain.Recurring, 0, len(p.in.Recurring))
		for _, r := range p.in.Recurring {
			if r.Active() {
				if r.AccountID == id && !r.Amount.IsZero() {
					if err = projectionSchedule(r, asOf, days); err != nil {
						break
					}
				}
				active = append(active, r)
			}
		}
		if err == nil {
			projected, err = accountview.PrepareProjectionInputs(active).Project(id, bal, asOf, days)
		}
	}
	if err != nil {
		f.Error = err.Error()
		f.Start, f.Low, f.End = value(bal, err), value(bal, err), value(bal, err)
		return f, nil
	}
	f.Known = true
	f.Start, f.Low, f.End = value(money.New(projected.Start, a.Currency), nil), value(money.New(projected.Low, a.Currency), nil), value(money.New(projected.End, a.Currency), nil)
	f.LowDate = projected.LowDate
	for _, d := range projected.Drivers {
		f.Drivers = append(f.Drivers, DriverFact{d.Label, d.Date, amount(money.New(d.Amount, a.Currency))})
	}
	return f, nil
}

// projectionSchedule guards the shared helper's 400-step safety bound. A bounded
// helper is safe to execute, but a truncated financial projection is not complete.
func projectionSchedule(r domain.Recurring, asOf time.Time, days int) error {
	if r.NextDue.IsZero() || !validCadence(r.Cadence) {
		return fmt.Errorf("account projection: recurring %q has an unavailable schedule", r.ID)
	}
	due, end := r.NextDue, asOf.AddDate(0, 0, days)
	for i := 0; i < 400 && !due.After(end); i++ {
		next := r.Cadence.Next(due)
		if !next.After(due) {
			return fmt.Errorf("account projection: recurring %q does not advance", r.ID)
		}
		due = next
	}
	if !due.After(end) {
		return fmt.Errorf("account projection: recurring %q exceeds the 400-occurrence preparation limit", r.ID)
	}
	return nil
}

// GoalFact uses canonical kind-aware progress and target-currency coverage.
type GoalFact struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Kind                  domain.GoalKind `json:"kind"`
	OwnerID               string          `json:"owner_id"`
	Scope                 domain.Scope    `json:"scope"`
	Archived              bool            `json:"archived"`
	Paused                bool            `json:"paused"`
	PausedUntil           time.Time       `json:"paused_until"`
	TargetDate            time.Time       `json:"target_date"`
	ObservedAt            time.Time       `json:"observed_at"`
	Known                 bool            `json:"known"`
	Complete              bool            `json:"complete"`
	State                 goals.GoalState `json:"state"`
	Pace                  goals.Pace      `json:"pace"`
	Progress              goals.Progress  `json:"progress"`
	TasksDone             int             `json:"tasks_done"`
	TasksTotal            int             `json:"tasks_total"`
	Target                *Amount         `json:"target,omitempty"`
	Saved                 Value           `json:"saved"`
	SetAside              Value           `json:"set_aside"`
	Covered               Value           `json:"covered"`
	Remaining             Value           `json:"remaining"`
	CoveragePercent       *int            `json:"coverage_percent"`
	MonthlyNeeded         Value           `json:"monthly_needed"`
	Assignment            Value           `json:"monthly_assignment"`
	ProjectedFinish       *time.Time      `json:"projected_finish"`
	PlanError             string          `json:"plan_error,omitempty"`
	UnavailableCurrencies []string        `json:"unavailable_currencies,omitempty"`
	Error                 string          `json:"error,omitempty"`
}

// Goals reads one ID or lists goals in stored order. Archived goals are included
// only when requested, except that an explicit ID always retrieves its record.
func (p *Planning) Goals(id string, includeArchived bool) ([]GoalFact, error) {
	if p.tasks == nil {
		p.tasks = goals.BuildTaskIndex(p.in.Tasks)
	}
	out := []GoalFact{}
	for _, g := range p.in.Goals {
		if id != "" && g.ID != id || id == "" && g.Archived && !includeArchived {
			continue
		}
		t := p.tasks[g.ID]
		e := goals.EvaluateGoal(g, t, p.in.Rates, p.in.Now)
		f := GoalFact{ID: g.ID, Name: g.Name, Kind: g.EffectiveKind(), OwnerID: g.OwnerID, Scope: g.Scope, Archived: g.Archived, Paused: g.IsPaused(p.in.Now), PausedUntil: g.PausedUntil, TargetDate: g.TargetDate, ObservedAt: p.in.Now, Known: e.Err == nil, Complete: e.Complete, State: e.State, Pace: e.Pace, Progress: e.Progress, TasksDone: t.Done, TasksTotal: t.Total, UnavailableCurrencies: e.Coverage.UnavailableCurrencies}
		if g.IsFinancial() {
			target := amount(g.TargetAmount)
			f.Target = &target
			if e.Coverage.SavedKnown {
				f.Saved = value(e.Coverage.Saved, nil)
			} else {
				f.Saved = value(money.Money{}, e.Err)
			}
			f.SetAside, f.Covered, f.Remaining = value(e.Coverage.SetAside, e.Err), value(e.Coverage.Covered, e.Err), value(e.Coverage.Remaining, e.Err)
			if e.Err == nil {
				percent := e.Coverage.CoveragePercent
				f.CoveragePercent = &percent
			}
			if e.HasMonthlyNeeded {
				f.MonthlyNeeded = value(e.MonthlyNeeded, nil)
			}
			m, ok, err := e.Assignment(g, p.in.Rates)
			if err != nil {
				f.PlanError = err.Error()
				f.Assignment = value(m, err)
			} else if ok {
				f.Assignment = value(m, nil)
				finish, known, err := e.Project(g, m, p.in.Now)
				if err != nil {
					f.PlanError = err.Error()
				} else if known {
					f.ProjectedFinish = &finish
				}
			}
		}
		if e.Err != nil {
			f.Error = e.Err.Error()
		}
		out = append(out, f)
	}
	if id != "" && len(out) == 0 {
		return nil, fmt.Errorf("no goal with ID %q", id)
	}
	return out, nil
}

// FundingFact is a read-only additional reservation proposal, not a transfer.
type FundingFact struct {
	Goal                  GoalFact          `json:"goal"`
	Known                 bool              `json:"known"`
	Suggestion            *FundingSource    `json:"suggestion"`
	Sources               []ReservationFact `json:"sources"`
	Incomplete            bool              `json:"incomplete"`
	UnavailableCurrencies []string          `json:"unavailable_currencies,omitempty"`
	SourceIssues          []string          `json:"source_issues,omitempty"`
}

// FundingSource identifies the account and amount in the goal's target currency.
type FundingSource struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Amount      Amount `json:"amount"`
}

// Funding reuses the reservation index and shared quick-funding calculation.
// No suggestion alone never proves that money is available or unavailable.
func (p *Planning) Funding(id string) (FundingFact, error) {
	if id == "" {
		return FundingFact{}, fmt.Errorf("provide a goal_id")
	}
	gs, err := p.Goals(id, true)
	if err != nil {
		return FundingFact{}, err
	}
	index := p.positions()
	f := FundingFact{Goal: gs[0], Known: gs[0].Known && !index.Incomplete, Incomplete: index.Incomplete, UnavailableCurrencies: index.ExcludedCurrencies}
	f.Sources, _ = p.Reservations("")
	for _, a := range p.in.Accounts {
		if !goalview.EarmarkSource(a) || !goalview.LiquidSource(a.Type) {
			continue
		}
		position := index.ByAccount[a.ID]
		if !position.Known {
			f.Known = false
			f.SourceIssues = append(f.SourceIssues, "Account "+a.ID+": reservation headroom is unavailable.")
			continue
		}
		if position.Free.Amount > 0 && gs[0].Target != nil {
			if _, err := p.in.Rates.Convert(position.Free, gs[0].Target.Currency); err != nil {
				f.Known = false
				f.SourceIssues = append(f.SourceIssues, "Account "+a.ID+": "+err.Error())
			}
		}
	}
	funding := goalview.BuildFundingWithReservations(p.in.Goals, p.in.Accounts, index, p.in.Rates, p.base())
	if v, ok := funding.Suggestions[id]; ok {
		f.Suggestion = &FundingSource{v.AccountID, v.AccountName, amount(v.Amount)}
	}
	return f, nil
}

// RecurringFact is one stored schedule, with explicit paused/source status.
type RecurringFact struct {
	ID                string                  `json:"id"`
	Label             string                  `json:"label"`
	Amount            Amount                  `json:"amount"`
	Cadence           domain.RecurringCadence `json:"cadence"`
	NextDue           time.Time               `json:"next_due"`
	AccountID         string                  `json:"account_id"`
	AccountName       string                  `json:"account_name"`
	AccountExists     bool                    `json:"account_exists"`
	AccountArchived   bool                    `json:"account_archived"`
	CategoryID        string                  `json:"category_id"`
	Paused            bool                    `json:"paused"`
	Autopost          bool                    `json:"autopost"`
	Autopay           bool                    `json:"autopay"`
	TransferAccountID string                  `json:"transfer_account_id,omitempty"`
	Error             string                  `json:"error,omitempty"`
}

// OccurrenceFact is a scheduled signed flow. Paid is filled by the live adapter;
// no aggregate mixes currencies or double-counts liability statement bills.
type OccurrenceFact struct {
	RecurringID string    `json:"recurring_id"`
	AccountID   string    `json:"account_id"`
	Due         time.Time `json:"due"`
	Amount      Amount    `json:"amount"`
	Paid        bool      `json:"paid"`
}

// RecurringResult includes schedule definitions and a paginated half-open
// calendar window [From, Before). Paused definitions have no active occurrences.
type RecurringResult struct {
	From            time.Time        `json:"from"`
	Before          time.Time        `json:"before"`
	ObservedAt      time.Time        `json:"observed_at"`
	Definitions     []RecurringFact  `json:"definitions"`
	DefinitionCount int              `json:"definition_count"`
	PausedExcluded  int              `json:"paused_excluded"`
	Occurrences     []OccurrenceFact `json:"occurrences"`
	Total           int              `json:"total"`
	NextOffset      *int             `json:"next_offset"`
	Incomplete      bool             `json:"incomplete"`
}

// Recurring returns canonical cadence dates without bill magnitudes: signed
// amounts retain MinInt64 safely. It describes recurring rules, not the combined
// deduplicated bill/statement obligation list. Unknown or too-stale schedules are
// explicit errors, rather than silently returning an empty upcoming list.
func (p *Planning) Recurring(id, accountID string, includePaused bool, from, before time.Time, offset, limit int) (RecurringResult, error) {
	from, before = dateutil.CalendarDayUTC(from), dateutil.CalendarDayUTC(before)
	f := RecurringResult{From: from, Before: before, ObservedAt: p.in.Now, Definitions: []RecurringFact{}, Occurrences: []OccurrenceFact{}}
	if !before.After(from) || before.After(from.AddDate(0, 0, 366)) {
		return f, fmt.Errorf("provide a nonempty recurring window no longer than 366 days")
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return f, fmt.Errorf("offset must be nonnegative and limit between 1 and 100")
	}
	if accountID != "" {
		if _, err := p.account(accountID); err != nil {
			return f, err
		}
	}
	occurrences := []OccurrenceFact{}
	matched := false
	for _, r := range p.in.Recurring {
		if id != "" && r.ID != id || accountID != "" && r.AccountID != accountID {
			continue
		}
		matched = true
		if r.Paused && !includePaused {
			f.PausedExcluded++
			continue
		}
		a, err := p.account(r.AccountID)
		d := RecurringFact{ID: r.ID, Label: r.Label, Amount: amount(r.Amount), Cadence: r.Cadence, NextDue: r.NextDue, AccountID: r.AccountID, AccountName: a.Name, AccountExists: err == nil, AccountArchived: a.Archived, CategoryID: r.CategoryID, Paused: r.Paused, Autopost: r.Autopost, Autopay: r.Autopay}
		if err != nil {
			d.Error = err.Error()
		}
		if destination, ok := txnclassify.RecurringLiabilityDestination(r, p.in.Accounts); ok {
			d.TransferAccountID = destination.ID
		}
		if !validCadence(r.Cadence) || r.NextDue.IsZero() {
			d.Error = "The recurring schedule is unavailable: missing due date or unsupported cadence."
		}
		var dates []time.Time
		if !r.Paused && d.Error == "" {
			var err error
			dates, err = recurringDates(r, from, before)
			if err != nil {
				d.Error = err.Error()
			}
		}
		if d.Error != "" {
			f.Incomplete = true
		}
		f.Definitions = append(f.Definitions, d)
		for _, due := range dates {
			occurrences = append(occurrences, OccurrenceFact{RecurringID: r.ID, AccountID: r.AccountID, Due: due, Amount: amount(r.Amount)})
		}
	}
	if id != "" && !matched {
		return f, fmt.Errorf("no recurring item with ID %q", id)
	}
	sort.SliceStable(occurrences, func(i, j int) bool {
		if !occurrences[i].Due.Equal(occurrences[j].Due) {
			return occurrences[i].Due.Before(occurrences[j].Due)
		}
		return occurrences[i].RecurringID < occurrences[j].RecurringID
	})
	f.DefinitionCount = len(f.Definitions)
	f.Total = len(occurrences)
	if offset < f.Total {
		end := min(f.Total, offset+limit)
		f.Occurrences = occurrences[offset:end]
		if end < f.Total {
			f.NextOffset = &end
		}
	}
	return f, nil
}

// recurringDates walks forward from the saved scheduling cursor using the domain
// cadence. Walking backward and forward is not equivalent for clamped month-end
// anchors (October31 -> September30 -> October30), so it must never reconstruct
// the cursor that way. Calendar-day conversion matches the bills occurrence API.
func recurringDates(r domain.Recurring, from, before time.Time) ([]time.Time, error) {
	due := dateutil.CalendarDayUTC(r.NextDue)
	var out []time.Time
	for i := 0; i < 4000; i++ {
		if !due.Before(before) {
			return out, nil
		}
		if !due.Before(from) {
			out = append(out, due)
		}
		next := dateutil.CalendarDayUTC(r.Cadence.Next(due))
		if !next.After(due) {
			return nil, fmt.Errorf("The recurring schedule is unavailable: its cadence does not advance.")
		}
		due = next
	}
	if due.Before(before) {
		return nil, fmt.Errorf("The recurring schedule is unavailable: more than 4000 cadence steps are required.")
	}
	return out, nil
}
func validCadence(c domain.RecurringCadence) bool {
	switch c {
	case domain.CadenceDaily, domain.CadenceWeekly, domain.CadenceBiweekly, domain.CadenceMonthly, domain.CadenceSemimonthly, domain.CadenceQuarterly, domain.CadenceYearly:
		return true
	}
	return false
}
