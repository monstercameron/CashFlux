// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/copytext"

// This file wires a feed item's re-renderable copy to the app catalog (C362).
// It is separate from notifyfeed_filter.go because that file carries no build
// constraints — the feed's types and filters are covered by plain `go test`,
// and uistate.T only exists on the wasm build.

// ResolvedTitle / ResolvedBody render an item's copy through the catalog,
// falling back to the English it fired with.
func (f FeedItem) ResolvedTitle() string { return f.TitleText.Resolve(itemTranslator(f.Title)) }

// ResolvedBody is ResolvedTitle's twin for the body line.
func (f FeedItem) ResolvedBody() string { return f.BodyText.Resolve(itemTranslator(f.Body)) }

// itemTranslator wires copytext to the app catalog, keeping the baked English as
// the last resort when neither the key nor the fallback is known.
func itemTranslator(baked string) copytext.Translator {
	return func(key string, args ...any) string {
		if got := T(key, args...); got != "" && got != key {
			return got
		}
		return baked
	}
}

// ResolveText renders any copytext through the app catalog, falling back to the
// English the text was created with (C408). The evidence fields on a
// notification's Reason are copytext for the same reason its title is — a
// persisted alert must be re-renderable in a language chosen after it fired —
// and this is the one place that resolution happens for them.
func ResolveText(t copytext.Text) string { return t.Resolve(itemTranslator(t.Fallback)) }
