// SPDX-License-Identifier: MIT

// Package prefs models user display preferences that persist locally across
// reloads (week start and date format), independent of the dataset. The display
// logic lives here as pure Go so it is unit-tested on native Go; the wasm layer
// only handles localStorage persistence and the settings UI.
package prefs

import (
	"strings"
	"time"
)

// WeekStart is the first day of the week for calendars and weekly periods.
type WeekStart string

// The supported week-start choices.
const (
	WeekSunday   WeekStart = "sunday"
	WeekMonday   WeekStart = "monday"
	WeekSaturday WeekStart = "saturday"
)

// DateStyle is how dates are rendered to the user.
type DateStyle string

// The supported date styles.
const (
	DateISO  DateStyle = "iso"  // 2006-01-02
	DateUS   DateStyle = "us"   // 01/02/2006
	DateEU   DateStyle = "eu"   // 02/01/2006
	DateLong DateStyle = "long" // Jan 2, 2006
)

// Theme is the color theme of the app.
type Theme string

// The supported themes. ThemeSystem follows the OS preference.
const (
	ThemeDark   Theme = "dark"
	ThemeLight  Theme = "light"
	ThemeSystem Theme = "system"
)

// Motion controls animated-flourish intensity. The UI maps it to the
// data-wonder attribute on <html>, which drives all CSS WONDER tokens.
type Motion string

// The supported motion levels, from most animated to fully static.
const (
	// MotionFull is the default: all flourishes at full intensity.
	MotionFull Motion = "full"
	// MotionSubtle reduces flourish intensity to ~55% (shorter durations, less travel).
	MotionSubtle Motion = "subtle"
	// MotionOff disables all animated flourishes; the app is fully static.
	MotionOff Motion = "off"
)

// Valid reports whether m is a known motion level.
func (m Motion) Valid() bool { return m == MotionFull || m == MotionSubtle || m == MotionOff }

// ServerMode distinguishes paid CashFlux Cloud from a user-managed backend.
type ServerMode string

// The supported backend choices.
const (
	ServerCloud      ServerMode = "cloud"
	ServerSelfHosted ServerMode = "self-hosted"
)

// ConnectionSegment refines ServerSelfHosted into how the address was found:
// "local" auto-detects a same-origin backend (your own infrastructure, with a
// manual fallback for unusual URLs like subdomains); "remote" always requires
// a manually-typed address and is framed with a trust disclosure since it may
// point at someone else's server. Meaningless when ServerMode is ServerCloud.
type ConnectionSegment string

// The two ServerSelfHosted flavors.
const (
	ConnectionLocal  ConnectionSegment = "local"
	ConnectionRemote ConnectionSegment = "remote"
)

// defaultAccent is the out-of-the-box accent color: a seagreen that clears WCAG
// AA for UI/large elements (3:1) against BOTH the dark and light theme surfaces
// (dark 4.09:1, light 3.63:1), unlike the original lighter mint #54b884 which
// failed on light (~2.1:1). Accent drives the focus ring and large strokes, so it
// must pass the 3:1 threshold on whichever surface the user runs (B15).
const defaultAccent = "#2e8b57"

// Display-scale bounds (whole percent). 100 is the unscaled default. The upper
// bound reaches 200 so the scale doubles as an accessibility text-resize control
// (WCAG 2.1 SC 1.4.4 "Resize text" — text must reach 200% without loss of
// content); the responsive layout (C10/C19) reflows at the resulting effective
// width instead of overflowing (C26).
const (
	ScaleMin     = 70
	ScaleMax     = 200
	ScaleDefault = 100
)

const DefaultServerURL = "http://127.0.0.1:8081"

