// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// ---------------------------------------------------------------------------
// Client-side mirrors of server/admin.go response types
// ---------------------------------------------------------------------------

// adminOverview mirrors server.AdminOverviewResponse (internal/server/admin.go).
// Field names match the JSON tags on the server struct exactly.
type adminOverview struct {
	TotalUsers        int64  `json:"totalUsers"`
	SubsActive        int64  `json:"subsActive"`
	SubsTrialing      int64  `json:"subsTrialing"`
	SubsPastDue       int64  `json:"subsPastDue"`
	SubsCanceled      int64  `json:"subsCanceled"`
	EstimatedMRRCents int64  `json:"estimatedMrrCents"`
	TotalBlobBytes    int64  `json:"totalBlobBytes"`
	TodayRequests     int64  `json:"todayRequests"`
	TodayTokens       int64  `json:"todayTokens"`
	Day               string `json:"day"`
}

// adminUserRow mirrors server.AdminUserRow (internal/server/repository.go).
type adminUserRow struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Email              string `json:"email"`
	CreatedAt          string `json:"createdAt"`
	SubscriptionPlan   string `json:"subscriptionPlan,omitempty"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
}

// adminUsersResponse mirrors server.AdminUsersResponse.
type adminUsersResponse struct {
	Users  []adminUserRow `json:"users"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// ---------------------------------------------------------------------------
// Fetch helpers
// ---------------------------------------------------------------------------

// fetchAdminOverview GETs /v1/admin/overview, invoking onDone on success,
// onForbidden on 401/403, or onError on any other failure.
func fetchAdminOverview(endpoint, token string, onDone func(adminOverview), onForbidden func(), onError func(string)) {
	endpoint = normalizedAdminEndpoint(endpoint)
	if endpoint == "" || strings.TrimSpace(token) == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/admin/overview", nil)
		if err != nil {
			onError(uistate.T("admin.errorOverview"))
			return
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("admin overview fetch error", "err", err)
			onError(uistate.T("admin.errorOverview"))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			onForbidden()
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			onError(fmt.Sprintf("%s (HTTP %d)", uistate.T("admin.errorOverview"), resp.StatusCode))
			return
		}
		var ov adminOverview
		if err := json.NewDecoder(resp.Body).Decode(&ov); err != nil {
			onError(uistate.T("admin.errorOverview"))
			return
		}
		onDone(ov)
	}()
}

// fetchAdminUsers GETs /v1/admin/users, invoking onDone on success or onError
// on failure. Forbidden responses are surfaced through onError with the access-
// denied key (the overview fetch already gated access — this is a belt-and-
// suspenders guard).
func fetchAdminUsers(endpoint, token string, onDone func([]adminUserRow), onError func(string)) {
	endpoint = normalizedAdminEndpoint(endpoint)
	if endpoint == "" || strings.TrimSpace(token) == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/admin/users?limit=50&offset=0", nil)
		if err != nil {
			onError(uistate.T("admin.errorUsers"))
			return
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("admin users fetch error", "err", err)
			onError(uistate.T("admin.errorUsers"))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			onError(fmt.Sprintf("%s (HTTP %d)", uistate.T("admin.errorUsers"), resp.StatusCode))
			return
		}
		var ur adminUsersResponse
		if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
			onError(uistate.T("admin.errorUsers"))
			return
		}
		onDone(ur.Users)
	}()
}

// normalizedAdminEndpoint strips trailing slashes from the endpoint, falling
// back to the localhost default when blank. Reuses the same trimming logic as
// internal/app/backend.go's normalizedBackendEndpoint.
func normalizedAdminEndpoint(endpoint string) string {
	return strings.TrimRight(strings.TrimSpace(endpoint), "/")
}

// ---------------------------------------------------------------------------
// Formatting helpers (local — not exported; no business logic)
// ---------------------------------------------------------------------------

// fmtCents formats a USD-denominated integer-cents MRR value as "$1,234.56"
// using the canonical money.FormatAccounting path.
func fmtCents(cents int64) string {
	return money.FormatAccounting(cents, 2, "$")
}

