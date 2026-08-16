// SPDX-License-Identifier: MIT

package i18n

// changesetKeys holds English copy for the AG1 changeset review card and the
// AG20 session receipt. Kept in its own file (not en.go) so it doesn't touch the
// concurrent-WIP catalog.
var changesetKeys = Catalog{
	// --- AG1 review card ---
	"changeset.title":       "Proposed changes",
	"changeset.subtitle":    "Review each step, then apply the ones you want.",
	"changeset.itemAria":    "Include this step",
	"changeset.applyAll":    "Apply all (%d)",
	"changeset.applyNone":   "Nothing selected",
	"changeset.dismiss":     "Not now",
	"changeset.dismissAria": "Dismiss these proposed changes",
	// --- receipt after apply ---
	"changeset.receiptTitle": "Applied %d of %d",
	"changeset.receiptOne":   "Applied 1 change",
	// %s = the failing step, %s = the error.
	"changeset.failed":      "Stopped at “%s”: %s. Earlier steps were applied.",
	"changeset.undoAll":     "Undo all",
	"changeset.undoAllAria": "Undo every change the assistant just applied",
	"changeset.undone":      "Undid the assistant's changes.",
	"changeset.applied":     "Applied %d change(s).",
	// --- AG20 cumulative session receipt ---
	"changeset.sessionAria": "What the assistant did in this chat",
	// --- C389 per-action history ---
	// %d = how many individual changes the assistant has made this session.
	"changeset.historySummary": "Every change, one by one (%d)",
	"changeset.undoLast":       "Undo the last one",
	"changeset.openActivity":   "See them in Activity",
	// --- C390 per-conversation model + token cap ---
	"assistant.budgetLabel": "Cap",
	"assistant.budgetPick":  "Cap what this chat may spend in total",
	"assistant.budgetNone":  "No cap",
	// %s = a token figure, already formatted with thousands separators.
	"assistant.budgetOption": "%s tokens",
	"assistant.budgetUsed":   "%s used",
	"assistant.budgetLeft":   "%s left",
	// %s = the cap that was reached. It names the way out, because the cap is the
	// user's own and changing their mind is a legitimate answer.
	"insights.budgetSpent": "This chat has spent its %s-token cap. Raise the cap above, or start a new chat.",
	// --- C391 confidence on inferred findings ---
	// --- G2-C7 chat polish: rename, search, export, edit-and-resend, feedback ---
	"insights.renameChat":        "Rename",
	"insights.renameChatFor":     "Rename the chat “%s”",
	"insights.renameAria":        "Chat name — leave it empty to go back to the automatic name",
	"insights.exportChat":        "Export",
	"insights.exportChatFor":     "Export the chat “%s” as Markdown",
	"insights.searchPlaceholder": "Search your chats",
	"insights.searchAria":        "Search chat names and messages",
	"insights.searchClear":       "Clear the search",
	// %s = what was searched for.
	"insights.searchNone": "Nothing matches “%s”.",
	"insights.editMsg":    "Edit and ask again",
	"insights.editAria":   "Reword this question",
	"insights.editSend":   "Ask again",
	"insights.editNote":   "Asking again replaces the answers below this point.",
	"insights.rateUp":     "This answer was useful",
	"insights.rateDown":   "This answer missed the point",
	// --- C250 the model and the running spend, at the point of asking ---
	"assistant.activeModelTitle": "The model that will answer your next question",
	// %s = a token count; the second %s = an estimated cost.
	"assistant.spentTokens":     "%s tokens this chat",
	"assistant.spentTokensCost": "%s tokens this chat · about %s",
	// --- C247 the key gate answers cost / where / privacy ---
	// %s = a ballpark price, %s = the provider's name.
	"assistant.keyCost":        "About %s a question on %s's cheapest model — you pay them directly, we take nothing.",
	"assistant.keyWhere":       "Get a key at",
	"assistant.keyPrivacy":     "Only what you ask and a summary of your figures is sent. Your transactions stay on this device.",
	"assistant.keyAdd":         "Add a key in Settings",
	"assistant.keyGateAria":    "What adding an API key involves",
	"assistant.confidenceHint": "This one was worked out from a pattern, not read straight off your ledger — worth checking before you act on it.",
}

func init() {
	for k, v := range changesetKeys {
		english[k] = v
	}
}
