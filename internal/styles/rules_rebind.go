// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerRebindCard styles the Cloud pane's account/workspace mismatch recovery
// (TODOS.md C694, C697).
//
// It is deliberately louder than the surrounding settings rows without being an
// error: the state it describes is not a failure the app is going to recover
// from on its own, and it needs a decision. So it reads as a panel that has
// something to say — a tinted, bordered block — rather than as red text that
// invites the reader to wait for it to clear.
func registerRebindCard() {
	rule(".rebind-card",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("padding", "0.9rem 1rem"),
		prop("margin", "0.25rem 0 0.5rem"),
		prop("border", "1px solid var(--warn)"),
		prop("border-radius", "12px"),
		// Tinted from the warning tone rather than filled with it: a solid warn
		// background would make the body text fight its own container.
		prop("background", "color-mix(in srgb, var(--warn) 8%, transparent)"),
	)
	rule(".rebind-card h3",
		prop("font-size", "1.05rem"),
		prop("margin", "0"),
	)
	rule(".rebind-card p",
		prop("margin", "0"),
	)

	// The facts block. A definition-list rhythm, so labels and values line up
	// down the left and right edges and can be scanned rather than read.
	rule(".rebind-facts",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("padding", "0.5rem 0.65rem"),
		prop("border-radius", "8px"),
		prop("background", "var(--surface-2)"),
	)
	rule(".rebind-fact",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("font-size", "0.8rem"),
		prop("padding", "0.15rem 0"),
	)
	rule(".rebind-fact-label",
		prop("color", "var(--text-dim)"),
		prop("flex", "0 0 auto"),
	)
	rule(".rebind-fact-value",
		prop("color", "var(--text)"),
		prop("font-weight", "600"),
		prop("text-align", "right"),
		prop("min-width", "0"),
		// Workspace ids and account ids are long opaque strings; they must wrap
		// rather than push the panel wider than the settings column.
		prop("overflow-wrap", "anywhere"),
	)

	rule(".rebind-picker",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.45rem"),
		prop("padding-top", "0.35rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rebind-picker h4",
		prop("margin", "0"),
		prop("font-size", "0.9rem"),
	)
	rule(".rebind-card .row-actions",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.4rem"),
	)
}
