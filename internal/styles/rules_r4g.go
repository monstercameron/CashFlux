// SPDX-License-Identifier: MIT

package styles

// registerR4G stabilizes the main scroll pane's inline width and suppresses its
// pointer-focus outline while preserving the shared keyboard focus indicator.
func registerR4G() {
	rule("#main.cf-scroll",
		prop("scrollbar-gutter", "stable"),
	)
	rule("#main:focus:not(:focus-visible)",
		prop("outline", "none"),
	)
}
