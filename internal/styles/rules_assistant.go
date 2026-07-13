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
		prop("font-size", "0.68rem"),
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
		prop("border-radius", "999px"),
		prop("font-size", "0.78rem"),
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
		prop("border-radius", "999px"),
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
		prop("border-radius", "999px"),
		prop("font-size", "0.78rem"),
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
	ruleMedia("(max-width: 1100px)", ".ask-deck",
		prop("grid-template-columns", "minmax(0, 1fr)"),
		prop("height", "auto"), // stacked: let the page flow
	)
	// Stacked below the chat on narrow screens: the aside flows with the page (no cap,
	// no inner scroll — that only makes sense beside the chat), and the chat takes a
	// fixed viewport height instead of filling the (now-auto) deck.
	ruleMedia("(max-width: 1100px)", ".ask-aside",
		prop("overflow-y", "visible"),
		prop("padding-right", "0"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-main .chat-console",
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
	// Inline quick-controls in the header: a small caption + a <select> (model + thinking
	// level) beside the New chat / Edit prompt buttons — a fast switch without a trip to
	// Settings. Styled to match the app's STANDARD toolbar select (the topbar member-
	// switcher): bg-elev chip, 1px border, 8px radius, NATIVE arrow (no custom chevron),
	// sized to the .btn-tool footprint so the row is uniform. The selector out-specifies
	// the generated bare-<select> rule (which forces a 44px look).
	rule(".ask-quickctl",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
	)
	rule(".ask-quickctl-lbl",
		prop("font-size", "0.68rem"),
		prop("font-weight", "600"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		prop("color", "var(--text-faint)"),
	)
	rule(".ask-head-actions select.ask-quickctl-sel",
		prop("height", "38px"),
		prop("min-height", "38px"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "8px"),
		prop("background", "var(--bg-elev)"),
		prop("color", "var(--text)"),
		prop("font-size", "0.82rem"),
		prop("padding", "0 1.5rem 0 0.6rem"),
		prop("max-width", "12rem"),
		prop("cursor", "pointer"),
		prop("transition", "border-color 120ms ease"),
	)
	rule(".ask-head-actions select.ask-quickctl-sel:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
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
		prop("font-size", "0.68rem"),
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
		prop("font-size", "0.72rem"),
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
