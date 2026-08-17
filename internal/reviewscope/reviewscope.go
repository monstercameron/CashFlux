// SPDX-License-Identifier: MIT

// Package reviewscope answers the two "how much does this touch?" questions the
// transaction Review surface has to be able to answer out loud.
//
//   - C554 — the QUEUE scope. Review opened from a ledger filtered to one payee
//     and one month still worked through the global queue, so "review what I am
//     looking at" was not a thing the app could do. Scope names the population:
//     what the ledger currently shows, or everything queued.
//
//   - C653 — the COMMIT scope. One-at-a-time review showed a single charge and a
//     button reading "Categorize & next", and that one click categorized 122
//     charges from the same merchant. Either behavior is defensible; a button
//     that does not say which one it is doing is not. CommitScope names it, and
//     Targets turns the choice into the exact list of transaction ids the write
//     is allowed to touch — so the number on the button and the number written
//     come from the same place and cannot drift.
//
// Both are pure data decisions over ids, so they live here rather than in the
// wasm-only surface, and they are unit-tested on native Go.
package reviewscope

// Scope names which population a Review session works through.
type Scope string

const (
	// ScopeCurrentView reviews only the charges the ledger's own search, filters
	// and period currently show.
	ScopeCurrentView Scope = "view"
	// ScopeEntireQueue reviews everything awaiting review, whatever the ledger is
	// filtered to.
	ScopeEntireQueue Scope = "all"
)

// Normalize maps anything unrecognized (an older persisted value, an empty
// string) onto the entire queue, which is the behavior every build before C554
// had — an unreadable preference must not silently narrow what the user sees.
func Normalize(s Scope) Scope {
	if s == ScopeCurrentView {
		return ScopeCurrentView
	}
	return ScopeEntireQueue
}

// Default picks the scope to open Review with. A user who has filtered the
// ledger and then pressed Review has already said what they want to work on, so
// the filtered view is the honest default; an unfiltered ledger has no narrower
// intent to inherit.
func Default(ledgerFiltered bool) Scope {
	if ledgerFiltered {
		return ScopeCurrentView
	}
	return ScopeEntireQueue
}

// CommitScope names how many charges one confirmation writes.
type CommitScope string

const (
	// CommitCharge writes only the charge on the card. It is the default because
	// it is the scope the card actually depicts.
	CommitCharge CommitScope = "charge"
	// CommitMerchant writes every charge the card says it is standing in for.
	CommitMerchant CommitScope = "merchant"
)

// NormalizeCommit maps anything unrecognized onto the single charge. When in
// doubt a review action must touch LESS, not more.
func NormalizeCommit(s CommitScope) CommitScope {
	if s == CommitMerchant {
		return CommitMerchant
	}
	return CommitCharge
}

// Targets returns the exact transaction ids one confirmation may write, given
// the charge on the card and the group it belongs to.
//
// This is the whole C653 fix in one function: the surface renders its count from
// len(Targets(...)) and the write path is handed the same slice, so "Categorize
// all 122" cannot write 121 or 251. Under CommitCharge the group is ignored
// entirely; under CommitMerchant the current charge is included exactly once even
// if the group list has already dropped it.
func Targets(s CommitScope, currentID string, groupIDs []string) []string {
	if currentID == "" {
		return nil
	}
	if NormalizeCommit(s) == CommitCharge {
		return []string{currentID}
	}
	out := make([]string, 0, len(groupIDs)+1)
	seen := make(map[string]bool, len(groupIDs)+1)
	out, seen[currentID] = append(out, currentID), true
	for _, id := range groupIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// CardIndex resolves which card one-at-a-time review is on, given the merchant
// keys currently in the queue and the position the session remembers.
//
// The key outranks the index, and that is the whole point. The queue is ranked by
// (confidence, group size, key), so categorizing ONE charge shrinks its group and
// can drop it below a same-confidence neighbour — an index that stayed at 3 would
// then be showing a merchant the user has never seen. Holding the key means "stay
// on this merchant until it is cleared".
//
// When the key matches nothing the index takes over, which is right rather than a
// fallback: the merchant has left the queue, so whatever sits at that position now
// IS the next piece of work. An out-of-range index wraps to the top.
func CardIndex(keys []string, cursor int, cursorKey string) int {
	if len(keys) == 0 {
		return 0
	}
	if cursorKey != "" {
		for i, k := range keys {
			if k == cursorKey {
				return i
			}
		}
	}
	if cursor < 0 || cursor >= len(keys) {
		return 0
	}
	return cursor
}

// Restrict returns the ids of queued that also appear in visible, preserving
// queued's order.
//
// The Review queue and the ledger's filtered result are computed by two different
// pieces of code from the same transactions; intersecting the two by id is what
// lets Review inherit the ledger's search, tags, category, member, period and
// money-in/out filters without this package knowing any of them exist.
func Restrict(queued, visible []string) []string {
	if len(queued) == 0 || len(visible) == 0 {
		return nil
	}
	in := make(map[string]bool, len(visible))
	for _, id := range visible {
		in[id] = true
	}
	out := make([]string, 0, len(queued))
	for _, id := range queued {
		if in[id] {
			out = append(out, id)
		}
	}
	return out
}
