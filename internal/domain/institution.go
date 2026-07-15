// SPDX-License-Identifier: MIT

package domain

import "strings"

// Institution is a financial institution the household banks with — a lightweight
// directory entity (AC10) that accounts reference by InstitutionID. It gives the
// ★★ Multi-Institution Analytics feature a real entity to group by instead of
// matching on the free-text Account.Institution string, colors account rows by
// their institution, and lets the documents drawer (AC8) roll up per institution
// ("everything about Chase in one place"). Only Name is required; the contact
// fields help build the estate emergency pack (AC16).
type Institution struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Color is an optional hex swatch (e.g. "#1a73e8") used to tint account rows and
	// institution chips. Empty means no assigned color.
	Color string `json:"color,omitempty"`
	// Icon is an optional short glyph or icon name shown beside the institution.
	Icon string `json:"icon,omitempty"`
	// SupportPhone is the institution's customer-support phone number (free text so
	// international / extension formats round-trip). Surfaced in the emergency pack.
	SupportPhone string `json:"supportPhone,omitempty"`
	// SupportURL is the institution's support or login landing page. This is a public
	// URL only — never a credential. Surfaced in the emergency pack.
	SupportURL string `json:"supportUrl,omitempty"`
	// Note is free text about the relationship (branch, account manager, reminders).
	// NEVER store logins or passwords here — that is the credential vault's job.
	Note string `json:"note,omitempty"`
}

// TrimmedName returns the institution's display name with surrounding whitespace
// removed, falling back to "Untitled institution" when blank so lists never render
// an empty label.
func (i Institution) TrimmedName() string {
	if n := strings.TrimSpace(i.Name); n != "" {
		return n
	}
	return "Untitled institution"
}

// ReassignAccountsOnInstitutionDelete clears InstitutionID on every account that
// referenced the deleted institution, so accounts fall back to no-institution
// rather than dangling at a missing entity (AC10 reassign-on-delete). It returns
// the accounts that changed (with InstitutionID cleared) so the caller can persist
// exactly those, leaving the rest untouched. Pure and allocation-light: accounts
// that did not reference delID are not included in the result.
func ReassignAccountsOnInstitutionDelete(accounts []Account, delID string) []Account {
	if delID == "" {
		return nil
	}
	var changed []Account
	for _, a := range accounts {
		if a.InstitutionID == delID {
			a.InstitutionID = ""
			changed = append(changed, a)
		}
	}
	return changed
}

// InstitutionByID indexes institutions by ID for O(1) lookup when coloring rows or
// building the emergency pack. Institutions with a blank ID are skipped.
func InstitutionByID(insts []Institution) map[string]Institution {
	m := make(map[string]Institution, len(insts))
	for _, in := range insts {
		if in.ID != "" {
			m[in.ID] = in
		}
	}
	return m
}
