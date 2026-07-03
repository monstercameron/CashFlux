// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerAssistantSurface holds the agent-first /assistant chat surface rules:
// a two-column layout (the conversation is the page; observations, saved chats
// and pins live in a side rail), the taller thread region, the agent intro, and
// the rail's vertical conversation list. Registered from Register() after the
// generated rules so these win equal-specificity ties.
func registerAssistantSurface() {
	rule(".asst-layout",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 2fr) minmax(0, 1fr)"),
		prop("gap", "1rem"),
		prop("align-items", "start"),
	)
	ruleMedia("(max-width: 1023px)", ".asst-layout",
		prop("grid-template-columns", "minmax(0, 1fr)"),
	)
	rule(".asst-rail",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1rem"),
		prop("min-width", "0"),
	)
	// The conversation thread owns the page's height: tall, scrolling in place,
	// with the composer pinned below it.
	rule(".asst-thread",
		prop("max-height", "60vh"),
		prop("min-height", "12rem"),
	)
	// Agent intro (empty thread): a calm welcome that leads with what the agent
	// can DO, in the display serif the app's heroes use.
	rule(".asst-intro",
		prop("padding", "1rem 0.25rem 0.5rem"),
	)
	rule(".asst-intro-title",
		prop("font-size", "1.35rem"),
		prop("line-height", "1.25"),
		prop("margin-bottom", "0.4rem"),
	)
	rule(".asst-intro-cap",
		prop("display", "flex"),
		prop("gap", "0.6rem"),
		prop("align-items", "flex-start"),
		prop("margin", "0.45rem 0"),
		prop("font-size", "0.9rem"),
	)
	rule(".asst-intro-cap .rec-tag",
		prop("flex", "0 0 auto"),
		prop("margin-top", "0.1rem"),
	)
	// Rail conversation list: a vertical stack of the existing chat pills.
	rule(".asst-convs",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("align-items", "flex-start"),
	)
	rule(".asst-convs .conv-pill",
		prop("max-width", "100%"),
	)
	// Keyless callout inside the intro: the single place the key pitch lives on
	// an empty thread.
	rule(".asst-key-callout",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("margin-top", "0.75rem"),
		prop("padding", "0.6rem 0.8rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "0.75rem"),
		prop("font-size", "0.85rem"),
	)
	rule(".asst-key-callout .btn",
		prop("flex", "0 0 auto"),
	)
	// The demo transcript reads as a demo, not the user's live thread: dashed
	// frame + dimmed bubbles.
	rule(".asst-examples",
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "0.75rem"),
		prop("padding", "0.75rem 0.9rem 0.25rem"),
		prop("opacity", "0.75"),
	)

	// ── The Insights briefing bento surface ──────────────────────────────────
	// Host grid: natural tile heights (the shared .bento fixes row heights for
	// the reconfigurable dashboard; a reading surface flows instead).
	rule(".bento.bento-assistant",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-assistant > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Flagged-activity all-clear state: a calm, positive stamp — the absence of
	// trouble is information, so the tile says so instead of vanishing.
	rule(".ast-clear",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("padding", "1rem 0.25rem"),
	)
	rule(".ast-clear-mark",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "2rem"),
		prop("height", "2rem"),
		prop("flex", "0 0 auto"),
		prop("border-radius", "50%"),
		prop("border", "1px solid var(--up, #54b884)"),
		prop("color", "var(--up, #54b884)"),
		prop("font-size", "0.95rem"),
	)
}