// Prefs holds the user's display preferences.
type Prefs struct {
	WeekStart WeekStart `json:"weekStart"`
	DateStyle DateStyle `json:"dateStyle"`
	Theme     Theme     `json:"theme"`
	Accent    string    `json:"accent"`          // hex color, e.g. "#54b884"
	Compact   bool      `json:"compact"`         // denser layout
	Scale     int       `json:"scale,omitempty"` // UI zoom percent (70..200); 0 means default 100
	// RememberAIKey opts into persisting the OpenAI key on this device across
	// reloads (off by default — the key is otherwise session-only). When on, the
	// key is written to its own localStorage entry, separate from the dataset.
	RememberAIKey bool       `json:"rememberAiKey,omitempty"`
	ServerMode    ServerMode `json:"serverMode,omitempty"`
	ServerURL     string     `json:"serverUrl,omitempty"`
	ServerToken   string     `json:"serverToken,omitempty"`
	ServerCSRF    string     `json:"serverCsrf,omitempty"`
	// ConnectionSegment only applies when ServerMode is ServerSelfHosted — see
	// its doc comment. Empty (e.g. prefs saved before this field existed)
	// normalizes to ConnectionLocal.
	ConnectionSegment ConnectionSegment `json:"connectionSegment,omitempty"`
	// Motion controls animated-flourish intensity for the WONDER system. The
	// wasm layer writes this as the data-wonder attribute on <html>, which the
	// CSS WONDER tokens key off. Defaults to MotionFull.
	Motion Motion `json:"motion,omitempty"`
	// BackendDisabled turns off all backend connections (sync + AI proxy) even when
	// a server URL and token are configured. It is inverted (default false = on) so
	// existing prefs keep working; the user flips it from the Settings modal to stop
	// the app dialing the backend (e.g. when the websocket is unreachable).
	BackendDisabled bool `json:"backendDisabled,omitempty"`
	// PayCycleAnchor is an ISO-8601 date ("2006-01-02") that identifies a known
	// payday — the anchor date for the biweekly 14-day grid. When set, biweekly
	// budget periods snap to the user's actual pay cycle instead of the fixed
	// internal epoch. Empty string means no anchor (use default epoch behavior).
	PayCycleAnchor string `json:"payCycleAnchor,omitempty"`
	// MonthlyIncomeMinor is the household's configured monthly take-home pay in
	// minor units of the base currency (e.g. cents for USD). When positive, it
	// is used by budgeting helpers (50/30/20 template, income-awareness banners,
	// safe-to-spend) in preference to the transaction-derived income figure.
	// Zero means unset — fall back to summing income transactions for the period.
	MonthlyIncomeMinor int64 `json:"monthlyIncomeMinor,omitempty"`
	// BudgetIncomeMode selects what income a ZERO-BASED budget assigns against:
	// "all" (every deposit last month — the default), "paychecks" (only deposits at
	// or above BudgetPaycheckMinMinor, so side hustles are ignored), or "fixed" (the
	// configured MonthlyIncomeMinor). Empty = "all". See budgeting.IncomeMode*.
	BudgetIncomeMode string `json:"budgetIncomeMode,omitempty"`
	// BudgetPaycheckMinMinor is the minimum deposit (minor units, base currency) that
	// counts as a paycheck when BudgetIncomeMode is "paychecks". Zero = no threshold
	// (every deposit counts, same as "all").
	BudgetPaycheckMinMinor int64 `json:"budgetPaycheckMinMinor,omitempty"`
	// BudgetIncomeAvgMonths averages the income basis over this many recent months
	// instead of using last month alone — so irregular income (freelance, commissions)
	// is budgeted off a steadier figure. 0 or 1 means "last month only"; 3 means the
	// three-month average. Applies to the actual-income modes (all/paychecks/categories),
	// never to the fixed figure.
	BudgetIncomeAvgMonths int `json:"budgetIncomeAvgMonths,omitempty"`
	// BudgetIncomeCategoryIDs lists the income categories that fund the budget when
	// BudgetIncomeMode is "categories" — the precise alternative to the paycheck
	// dollar-threshold. Only income in these categories counts toward the amount to
	// assign, so a household can budget against its salary and hold aside (or add
	// back) each side-income source by name. Empty in "categories" mode means no
	// source is chosen yet, so the income basis is zero until one is picked.
	BudgetIncomeCategoryIDs []string `json:"budgetIncomeCategoryIds,omitempty"`
	// BudgetRolloverLeftover, when true, rolls LAST month's unspent budget (each
	// budget's limit minus what was spent, summed and clamped at zero — excluding
	// budgets that already carry their own remaining via Budget.Rollover) into THIS
	// month's assignable pool in the zero-based view, so leftover becomes extra
	// budget to assign next month. Off by default.
	BudgetRolloverLeftover bool `json:"budgetRolloverLeftover,omitempty"`

	// IdleCashBenchmarkAPR is the user-entered annual yield (percent, e.g. 4.5) the
	// idle-cash figure (AC15) compares against — what a high-yield savings account or
	// money-market fund would pay. It is a stated assumption, never a live feed: the
	// idle-cash copy names the rate. Zero (the default) disables the forgone-yield
	// figure. Omitted from JSON when zero so existing prefs round-trip unchanged.
	IdleCashBenchmarkAPR float64 `json:"idleCashBenchmarkApr,omitempty"`

	// SweepEnabled turns on the monthly surplus-sweep job. When false (the
	// default), RunDueSweeps is a no-op even if the other sweep fields are set.
	SweepEnabled bool `json:"sweepEnabled,omitempty"`
	// SweepFromAccountID is the source account whose surplus is swept each month
	// (typically a checking account). Empty means the sweep is disabled.
	SweepFromAccountID string `json:"sweepFromAccountId,omitempty"`
	// SweepToAccountID is the destination savings account that receives the
	// swept surplus. Empty means the sweep is disabled.
	SweepToAccountID string `json:"sweepToAccountId,omitempty"`
	// SweepBufferMinor is the minimum balance (in the source account's minor
	// currency units) to keep in the source account after a sweep. The sweep
	// only transfers the amount above this floor, so the source account always
	// retains at least SweepBufferMinor. Zero means no floor — sweep the full surplus.
	SweepBufferMinor int64 `json:"sweepBufferMinor,omitempty"`
	// SweepLastPeriod is the PeriodKey("monthly") value for the most recent
	// month in which a sweep was successfully executed. It acts as a once-per-month
	// guard: RunDueSweeps skips the sweep when SweepLastPeriod equals the
	// current month's period key. Format: "2006-01".
	SweepLastPeriod string `json:"sweepLastPeriod,omitempty"`

	// RoundUpEnabled turns on the monthly round-up savings batch. When false
	// (the default), RunDueRoundUps is a no-op even if the other round-up
	// fields are set.
	RoundUpEnabled bool `json:"roundUpEnabled,omitempty"`
	// RoundUpFromAccountID is the spending account whose expense transactions
	// are rounded up each month. Empty means the feature is effectively disabled.
	RoundUpFromAccountID string `json:"roundUpFromAccountId,omitempty"`
	// RoundUpToAccountID is the savings account that receives the consolidated
	// round-up transfer at the end of each month.
	RoundUpToAccountID string `json:"roundUpToAccountId,omitempty"`
	// RoundUpGranularityMinor is the rounding bucket size in minor currency
	// units. 100 means round each spend up to the nearest dollar (default);
	// 500 rounds to the nearest $5; 1000 to the nearest $10. Zero is treated
	// as 100 by RunDueRoundUps.
	RoundUpGranularityMinor int64 `json:"roundUpGranularityMinor,omitempty"`
	// RoundUpLastPeriod is the PeriodKey("monthly") value for the most recent
	// month in which a round-up batch was successfully executed. It acts as a
	// once-per-month guard: RunDueRoundUps skips the batch when
	// RoundUpLastPeriod equals the current month's period key. Format: "2006-01".
	RoundUpLastPeriod string `json:"roundUpLastPeriod,omitempty"`
}

