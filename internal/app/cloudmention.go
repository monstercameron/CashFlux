// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

const cloudMentionDismissedKey = "cashflux:cloud-mention-dismissed"

// CloudMention is a calm, one-time, dismissible banner that introduces the optional
// CashFlux Cloud tier (sync + backup + AI proxy) without nagging (§7.11). It shows
// only on first runs that haven't dismissed it and aren't already syncing — the app
// stays fully free/local whether or not the user ever engages. "Learn more" opens
// the Cloud settings; "Not now" dismisses it for good. Its own component so the
// dismiss + atom hooks stay at a stable render position.
func CloudMention() uic.Node {
	dismissed := uic.UseState(lsGet(cloudMentionDismissedKey) != "")

	// Hide once dismissed, or if Cloud sync is already in use (any sync status set).
	if dismissed.Get() || loadSyncStatus().State != "" {
		return Fragment()
	}

	onDismiss := uic.UseEvent(func() {
		lsSet(cloudMentionDismissedKey, "1")
		dismissed.Set(true)
	})
	onLearn := uic.UseEvent(func() {
		lsSet(cloudMentionDismissedKey, "1")
		dismissed.Set(true)
		ShowUpgradeSheet() // open the benefits/pricing sheet (§7.11 upsell funnel)
	})

	return Div(css.Class("cloud-mention", tw.Flex, tw.FlexCol, tw.Gap1),
		Attr("role", "note"),
		P(css.Class("cloud-mention-title"), uistate.T("cloud.mentionTitle")),
		P(css.Class("cloud-mention-body", tw.Text12, tw.TextDim), uistate.T("cloud.mentionBody")),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
			Button(css.Class("btn", "btn-sm"), Type("button"), OnClick(onLearn), uistate.T("cloud.mentionLearn")),
			Button(css.Class("btn", "btn-sm", tw.TextFaint), Type("button"), OnClick(onDismiss), uistate.T("cloud.mentionDismiss")),
		),
	)
}
