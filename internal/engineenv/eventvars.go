// SPDX-License-Identifier: MIT

package engineenv

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// EventDef is a spending event passed in from the wasm layer (which holds the
// persisted events and their transaction membership), so the pure engine can
// expose each event as a set of named variables (TX10). TxnIDs are the ids of the
// transactions mapped to the event.
type EventDef struct {
	Name    string
	VarName string
	TxnIDs  []string
}

// EventVarFields are the per-event metric suffixes exposed on the surface.
var EventVarFields = []string{"total", "spend", "count"}

// EventVarBase pairs an event with the disambiguated variable prefix its values
// are keyed under ("event_<slug>_"). Single source of truth for per-event naming.
type EventVarBase struct {
	Event  EventDef
	Prefix string // e.g. "event_portugal_trip_"
}

// EventVarBases returns one entry per event, in stable order, with same-name
// events disambiguated by a numeric suffix. An explicit VarName wins over the
// display name.
func EventVarBases(events []EventDef) []EventVarBase {
	used := map[string]bool{}
	out := make([]EventVarBase, 0, len(events))
	for _, e := range events {
		src := e.Name
		if e.VarName != "" {
			src = e.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, EventVarBase{Event: e, Prefix: "event_" + slug + "_"})
	}
	return out
}

// EventVarSlug exposes the slugging used for per-event variable names (UI preview).
func EventVarSlug(s string) string { return budgetVarSlug(s) }

// addEventVars exposes each event as event_<slug>_{total,spend,count} variables so
// a formula or widget can reference an event's aggregate by name, like pools and
// goals. total is the net (income − spending) over the event's mapped
// transactions, spend is the magnitude of net outflow (max(0, −total)), and count
// is how many transactions are mapped. Amounts are FX-converted to base major
// units; transfers among member transactions are ignored. Splits do not change
// the whole-transaction total, so this reads each mapped transaction's own amount.
func addEventVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	if len(d.Events) == 0 {
		return
	}
	byID := make(map[string]domain.Transaction, len(d.Transactions))
	for _, t := range d.Transactions {
		byID[t.ID] = t
	}
	for _, base := range EventVarBases(d.Events) {
		var totalBase int64
		count := 0
		for _, tid := range base.Event.TxnIDs {
			t, ok := byID[tid]
			if !ok || t.IsTransfer() {
				continue
			}
			totalBase += toBase(t.Amount.Amount, t.Amount.Currency)
			count++
		}
		out[base.Prefix+"total"] = major(totalBase)
		spend := int64(0)
		if totalBase < 0 {
			spend = -totalBase
		}
		out[base.Prefix+"spend"] = major(spend)
		out[base.Prefix+"count"] = float64(count)
	}
}
