// SPDX-License-Identifier: MIT

// Package docexpiry turns account documents that carry a renewal / expiry date
// (AC17 — insurance policy, registration, warranty) into due reminders, and decides
// when a reminder should auto-resolve because a newer document of the same kind has
// been filed (the XC8 self-resolving-task pattern, applied to an attach event rather
// than a posting transaction). It is pure and deterministic: the same documents at
// the same clock always yield the same reminders, so it unit-tests on native Go and
// the app layer only reconciles tasks against its output on each store mutation.
package docexpiry

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// DefaultLeadDays is the reminder lead time used when a caller passes a
// non-positive lead: nudge 30 days before the document expires.
const DefaultLeadDays = 30

// Reminder is one document-renewal nudge that is due (its lead window has opened
// and it has not been superseded). Key is a stable identity the app layer uses to
// dedupe the generated task, so re-running the reconcile never spawns a duplicate.
type Reminder struct {
	AccountID  string
	ArtifactID string
	Label      string
	ExpiresAt  time.Time
	DueAt      time.Time // ExpiresAt minus the lead
	Key        string    // stable: "docexpiry:<accountID>:<labelKey-or-artifactID>"
}

// key builds the stable reminder identity. Documents that share a label are the
// same renewable thing, so the key is scoped by label when present (a renewal keeps
// the same key and the same task); label-less documents fall back to the artifact ID.
func key(accountID string, r domain.AccountDocRef) string {
	id := r.LabelKey()
	if id == "" {
		id = r.ArtifactID
	}
	return "docexpiry:" + accountID + ":" + id
}

// superseded reports whether doc i is replaced by a strictly-newer document of the
// same label on the same account. A blank label can never be superseded (no join
// key), so each label-less document stands alone.
func superseded(docs []domain.AccountDocRef, i int) bool {
	k := docs[i].LabelKey()
	if k == "" {
		return false
	}
	for j := range docs {
		if j == i {
			continue
		}
		if docs[j].LabelKey() == k && docs[j].AttachedAt.After(docs[i].AttachedAt) {
			return true
		}
	}
	return false
}

// DueReminders returns the renewal reminders that are DUE for one account's
// documents as of `now`: each document with an ExpiresAt whose lead window has
// opened (now ≥ ExpiresAt − lead) and that has not been superseded by a newer
// same-label document. leadDays ≤ 0 uses DefaultLeadDays. Results are sorted by
// due date (soonest first) for a stable, actionable order.
func DueReminders(accountID string, docs []domain.AccountDocRef, leadDays int, now time.Time) []Reminder {
	if leadDays <= 0 {
		leadDays = DefaultLeadDays
	}
	var out []Reminder
	for i, d := range docs {
		if d.ExpiresAt.IsZero() || d.ArtifactID == "" {
			continue
		}
		if superseded(docs, i) {
			continue
		}
		dueAt := d.ExpiresAt.AddDate(0, 0, -leadDays)
		if now.Before(dueAt) {
			continue // lead window has not opened yet
		}
		out = append(out, Reminder{
			AccountID:  accountID,
			ArtifactID: d.ArtifactID,
			Label:      d.Label,
			ExpiresAt:  d.ExpiresAt,
			DueAt:      dueAt,
			Key:        key(accountID, d),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ExpiresAt.Equal(out[j].ExpiresAt) {
			return out[i].ExpiresAt.Before(out[j].ExpiresAt)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// ActiveKeys returns the set of reminder keys that SHOULD have a live task across
// all accounts as of `now` — the reconcile target. The app layer creates a task for
// any active key it has not already spawned, and auto-resolves (completes) any task
// whose key is no longer active (the document expired-and-was-renewed, or its expiry
// was cleared). This is the AC17 ⇄ XC8 bridge, evaluated on document mutations.
func ActiveKeys(accounts []domain.Account, leadDays int, now time.Time) map[string]Reminder {
	keys := map[string]Reminder{}
	for _, a := range accounts {
		for _, r := range DueReminders(a.ID, a.DocRefs, leadDays, now) {
			keys[r.Key] = r
		}
	}
	return keys
}
