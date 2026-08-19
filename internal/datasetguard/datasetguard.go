// SPDX-License-Identifier: MIT

// Package datasetguard decides whether one dataset replacing another is a
// legitimate edit or a destructive overwrite.
//
// It exists because of a real incident (2026-08-19). Two accounts shared one
// browser; local storage bound the loaded dataset to a workspace but never to an
// account, so the second account inherited the first's data, won the
// last-write-wins comparison on timestamp alone, and pushed 432 transactions
// over a stored 528. Nothing anywhere objected: not the client, which believed
// it was syncing its own work, and not the server, which compares only
// timestamps and treats the payload as opaque bytes.
//
// The lesson is narrow and worth encoding: last-write-wins is a rule for
// ordering CONCURRENT edits, not a licence to accept an arbitrary replacement.
// A write that destroys most of a household's records is a different kind of
// event from an edit, and it should have to say so.
//
// The package is pure and platform-independent so both the wasm client and the
// server reach the same verdict from the same code.
package datasetguard

import (
	"encoding/json"
	"fmt"
)

// Counts is the population of a dataset — enough to tell an edit from an
// erasure without the guard needing to understand what any record means.
type Counts struct {
	Transactions int `json:"transactions"`
	Accounts     int `json:"accounts"`
	Categories   int `json:"categories"`
	Budgets      int `json:"budgets"`
	Goals        int `json:"goals"`
}

// Total is every counted record. Used for the "is this dataset empty" question,
// where any one collection being populated means the dataset is not empty.
func (c Counts) Total() int {
	return c.Transactions + c.Accounts + c.Categories + c.Budgets + c.Goals
}

// datasetShape is the subset of the dataset JSON the guard reads. Deliberately
// not store.Dataset: this must parse a payload written by ANY version of the
// client, including a future one with fields this build has never heard of, and
// decoding into the full type would couple the guard to schema churn it has no
// stake in.
type datasetShape struct {
	Transactions []json.RawMessage `json:"transactions"`
	Accounts     []json.RawMessage `json:"accounts"`
	Categories   []json.RawMessage `json:"categories"`
	Budgets      []json.RawMessage `json:"budgets"`
	Goals        []json.RawMessage `json:"goals"`
}

// Inspect counts the records in a dataset payload.
//
// ok is false when the payload cannot be counted — it is encrypted at rest, or
// it is not JSON at all. That is a real and expected case (App Lock seals the
// dataset before it is uploaded), and it is reported rather than guessed at:
// a guard that treated an unreadable payload as "zero records" would refuse
// every encrypted sync, and one that treated it as "fine" would silently stop
// protecting the accounts that most wanted protection.
func Inspect(raw []byte) (Counts, bool) {
	if len(raw) == 0 {
		return Counts{}, false
	}
	// An encrypted envelope is binary-prefixed and never starts with '{'.
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			var shape datasetShape
			if err := json.Unmarshal(raw, &shape); err != nil {
				return Counts{}, false
			}
			return Counts{
				Transactions: len(shape.Transactions),
				Accounts:     len(shape.Accounts),
				Categories:   len(shape.Categories),
				Budgets:      len(shape.Budgets),
				Goals:        len(shape.Goals),
			}, true
		default:
			return Counts{}, false
		}
	}
	return Counts{}, false
}

// Policy is how much loss is tolerated without an explicit confirmation.
type Policy struct {
	// MinRecordsToGuard is the stored transaction count below which the guard
	// stays out of the way. A household with a handful of records is still being
	// set up, and percentage rules on tiny numbers block ordinary editing —
	// deleting 2 of 5 transactions is a 40% "loss" and completely normal.
	MinRecordsToGuard int
	// MaxLossFraction is the share of stored transactions a single write may
	// remove without confirmation. 0.25 means a write that deletes more than a
	// quarter of the household's transactions has to be confirmed.
	MaxLossFraction float64
}

// DefaultPolicy is deliberately permissive. The guard is a backstop against
// catastrophic replacement, not a nanny for ordinary editing — a user pruning a
// few months of records must not be nagged.
//
// The threshold is applied PER COLLECTION, which is what makes a modest number
// sufficient. The incident that prompted this lost 18% of transactions, which is
// within anyone's reasonable editing range and would have to be tolerated on its
// own — but the same write took categories from 52 to 10. A dataset losing four
// fifths of one collection is not an edit, whatever happened to the others, and
// looking at each collection separately catches that without setting the
// transaction limit so low that real deletions trip it.
var DefaultPolicy = Policy{MinRecordsToGuard: 25, MaxLossFraction: 0.25}

// Verdict is the guard's answer.
type Verdict struct {
	// Destructive is true when the write removes more than the policy allows.
	Destructive bool
	// Reason is a plain-English explanation, safe to show a person. Empty when
	// the write is unremarkable.
	Reason string
	// Collection names which population raised the objection ("transactions",
	// "categories", ...). Empty when nothing did.
	Collection string
	// Lost is how many records of that collection the write removes.
	Lost int
}

// Check reports whether replacing prev with next is destructive under policy.
//
// Every counted collection is judged, and the WORST proportional loss decides.
// Judging only transactions would have missed the incident this exists for; a
// write can gut the category tree, the budgets or the accounts while leaving the
// transaction count almost intact, and the result is just as unusable.
//
// Only losses are judged. A write that adds records, or leaves a count alone, is
// never blocked however large it is — growth is what an app is for, and a guard
// that questioned it would be noise.
//
// Both counts must have come from Inspect with ok == true. A caller that could
// not read one of the payloads must not call this: see Inspect's doc comment on
// why an unreadable payload is reported rather than assumed.
func Check(prev, next Counts, policy Policy) Verdict {
	worst := Verdict{}
	worstFraction := policy.MaxLossFraction
	for _, c := range []struct {
		name       string
		prev, next int
	}{
		{"transactions", prev.Transactions, next.Transactions},
		{"accounts", prev.Accounts, next.Accounts},
		{"categories", prev.Categories, next.Categories},
		{"budgets", prev.Budgets, next.Budgets},
		{"goals", prev.Goals, next.Goals},
	} {
		// A collection with few records is still being set up, and percentage
		// rules on small numbers block ordinary editing — removing 2 of 5 is a
		// 40% "loss" and completely normal.
		if c.prev < policy.MinRecordsToGuard || c.next >= c.prev {
			continue
		}
		lost := c.prev - c.next
		fraction := float64(lost) / float64(c.prev)
		if fraction <= worstFraction {
			continue
		}
		worstFraction = fraction
		worst = Verdict{
			Destructive: true,
			Collection:  c.name,
			Lost:        lost,
			Reason: fmt.Sprintf(
				"this would remove %d of %d %s (%.0f%%) from the copy already saved",
				lost, c.prev, c.name, fraction*100),
		}
	}
	return worst
}
