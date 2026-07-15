// SPDX-License-Identifier: MIT

// Package savedtxnview is the pure model for named transaction "views" (saved
// filter sets / watchlists, TX3). A SavedTxnView pairs a human name with a
// faithful COPY of the ledger's txnfilter.Criteria, plus an optional amount
// threshold on the view's live total. Everything here is platform-independent
// (no syscall/js) so the model, its validation, its live count/total evaluation,
// and the threshold check are all unit-tested on native Go; the wasm UI and the
// settings-KV persistence layer just hold and serialize SavedTxnView values.
package savedtxnview

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// SavedTxnView is a user-named snapshot of the ledger's filter state. Criteria is
// stored by value (a faithful copy of txnfilter.Criteria), never a reference, so a
// view is a stable definition — re-applying it reconstructs the same scope, and it
// stores the criteria, not copies of the matched transactions.
type SavedTxnView struct {
	// ID is the stable, opaque storage key for this view (a UUID-like string).
	ID string `json:"id"`
	// Name is the human-readable label shown in the Views list.
	Name string `json:"name"`
	// Criteria is the captured filter/sort scope. It is a plain value copy of the
	// live ledger's criteria so the view is independent of the current filter.
	Criteria txnfilter.Criteria `json:"criteria"`
	// Threshold is an optional amount (absolute minor units, base currency) on the
	// view's live total. Zero means no threshold. When the view's total crosses it,
	// the UI surfaces a dismissible notice (see CrossedThreshold / DismissalKey).
	Threshold int64 `json:"threshold,omitempty"`
	// CreatedAt records when the view was first saved (for stable secondary sort).
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// Validate reports why a view can't be saved, or nil when it's valid. A view must
// have a non-empty (trimmed) name; the threshold, when set, must be positive.
func (v SavedTxnView) Validate() error {
	if strings.TrimSpace(v.Name) == "" {
		return errNameRequired
	}
	if v.Threshold < 0 {
		return errThresholdNegative
	}
	return nil
}

// validationError is a small typed error so callers (and tests) can match on the
// specific reason without string comparisons.
type validationError string

func (e validationError) Error() string { return string(e) }

const (
	// ErrNameRequired is returned when a view has a blank name.
	ErrNameRequired = validationError("saved view: name is required")
	// ErrNameTaken is returned when another view already uses the name.
	ErrNameTaken = validationError("saved view: that name is already used")
	// ErrThresholdNegative is returned for a negative threshold.
	ErrThresholdNegative = validationError("saved view: threshold cannot be negative")
	// ErrNotFound is returned when updating a view whose id is not stored.
	ErrNotFound = validationError("saved view: not found")
)

// Package-private aliases keep the exported-name check on the constants above.
var (
	errNameRequired      = ErrNameRequired
	errThresholdNegative = ErrThresholdNegative
)

// AmountFunc yields the base-currency (or any caller-chosen) minor-unit value of a
// transaction for totalling. It lets the pure summary stay currency-agnostic: the
// UI injects an FX conversion, tests inject the identity.
type AmountFunc func(domain.Transaction) int64

// Summary evaluates the view's criteria over txns and returns how many match and
// their signed total (via amount). It reuses the live ledger's filter engine
// (txnfilter.ApplyWithLabels) so a view matches EXACTLY the rows the ledger would
// show for the same criteria. amount may be nil, in which case each transaction's
// own stored minor amount is summed (naive, single-currency).
func (v SavedTxnView) Summary(txns []domain.Transaction, amount AmountFunc) (count int, total int64) {
	rows := txnfilter.Apply(txns, v.Criteria)
	for _, t := range rows {
		if amount != nil {
			total += amount(t)
		} else {
			total += t.Amount.Amount
		}
	}
	return len(rows), total
}

// CrossedThreshold reports whether a live total trips the view's threshold: the
// threshold is set (>0) and the total's magnitude is at least the threshold. The
// magnitude is used so a "spending over $500" view fires on a −$520 total.
func (v SavedTxnView) CrossedThreshold(total int64) bool {
	if v.Threshold <= 0 {
		return false
	}
	if total < 0 {
		total = -total
	}
	return total >= v.Threshold
}

// DismissalKey identifies one threshold-crossed notice: the view id plus its
// current threshold. Encoding the threshold means raising or lowering it produces
// a fresh, undismissed notice, while a repeated crossing at the same threshold
// stays dismissed.
func (v SavedTxnView) DismissalKey() string {
	return v.ID + "@" + strconv.FormatInt(v.Threshold, 10)
}

// List decodes every valid entry in the id→JSON KV map into a SavedTxnView and
// returns them ordered by name (case-insensitively), then by creation time, then
// id, so the list is stable. Corrupt entries are skipped so one bad record never
// hides the rest.
func List(kv map[string]string) []SavedTxnView {
	out := make([]SavedTxnView, 0, len(kv))
	for _, raw := range kv {
		var v SavedTxnView
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Put serializes v into the id→JSON KV map under v.ID (allocating the map if nil)
// and returns it. The caller persists the returned map.
func Put(kv map[string]string, v SavedTxnView) map[string]string {
	if kv == nil {
		kv = make(map[string]string)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return kv
	}
	kv[v.ID] = string(b)
	return kv
}

// Delete removes the entry for id (no-op if absent) and returns the map.
func Delete(kv map[string]string, id string) map[string]string {
	if kv != nil {
		delete(kv, id)
	}
	return kv
}

// NameTaken reports whether any view other than exceptID already uses name
// (compared case-insensitively, trimmed). exceptID lets a rename/update keep its
// own name.
func NameTaken(views []SavedTxnView, name, exceptID string) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, v := range views {
		if v.ID == exceptID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(v.Name)) == want {
			return true
		}
	}
	return false
}
