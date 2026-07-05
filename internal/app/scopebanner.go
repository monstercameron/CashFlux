// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// ScopeBanner renders a persistent chip below the top bar summarising every
// active scope dimension (#444). It shows nothing when the scope is "all"
// (no restriction — sc.IsAll()) and otherwise renders a human-readable label
// of the form "Viewing: <institutions> · <owners> · <types>" with a Clear /
// View-all button.
//
// Hook positions are unconditionally stable: UseActiveScope() is always hook 1
// and clearScope UseEvent is always hook 2, regardless of whether the banner is
// visible. This satisfies the framework's hooks-must-not-be-in-conditionals rule.
func ScopeBanner() uic.Node {
	sc := uistate.UseActiveScope().Get()

	clearScope := uic.UseEvent(func() {
		uistate.SetActiveScope(scope.ReportScope{})
	})

	if sc.IsAll() {
		return Fragment()
	}

	// Build a slice of per-dimension summaries; each non-empty dimension
	// contributes one element joined by " · " in the banner label.
	var parts []string

	// Institutions: join raw institution names with "/".
	if len(sc.Institutions) > 0 {
		parts = append(parts, strings.Join(sc.Institutions, "/"))
	}

	// Owners: resolve member IDs to display names; use "Shared" for GroupOwnerID.
	if len(sc.Owners) > 0 {
		memberName := map[string]string{}
		if a := appstate.Default; a != nil {
			for _, m := range a.Members() {
				memberName[m.ID] = m.Name
			}
		}
		names := make([]string, 0, len(sc.Owners))
		for _, ownerID := range sc.Owners {
			if ownerID == domain.GroupOwnerID {
				names = append(names, uistate.T("scope.shared"))
			} else if n := memberName[ownerID]; n != "" {
				names = append(names, n)
			} else {
				names = append(names, ownerID)
			}
		}
		parts = append(parts, strings.Join(names, "/"))
	}

	// Types: convert snake_case AccountType → Title Case label.
	if len(sc.Types) > 0 {
		labels := make([]string, 0, len(sc.Types))
		for _, t := range sc.Types {
			labels = append(labels, bannerTypeLabel(t))
		}
		parts = append(parts, strings.Join(labels, "/"))
	}

	label := uistate.T("scope.viewing") + " " + strings.Join(parts, " · ")
	viewAllTitle := uistate.T("scope.viewAllTitle")

	return Div(
		css.Class("scope-banner"),
		Attr("role", "status"),
		Attr("aria-label", uistate.T("scope.bannerLabel")),
		Attr("data-testid", "scope-banner"),
		Span(css.Class("scope-banner-text"), label),
		Button(
			css.Class("scope-banner-btn"),
			Type("button"),
			Attr("title", viewAllTitle),
			Attr("aria-label", viewAllTitle),
			Attr("data-testid", "scope-banner-clear"),
			OnClick(clearScope),
			uistate.T("scope.viewAll"),
		),
	)
}

// bannerTypeLabel converts a snake_case domain.AccountType to a human-readable
// Title Case label (e.g. "credit_card" → "Credit Card").
func bannerTypeLabel(t domain.AccountType) string {
	words := strings.Split(string(t), "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
