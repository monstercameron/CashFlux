// SPDX-License-Identifier: MIT

// Package appstate — occurrence paid-status persistence (C154).
//
// Per-bill paid status is keyed by (billID + due-date) and stored as a JSON
// map in the shared KV table under cashflux:occurrences:paid. This lets
// paid/autopay state survive reloads without touching the store schema.
//
// The key format mirrors domain.OccurrenceKey: "billID|YYYY-MM-DD".
// For account-derived bills billID is the account ID; for recurring-derived
// bills billID is "recurring:<recurringID>".
package appstate

import (
	"encoding/json"
	"time"
)

const occurrencesPaidKVKey = "cashflux:occurrences:paid"

// loadPaidMap reads the persisted paid-status map from KV storage. It returns
// an empty (non-nil) map when no data has been written yet.
func (a *App) loadPaidMap() map[string]int64 {
	raw, ok := a.GetKV(occurrencesPaidKVKey)
	if !ok || raw == "" {
		return map[string]int64{}
	}
	var m map[string]int64
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		a.log.Error("occurrences: corrupt paid map; resetting", "err", err)
		return map[string]int64{}
	}
	return m
}

// savePaidMap writes the paid-status map back to KV storage.
func (a *App) savePaidMap(m map[string]int64) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return a.SetKV(occurrencesPaidKVKey, string(b))
}

// occurrenceKey returns the composite map key for one bill occurrence. billID
// is the account ID or "recurring:<id>"; due is the due date (time-of-day
// ignored).
func occurrenceKey(billID string, due time.Time) string {
	return billID + "|" + due.Format("2006-01-02")
}

// MarkOccurrencePaid records billID/due as paid at now. It is idempotent: a
// repeated call on the same (billID, due) simply refreshes the timestamp.
func (a *App) MarkOccurrencePaid(billID string, due time.Time) error {
	m := a.loadPaidMap()
	m[occurrenceKey(billID, due)] = time.Now().Unix()
	if err := a.savePaidMap(m); err != nil {
		return err
	}
	a.log.Info("occurrence marked paid", "billID", billID, "due", due.Format("2006-01-02"))
	return nil
}

// UnmarkOccurrencePaid removes the paid record for billID/due so the bill
// appears unpaid again. It is a no-op when no record exists.
func (a *App) UnmarkOccurrencePaid(billID string, due time.Time) error {
	m := a.loadPaidMap()
	delete(m, occurrenceKey(billID, due))
	if err := a.savePaidMap(m); err != nil {
		return err
	}
	a.log.Info("occurrence unmarked paid", "billID", billID, "due", due.Format("2006-01-02"))
	return nil
}

// OccurrencePaid reports whether billID/due has been marked paid. It is safe
// to call from render functions; the underlying KV read is O(1) per call.
func (a *App) OccurrencePaid(billID string, due time.Time) bool {
	m := a.loadPaidMap()
	_, ok := m[occurrenceKey(billID, due)]
	return ok
}
