// SPDX-License-Identifier: MIT

// Package icon is the pure, type-safe icon registry for CashFlux: a curated set
// of named line icons (Lucide-format, 24×24, stroked, inheriting currentColor)
// exposed as compile-checked Name constants rather than stringly-typed lookups.
// Each Name resolves to its inner SVG markup (the shapes inside the <svg>); the
// wasm UI layer wraps that in an <svg> with the shared viewBox/stroke defaults.
//
// Pure Go, no platform dependencies; unit-tested on native Go. Keeping the data
// here (not in the //go:build js view layer) means the icon set is verifiable in
// plain `go test` and callers get a closed, compile-checked vocabulary.
package icon

import "sort"

// Name is a curated icon id. Only the constants below are valid; using any other
// value is a compile error, so call sites can't reference an icon that doesn't
// exist. The string values match the historical hand-rolled names so the UI swap
// is mechanical.
type Name string

// The curated icon set the app actually uses (sidebar nav, top bar, controls).
const (
	Dashboard     Name = "dashboard"
	Accounts      Name = "accounts"
	Transactions  Name = "transactions"
	Budgets       Name = "budgets"
	Goals         Name = "goals"
	Todo          Name = "todo"
	Settings      Name = "settings"
	Page          Name = "page"
	Plus          Name = "plus"
	Menu          Name = "menu"
	Tag           Name = "tag"
	Users         Name = "users"
	Planning      Name = "planning"
	Allocate      Name = "allocate"
	Insights      Name = "insights"
	Customize     Name = "customize"
	Reports       Name = "reports"
	Subscriptions Name = "subscriptions"
	Bills         Name = "bills"
	Split         Name = "split"

	// Controls — real glyphs for the ad-hoc Unicode the chrome used (▾ ‹ › ✕ ⋯).
	ChevronDown  Name = "chevron-down"
	ChevronUp    Name = "chevron-up"
	ChevronLeft  Name = "chevron-left"
	ChevronRight Name = "chevron-right"
	Close        Name = "x"
	MoreH        Name = "more-horizontal"
	Grip         Name = "grip"   // six-dot drag handle (signals draggable, vs MoreH = menu)
	Search       Name = "search" // magnifier for search/filter inputs

	// Status + trend glyphs (carry meaning at a glance; color via caller classes).
	Check           Name = "check"
	CheckCircle     Name = "check-circle"
	AlertCircle     Name = "alert-circle"
	AlertTriangle   Name = "alert-triangle"
	Clock           Name = "clock"
	TrendingUp      Name = "trending-up"
	TrendingDown    Name = "trending-down"
	ArrowUp         Name = "arrow-up"
	ArrowDown       Name = "arrow-down"
	ArrowUpCircle   Name = "arrow-up-circle"
	ArrowDownCircle Name = "arrow-down-circle"

	// Row + section actions.
	Pencil     Name = "pencil"
	Refresh    Name = "refresh-cw"
	List       Name = "list"
	PlusCircle Name = "plus-circle"

	// AI + content.
	Sparkles      Name = "sparkles"
	MessageCircle Name = "message-circle"
	FileText      Name = "file-text"
	Copy          Name = "copy"

	// Screens + domain accents.
	CreditCard Name = "credit-card"
	Receipt    Name = "receipt"
	Landmark   Name = "landmark"
	Filter     Name = "filter"
	Box        Name = "box"
	Workflow   Name = "workflow"
	Scale      Name = "scale"
	Repeat     Name = "repeat"
	Calculator Name = "calculator"
	ScanLine   Name = "scan-line"
	Upload     Name = "upload"
	History    Name = "history"
	Ban        Name = "ban"
	HelpCircle Name = "help-circle"

	// Media controls.
	Volume     Name = "volume-2"
	VolumeMute Name = "volume-x"

	// Security.
	Lock Name = "lock"

	// Notifications.
	Bell Name = "bell"

	// Attachment.
	Paperclip Name = "paperclip"

	// Appearance / theming.
	Appearance Name = "appearance"
)

