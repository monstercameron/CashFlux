// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"time"

	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// panelFadeDuration is how long a fading panel takes to open or close. It matches the
// .fade-panel timing in the styles.
const panelFadeDuration = 250 * time.Millisecond

// UsePanelFade reports whether a panel should stay in the DOM. It is true while the
// panel is open and for the fade-out after it closes, so closing can animate rather than
// vanish. Reduced motion skips the fade, and the panel leaves at once. Call it
// unconditionally at a stable render position.
func UsePanelFade(open bool) bool {
	mounted := uic.UseState(false)
	reduced := uic.UsePrefersReducedMotion()
	uic.UseEffect(func() func() {
		if open {
			mounted.Set(true)
		}
		return nil
	}, open)
	// Once the panel closes, drop it after its fade. The timer is re-armed whenever open or
	// the reduced-motion preference changes, so a panel reopened mid-fade is never removed.
	delay := panelFadeDuration
	if reduced {
		delay = 0
	}
	uic.UseTimeout(func() {
		if !open {
			mounted.Set(false)
		}
	}, delay, open, reduced)
	return open || mounted.Get()
}

// FadeClass is a class for content that fades and grows in when it opens, and fades out
// when it closes. Pair it with UsePanelFade, which keeps the content mounted for the fade-out.
func FadeClass(base string, open bool) string {
	if open {
		return base + " fade-panel"
	}
	return base + " fade-panel is-leaving"
}
