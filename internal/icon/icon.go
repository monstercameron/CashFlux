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
