// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// assistantPlanningSnapshot reads a fresh, fallible snapshot for EACH tool call.
// Construction of the tool group must never capture account/goal/FX snapshots.
func assistantPlanningSnapshot(app *appstate.App, now time.Time) (*assistantdata.Planning, error) {
	if app == nil {
		return nil, fmt.Errorf("the local workspace is unavailable")
	}
	s := app.Store()
	settings, err := s.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read planning settings: %w", err)
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	in := assistantdata.PlanningInput{Now: now, Rates: currency.Rates{Base: base, Rates: settings.FXRates}}
	in.Accounts, err = s.ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("read planning accounts: %w", err)
	}
	in.Transactions, err = s.ListTransactions()
	if err != nil {
		return nil, fmt.Errorf("read planning transactions: %w", err)
	}
	in.Goals, err = s.ListGoals()
	if err != nil {
		return nil, fmt.Errorf("read planning goals: %w", err)
	}
	in.Tasks, err = s.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("read planning to-dos: %w", err)
	}
	in.Recurring, err = s.ListRecurring()
	if err != nil {
		return nil, fmt.Errorf("read planning recurring schedules: %w", err)
	}
	return assistantdata.NewPlanning(in), nil
}

type assistantPlanningArgs struct {
	AccountID         string `json:"account_id"`
	GoalID            string `json:"goal_id"`
	RecurringID       string `json:"recurring_id"`
	IncludeArchived   bool   `json:"include_archived"`
	IncludePaused     bool   `json:"include_paused"`
	HorizonDays       int    `json:"horizon_days"`
	AsOf              string `json:"as_of"`
	From              string `json:"from"`
	Before            string `json:"before"`
	Offset            int    `json:"offset"`
	Limit             int    `json:"limit"`
	DefinitionsOffset int    `json:"definitions_offset"`
}

func assistantPlanningArgsFrom(raw json.RawMessage) (assistantPlanningArgs, error) {
	var a assistantPlanningArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, fmt.Errorf("couldn't read the planning arguments: %w", err)
	}
	if a.Offset < 0 || a.DefinitionsOffset < 0 {
		return a, fmt.Errorf("offsets must be nonnegative")
	}
	if a.Limit == 0 {
		a.Limit = 50
	}
	if a.Limit < 1 || a.Limit > 100 {
		return a, fmt.Errorf("limit must be between 1 and 100")
	}
	return a, nil
}

func assistantPlanningTime(raw string, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("date %q must be YYYY-MM-DD or RFC3339", raw)
}

type assistantPlanningPage[T any] struct {
	Items      []T  `json:"items"`
	Total      int  `json:"total"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset"`
}

func assistantPlanningPaginate[T any](rows []T, offset, limit int) assistantPlanningPage[T] {
	f := assistantPlanningPage[T]{Items: []T{}, Total: len(rows), Offset: offset}
	if offset < len(rows) {
		end := min(len(rows), offset+limit)
		f.Items = rows[offset:end]
		if end < len(rows) {
			f.NextOffset = &end
		}
	}
	return f
}

func assistantPlanningJSON(app *appstate.App, now time.Time, data any, err error) string {
	out := map[string]any{"read_only": true, "observed_at": now, "money_units": "Amounts use exact integer minor-unit strings with currency and decimal precision."}
	if app != nil {
		out["revision"] = fmt.Sprint(app.Store().Rev())
	}
	if err != nil {
		out["error"] = err.Error()
	} else {
		out["data"] = data
	}
	b, marshalErr := json.Marshal(out)
	if marshalErr != nil {
		b, _ = json.Marshal(map[string]any{"read_only": true, "error": "The planning result could not be encoded: " + marshalErr.Error()})
	}
	return string(b)
}