// fmtInt64 renders a int64 count with comma-grouping.
func fmtInt64(n int64) string {
	return money.Group(fmt.Sprintf("%d", n))
}

// fmtBytes renders a byte count as a human-readable string (B / KB / MB / GB).
func fmtBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

// adminScreenState is the full fetch state for the admin console screen.
type adminScreenState int

const (
	adminStateLoading   adminScreenState = iota // initial fetch in flight
	adminStateSignIn                            // no endpoint+token configured
	adminStateForbidden                         // 401/403 from overview
	adminStateError                             // other network / decode error
	adminStateReady                             // data loaded OK
)

// AdminConsole is the operator admin console screen (/admin). It renders two
// sections: a platform-overview stat grid and a users table. Access is gated:
//   - No endpoint/token → "sign in" empty state.
//   - 401/403 from overview → "admin only" empty state.
//   - Network/decode error → error state with retry.
//   - HTTP 200 → stat grid + users table.
//
// All labels are resolved via uistate.T so i18n works. All interactive elements
// carry aria-label/title and are keyboard-reachable (Type("button") on every
// Button). Uses the canonical Card/StatGrid/DataTable primitives.
func AdminConsole() ui.Node {
	prefsAtom := uistate.UsePrefs()
	pr := prefsAtom.Get().Normalize()
	endpoint, token := pr.ServerURL, pr.ServerToken
	signedIn := strings.TrimSpace(endpoint) != "" && strings.TrimSpace(token) != ""

	// Component state atoms.
	screenState := ui.UseState(adminStateLoading)
	errMsg := ui.UseState("")
	overview := ui.UseState(adminOverview{})
	users := ui.UseState([]adminUserRow(nil))
	loadRev := ui.UseState(0) // bumped by Retry to re-fire the effect

	// On mount (and Retry): fetch overview first, then users.
	ui.UseEffect(func() func() {
		if !signedIn {
			screenState.Set(adminStateSignIn)
			return nil
		}
		screenState.Set(adminStateLoading)
		fetchAdminOverview(endpoint, token,
			func(ov adminOverview) {
				overview.Set(ov)
				// Overview succeeded — now fetch users.
				fetchAdminUsers(endpoint, token,
					func(rows []adminUserRow) {
						users.Set(rows)
						screenState.Set(adminStateReady)
					},
					func(msg string) {
						errMsg.Set(msg)
						screenState.Set(adminStateError)
					},
				)
			},
			func() { screenState.Set(adminStateForbidden) },
			func(msg string) {
				errMsg.Set(msg)
				screenState.Set(adminStateError)
			},
		)
		return nil
	}, endpoint+"\x00"+token+"\x00"+itoa64(loadRev.Get()))

	retry := ui.UseEvent(func() { loadRev.Set(loadRev.Get() + 1) })
	// Declared before the switch so its hook position is stable across states.
	openCloudSettings := ui.UseEvent(func() { uistate.OpenGlobalSettingsAt("cloud") })

	switch screenState.Get() {
	case adminStateSignIn:
		// §8.9 gated state: title + why it's gated + the value + one primary action
		// (open Settings → Cloud to sign in), instead of a lone dead sentence.
		// Use uiw.Card so the scaffold guard in internal/screenlint stays clean.
		return uiw.Card(uiw.CardProps{
			ClassParts: []any{"admin-gate"},
			Title:      uistate.T("admin.signInTitle"),
			Body: Fragment(
				P(css.Class("t-body text-dim"), uistate.T("admin.signInPrompt")),
				Div(css.Class(tw.Fold(tw.Mt3)),
					Button(css.Class("btn btn-primary"), Type("button"),
						Attr("data-testid", "admin-signin-cta"),
						OnClick(openCloudSettings), uistate.T("admin.signInCta")),
				),
			),
		})

	case adminStateForbidden:
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Icon:    icon.Ban,
			Message: uistate.T("admin.accessDenied"),
		})

	case adminStateError:
		return uiw.Card(uiw.CardProps{
			Title: uistate.T("admin.overviewTitle"),
			Body: Fragment(
				P(css.Class("empty"), errMsg.Get()),
				Button(css.Class("btn btn-primary"), Type("button"),
					Attr("aria-label", uistate.T("admin.retry")),
					Title(uistate.T("admin.retry")),
					OnClick(retry),
					uistate.T("admin.retry"),
				),
			),
		})

	case adminStateLoading:
		return uiw.Card(uiw.CardProps{
			Title: uistate.T("admin.overviewTitle"),
			Body:  uiw.Skeleton(uiw.SkeletonProps{Lines: 4, AriaLabel: uistate.T("admin.loading")}),
		})
	}

	// adminStateReady — render overview cards + users table.
	ov := overview.Get()
	stats := []uiw.Stat{
		{Label: uistate.T("admin.totalUsers"), Value: fmtInt64(ov.TotalUsers)},
		{Label: uistate.T("admin.estimatedMRR"), Value: fmtCents(ov.EstimatedMRRCents), Tone: "pos"},
		{Label: uistate.T("admin.subsActive"), Value: fmtInt64(ov.SubsActive), Tone: "pos"},
		{Label: uistate.T("admin.subsTrialing"), Value: fmtInt64(ov.SubsTrialing)},
		{Label: uistate.T("admin.subsPastDue"), Value: fmtInt64(ov.SubsPastDue), Tone: "neg"},
		{Label: uistate.T("admin.subsCanceled"), Value: fmtInt64(ov.SubsCanceled), Tone: "neg"},
		{Label: uistate.T("admin.totalStorage"), Value: fmtBytes(ov.TotalBlobBytes)},
		{Label: uistate.T("admin.todayRequests"), Value: fmtInt64(ov.TodayRequests)},
		{Label: uistate.T("admin.todayTokens"), Value: fmtInt64(ov.TodayTokens)},
	}

	dayLabel := Fragment()
	if ov.Day != "" {
		dayLabel = P(css.Class("muted", tw.Text12), uistate.T("admin.dayLabel")+" "+ov.Day)
	}

	overviewCard := uiw.Card(uiw.CardProps{
		Title: uistate.T("admin.overviewTitle"),
		Body: Fragment(
			uiw.StatGrid(stats),
			dayLabel,
		),
	})

	// Users table — build rows (no interactive elements per row so no component
	// needed; safe to call inside MapKeyed).
	userList := users.Get()
	cols := []uiw.Column{
		{Label: uistate.T("admin.colEmail")},
		{Label: uistate.T("admin.colProvider")},
		{Label: uistate.T("admin.colPlan")},
		{Label: uistate.T("admin.colStatus")},
		{Label: uistate.T("admin.colCreated")},
	}

	var tableBody any
	if len(userList) == 0 {
		tableBody = Tr(Td(Attr("colspan", "5"), css.Class("empty"), uistate.T("admin.noUsers")))
	} else {
		tableBody = MapKeyed(userList,
			func(u adminUserRow) any { return u.ID },
			func(u adminUserRow) ui.Node {
				created := u.CreatedAt
				if len(created) >= 10 {
					created = created[:10] // trim to YYYY-MM-DD
				}
				plan := u.SubscriptionPlan
				if plan == "" {
					plan = "—"
				}
				status := u.SubscriptionStatus
				if status == "" {
					status = "—"
				}
				return Tr(
					Td(u.Email),
					Td(u.Provider),
					Td(plan),
					Td(status),
					Td(created),
				)
			},
		)
	}

	usersCard := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("admin.usersTitle"),
		Body: uiw.DataTable(uiw.DataTableProps{
			Class:   "admin-users-table",
			Columns: cols,
			Body:    tableBody,
		}),
	})

	return Div(overviewCard, usersCard)
}

// itoa64 converts an int to its decimal string representation.
// Mirrors the itoa helper in deviceslist.go but accepts int directly
// (loadRev state uses int, not int64).
func itoa64(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