// inner maps each icon to its inner SVG markup — the child shapes only. Stroke,
// fill, and viewBox live on the wrapping <svg> the UI layer renders, so these
// shapes inherit them (matching the original hand-rolled set exactly).
var inner = map[Name]string{
	Dashboard:     `<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/>`,
	Accounts:      `<rect x="3" y="6" width="18" height="13" rx="2"/><path d="M3 10h18"/><circle cx="16.5" cy="14.5" r="1.1"/>`,
	Transactions:  `<path d="M16 3l4 4-4 4"/><path d="M20 7H5"/><path d="M8 21l-4-4 4-4"/><path d="M4 17h15"/>`,
	Budgets:       `<circle cx="12" cy="12" r="9"/><path d="M12 3a9 9 0 0 1 9 9h-9z"/>`,
	Goals:         `<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1.2"/>`,
	Todo:          `<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 12l3 3 5-6"/>`,
	Settings:      `<path d="M20 7h-9"/><path d="M14 17H5"/><circle cx="17" cy="17" r="3"/><circle cx="7" cy="7" r="3"/>`,
	Page:          `<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>`,
	Plus:          `<path d="M12 5v14M5 12h14"/>`,
	Menu:          `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>`,
	Tag:           `<path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><circle cx="7" cy="7" r="1.4"/>`,
	Users:         `<circle cx="9" cy="8" r="3"/><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6"/><path d="M16 5.3a3 3 0 0 1 0 5.4"/><path d="M21 20c0-2.6-1.6-4.8-3.9-5.7"/>`,
	Planning:      `<path d="M4 19V5"/><path d="M4 19h16"/><path d="M7 15l3-4 3 2 4-6"/>`,
	Allocate:      `<circle cx="12" cy="12" r="9"/><path d="M12 3v9l7 4"/>`,
	Insights:      `<path d="M9 18h6"/><path d="M10 21h4"/><path d="M12 3a6 6 0 0 1 4 10.5c-.7.7-1 1.2-1 2.5H9c0-1.3-.3-1.8-1-2.5A6 6 0 0 1 12 3z"/>`,
	Customize:     `<path d="M4 7h16"/><path d="M4 12h16"/><path d="M4 17h16"/><circle cx="9" cy="7" r="1.8"/><circle cx="15" cy="12" r="1.8"/><circle cx="7" cy="17" r="1.8"/>`,
	Reports:       `<path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/>`,
	Subscriptions: `<path d="M17 2l4 4-4 4"/><path d="M3 11v-1a4 4 0 0 1 4-4h14"/><path d="M7 22l-4-4 4-4"/><path d="M21 13v1a4 4 0 0 1-4 4H3"/>`,
	Bills:         `<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4"/><path d="M8 2v4"/><path d="M3 10h18"/>`,
	Split:         `<path d="M16 3h5v5"/><path d="M8 3H3v5"/><path d="M12 22v-8.3a4 4 0 0 0-1.17-2.83L3 3"/><path d="M21 3l-7.83 7.83A4 4 0 0 0 12 13.67V22"/>`,

	ChevronDown:  `<path d="M6 9l6 6 6-6"/>`,
	ChevronUp:    `<path d="M18 15l-6-6-6 6"/>`,
	ChevronLeft:  `<path d="M15 18l-6-6 6-6"/>`,
	ChevronRight: `<path d="M9 18l6-6-6-6"/>`,
	Close:        `<path d="M18 6 6 18"/><path d="M6 6l12 12"/>`,
	MoreH:        `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
	Grip:         `<circle cx="9" cy="6" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="9" cy="18" r="1"/><circle cx="15" cy="6" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="15" cy="18" r="1"/>`,
	Search:       `<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.35-4.35"/>`,

	Check:           `<path d="M20 6 9 17l-5-5"/>`,
	CheckCircle:     `<circle cx="12" cy="12" r="9"/><path d="M8.5 12.5l2.5 2.5 4.5-5"/>`,
	AlertCircle:     `<circle cx="12" cy="12" r="9"/><path d="M12 8v4"/><path d="M12 16h.01"/>`,
	AlertTriangle:   `<path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9v4"/><path d="M12 17h.01"/>`,
	Clock:           `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
	TrendingUp:      `<path d="M22 7 13.5 15.5 8.5 10.5 2 17"/><path d="M16 7h6v6"/>`,
	TrendingDown:    `<path d="M22 17 13.5 8.5 8.5 13.5 2 7"/><path d="M16 17h6v-6"/>`,
	ArrowUp:         `<path d="M12 19V5"/><path d="m5 12 7-7 7 7"/>`,
	ArrowDown:       `<path d="M12 5v14"/><path d="m19 12-7 7-7-7"/>`,
	ArrowUpCircle:   `<circle cx="12" cy="12" r="9"/><path d="M12 16V8"/><path d="m8.5 11.5 3.5-3.5 3.5 3.5"/>`,
	ArrowDownCircle: `<circle cx="12" cy="12" r="9"/><path d="M12 8v8"/><path d="m8.5 12.5 3.5 3.5 3.5-3.5"/>`,

	Pencil:     `<path d="M17 3a2.83 2.83 0 0 1 4 4L7.5 20.5 2 22l1.5-5.5z"/>`,
	Refresh:    `<path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/>`,
	List:       `<path d="M8 6h13"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M3 6h.01"/><path d="M3 12h.01"/><path d="M3 18h.01"/>`,
	PlusCircle: `<circle cx="12" cy="12" r="9"/><path d="M12 8v8"/><path d="M8 12h8"/>`,

	Sparkles:      `<path d="M12 3l1.6 4.4L18 9l-4.4 1.6L12 15l-1.6-4.4L6 9z"/><path d="M19 14l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z"/>`,
	MessageCircle: `<path d="M21 11.5a8.5 8.5 0 0 1-11.8 7.8L3 21l1.7-6.2A8.5 8.5 0 1 1 21 11.5z"/>`,
	FileText:      `<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/><path d="M9 13h6"/><path d="M9 17h6"/>`,
	Copy:          `<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,

	CreditCard: `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>`,
	Receipt:    `<path d="M4 2v20l2-1 2 1 2-1 2 1 2-1 2 1 2-1 2 1V2l-2 1-2-1-2 1-2-1-2 1-2-1z"/><path d="M8 7h8"/><path d="M8 11h8"/><path d="M8 15h5"/>`,
	Landmark:   `<path d="M3 21h18"/><path d="M5 21V10"/><path d="M19 21V10"/><path d="M9 21V14"/><path d="M15 21V14"/><path d="M12 3 3 8h18z"/>`,
	Filter:     `<path d="M22 3H2l8 9.5V19l4 2v-9.5z"/>`,
	Box:        `<path d="M21 8 12 3 3 8v8l9 5 9-5z"/><path d="M3 8l9 5 9-5"/><path d="M12 13v8"/>`,
	Workflow:   `<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/><path d="M10 6.5h4a3 3 0 0 1 3 3V14"/>`,
	Scale:      `<path d="M12 3v18"/><path d="M7 21h10"/><path d="M5 7h14"/><path d="M5 7l-3 6h6z"/><path d="M19 7l3 6h-6z"/>`,
	Repeat:     `<path d="m17 2 4 4-4 4"/><path d="M3 11v-1a4 4 0 0 1 4-4h14"/><path d="m7 22-4-4 4-4"/><path d="M21 13v1a4 4 0 0 1-4 4H3"/>`,
	Calculator: `<rect x="4" y="2" width="16" height="20" rx="2"/><path d="M8 6h8"/><path d="M8 14h.01"/><path d="M12 14h.01"/><path d="M16 14h.01"/><path d="M8 18h.01"/><path d="M12 18h.01"/><path d="M16 18h.01"/>`,
	ScanLine:   `<path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><path d="M7 12h10"/>`,
	Upload:     `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="M7 9l5-5 5 5"/><path d="M12 4v12"/>`,
	History:    `<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/><path d="M12 7v5l4 2"/>`,
	Ban:        `<circle cx="12" cy="12" r="9"/><path d="M5.6 5.6l12.8 12.8"/>`,
	Volume:     `<path d="M11 5 6 9 2 9 2 15 6 15 11 19Z"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>`,
	VolumeMute: `<path d="M11 5 6 9 2 9 2 15 6 15 11 19Z"/><path d="M22 9l-6 6"/><path d="M16 9l6 6"/>`,
	Lock:       `<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`,
	Bell:       `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>`,
	HelpCircle: `<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 0 1 4.6 1.4c0 1.6-2.1 2-2.1 3.1"/><path d="M12 17h.01"/>`,

	Paperclip:  `<path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>`,
	Appearance: `<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="2"/><path d="M12 3v2"/><path d="M12 19v2"/><path d="M3 12h2"/><path d="M19 12h2"/><path d="M5.6 5.6l1.4 1.4"/><path d="M17 17l1.4 1.4"/><path d="M5.6 18.4l1.4-1.4"/><path d="M17 7l1.4-1.4"/>`,
}

// Inner returns the icon's inner SVG markup, or "" for an unknown name.
func (n Name) Inner() string { return inner[n] }

// Valid reports whether n is a known icon.
func (n Name) Valid() bool {
	_, ok := inner[n]
	return ok
}

// All returns every known icon name, sorted, so callers (e.g. a gallery or test)
// can enumerate the set deterministically.
func All() []Name {
	out := make([]Name, 0, len(inner))
	for n := range inner {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