// BackendActive reports whether the app should talk to the backend: a server URL
// and token are configured AND the user hasn't switched the backend off. Every
// sync and AI-proxy path gates on this, so toggling BackendDisabled cleanly stops
// all backend connection attempts.
func (p Prefs) BackendActive() bool {
	return !p.BackendDisabled &&
		strings.TrimSpace(p.ServerURL) != "" &&
		strings.TrimSpace(p.ServerToken) != ""
}

// Default returns the out-of-the-box preferences (Sunday week start, friendly
// long dates, dark theme, green accent, comfortable density, 100% scale, full
// motion). Dates default to the human "Jan 2, 2006" style rather than raw ISO so
// the app reads friendly out of the box (C155); users who want ISO/US/EU can
// switch in Appearance. The backend is DISABLED by default so a fresh install is
// fully local — matching the About page's "Cloud sync is off by default" promise;
// the saved ServerURL is only a prefill for when the user opts in.
func Default() Prefs {
	return Prefs{WeekStart: WeekSunday, DateStyle: DateLong, Theme: ThemeDark, Accent: defaultAccent, Scale: ScaleDefault, ServerMode: ServerSelfHosted, ServerURL: DefaultServerURL, ConnectionSegment: ConnectionLocal, Motion: MotionFull, BackendDisabled: true}
}

