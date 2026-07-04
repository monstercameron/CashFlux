// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerAssistantSurface holds the agent-first /assistant Ask surface: a bento
// host whose dominant tile is the conversation CANVAS (a recessed, bottom-anchored
// scrolling region with a centered hero on an empty thread and a docked composer),
// plus the empty-thread hero, the agent/user message treatments, and the rail's
// vertical conversation list. Registered from Register() after the generated rules
// so these win equal-specificity ties.
func registerAssistantSurface() {
	// ── Empty-thread hero: a calm, centered welcome that leads with what the agent
	// can DO, in the display serif the app's heroes use. ────────────────────────
	rule(".asst-hero",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("padding", "0.5rem 0"),
	)
	rule(".asst-intro",
		prop("padding", "0.25rem 0.25rem 0.5rem"),
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
	// Keyless callout inside the intro: the single place the full key pitch lives
	// on an empty thread (cost, privacy, where-to-get).
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

	// ── The Insights briefing bento surface (the sibling "Insights" tab) ────────
	// Host grid: natural tile heights (the shared .bento fixes row heights for the
	// reconfigurable dashboard; a reading surface flows instead).
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

	// ── Message treatments ──────────────────────────────────────────────────────
	// The user's words: a quiet accent-tinted pill, asymmetric radius pointing at
	// the sender's side.
	rule(".asst-msg-user",
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		prop("border-radius", "14px 14px 4px 14px"),
		prop("padding", "0.55rem 0.9rem"),
	)
	// The composer: the elevated centerpiece.
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
	// The keyless truth, mid-conversation: a slim one-line strip (the full pitch
	// lives once in the empty-thread intro callout).
	rule(".asst-keystrip",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-top", "0.6rem"),
		prop("padding", "0.35rem 0.2rem 0"),
	)
	rule(".asst-keystrip-dot",
		prop("width", "7px"),
		prop("height", "7px"),
		prop("border-radius", "50%"),
		prop("flex", "0 0 auto"),
		prop("background", "var(--warn, #d9a23f)"),
	)

	// ── The agent console ───────────────────────────────────────────────────────
	// A flex column: a scrolling canvas that fills the height and a docked composer
	// pinned below it.
	rule(".chat-console",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("height", "calc(100vh - 12.5rem)"),
		prop("min-height", "34rem"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("overflow", "hidden"),
	)
	// The scroll region sizes to its CONTENT (flex-basis auto) so a short thread
	// leaves no void — the console shrinks to fit and the composer sits right
	// beneath the last reply. Only when the content exceeds the console's
	// max-height does this shrink and scroll.
	rule(".chat-scroll",
		prop("flex", "1 1 auto"),
		prop("min-height", "0"),
		prop("overflow-y", "auto"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("padding", "1.4rem 1.1rem 0.8rem"),
	)
	// The reading measure, horizontally centered within the canvas.
	rule(".chat-measure",
		prop("width", "100%"),
		prop("max-width", "46rem"),
		prop("margin-left", "auto"),
		prop("margin-right", "auto"),
	)
	// Agent rows: an accent avatar in a gutter, an editorial reply on a soft raised
	// surface (mirrors the user pill's radius on the opposite corner) so the answer
	// — the whole point of the page — reads as designed, not raw text in a void.
	rule(".chat-row-agent",
		prop("display", "flex"),
		prop("gap", "0.8rem"),
		prop("align-items", "flex-start"),
	)
	rule(".chat-avatar",
		prop("flex", "0 0 auto"),
		prop("width", "1.7rem"),
		prop("height", "1.7rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("border-radius", "50%"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("color", "var(--accent)"),
		prop("font-size", "0.8rem"),
		prop("margin-top", "0.1rem"),
		prop("background", "color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".chat-agent-body",
		prop("line-height", "1.65"),
		prop("min-width", "0"),
		prop("flex", "1"),
		prop("background", "color-mix(in srgb, var(--text) 4%, transparent)"),
		prop("border", "1px solid color-mix(in srgb, var(--text) 7%, transparent)"),
		prop("border-radius", "4px 14px 14px 14px"),
		prop("padding", "0.7rem 0.95rem"),
	)
	rule(".chat-thinking",
		prop("font-style", "italic"),
	)
	rule(".chat-thinking::after",
		prop("content", "'…'"),
		prop("animation", "chat-ellipsis 1.4s infinite steps(4)"),
		prop("display", "inline-block"),
		prop("width", "1.2em"),
		prop("text-align", "left"),
	)
	keyframes("chat-ellipsis",
		at("0%", prop("clip-path", "inset(0 100% 0 0)")),
		at("100%", prop("clip-path", "inset(0 -0.2em 0 0)")),
	)
	// The dock: content scrolls beneath; a soft fade sells the depth.
	rule(".chat-dock",
		prop("flex", "0 0 auto"),
		prop("padding", "0.7rem 1.1rem 0.9rem"),
		prop("border-top", "1px solid var(--border)"),
		prop("background", "linear-gradient(to top, var(--bg-card) 75%, color-mix(in srgb, var(--bg-card) 40%, transparent))"),
	)
	rule(".chat-dock-hint",
		prop("margin", "0.4rem 0.2rem 0"),
		prop("font-size", "0.7rem"),
		prop("letter-spacing", "0.02em"),
	)
	rule(".chat-send",
		prop("flex", "0 0 auto"),
		prop("width", "2.4rem"),
		prop("height", "2.4rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("border-radius", "50%"),
		prop("border", "none"),
		prop("background", "var(--accent)"),
		prop("color", "var(--accent-fg, #08120c)"),
		prop("cursor", "pointer"),
		prop("transition", "transform 120ms ease, filter 120ms ease"),
	)
	rule(".chat-send:hover",
		prop("filter", "brightness(1.1)"),
		prop("transform", "translateY(-1px)"),
	)
	// Status dot on the hero eyebrow above the canvas.
	rule(".chat-status-line",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.45rem"),
		prop("margin", "0 0 0.35rem"),
	)
	rule(".chat-status-dot",
		prop("width", "8px"),
		prop("height", "8px"),
		prop("border-radius", "50%"),
		prop("align-self", "center"),
		prop("flex", "0 0 auto"),
	)
	rule(".chat-status-dot.is-live",
		prop("background", "var(--accent)"),
		prop("box-shadow", "0 0 6px color-mix(in srgb, var(--accent) 70%, transparent)"),
	)
	rule(".chat-status-dot.is-local",
		prop("background", "var(--warn, #d9a23f)"),
	)
	// Starter prompts as inviting tiles (they render inside the hero).
	rule(".chip-suggest",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "12px"),
		prop("padding", "0.6rem 0.9rem"),
		prop("background", "var(--bg-elev)"),
		prop("text-align", "left"),
		prop("transition", "border-color 130ms ease, transform 130ms ease"),
	)
	rule(".chip-suggest:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 50%, var(--border))"),
		prop("transform", "translateY(-1px)"),
	)
	// The intro hero scales up inside the console canvas.
	rule(".chat-measure .asst-intro-title",
		prop("font-size", "1.9rem"),
	)

	// ── The Ask DECK — a bespoke, from-scratch layout (NO bento host, NO Widget
	// tile, NO card rail): a dominant conversation column with its own header bar
	// over the content-height canvas, and a quiet "margin notes" aside for the
	// agent's periphery. ────────────────────────────────────────────────────────
	rule(".ask-deck",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 19rem"),
		prop("gap", "1.75rem"),
		prop("align-items", "start"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-deck",
		prop("grid-template-columns", "minmax(0, 1fr)"),
	)
	rule(".ask-main",
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	// The header bar: status dot + serif agent name on the left, quiet ghost
	// actions on the right, a hairline rule beneath.
	rule(".ask-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("flex-wrap", "wrap"),
		prop("padding-bottom", "0.85rem"),
		prop("margin-bottom", "0.9rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".ask-head-id",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.6rem"),
		prop("min-width", "0"),
	)
	rule(".ask-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.5rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.01em"),
		prop("margin", "0"),
	)
	rule(".ask-status",
		prop("font-size", "0.72rem"),
		prop("letter-spacing", "0.04em"),
		prop("color", "var(--text)"),
		prop("opacity", "0.6"),
	)
	rule(".ask-head-actions",
		prop("flex", "0 0 auto"),
	)
	// The canvas: content-height (grows with the conversation up to a viewport cap,
	// so a short thread strands no void) recessed reading surface.
	rule(".ask-main .chat-console",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "16px"),
		prop("background", "color-mix(in srgb, var(--text) 2.5%, transparent)"),
		prop("height", "auto"),
		prop("max-height", "calc(100vh - 15rem)"),
		prop("min-height", "20rem"),
	)
	rule(".ask-main .chat-dock",
		prop("background", "none"),
	)

	// ── The aside as quiet MARGIN NOTES — chrome-less typographic groups, not
	// tiles. Any legacy .card that lands here (the two detector groups) sheds its
	// card skin and adopts the same bespoke group language. ─────────────────────
	rule(".ask-aside",
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.6rem"),
		prop("padding-top", "0.15rem"),
	)
	rule(".ask-note",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	rule(".ask-note-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.6rem"),
	)
	// Group label + the dissolved card titles share one serif accent-tick language.
	rule(".ask-note-label, .ask-aside .card-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "0.95rem"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.55rem"),
		prop("color", "var(--text)"),
	)
	rule(".ask-note-link",
		prop("font-size", "0.72rem"),
		prop("color", "var(--accent)"),
		prop("white-space", "nowrap"),
		prop("background", "none"),
		prop("border", "none"),
		prop("padding", "0"),
		prop("cursor", "pointer"),
	)
	rule(".ask-note-link:hover",
		prop("text-decoration", "underline"),
	)
	rule(".ask-note-body",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".ask-note-hint",
		prop("font-size", "0.72rem"),
		prop("color", "var(--text)"),
		prop("opacity", "0.5"),
		prop("margin", "0.45rem 0 0"),
	)
	// Dissolve any legacy card chrome inside the aside.
	rule(".ask-aside .card",
		prop("background", "none"),
		prop("border", "none"),
		prop("box-shadow", "none"),
		prop("border-radius", "0"),
		prop("padding", "0"),
	)
	rule(".ask-aside .card-head",
		prop("padding", "0"),
		prop("margin-bottom", "0.5rem"),
		prop("flex-wrap", "wrap"),
		prop("row-gap", "0.2rem"),
	)
	// Calm, index-like rows: hairline separators, quieter type, tight rhythm.
	rule(".ask-aside .row",
		prop("padding", "0.55rem 0"),
		prop("border", "none"),
	)
	rule(".ask-aside .ask-note-body > .row + .row",
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".ask-aside .insight-row",
		prop("padding-block", "0.4rem"),
	)
	rule(".ask-aside .insights-answer",
		prop("font-size", "0.82rem"),
		prop("line-height", "1.5"),
	)
	rule(".ask-aside .insights-answer.line-clamp-3",
		prop("display", "-webkit-box"),
		prop("-webkit-box-orient", "vertical"),
		prop("-webkit-line-clamp", "2"),
		prop("line-clamp", "2"),
		prop("overflow", "hidden"),
	)
	rule(".ask-aside .insights-answer.line-clamp-3 p",
		prop("margin", "0"),
	)
	rule(".ask-aside .row-meta",
		prop("font-size", "0.68rem"),
		prop("opacity", "0.55"),
	)
	rule(".ask-aside .btn-link",
		prop("white-space", "nowrap"),
	)
}
