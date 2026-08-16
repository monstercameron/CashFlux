// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// txnShortLabel is how a transaction names itself inside a sentence — a
// confirmation, a toast, or an accessible label. Description first (it is what the
// ledger row shows), then the payee, and finally a neutral phrase, so a sentence
// like `Exclude "…" from reports?` never renders an empty pair of quotes.
func txnShortLabel(t domain.Transaction) string {
	if d := strings.TrimSpace(t.Desc); d != "" {
		return d
	}
	if p := strings.TrimSpace(t.Payee); p != "" {
		return p
	}
	return uistate.T("transactions.thisTransaction")
}

// CategoryPickOptions builds the option list every category picker in the app
// should use: sorted by catname.Less (so "Item 9" precedes "Item 10") and labelled
// with the full path (`Housing > Mortgage`), not the bare leaf name.
//
// C570: the ledger's review and edit surfaces already qualified their choices this
// way while quick-add, bulk categorize and the split editor listed leaf names only,
// so two categories called "Gas" were indistinguishable at exactly the moments a
// user files a charge. One builder now answers "which categories can I pick, and
// what are they called?" for all of them.
func CategoryPickOptions(cats []domain.Category) []uiw.SelectOption {
	out := make([]uiw.SelectOption, 0, len(cats))
	for _, c := range cats {
		out = append(out, uiw.SelectOption{Value: c.ID, Label: catname.Path(cats, c)})
	}
	// Sorted by the PATH, not the leaf: that is what puts a parent immediately above
	// its children in the list, and it is the string the user actually reads.
	// catname.Less (not a byte compare) keeps "Item 9" ahead of "Item 10".
	sort.SliceStable(out, func(i, j int) bool { return catname.Less(out[i].Label, out[j].Label) })
	return out
}
