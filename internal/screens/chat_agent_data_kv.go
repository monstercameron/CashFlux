// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/notifyhistory"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// assistantConfigRecord distinguishes effective defaults from persisted input.
// Config is the canonical normalized value used by the corresponding app page.
type assistantConfigRecord[T any] struct {
	ID     string `json:"id"`
	Stored bool   `json:"stored"`
	Config T      `json:"config"`
}

func assistantConfigRows[T any](app *appstate.App, key string, parse func(string) (T, error)) ([]assistantConfigRecord[T], error) {
	raw, present, err := app.Store().GetKV(key)
	if err != nil {
		return nil, fmt.Errorf("configuration source unavailable: %w", err)
	}
	cfg, err := parse(raw)
	if err != nil {
		return nil, err
	}
	return []assistantConfigRecord[T]{{ID: "household", Stored: present && raw != "", Config: cfg}}, nil
}

func assistantKVArray[T any](app *appstate.App, key string) ([]T, error) {
	raw, present, err := app.Store().GetKV(key)
	if err != nil {
		return nil, fmt.Errorf("collection source unavailable: %w", err)
	}
	if !present || raw == "" {
		return []T{}, nil
	}
	var out []T
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("stored collection unavailable: %w", err)
	}
	return out, nil
}

// assistantFinancialKVCollections is an explicit fixed-key whitelist. It never
// invokes the global KV bridge, whose legacy browser migration can write during
// reads, or tolerant UI getters that hide corrupt data behind defaults.
func assistantFinancialKVCollections(app *appstate.App) []assistantdata.Collection {
	feed := assistantdata.NewCollection("notification_feed", "id", func() ([]uistate.FeedItem, error) {
		return assistantKVArray[uistate.FeedItem](app, "cashflux:notify:feed")
	})
	feed.UnixSecondsDateFields = map[string]assistantdata.UnixSecondsDateField{"at": {ZeroIsMissing: true}, "dueAt": {ZeroIsMissing: true}, "snoozedUntil": {ZeroIsMissing: true}}
	feed.Notes = "This is every retained persisted live-feed row, including read and snoozed items; normal app writes retain the newest 50. No UI filters, deduplication, recap or archive synchronization is performed. at, dueAt and snoozedUntil are Unix seconds, not ISO date strings. Stored title/body/reason describe the event when emitted, not a recomputation of current finances."
	history := assistantdata.NewCollection("notification_history", "id", func() ([]notifyhistory.Record, error) {
		raw, present, err := app.Store().GetKV("cashflux:notifhistory")
		if err != nil {
			return nil, fmt.Errorf("notification history source unavailable: %w", err)
		}
		if !present || raw == "" {
			return []notifyhistory.Record{}, nil
		}
		trimmed := strings.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return nil, fmt.Errorf("notification history must be a JSON archive object")
		}
		var archive notifyhistory.Archive
		if err := json.Unmarshal([]byte(raw), &archive); err != nil {
			return nil, fmt.Errorf("notification history unavailable: %w", err)
		}
		return archive.Items, nil
	})
	history.Notes = "This is every retained persisted archive row, including read and dismissed historical alerts. Normal archive writes retain the newest 500. Older pruned history cannot be recovered; the archive may not yet contain the current live feed. No archive synchronization, pruning or other write occurs on read. at is Unix seconds. Recorded messages describe their historical emission, not today's financial state."
	history.UnixSecondsDateFields = map[string]assistantdata.UnixSecondsDateField{"at": {ZeroIsMissing: true}}
	alloc := assistantdata.NewCollection("allocation_config", "id", func() ([]assistantConfigRecord[uistate.AllocConfig], error) {
		return assistantConfigRows(app, "cashflux:allocate:config", uistate.ParseAllocConfig)
	})
	planning := assistantdata.NewCollection("planning_config", "id", func() ([]assistantConfigRecord[uistate.PlanningConfig], error) {
		return assistantConfigRows(app, "cashflux:planning:config", uistate.ParsePlanningConfig)
	})
	bills := assistantdata.NewCollection("bills_smart_config", "id", func() ([]assistantConfigRecord[uistate.BillsSmartConfig], error) {
		return assistantConfigRows(app, "cashflux:bills:smartconfig", uistate.ParseBillsSmartConfig)
	})
	for _, c := range []*assistantdata.Collection{&alloc, &planning, &bills} {
		c.Notes = "config is the page's effective configuration after its canonical defaults and value normalization. stored=false means no persisted override; malformed/wrong-shaped storage is unavailable. All monetary Minor fields use financial_settings.baseCurrency; no conversion or recommendation is performed here."
	}
	bills.Notes += " The payday anchor is financial_preferences.payCycleAnchor."
	pools := assistantdata.NewCollection("investment_pools", "id", func() ([]uistate.InvestPool, error) {
		return assistantKVArray[uistate.InvestPool](app, "cashflux:invest:pools")
	})
	pools.Notes = "These are all persisted investment-pool definitions and exact account memberships, including references to archived or removed accounts. No membership filtering or investment calculation is performed."
	return []assistantdata.Collection{feed, history, alloc, planning, bills, pools}
}
