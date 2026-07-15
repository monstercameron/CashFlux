// SPDX-License-Identifier: MIT

package nlfilter

// Route is the decision for a natural-language query: whether the deterministic
// FREE-tier parser already handled it, or whether it should fall back to the
// opt-in AI tier — and, if so, whether that tier is usable yet.
type Route int

const (
	// RouteLocal — the free parser recognized at least one clause; use its
	// Criteria directly. No AI call.
	RouteLocal Route = iota
	// RoutePlainText — nothing structured was recognized and the AI tier is not
	// available; keep the query as a plain text search.
	RoutePlainText
	// RouteNeedsKey — the query is a candidate for the AI fallback, but the AI
	// tier is off or has no provider key configured. The UI explains that turning
	// on the AI feature (and adding a key) unlocks the assistant fallback.
	RouteNeedsKey
	// RouteAI — the AI tier is enabled with a key; the query should route through
	// the assistant, which returns the same Criteria shape. The assistant tool
	// itself belongs to the AG series and is not implemented here.
	RouteAI
)

// Decide picks the route for a query. freeOK is Parse's ok return (did the local
// grammar recognize anything). aiEnabled reports the SMART-T3 (AI) feature toggle;
// hasKey reports whether a provider key is configured. The AI tier is only reached
// for queries the free parser could not structure — it never overrides a clean
// local parse.
//
// This is a routing STUB: RouteAI names the hand-off, but the assistant that would
// fulfill it lives in the AG series. Until then, an AI-eligible query with no key
// resolves to RouteNeedsKey so the UI can explain what to enable.
func Decide(freeOK, aiEnabled, hasKey bool) Route {
	if freeOK {
		return RouteLocal
	}
	if !aiEnabled {
		return RoutePlainText
	}
	if !hasKey {
		return RouteNeedsKey
	}
	return RouteAI
}
