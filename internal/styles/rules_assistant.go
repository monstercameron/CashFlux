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

	// ── Ask-tab cohesion pass (hub review): the conversation cockpit joins the
	// design system — serif accent-tick titles like every redesigned surface,
	// and the chat card fills the viewport so the composer doesn't strand a
	// void beneath a short thread. ─────────────────────────────────────────────
	rule(".asst-layout .card-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
	)
	rule(".asst-main > .card",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("min-height", "46vh"),
	)
	// EntityListSection flattens fragments, so the thread/composer/keynote are
	// DIRECT card children — the thread alone flexes to anchor the composer.
	rule(".asst-thread",
		prop("flex", "1"),
		prop("min-height", "0"),
	)
	// The flexed card stretches block children; action buttons keep their
	// natural width.
	rule(".asst-main > .card > .btn",
		prop("align-self", "flex-start"),
	)
	rule(".asst-keynote",
		prop("flex", "0 0 auto"),
	)

	// ── The conversation itself, restyled (Cam: "still looks dated"). The agent
	// speaks in the house voice — an accent-ruled editorial column, no gray SMS
	// blob; the user's words sit in a quiet accent-tinted pill; the composer is
	// the elevated centerpiece. ────────────────────────────────────────────────
	rule(".asst-msg-user",
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		prop("border-radius", "14px 14px 4px 14px"),
		prop("padding", "0.55rem 0.9rem"),
	)
	rule(".asst-msg-agent",
		prop("border-left", "2px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("padding", "0.1rem 0 0.1rem 0.95rem"),
	)
	rule(".asst-msg-agent .md",
		prop("line-height", "1.65"),
	)
	rule(".asst-msg-speaker",
		prop("font-size", "0.66rem"),
		prop("letter-spacing", "0.09em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--accent)"),
		prop("opacity", "0.85"),
		prop("margin-bottom", "0.25rem"),
	)
	rule(".asst-thinking",
		prop("font-style", "italic"),
	)
	rule(".asst-composer",
		prop("background", "var(--bg-elev)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "14px"),
		prop("padding", "0.45rem 0.45rem 0.45rem 0.9rem"),
		prop("transition", "border-color 140ms ease, box-shadow 140ms ease"),
	)
	rule(".asst-composer:focus-within",
		prop("border-color", "color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("box-shadow", "0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent)"),
	)
	rule(".asst-composer .field",
		prop("background", "transparent"),
		prop("border", "none"),
		prop("box-shadow", "none"),
		prop("outline", "none"),
		prop("font-size", "0.95rem"),
	)
	rule(".asst-keynote",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("margin-top", "0.75rem"),
		prop("padding", "0.65rem 0.9rem"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "12px"),
	)
	rule(".asst-keynote-text p",
		prop("margin", "0 0 0.2rem"),
		prop("font-size", "0.82rem"),
		prop("color", "var(--fg)"),
		prop("opacity", "0.85"),
	)
	rule(".asst-keynote .btn",
		prop("flex", "0 0 auto"),
		prop("align-self", "center"),
	)
}
