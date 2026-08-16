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
		prop("font-size", "var(--type-14)"),
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
		// 2026-07-17 audit: long unbroken strings (URLs, pasted ids) overflowed the
		// pill and read as clipped — break anywhere inside the bubble instead.
		prop("overflow-wrap", "anywhere"),
		prop("min-width", "0"),
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
	// Attached flag-context bubbles sit above the composer — a labelled row of
	// removable chips that read as "carried context", visually distinct from the
	// editable input below (accent-tinted fill + hairline, not a plain field).
	rule(".asst-ctx-row",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("margin-bottom", "0.5rem"),
	)
	rule(".asst-ctx-lead",
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "600"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		prop("margin-right", "0.1rem"),
	)
	rule(".asst-ctx",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("max-width", "100%"),
		prop("padding", "0.28rem 0.35rem 0.28rem 0.6rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("font-size", "var(--type-12)"),
		prop("line-height", "1.2"),
		prop("color", "var(--text)"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 40%, var(--border))"),
	)
	rule(".asst-ctx-icon",
		prop("color", "var(--accent)"),
		prop("opacity", "0.9"),
	)
	rule(".asst-ctx-label",
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
		prop("max-width", "22rem"),
		prop("font-weight", "500"),
	)
	rule(".asst-ctx-x",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "1.15rem"),
		prop("height", "1.15rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("border", "none"),
		prop("background", "transparent"),
		prop("color", "var(--text-faint)"),
		prop("cursor", "pointer"),
		prop("transition", "background 120ms ease, color 120ms ease"),
	)
	rule(".asst-ctx-x:hover",
		prop("background", "color-mix(in srgb, var(--accent) 22%, transparent)"),
		prop("color", "var(--text)"),
	)
	// Remediation action chips: one-click fixes for the attached flag, sitting between
	// the context bubbles and the input. Quiet outline pills that light up on hover —
	// clearly interactive (unlike the context bubbles, which are carried state).
	rule(".asst-remedy-row",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.4rem"),
		prop("margin-bottom", "0.55rem"),
	)
	rule(".asst-remedy",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.32rem"),
		prop("padding", "0.32rem 0.72rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "500"),
		prop("color", "var(--text)"),
		prop("background", "var(--bg-elev)"),
		prop("border", "1px solid var(--border)"),
		prop("cursor", "pointer"),
		prop("transition", "border-color 120ms ease, background 120ms ease, box-shadow 120ms ease"),
	)
	rule(".asst-remedy:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		prop("box-shadow", "0 1px 5px color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".asst-remedy:focus-visible",
		prop("outline", "2px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("outline-offset", "2px"),
	)
	rule(".asst-remedy-icon",
		prop("color", "var(--accent)"),
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
		// 2026-07-17 audit: the old 34rem floor pushed the docked composer below
		// the fold at short desktop heights (720px). 20rem keeps a usable canvas
		// while letting the console actually fit the viewport.
		prop("min-height", "20rem"),
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
		// The assistant's identity mark. Deliberately a rounded SQUARE (not a circle) with a
		// soft accent fill and no button-style outline — so it reads as an AI chip, not the
		// app's round green "+" add-button, whose vocabulary it must not borrow.
		prop("flex", "0 0 auto"),
		prop("width", "1.7rem"),
		prop("height", "1.7rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("border-radius", "7px"),
		prop("color", "var(--accent)"),
		prop("font-size", "var(--type-14)"),
		prop("margin-top", "0.1rem"),
		prop("background", "color-mix(in srgb, var(--accent) 15%, transparent)"),
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
		prop("font-size", "var(--type-11)"),
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
		prop("border-radius", "var(--radius-xl)"),
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
	// The assistant page never scrolls: the deck fills the viewport and the chat + aside
	// scroll INTERNALLY, so clip #main's residual scroll area on this route (a phantom
	// the grid + view-transition leave in scrollHeight below the actual content, which
	// otherwise lets the page scroll ~66px past the fixed layout). data-route is on #main.
	rule("#main[data-route=\"/assistant\"]",
		prop("overflow", "hidden"),
	)
	rule(".ask-deck",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 19rem"),
		// A single row BOUNDED to the deck height (minmax(0,1fr), not content) so the
		// columns are constrained and their overflow scrolls INSIDE them — otherwise the
		// row grows to the aside's tall content and leaks into the page scroll.
		prop("grid-template-rows", "minmax(0, 1fr)"),
		prop("gap", "1.75rem"),
		prop("align-items", "stretch"),
		// Fixed to the available viewport height so the assistant page NEVER scrolls: the
		// chat column and the aside each fill this height and scroll INTERNALLY instead of
		// pushing the page. The offset reserves the topbar, sample banner, and tab bar above.
		prop("height", "calc(100vh - 12.5rem)"),
		prop("min-height", "30rem"),
		prop("overflow", "hidden"),
	)
	ruleContentMax(contentTwoCol, ".ask-deck",
		prop("grid-template-columns", "minmax(0, 1fr)"),
		prop("height", "auto"), // stacked: let the page flow
	)
	// Stacked below the chat on narrow screens: the aside flows with the page (no cap,
	// no inner scroll — that only makes sense beside the chat), and the chat takes a
	// fixed viewport height instead of filling the (now-auto) deck.
	ruleContentMax(contentTwoCol, ".ask-aside",
		prop("overflow-y", "visible"),
		prop("padding-right", "0"),
	)
	ruleContentMax(contentTwoCol, ".ask-main .chat-console",
		prop("flex", "none"),
		prop("height", "calc(100vh - 15rem)"),
	)
	rule(".ask-main",
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("min-height", "0"),
	)
	// The header bar: status dot + serif agent name on the left, quiet ghost
	// actions on the right, a hairline rule beneath.
	rule(".ask-head",
		// Stack: the agent name (with its live-status dot) on line 1, the on-device
		// caption as a quiet subtitle on line 2 — not crammed onto one baseline.
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.15rem"),
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
		prop("font-size", "var(--type-12)"),
		prop("letter-spacing", "0.04em"),
		prop("color", "var(--text)"),
		prop("opacity", "0.6"),
	)
	rule(".ask-head-actions",
		prop("flex", "0 0 auto"),
	)
	// ── The assistant CONTROL CELL: a full-width bordered toolbar that groups the
	// model / thinking / privacy selectors and the New chat / Edit prompt actions into
	// one aligned row — the app's control-cell language, not a loose inline strip. ──
	// With the controls now in their own cell below the title, the title bar drops its
	// divider so the two don't read as stacked rules.
	rule(".ask-head",
		prop("border-bottom", "none"),
		prop("padding-bottom", "0.2rem"),
		prop("margin-bottom", "0.55rem"),
	)
	rule(".ask-controls",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		// The controls are now .todo-ctrl pills (single-height), so center them.
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("width", "100%"),
		prop("padding", "0.7rem 0.85rem"),
		prop("margin-bottom", "0.9rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-xl)"),
		prop("background", "color-mix(in srgb, var(--text) 2.5%, transparent)"),
	)
	// Each captioned field stacks an uppercase label over its control.
	rule(".ask-ctrl-field",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
		prop("min-width", "0"),
	)
	// 2026-07-17 audit: the control cell must never push the page sideways — every
	// child is allowed to shrink/wrap, and anything still wider clips at the cell.
	rule(".ask-controls",
		prop("max-width", "100%"),
		prop("overflow-x", "clip"),
	)
	rule(".ask-controls > *",
		prop("min-width", "0"),
		prop("max-width", "100%"),
	)
	rule(".ask-ctrl-lbl",
		prop("font-size", "0.62rem"),
		prop("font-weight", "600"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.07em"),
		prop("color", "var(--text-faint)"),
	)
	// MODEL + THINKING use the app's STANDARD styled select (.set-input, the Settings
	// control), given a custom chevron so the raw OS-native arrow never shows. Scoped
	// to the cell so the global .set-input elsewhere is untouched. This selector
	// out-specifies both .set-input and the generated bare-<select> rule.
	rule(".ask-controls select.ask-ctrl-sel",
		prop("width", "auto"),
		prop("min-width", "9rem"),
		prop("max-width", "13rem"),
		prop("height", "38px"),
		prop("min-height", "38px"),
		prop("padding", "0 1.9rem 0 0.7rem"),
		prop("border-radius", "var(--radius-lg)"),
		prop("font-size", "var(--type-13)"),
		prop("cursor", "pointer"),
		prop("appearance", "none"),
		prop("-webkit-appearance", "none"),
		prop("-moz-appearance", "none"),
		prop("background-image", "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'%3E%3Cpath d='M2.5 4.5L6 8l3.5-3.5' fill='none' stroke='%238a8a92' stroke-width='1.4' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E\")"),
		prop("background-repeat", "no-repeat"),
		prop("background-position", "right 0.6rem center"),
		prop("background-size", "0.72rem"),
		prop("transition", "border-color 120ms ease, box-shadow 120ms ease"),
	)
	rule(".ask-controls select.ask-ctrl-sel:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)
	rule(".ask-controls select.ask-ctrl-sel:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "1px"),
	)
	// PRIVACY: the value inside its .todo-ctrl pill — a borderless toggle button matching
	// the borderless .todo-select value (the pill supplies the border/background), so it
	// reads as one more standard control rather than a nested button.
	rule(".asst-privacy-btn",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("padding", "0"),
		prop("border", "0"),
		prop("background", "transparent"),
		prop("color", "var(--text)"),
		prop("font-size", "var(--type-14)"),
		prop("font-weight", "500"),
		prop("cursor", "pointer"),
	)
	rule(".asst-privacy-btn svg",
		prop("color", "var(--text-faint)"),
	)
	// Aggregates-only reads in the accent tone so the tighter privacy is legible.
	rule(".asst-privacy-btn.is-aggregates, .asst-privacy-btn.is-aggregates svg",
		prop("color", "var(--accent)"),
	)
	// The action buttons group pushes to the right edge and sits on the fields' baseline.
	rule(".ask-ctrl-actions",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("margin-left", "auto"),
	)
	// The canvas FILLS the viewport: a fixed full height (rather than content-height)
	// so the input dock sits at the bottom of the screen and the thread scrolls inside
	// it — the chat surface owns the vertical space instead of floating mid-page on a
	// short thread. The offset reserves room for the topbar, the tab bar, and the
	// ask-head above it.
	rule(".ask-main .chat-console",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "16px"),
		prop("background", "color-mix(in srgb, var(--text) 2.5%, transparent)"),
		// Fill the deck row below the ask-head; the thread scrolls inside, the dock pins.
		prop("flex", "1 1 auto"),
		prop("min-height", "0"),
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
		// Fills the deck's fixed height (grid stretch) and scrolls INTERNALLY, so a long
		// list of flagged items / pins / saved chats scrolls in place instead of pushing
		// the page. min-height:0 lets it shrink below its content so overflow engages; the
		// right padding keeps rows clear of the scrollbar.
		prop("min-height", "0"),
		prop("overflow-y", "auto"),
		prop("padding-right", "0.5rem"),
		prop("scrollbar-width", "thin"),
		prop("scrollbar-color", "color-mix(in srgb, var(--text) 22%, transparent) transparent"),
	)
	// A neat, unobtrusive scrollbar for the aside (WebKit): a thin rounded thumb inset
	// from the edge, an invisible track, brightening on hover.
	rule(".ask-aside::-webkit-scrollbar",
		prop("width", "9px"),
	)
	rule(".ask-aside::-webkit-scrollbar-track",
		prop("background", "transparent"),
	)
	rule(".ask-aside::-webkit-scrollbar-thumb",
		prop("background", "color-mix(in srgb, var(--text) 16%, transparent)"),
		prop("border-radius", "9px"),
		prop("border", "2px solid transparent"),
		prop("background-clip", "padding-box"),
	)
	rule(".ask-aside::-webkit-scrollbar-thumb:hover",
		prop("background", "color-mix(in srgb, var(--text) 30%, transparent)"),
		prop("background-clip", "padding-box"),
	)
	rule(".ask-note",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	rule(".ask-note-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "0.6rem"),
	)
	// The collapsible section header: a borderless full-label toggle (chevron + serif
	// label + a small count), with the section's link (if any) to its right.
	rule(".ask-note-toggle",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("background", "none"),
		prop("border", "0"),
		prop("padding", "0"),
		prop("cursor", "pointer"),
		prop("color", "inherit"),
		prop("text-align", "left"),
	)
	rule(".ask-note-chev",
		prop("flex", "0 0 auto"),
		prop("opacity", "0.5"),
	)
	rule(".ask-note-count",
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "700"),
		prop("color", "var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// A flagged-activity row: the finding, then two small actions (Source / Discuss).
	// No longer a single click-through button, so the two intents are explicit.
	rule(".insight-row-flagged",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("gap", "0.6rem"),
	)
	rule(".insight-row-actions",
		prop("display", "flex"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.45rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".insight-row-btn",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.25rem"),
		prop("padding", "0.2rem 0.5rem"),
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("border-radius", "7px"),
		prop("border", "1px solid var(--border)"),
		prop("background", "var(--bg-elev)"),
		prop("color", "var(--text-dim)"),
		prop("cursor", "pointer"),
		prop("transition", "border-color 0.12s ease, color 0.12s ease, background 0.12s ease"),
	)
	rule(".insight-row-btn:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		prop("color", "var(--text)"),
		prop("background", "color-mix(in srgb, var(--accent) 8%, var(--bg-elev))"),
	)
	rule(".insight-row-btn svg",
		prop("opacity", "0.7"),
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
		prop("font-size", "var(--type-12)"),
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
		prop("font-size", "var(--type-12)"),
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
		prop("font-size", "var(--type-13)"),
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
		prop("font-size", "var(--type-11)"),
		prop("opacity", "0.55"),
	)
	rule(".ask-aside .btn-link",
		prop("white-space", "nowrap"),
	)
	registerApprovalCard()
	registerCitations()
	registerActionHistory()
	registerBudgetReadout()
	registerConfidenceChip()
	registerTabJob()
	registerChatPolish()
	registerStreaming()
	registerKeyGate()
	registerSpendMeter()
	registerMonthlyReview()
	registerSpotlight()
	registerCapabilitySheet()
	registerKeyGateMark()
}

// registerKeyGateMark styles the "needs a key" label on an AI-gated control (R24).
// It is a quiet inline note rather than a badge: the control still works as a
// control, and the mark's job is to answer "will this do anything?" before the
// click, not to decorate.
func registerKeyGateMark() {
	rule(".key-gate-mark",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.2rem"),
		prop("margin-left", "0.4rem"),
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
		prop("font-weight", "400"),
		prop("white-space", "nowrap"),
	)
}

// registerCapabilitySheet styles the "everything it can change" disclosure (PS7).
// Collapsed it is a single quiet line; opened it is a plain list, deliberately
// undesigned — a list of capabilities that looks marketed is a list people stop
// believing.
func registerCapabilitySheet() {
	rule(".asst-cap",
		prop("width", "100%"),
	)
	rule(".asst-cap-summary",
		prop("display", "inline-block"),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("cursor", "pointer"),
		prop("border-bottom", "1px dotted var(--border)"),
		prop("padding", "0.15rem 0"),
	)
	rule(".asst-cap-summary:hover",
		prop("color", "var(--text)"),
	)
	rule(".asst-cap-lead",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.5rem 0 0.35rem"),
	)
	rule(".asst-cap-list",
		prop("list-style", "none"),
		prop("margin", "0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
	)
	rule(".asst-cap-row",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("padding-left", "0.7rem"),
		prop("border-left", "2px solid var(--border)"),
	)
}

// registerSpotlight styles the ring the assistant points with (PS8).
//
// It rings the real control in place rather than dimming the page around it. A
// modal spotlight would block the very interaction it is describing — the point is
// that the person can now DO the thing — so this is an outline and a soft glow that
// sit on top of whatever the control already looks like, and can be worked through
// without dismissing anything. The pulse stops under reduced motion; a ring that
// throbs indefinitely beside a form is a distraction, not a signpost.
func registerSpotlight() {
	rule(".is-spotlit",
		prop("position", "relative"),
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "3px"),
		prop("border-radius", "var(--radius)"),
		prop("box-shadow", "0 0 0 6px color-mix(in srgb, var(--accent) 22%, transparent)"),
		prop("animation", "cf-spotlight 1.6s ease-out 3"),
	)
	keyframes("cf-spotlight",
		at("0%", prop("box-shadow", "0 0 0 0 color-mix(in srgb, var(--accent) 45%, transparent)")),
		at("70%", prop("box-shadow", "0 0 0 12px color-mix(in srgb, var(--accent) 0%, transparent)")),
		at("100%", prop("box-shadow", "0 0 0 6px color-mix(in srgb, var(--accent) 22%, transparent)")),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".is-spotlit",
		prop("animation", "none"),
	)
	// The note names what was highlighted, so the ring explains itself instead of
	// just glowing at somebody.
	rule(".spot-note",
		prop("position", "fixed"),
		prop("bottom", "1rem"),
		prop("left", "50%"),
		prop("transform", "translateX(-50%)"),
		prop("z-index", "60"),
		prop("padding", "0.5rem 0.9rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("background", "var(--bg-elev)"),
		prop("border", "1px solid var(--accent)"),
		prop("color", "var(--text)"),
		prop("font-size", "var(--type-13)"),
		prop("box-shadow", "0 6px 24px rgba(0,0,0,0.18)"),
		prop("pointer-events", "none"),
	)
}

// registerMonthlyReview styles the guided month-end review (AG10). It is a card,
// not a modal, and the styling has to keep saying so: no backdrop, no trapped
// focus, and a close control that reads as "not now" rather than as an escape
// hatch. The accent spine marks it as the one thing on the page asking for
// something, without shouting over the briefing below it.
func registerMonthlyReview() {
	rule(".mrev",
		prop("border", "1px solid var(--border)"),
		prop("border-left", "3px solid var(--accent)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
		prop("padding", "1rem 1.1rem"),
		prop("margin", "0 0 1rem"),
		prop("max-width", "44rem"),
	)
	rule(".mrev-head",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
	)
	rule(".mrev-eyebrow",
		prop("font-size", "var(--type-11)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		prop("color", "var(--text-faint)"),
		prop("margin", "0 0 0.2rem"),
	)
	rule(".mrev-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.2rem"),
		prop("line-height", "1.2"),
		prop("margin", "0"),
	)
	rule(".mrev-close",
		prop("flex", "0 0 auto"),
		prop("border", "none"),
		prop("background", "transparent"),
		prop("color", "var(--text-faint)"),
		prop("cursor", "pointer"),
		prop("padding", "0.2rem"),
	)
	rule(".mrev-close:hover",
		prop("color", "var(--text)"),
	)
	rule(".mrev-body",
		prop("font-size", "var(--type-14)"),
		prop("line-height", "1.5"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.6rem 0 0"),
	)
	rule(".mrev-body p",
		prop("margin", "0 0 0.4rem"),
	)
	rule(".mrev-fine",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-faint)"),
	)
	rule(".mrev-actions",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.85rem"),
	)
}

// registerSpendMeter styles the AI spend meter (EC-15). The figure is the largest
// thing in the card because it is the one somebody opened this to read; everything
// else is context for it. The pace line takes a tone only when it has something to
// warn about — a meter that is always coloured is a meter nobody looks at twice.
func registerSpendMeter() {
	rule(".ai-meter",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-card)"),
		prop("padding", "0.9rem 1rem"),
		prop("margin", "0.75rem 0"),
		prop("max-width", "40rem"),
	)
	rule(".ai-meter-title",
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.03em"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0 0 0.5rem"),
	)
	rule(".ai-meter-empty",
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0"),
	)
	rule(".ai-meter-total",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "baseline"),
		prop("gap", "0.6rem"),
	)
	rule(".ai-meter-figure",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.6rem"),
		prop("line-height", "1.1"),
		prop("color", "var(--text)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".ai-meter-sub",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-faint)"),
	)
	rule(".ai-meter-pace",
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.5rem 0 0"),
	)
	rule(".ai-meter-pace.is-warn",
		prop("color", "var(--warn, #d9a23f)"),
	)
	rule(".ai-meter-pace.is-over",
		prop("color", "var(--down, #d1685f)"),
		prop("font-weight", "600"),
	)
	rule(".ai-meter-list",
		prop("list-style", "none"),
		prop("margin", "0.7rem 0 0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".ai-meter-row",
		prop("display", "grid"),
		prop("grid-template-columns", "1fr auto auto"),
		prop("gap", "0.75rem"),
		prop("align-items", "baseline"),
		prop("padding", "0.3rem 0"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".ai-meter-row:last-child",
		prop("border-bottom", "none"),
	)
	rule(".ai-meter-calls",
		prop("color", "var(--text-faint)"),
		prop("font-size", "var(--type-11)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".ai-meter-cost",
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "var(--text)"),
	)
	rule(".ai-meter-cap",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("align-items", "center"),
		prop("margin-top", "0.7rem"),
	)
	rule(".ai-meter-cap .field",
		prop("max-width", "8rem"),
	)
}

// registerKeyGate styles the explanation shown when a feature is waiting for a key
// (C247) and the model/spend line under the composer (C250). Both are quiet by
// design: the gate is a card because it is asking for a decision, the status line
// is a caption because it is only reporting one.
func registerKeyGate() {
	rule(".asst-key-callout",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.6rem"),
		prop("margin-top", "0.75rem"),
		prop("padding", "0.85rem 1rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
		prop("max-width", "44rem"),
	)
	rule(".asst-key-lead",
		prop("font-size", "var(--type-14)"),
		prop("line-height", "1.5"),
		prop("margin", "0"),
	)
	rule(".asst-key-points",
		prop("list-style", "none"),
		prop("margin", "0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
	)
	rule(".asst-key-point",
		prop("display", "flex"),
		prop("gap", "0.55rem"),
		prop("align-items", "flex-start"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.45"),
		prop("color", "var(--text-dim)"),
	)
	rule(".asst-key-icon",
		prop("color", "var(--accent)"),
		prop("margin-top", "0.15rem"),
		prop("opacity", "0.85"),
	)
	rule(".asst-key-point a",
		prop("overflow-wrap", "anywhere"),
	)
	rule(".asst-status",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.55rem"),
		prop("align-items", "baseline"),
		prop("margin", "0.35rem 0.15rem 0"),
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
	)
	rule(".asst-status-model",
		prop("font-variant-numeric", "tabular-nums"),
		prop("letter-spacing", "0.01em"),
	)
	rule(".asst-status-spend",
		prop("font-variant-numeric", "tabular-nums"),
	)
	// The two facts are separated by a hairline rather than punctuation, so the
	// line stays readable when the spend half is absent.
	rule(".asst-status-spend::before",
		prop("content", "\"\""),
		prop("display", "inline-block"),
		prop("width", "1px"),
		prop("height", "0.8em"),
		prop("margin-right", "0.55rem"),
		prop("vertical-align", "-0.05em"),
		prop("background", "var(--border)"),
	)
}

// registerStreaming styles the answer as it is being written (G2-C7). It matches
// the finished answer's type and width exactly, so the swap from streaming text to
// rendered Markdown does not shift the layout under the reader's eye. The caret is
// the only added ornament, and it stops blinking under reduced-motion — a blinking
// element is one of the few things that reliably triggers discomfort.
func registerStreaming() {
	rule(".asst-streaming",
		prop("white-space", "pre-wrap"),
		prop("overflow-wrap", "anywhere"),
		prop("line-height", "1.6"),
	)
	rule(".asst-caret",
		prop("display", "inline-block"),
		prop("width", "0.45rem"),
		prop("height", "1em"),
		prop("margin-left", "0.15rem"),
		prop("vertical-align", "text-bottom"),
		prop("background", "var(--accent)"),
		prop("opacity", "0.7"),
		prop("animation", "cf-caret 1s steps(2, start) infinite"),
	)
	keyframes("cf-caret", Frame{Offset: "50%", Decls: []decl{prop("opacity", "0")}})
	ruleMedia("(prefers-reduced-motion: reduce)", ".asst-caret",
		prop("animation", "none"),
	)
}

// registerChatPolish styles the conversation-management controls (G2-C7): the
// rail's search box and its results, in-place rename, the edit-and-ask-again box,
// and the answer rating.
func registerChatPolish() {
	rule(".conv-search",
		prop("position", "relative"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("margin-bottom", "0.5rem"),
	)
	rule(".conv-search .field",
		prop("width", "100%"),
		prop("font-size", "var(--type-12)"),
		prop("padding-right", "1.8rem"),
	)
	rule(".conv-search-clear",
		prop("position", "absolute"),
		prop("right", "0.35rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("border", "none"),
		prop("background", "transparent"),
		prop("color", "var(--text-faint)"),
		prop("cursor", "pointer"),
	)
	rule(".conv-search-clear:hover",
		prop("color", "var(--text)"),
	)
	rule(".conv-hit",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("align-items", "flex-start"),
	)
	// The matched line is why the result is a result, so it sits directly under
	// its pill and clamps to one line rather than reflowing the rail.
	rule(".conv-hit-excerpt",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
		prop("margin", "0 0 0 0.6rem"),
		prop("max-width", "100%"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".conv-rename .field",
		prop("width", "100%"),
		prop("font-size", "var(--type-12)"),
	)
	// The edit box replaces the bubble in place, so it keeps the bubble's width
	// and side rather than jumping to the composer.
	rule(".asst-msg-edit",
		prop("width", "min(85%, 36rem)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 35%, var(--border))"),
		prop("border-radius", "14px 14px 4px 14px"),
		prop("background", "var(--bg-elev)"),
		prop("padding", "0.6rem 0.7rem"),
	)
	rule(".asst-msg-edit .field",
		prop("width", "100%"),
		prop("font-size", "var(--type-14)"),
		prop("resize", "vertical"),
	)
	rule(".asst-msg-edit-actions",
		prop("display", "flex"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.5rem"),
	)
	// The consequence of asking again is stated inside the box, at the moment of
	// deciding — not in a toast after the answers have already gone.
	rule(".asst-msg-edit-note",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.45rem 0 0"),
	)
	rule(".asst-rated",
		prop("color", "var(--accent)"),
		prop("opacity", "1"),
	)
}

// registerTabJob styles the one-line job statement under the hub's tab bar
// (C392). It is set in the reading size rather than a caption size: it is the
// sentence that decides which tab someone wants, so it has to be readable at the
// moment of choosing rather than after squinting.
func registerTabJob() {
	rule(".asst-tab-job",
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.45rem 0 0"),
		prop("max-width", "44rem"),
	)
}

// registerConfidenceChip styles the hedge on an inferred finding (C391). It is a
// small outlined pill rather than a filled badge: the finding's own severity
// already owns the colour on that row, and a second saturated mark would compete
// with it for the same glance. "Worth a look" takes the warning tone because it is
// the tier that most often turns out to be wrong.
func registerConfidenceChip() {
	rule(".insight-conf",
		prop("align-self", "flex-start"),
		prop("margin-top", "0.25rem"),
		prop("display", "inline-block"),
		prop("padding", "0.05rem 0.4rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-pill)"),
		prop("font-size", "var(--type-11)"),
		prop("line-height", "1.5"),
		prop("color", "var(--text-faint)"),
		prop("white-space", "nowrap"),
	)
	rule(".insight-conf.is-possible",
		prop("border-color", "color-mix(in srgb, var(--warn, #d9a23f) 45%, var(--border))"),
		prop("color", "var(--warn, #d9a23f)"),
	)
}

// registerBudgetReadout styles the per-conversation cap's remaining figure (C390).
// It sits beside its select as a quiet number until the budget is spent, at which
// point it takes the warning tone — that is the one moment it needs to be read.
func registerBudgetReadout() {
	rule(".asst-budget-readout",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("margin-left", "0.35rem"),
		prop("white-space", "nowrap"),
	)
	rule(".asst-budget-readout.is-spent",
		prop("color", "var(--warn, #d9a23f)"),
		prop("font-weight", "600"),
	)
}

// registerActionHistory styles the per-action list under the session receipt
// (C389). It borrows the citation panel's footnote treatment on purpose: both
// answer "show me the detail behind that summary", and giving them one visual
// language means learning the pattern once.
func registerActionHistory() {
	rule(".asst-receipt",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.25rem"),
	)
	rule(".asst-act",
		prop("max-width", "44rem"),
	)
	rule(".asst-act-summary",
		prop("display", "inline-block"),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("cursor", "pointer"),
		prop("padding", "0.15rem 0"),
		prop("border-bottom", "1px dotted var(--border)"),
	)
	rule(".asst-act-summary:hover",
		prop("color", "var(--text)"),
	)
	rule(".asst-act-list",
		prop("list-style", "none"),
		prop("margin", "0.4rem 0 0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
	)
	rule(".asst-act-item",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text)"),
		prop("padding", "0.2rem 0"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".asst-act-item:last-child",
		prop("border-bottom", "none"),
	)
	// The time is a locator, not a fact worth reading — it recedes so the list
	// scans as a column of what happened.
	rule(".asst-act-when",
		prop("flex", "0 0 auto"),
		prop("color", "var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".asst-act-actions",
		prop("display", "flex"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.5rem"),
		prop("flex-wrap", "wrap"),
	)
}

// registerCitations styles the "How I got this" panel under an answer (C387). It is
// deliberately quiet: a dotted-underline summary that reads as a footnote, not a
// second answer competing with the first. Opened, the evidence is monospaced —
// these are rows and totals, and a proportional font makes columns of figures
// harder to scan than the tool's own plain text already is.
func registerCitations() {
	rule(".asst-cite",
		prop("margin", "0.4rem 0 0"),
		prop("max-width", "44rem"),
	)
	rule(".asst-cite-summary",
		prop("display", "inline-block"),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("cursor", "pointer"),
		prop("padding", "0.15rem 0"),
		prop("border-bottom", "1px dotted var(--border)"),
	)
	rule(".asst-cite-summary:hover",
		prop("color", "var(--text)"),
	)
	rule(".asst-cite-list",
		prop("list-style", "none"),
		prop("margin", "0.5rem 0 0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".asst-cite-item",
		prop("border-left", "2px solid var(--border)"),
		prop("padding-left", "0.7rem"),
	)
	rule(".asst-cite-title",
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("color", "var(--text)"),
		prop("margin", "0 0 0.25rem"),
	)
	// The evidence can be long, so it scrolls inside its own box rather than
	// stretching the thread — and it wraps, because a tool result is prose with
	// figures in it, not a code listing with meaningful line breaks.
	rule(".asst-cite-evidence",
		prop("font-family", "var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)"),
		prop("font-size", "var(--type-11)"),
		prop("line-height", "1.5"),
		prop("color", "var(--text-dim)"),
		prop("background", "var(--bg-card)"),
		prop("border-radius", "var(--radius)"),
		prop("padding", "0.5rem 0.6rem"),
		prop("margin", "0"),
		prop("max-height", "16rem"),
		prop("overflow", "auto"),
		prop("white-space", "pre-wrap"),
		prop("word-break", "break-word"),
	)
}

// registerApprovalCard styles the permission preview a mutating tool waits behind
// (C388). It is deliberately the loudest thing in the thread — a raised card with
// an accent spine — because it is the one moment the conversation stops and asks
// for consent. The effects read as a checklist rather than a paragraph: changes
// first in full-strength text, reads second in dimmed text, so the eye lands on
// what is at stake before what is merely being looked at.
func registerApprovalCard() {
	rule(".asst-approve",
		prop("border", "1px solid var(--border)"),
		prop("border-left", "3px solid var(--accent)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
		prop("padding", "0.75rem 0.9rem"),
		prop("margin", "0.5rem 0"),
		prop("max-width", "44rem"),
	)
	rule(".asst-approve-title",
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.02em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0 0 0.35rem"),
	)
	rule(".asst-approve-preview",
		prop("font-size", "var(--type-14)"),
		prop("line-height", "1.45"),
		prop("color", "var(--text)"),
		prop("margin", "0"),
		prop("white-space", "pre-wrap"),
	)
	rule(".asst-approve-effects",
		prop("list-style", "none"),
		prop("margin", "0.6rem 0 0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
	)
	rule(".asst-approve-effect",
		prop("display", "flex"),
		prop("gap", "0.5rem"),
		prop("align-items", "baseline"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.4"),
	)
	rule(".asst-approve-effect.is-write",
		prop("color", "var(--text)"),
	)
	rule(".asst-approve-effect.is-read",
		prop("color", "var(--text-dim)"),
	)
	// The dot is the only thing separating a change from a read at a glance, so it
	// carries the accent on writes and stays quiet on reads.
	rule(".asst-approve-dot",
		prop("flex", "0 0 auto"),
		prop("width", "0.4rem"),
		prop("height", "0.4rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("background", "var(--text-faint)"),
		prop("transform", "translateY(-0.15em)"),
	)
	rule(".asst-approve-effect.is-write .asst-approve-dot",
		prop("background", "var(--accent)"),
	)
	rule(".asst-approve-undo",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.55rem 0 0"),
	)
	rule(".asst-approve-undo.is-permanent",
		prop("color", "var(--warn, #d9a23f)"),
	)
	rule(".asst-approve-actions",
		prop("display", "flex"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.75rem"),
		prop("flex-wrap", "wrap"),
	)
}
