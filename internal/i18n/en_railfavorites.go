// SPDX-License-Identifier: MIT

package i18n

// Strings for the rail's pinned destinations and the pin control.
//
// The vocabulary is "pin", once, everywhere: the control pins, the section is
// Pinned, the shortcut jumps to a pinned destination. An interface that calls the
// same thing a favourite in one place and a pin in another makes the reader work
// out that they are one feature.
func init() {
	for k, v := range map[string]string{
		"rail.pinnedSection": "Pinned",
		// The empty state names the control and what it buys, because a section
		// with nothing in it otherwise reads as something that is broken.
		"rail.pinnedEmpty": "Pin the screens you use most and reach them with %s.",
		"rail.pinnedKeys":  "Alt and 1–9 or 0",
		// Verb first: the label says what a click does, not what the row is.
		"rail.pinAdd":    "Pin %s",
		"rail.pinRemove": "Unpin %s",
		// Announced with the slot so a screen-reader user learns the key without
		// having to find the badge.
		"rail.pinnedAs": "%s, pinned — press Alt and %s to jump here",
		// The eleventh pin does not fail — it asks which slot to give up. The prompt
		// names the incoming screen, because by the time the list is scanned the
		// click that started this is several seconds in the past.
		"rail.swapPrompt":  "Which one should %s replace?",
		"rail.swapHint":    "Pick a pinned screen. It keeps that same number key.",
		"rail.swapCancel":  "Cancel",
		"rail.swapRowAria": "Replace %s with %s, keeping Alt and %s",
		"rail.swapDone":    "%s replaced %s at %s.",
		"rail.pinFullAria": "Pinned is full — choose which screen %s replaces",
		// The section a screen sits in when it is not in one of the named groups.
		"rail.mainSection": "Everyday",
		"rail.jumpHint":    "Alt+%s",
		// Reordering is announced on the row itself: a drag has no keyboard
		// equivalent unless someone is told there is one.
		"rail.reorderHint": "Alt and the up or down arrow moves this",
		"pages.menuFor":    "Options for %s",
	} {
		english[k] = v
	}
}
