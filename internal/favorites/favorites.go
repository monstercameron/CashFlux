// SPDX-License-Identifier: MIT

// Package favorites is the ordered, capped set of rail destinations a household
// has pinned, and the mapping from those slots to the number keys.
//
// The rail's shortcuts used to be positional: Alt+1..9 went to the first nine
// PRIMARY screens, in registry order, and nobody could change that. The screens
// someone actually lives in are not the same nine for any two households — a
// freelancer is in Invoices and Taxes, a couple splitting bills is in Household
// and Recurring — so the fastest keys in the app were spent on a list nobody
// chose.
//
// Pinning makes that list the user's. The cap is ten because the keyboard has ten
// digits: this is not an arbitrary limit that could later be raised, it is the
// shape of the input device, and Slot/DigitFor below are the only places that
// know 1..9 is followed by 0 rather than 10.
package favorites

// Max is how many destinations can be pinned: one per number key.
const Max = 10

// DigitFor returns the key that jumps to slot i (0-based), and whether that slot
// has one. Slots 0..8 are "1".."9" and slot 9 is "0" — the digit row's own order,
// which is what a hand reaching for the tenth item expects, rather than the "10"
// that a naive count would produce and that no single key can send.
func DigitFor(i int) (string, bool) {
	if i < 0 || i >= Max {
		return "", false
	}
	if i == 9 {
		return "0", true
	}
	return string(rune('1' + i)), true
}

// SlotForDigit is DigitFor's inverse: the 0-based slot a pressed digit selects.
// Returns false for any other key, including a digit outside the row.
func SlotForDigit(d byte) (int, bool) {
	switch {
	case d == '0':
		return 9, true
	case d >= '1' && d <= '9':
		return int(d - '1'), true
	}
	return -1, false
}

// Contains reports whether path is pinned.
func Contains(list []string, path string) bool {
	for _, p := range list {
		if p == path {
			return true
		}
	}
	return false
}

// IndexOf returns the 0-based slot of path, or -1.
func IndexOf(list []string, path string) int {
	for i, p := range list {
		if p == path {
			return i
		}
	}
	return -1
}

// Toggle pins path if it is absent and unpins it if present, returning the new
// list and whether it is now pinned.
//
// A new pin goes on the END. Inserting at the front would be the "most recent
// first" reading, but these slots are muscle memory: pinning an eleventh thing
// must never silently move what Alt+3 does, and appending is the only rule where
// existing slots survive the change.
func Toggle(list []string, path string) ([]string, bool) {
	if path == "" {
		return list, false
	}
	if i := IndexOf(list, path); i >= 0 {
		out := make([]string, 0, len(list)-1)
		out = append(out, list[:i]...)
		out = append(out, list[i+1:]...)
		return out, false
	}
	if len(list) >= Max {
		// Full. The list is returned unchanged and the caller is told the intent was
		// to pin: it asks which slot to give up and finishes with ReplaceAt. Evicting
		// silently would drop someone's pin for a click they may not have understood;
		// refusing outright would leave the eleventh screen unreachable by key.
		return list, true
	}
	out := make([]string, 0, len(list)+1)
	out = append(out, list...)
	return append(out, path), true
}

// ReplaceAt swaps path into slot `at`, returning the new list.
//
// This is the eleventh pin. The list is full, the user has chosen which pinned
// screen to give up, and the newcomer takes THAT SLOT — not the end of the list.
// Position is the whole point: the slots are number keys, so appending the
// newcomer and closing the gap where the old one sat would renumber every slot
// after it, and a swap the user made to reach one screen would silently move
// several others they did not touch.
//
// If path is already pinned elsewhere this is a reorder, and the vacated slot
// closes up behind it — there is no way to swap something with itself, and
// leaving a hole would mean a number key that opens nothing.
func ReplaceAt(list []string, at int, path string) []string {
	if path == "" || at < 0 || at >= len(list) {
		return list
	}
	out := make([]string, len(list))
	copy(out, list)
	if from := IndexOf(out, path); from >= 0 {
		if from == at {
			return out
		}
		// Already pinned: move it, and let everything between shuffle by one rather
		// than leaving a gap.
		return Move(out, from, at)
	}
	out[at] = path
	return out
}

// Full reports whether another pin would be refused.
func Full(list []string) bool { return len(list) >= Max }

// Move relocates the pin at from to index to, shifting the rest. Out-of-range
// indices leave the list untouched, so a drag that ends nowhere is a no-op rather
// than a reordering the user did not intend.
func Move(list []string, from, to int) []string {
	if from < 0 || from >= len(list) || to < 0 || to >= len(list) || from == to {
		return list
	}
	out := make([]string, 0, len(list))
	out = append(out, list...)
	item := out[from]
	out = append(out[:from], out[from+1:]...)
	rest := make([]string, 0, len(out)+1)
	rest = append(rest, out[:to]...)
	rest = append(rest, item)
	rest = append(rest, out[to:]...)
	return rest
}

// Clean drops pins that no longer resolve and trims to Max, preserving order.
//
// It exists because pins outlive what they point at: a custom page is deleted, a
// module is switched off, a route is renamed between releases. Without this a
// dead pin holds a number key that then does nothing — the worst outcome for a
// shortcut, because the user cannot see why it stopped working.
func Clean(list []string, exists func(string) bool) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, p := range list {
		if p == "" || seen[p] || (exists != nil && !exists(p)) {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) == Max {
			break
		}
	}
	return out
}