// agToolsPlanning supplies account, goal and recurring reads. Every handler
// resolves exact IDs against live state; all arithmetic lives in native models.
func agToolsPlanning(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	tool := func(name, description, schema string, run func(*assistantdata.Planning, assistantPlanningArgs, time.Time) (any, error)) chatTool {
		return chatTool{spec: ai.FunctionTool(name, description, json.RawMessage(schema)), run: func(raw json.RawMessage) string {
			now := time.Now()
			a, err := assistantPlanningArgsFrom(raw)
			if err != nil {
				return assistantPlanningJSON(app, now, nil, err)
			}
			p, err := assistantPlanningSnapshot(app, now)
			if err != nil {
				return assistantPlanningJSON(app, now, nil, err)
			}
			data, err := run(p, a, now)
			return assistantPlanningJSON(app, now, data, err)
		}}
	}
	return []chatTool{
		tool("get_account_details", "Read one account by exact account_id: profile, owner, archive/exclusion flags, native and base balance, liability sign, lock/liquidity status, and retained goal reservations. Unknown FX never hides the native balance. This reads current ledger state; use query_data for raw profile/holding fields.",
			`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, _ time.Time) (any, error) {
				return p.Account(a.AccountID)
			}),
		tool("project_account", "Forecast one exact account_id from its CURRENT checked ledger balance using active recurring cash flows. horizon_days is 1–366 (default30). as_of accepts YYYY-MM-DD or RFC3339 and controls the inclusive occurrence window; it does not request a historical starting balance. Returns start/end/low, date, signed drivers and explicit unknown errors.",
			`{"type":"object","properties":{"account_id":{"type":"string"},"horizon_days":{"type":"integer","minimum":1,"maximum":366},"as_of":{"type":"string"}},"required":["account_id"]}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, now time.Time) (any, error) {
				if a.HorizonDays == 0 {
					a.HorizonDays = 30
				}
				asOf, err := assistantPlanningTime(a.AsOf, now)
				if err != nil {
					return nil, err
				}
				return p.Project(a.AccountID, asOf, a.HorizonDays)
			}),
		tool("list_goals", "Read canonical goal facts: kind/task/habit progress, state, pause/archive status, FX-aware saved+earmarked coverage, remaining, required monthly pace and linear plan finish in TARGET currency. Unknown FX/overflow is explicit. Optional exact goal_id retrieves even an archived goal; lists default to active, include_archived retrieves all. Paginate with offset/limit (max100); no changes.",
			`{"type":"object","properties":{"goal_id":{"type":"string"},"include_archived":{"type":"boolean"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":100}}}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, _ time.Time) (any, error) {
				rows, err := p.Goals(a.GoalID, a.IncludeArchived)
				if err != nil {
					return nil, err
				}
				return assistantPlanningPaginate(rows, a.Offset, a.Limit), nil
			}),
		tool("goal_funding_plan", "Read a safe additional EARMARK suggestion for exact goal_id using canonical current balances and all retained reservations, including archived goals. A reservation is virtual, not a transfer or posted contribution. Unknown source/FX headroom is explicit; suggestion null alone does not mean zero funds. Account sources are paginated by offset/limit.",
			`{"type":"object","properties":{"goal_id":{"type":"string"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":100}},"required":["goal_id"]}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, _ time.Time) (any, error) {
				f, err := p.Funding(a.GoalID)
				if err != nil {
					return nil, err
				}
				page := assistantPlanningPaginate(f.Sources, a.Offset, a.Limit)
				f.Sources = page.Items
				return map[string]any{"plan": f, "source_total": page.Total, "source_next_offset": page.NextOffset}, nil
			}),
		tool("list_goal_reservations", "Read per-account reserved/free/overbooked money in each account's native currency, unique claiming goal IDs and explicit unknown errors. Retains archived goals and archived/missing source claims. Optional exact account_id. Valid zero and unavailable amounts differ. Paginate with offset/limit; this is not a funding proposal or a write.",
			`{"type":"object","properties":{"account_id":{"type":"string"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":100}}}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, _ time.Time) (any, error) {
				rows, err := p.Reservations(a.AccountID)
				if err != nil {
					return nil, err
				}
				return assistantPlanningPaginate(rows, a.Offset, a.Limit), nil
			}),
		tool("list_recurring", "Read recurring income/expense RULE definitions with stable IDs, source account/archive status, autopost/autopay/pause flags and signed schedule occurrences. This is not the combined deduplicated bill/statement obligation list and must not be totaled as one. from/before are calendar dates defining [from,before), default today through next30days, maximum366days. Paused definitions require include_paused and have no active occurrences; paused_excluded states omissions. offset/limit paginate occurrences; definitions_offset paginates definitions independently. Both expose totals/next offsets. Paid flags are recorded manual or matched payments, not predicted payments. No writes.",
			`{"type":"object","properties":{"recurring_id":{"type":"string"},"account_id":{"type":"string"},"include_paused":{"type":"boolean"},"from":{"type":"string"},"before":{"type":"string"},"offset":{"type":"integer","minimum":0},"definitions_offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":100}}}`,
			func(p *assistantdata.Planning, a assistantPlanningArgs, now time.Time) (any, error) {
				from, err := assistantPlanningTime(a.From, dateutil.CalendarDayUTC(now))
				if err != nil {
					return nil, err
				}
				before, err := assistantPlanningTime(a.Before, from.AddDate(0, 0, 30))
				if err != nil {
					return nil, err
				}
				f, err := p.Recurring(a.RecurringID, a.AccountID, a.IncludePaused, from, before, a.Offset, a.Limit)
				if err != nil {
					return nil, err
				}
				matched := app.BillMatchPaidOccurrences()
				for i := range f.Occurrences {
					o := &f.Occurrences[i]
					o.Paid = app.OccurrencePaid("recurring:"+o.RecurringID, o.Due) || matched[billmatch.Key(o.RecurringID, o.Due)]
				}
				page := assistantPlanningPaginate(f.Definitions, a.DefinitionsOffset, a.Limit)
				f.Definitions = page.Items
				return map[string]any{"schedules": f, "definitions_next_offset": page.NextOffset}, nil
			}),
	}
}