// Normalize fills any blank or unrecognized field with its default, so partial or
// older persisted data is always usable.
func (p Prefs) Normalize() Prefs {
	switch p.WeekStart {
	case WeekSunday, WeekMonday, WeekSaturday:
	default:
		p.WeekStart = WeekSunday
	}
	switch p.DateStyle {
	case DateISO, DateUS, DateEU, DateLong:
	default:
		p.DateStyle = DateLong // friendly default (matches Default()); see C155
	}
	switch p.Theme {
	case ThemeDark, ThemeLight, ThemeSystem:
	default:
		p.Theme = ThemeDark
	}
	if !isHexColor(p.Accent) {
		p.Accent = defaultAccent
	}
	switch p.ServerMode {
	case ServerCloud, ServerSelfHosted:
	default:
		p.ServerMode = ServerSelfHosted
	}
	switch p.ConnectionSegment {
	case ConnectionLocal, ConnectionRemote:
	default:
		p.ConnectionSegment = ConnectionLocal
	}
	if p.ServerURL == "" {
		p.ServerURL = DefaultServerURL
	}
	switch {
	case p.Scale == 0:
		p.Scale = ScaleDefault
	case p.Scale < ScaleMin:
		p.Scale = ScaleMin
	case p.Scale > ScaleMax:
		p.Scale = ScaleMax
	}
	if !p.Motion.Valid() {
		p.Motion = MotionFull
	}
	return p
}

// ScaleFraction returns the display scale as a CSS zoom multiplier (e.g. 1.1 for
// 110%), normalized into the supported range.
func (p Prefs) ScaleFraction() float64 {
	return float64(p.Normalize().Scale) / 100
}

// isHexColor reports whether s is a "#rgb" or "#rrggbb" hex color.
func isHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 {
		return false
	}
	if s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// dateLayout maps a date style to its Go reference layout.
func dateLayout(s DateStyle) string {
	switch s {
	case DateUS:
		return "01/02/2006"
	case DateEU:
		return "02/01/2006"
	case DateLong:
		return "Jan 2, 2006"
	default:
		return "2006-01-02"
	}
}

// FormatDate renders a time using the preferred date style.
func (p Prefs) FormatDate(t time.Time) string {
	return t.Format(dateLayout(p.Normalize().DateStyle))
}

// FormatMonthYear renders a month+year label honoring the user's DateStyle.
// For DateISO it returns the compact numeric form ("2006-01"); for all other
// styles (US, EU, Long, and the normalized default) it returns the
// human-friendly abbreviated form ("Jan 2006"). This is used by planning and
// debt-payoff chart labels where a full date would be too wide.
func (p Prefs) FormatMonthYear(t time.Time) string {
	if p.Normalize().DateStyle == DateISO {
		return t.Format("2006-01")
	}
	return t.Format("Jan 2006")
}

// WeekStartWeekday returns the configured first day of the week as a time.Weekday.
func (p Prefs) WeekStartWeekday() time.Weekday {
	switch p.Normalize().WeekStart {
	case WeekMonday:
		return time.Monday
	case WeekSaturday:
		return time.Saturday
	default:
		return time.Sunday
	}
}

// WeekStartOf returns the start of the week (at 00:00 in t's location) that
// contains t, honoring the configured first day of the week.
func (p Prefs) WeekStartOf(t time.Time) time.Time {
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	offset := (int(day.Weekday()) - int(p.WeekStartWeekday()) + 7) % 7
	return day.AddDate(0, 0, -offset)
}
