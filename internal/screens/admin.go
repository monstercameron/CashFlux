// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/adminclient"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type adminOverview = adminclient.Overview
type adminUserRow = adminclient.User

// ---------------------------------------------------------------------------
// Fetch helpers
// ---------------------------------------------------------------------------

// fetchAdminOverview GETs /v1/admin/overview, invoking onDone on success,
// onForbidden on 401/403, or onError on any other failure.
func fetchAdminOverview(endpoint, token string, onDone func(adminclient.OverviewResult), onForbidden func(), onError func(string)) {
	go func() {
		result, err := adminclient.Default.Overview(context.Background(), endpoint, token)
		if err != nil {
			var requestErr *adminclient.RequestError
			if errors.As(err, &requestErr) && (requestErr.Status == 401 || requestErr.Status == 403) {
				onForbidden()
				return
			}
			if errors.As(err, &requestErr) && requestErr.Status != 0 {
				onError(fmt.Sprintf("%s (HTTP %d)", uistate.T("admin.errorOverview"), requestErr.Status))
				return
			}
			onError(uistate.T("admin.errorOverview"))
			return
		}
		onDone(result)
	}()
}

// fetchAdminUsers GETs /v1/admin/users, invoking onDone on success or onError
// on failure. Forbidden responses remain an explicit HTTP error after the
// overview has gated access.
func fetchAdminUsers(endpoint, token string, onDone func(adminclient.UsersResult), onError func(string)) {
	go func() {
		result, err := adminclient.Default.Users(context.Background(), endpoint, token, 50, 0)
		if err != nil {
			var requestErr *adminclient.RequestError
			if errors.As(err, &requestErr) && requestErr.Status != 0 {
				onError(fmt.Sprintf("%s (HTTP %d)", uistate.T("admin.errorUsers"), requestErr.Status))
				return
			}
			onError(uistate.T("admin.errorUsers"))
			return
		}
		onDone(result)
	}()
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
	overviewFetch := ui.UseState(adminclient.FetchMeta{})
	users := ui.UseState([]adminUserRow(nil))
	usersFetch := ui.UseState(adminclient.FetchMeta{})
	loadRev := ui.UseState(0) // bumped by Retry to re-fire the effect

	// On mount (and Retry): fetch overview first, then users.
	ui.UseEffect(func() func() {
		if !signedIn {
			screenState.Set(adminStateSignIn)
			return nil
		}
		screenState.Set(adminStateLoading)
		fetchAdminOverview(endpoint, token,
			func(result adminclient.OverviewResult) {
				overview.Set(result.Overview)
				overviewFetch.Set(result.FetchMeta)
				// Overview succeeded — now fetch users.
				fetchAdminUsers(endpoint, token,
					func(result adminclient.UsersResult) {
						users.Set(result.Users)
						usersFetch.Set(result.FetchMeta)
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
	overviewFetched := adminFetchLabel(overviewFetch.Get(), "overview", "admin.overviewTitle")

	overviewCard := uiw.Card(uiw.CardProps{
		Title: uistate.T("admin.overviewTitle"),
		Body: Fragment(
			uiw.StatGrid(stats),
			dayLabel,
			overviewFetched,
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
		Body: Div(
			adminFetchLabel(usersFetch.Get(), "users", "admin.usersTitle"),
			uiw.DataTable(uiw.DataTableProps{
				Class:   "admin-users-table",
				Columns: cols,
				Body:    tableBody,
			}),
		),
	})

	return Div(overviewCard, usersCard)
}

func adminFetchLabel(meta adminclient.FetchMeta, section, sectionLabelKey string) ui.Node {
	if meta.FetchedAt.IsZero() {
		return Fragment()
	}
	label := uistate.T("admin.fetchedAt", uistate.T(sectionLabelKey), meta.FetchedAt.Local().Format("Jan 2, 2006 3:04 PM"))
	if meta.FromCache {
		label += " · " + uistate.T("admin.cached")
	}
	return P(css.Class("muted", tw.Text12), Attr("data-testid", "admin-fetch-meta-"+section), label)
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
