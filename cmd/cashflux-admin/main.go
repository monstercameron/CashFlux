// SPDX-License-Identifier: MIT

//go:build js && wasm

// cashflux-admin is a Go→WebAssembly SPA that serves as the CashFlux operator
// console. It is loaded from web/admin/index.html under the /console/ route of
// the CashFlux backend server.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
	"github.com/monstercameron/GoWebComponents/utils"
)

// ---------------------------------------------------------------------------
// Domain types (mirrors of internal/server admin response shapes)
// ---------------------------------------------------------------------------

// adminOverview mirrors server.AdminOverviewResponse.
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

// adminUserRow mirrors server.AdminUserRow.
type adminUserRow struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Email              string `json:"email"`
	CreatedAt          string `json:"createdAt"`
	SubscriptionPlan   string `json:"subscriptionPlan,omitempty"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
}

// adminUsersResp mirrors server.AdminUsersResponse.
type adminUsersResp struct {
	Users []adminUserRow `json:"users"`
}

// ---------------------------------------------------------------------------
// View state
// ---------------------------------------------------------------------------

type viewState int

const (
	viewLogin   viewState = iota // token entry form
	viewLoading                  // skeleton while fetching
	viewAuthErr                  // 401/403 from the API
	viewNetErr                   // network/other error
	viewReady                    // data loaded successfully
)

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

// formatMRR formats an integer cents value as a dollar amount: "$12.34".
func formatMRR(cents int64) string {
	dollars := cents / 100
	c := cents % 100
	return fmt.Sprintf("$%d.%02d", dollars, c)
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// fetchAdminData fetches GET /v1/admin/overview and GET /v1/admin/users?limit=100
// using a bearer token. Same-origin relative URLs are used so the SPA works
// regardless of hostname.
func fetchAdminData(token string) (ov *adminOverview, users []adminUserRow, authErr bool, err error) {
	// overview
	req, e := http.NewRequest("GET", "/v1/admin/overview", nil)
	if e != nil {
		return nil, nil, false, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, nil, false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, nil, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, nil, false, fmt.Errorf("overview: HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, nil, false, e
	}
	var o adminOverview
	if e := json.Unmarshal(body, &o); e != nil {
		return nil, nil, false, e
	}

	// users
	req2, e := http.NewRequest("GET", "/v1/admin/users?limit=100", nil)
	if e != nil {
		return &o, nil, false, e
	}
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, e := http.DefaultClient.Do(req2)
	if e != nil {
		return &o, nil, false, e
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		return &o, nil, false, fmt.Errorf("users: HTTP %d", resp2.StatusCode)
	}
	body2, e := io.ReadAll(resp2.Body)
	if e != nil {
		return &o, nil, false, e
	}
	var ur adminUsersResp
	if e := json.Unmarshal(body2, &ur); e != nil {
		return &o, nil, false, e
	}
	return &o, ur.Users, false, nil
}

// ---------------------------------------------------------------------------
// localStorage helpers
// ---------------------------------------------------------------------------

const localStorageKey = "cashflux.admin.token"

func lsGet() string {
	v := js.Global().Get("localStorage").Call("getItem", localStorageKey)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	s := v.String()
	if s == "null" || s == "undefined" {
		return ""
	}
	return s
}

func lsSet(token string) {
	js.Global().Get("localStorage").Call("setItem", localStorageKey, token)
}

func lsRemove() {
	js.Global().Get("localStorage").Call("removeItem", localStorageKey)
}

// ---------------------------------------------------------------------------
// Sub-views (pure render functions — no hooks, only receive plain values and
// func callbacks)
// ---------------------------------------------------------------------------

func loginView(tokenVal string, onInput ui.Handler, onSignIn ui.Handler) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			H1(css.Class("login-title"), Text("CashFlux — Operator Console")),
			P(css.Class("login-sub"), Text("Enter the admin token to sign in.")),
			Label(
				For("admin-token"),
				css.Class("login-label"),
				Text("Admin token"),
			),
			Input(
				ID("admin-token"),
				Type("password"),
				css.Class("login-input"),
				Placeholder("Bearer token…"),
				Value(tokenVal),
				OnInput(onInput),
			),
			Button(
				Type("button"),
				css.Class("btn btn-primary"),
				OnClick(onSignIn),
				Text("Sign in"),
			),
		),
	)
}

func loadingView() ui.Node {
	return Div(
		css.Class("loading-page"),
		Div(css.Class("skeleton skeleton-title")),
		Div(
			css.Class("skeleton-grid"),
			Div(css.Class("skeleton skeleton-card")),
			Div(css.Class("skeleton skeleton-card")),
			Div(css.Class("skeleton skeleton-card")),
			Div(css.Class("skeleton skeleton-card")),
			Div(css.Class("skeleton skeleton-card")),
			Div(css.Class("skeleton skeleton-card")),
		),
	)
}

func authErrView(onSignOut ui.Handler) ui.Node {
	return Div(
		css.Class("error-page"),
		P(css.Class("error-msg"), Text("Not authorized — check the admin token.")),
		Button(
			Type("button"),
			css.Class("btn btn-secondary"),
			OnClick(onSignOut),
			Text("Sign out"),
		),
	)
}

func netErrView(msg string, onSignOut ui.Handler) ui.Node {
	return Div(
		css.Class("error-page"),
		P(css.Class("error-msg"), Text("Network error: "+msg)),
		Button(
			Type("button"),
			css.Class("btn btn-secondary"),
			OnClick(onSignOut),
			Text("Sign out"),
		),
	)
}

func statCard(label, value string) ui.Node {
	return Div(
		css.Class("stat-card"),
		Div(css.Class("stat-label"), Text(label)),
		Div(css.Class("stat-value"), Text(value)),
	)
}

func readyView(ov *adminOverview, users []adminUserRow, onSignOut, onRefresh ui.Handler) ui.Node {
	return Div(
		css.Class("console-page"),
		// Header bar
		Div(
			css.Class("console-header"),
			H1(css.Class("console-title"), Text("CashFlux — Operator Console")),
			Div(
				css.Class("header-actions"),
				Button(
					Type("button"),
					css.Class("btn btn-secondary"),
					OnClick(onRefresh),
					Text("Refresh"),
				),
				Button(
					Type("button"),
					css.Class("btn btn-danger"),
					OnClick(onSignOut),
					Text("Sign out"),
				),
			),
		),
		// Stat cards
		Div(
			css.Class("stat-grid"),
			statCard("Total users", fmt.Sprintf("%d", ov.TotalUsers)),
			statCard("Estimated MRR", formatMRR(ov.EstimatedMRRCents)),
			statCard("Active subs", fmt.Sprintf("%d", ov.SubsActive)),
			statCard("Trialing", fmt.Sprintf("%d", ov.SubsTrialing)),
			statCard("Past-due", fmt.Sprintf("%d", ov.SubsPastDue)),
			statCard("Canceled", fmt.Sprintf("%d", ov.SubsCanceled)),
			statCard("Storage", formatBytes(ov.TotalBlobBytes)),
			statCard("Today's requests", fmt.Sprintf("%d", ov.TodayRequests)),
			statCard("Today's tokens", fmt.Sprintf("%d", ov.TodayTokens)),
		),
		// Users table — rows have no interactive elements so Map is safe.
		Div(
			css.Class("table-section"),
			H2(css.Class("table-title"), Text("Users")),
			Table(
				css.Class("users-table"),
				Thead(
					Tr(
						Th(Text("Email")),
						Th(Text("Provider")),
						Th(Text("Plan")),
						Th(Text("Status")),
						Th(Text("Created")),
					),
				),
				Tbody(
					Map(users, func(u adminUserRow) ui.Node {
						created := u.CreatedAt
						if len(created) >= 10 {
							created = created[:10]
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
							Td(Text(u.Email)),
							Td(Text(u.Provider)),
							Td(Text(plan)),
							Td(Text(status)),
							Td(Text(created)),
						)
					}),
				),
			),
		),
	)
}

// ---------------------------------------------------------------------------
// Root App component
// ---------------------------------------------------------------------------

// App is the root component for the CashFlux operator console SPA.
func App() ui.Node {
	view := ui.UseState(viewLogin)
	tokenInput := ui.UseState("")
	overview := ui.UseState[*adminOverview](nil)
	users := ui.UseState[[]adminUserRow](nil)
	netErrMsg := ui.UseState("")

	// handleTokenInput captures the typed value from the password input.
	handleTokenInput := ui.UseEvent(func(v string) {
		tokenInput.Set(v)
	})

	// handleSignIn validates, sets loading, fetches, and transitions state.
	handleSignIn := ui.UseEvent(func() {
		tok := strings.TrimSpace(tokenInput.Get())
		if tok == "" {
			return
		}
		view.Set(viewLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(viewAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(viewNetErr)
				return
			}
			lsSet(tok)
			overview.Set(ov)
			users.Set(us)
			view.Set(viewReady)
		}()
	})

	// handleSignOut clears stored state and returns to the login view.
	handleSignOut := ui.UseEvent(func() {
		lsRemove()
		overview.Set(nil)
		users.Set(nil)
		tokenInput.Set("")
		view.Set(viewLogin)
	})

	// handleRefresh re-fetches using the stored token.
	handleRefresh := ui.UseEvent(func() {
		tok := lsGet()
		if tok == "" {
			return
		}
		view.Set(viewLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(viewAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(viewNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			view.Set(viewReady)
		}()
	})

	// Auto-load any stored token on mount (runs once — no deps).
	ui.UseEffect(func() func() {
		tok := lsGet()
		if tok == "" {
			return nil
		}
		tokenInput.Set(tok)
		view.Set(viewLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(viewAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(viewNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			view.Set(viewReady)
		}()
		return nil
	})

	// Render based on the current view state.
	switch view.Get() {
	case viewLoading:
		return loadingView()
	case viewAuthErr:
		return authErrView(handleSignOut)
	case viewNetErr:
		return netErrView(netErrMsg.Get(), handleSignOut)
	case viewReady:
		ov := overview.Get()
		us := users.Get()
		if ov == nil {
			return loadingView()
		}
		return readyView(ov, us, handleSignOut, handleRefresh)
	default: // viewLogin
		return loginView(tokenInput.Get(), handleTokenInput, handleSignIn)
	}
}

func main() {
	ui.Render(ui.CreateElement(App), "#app")
	utils.WaitForever()
}
