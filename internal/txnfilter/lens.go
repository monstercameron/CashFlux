// SPDX-License-Identifier: MIT

package txnfilter

import "strings"

// WithOwnerLens layers an ambient member perspective (the top bar's "View as"
// control) onto the criteria and reports whether it actually applied.
//
// The lens is AMBIENT: it belongs to the whole app, not to this page's filter, so
// it must never be written into the persisted criteria. It applies only when the
// user's own filter names no member — an explicit per-page member filter is a
// deliberate, local decision and always wins over the ambient perspective.
//
// applied is the signal the toolbar needs to render the lens as its own removable
// chip: without it the page cannot tell "Everyone" from "scoped to Priya, but the
// page filter already said Marcus", and would offer a chip whose ✕ removes nothing
// (C574 — every clear control must state exactly what it removes, and remove it).
func (c Criteria) WithOwnerLens(owners []string) (out Criteria, applied bool) {
	clean := make([]string, 0, len(owners))
	for _, o := range owners {
		if s := strings.TrimSpace(o); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) == 0 {
		return c, false
	}
	if len(c.SelectedValues(FieldMember)) > 0 {
		return c, false
	}
	c.Members = strings.Join(clean, ",")
	return c, true
}
