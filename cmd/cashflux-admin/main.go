// SPDX-License-Identifier: MIT

//go:build js && wasm

// cashflux-admin is a Go→WebAssembly SPA that serves as the CashFlux operator
// console. It is loaded from web/admin/index.html under the /console/ route of
// the CashFlux backend server.
//
// Navigation flow: Home (landing) → Login → Console.
// If a valid token already exists in localStorage the user lands on Console
// directly (auto-load on mount). Sign-out returns to Home.
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

// devCredsResp mirrors server.devCredsResponse.
type devCredsResp struct {
	AdminToken string `json:"adminToken"`
}

// ---------------------------------------------------------------------------
// View state — top-level navigation
// ---------------------------------------------------------------------------

// screen is the top-level navigation state for the SPA.
type screen int

const (
	screenHome    screen = iota // landing / hero
	screenLogin                 // token-entry form
	screenLoading               // skeleton while fetching data
	screenAuthErr               // 401/403 from the API
	screenNetErr                // network / other error
	screenReady                 // data loaded, console visible
)

// ---------------------------------------------------------------------------
// Feature highlight data (static — no interactivity, safe to range)
// ---------------------------------------------------------------------------

type featureCard struct {
	icon  string
	title string
	body  string
}

var featureCards = []featureCard{
	{icon: "🔐", title: "Tenant-safe admin API", body: "Bearer-authenticated endpoints with per-user rate limits and full audit trail."},
	{icon: "☁️", title: "Encrypted artifact sync", body: "Zero-knowledge AES-GCM blob store — the server never sees plaintext client data."},
	{icon: "📈", title: "Subscriptions & MRR at a glance", body: "Active, trialing, past-due and canceled counts with estimated monthly recurring revenue."},
	{icon: "🔢", title: "Per-user usage metering", body: "Daily request and token counters per account, with configurable alert thresholds."},
	{icon: "📋", title: "Audit log", body: "Every sensitive admin action is recorded with actor, resource, and timestamp."},
	{icon: "🖥️", title: "Self-host or cloud", body: "Run locally with a single binary, or deploy to any server. No vendor lock-in."},
}

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

// fetchDevCreds calls GET /console/devcreds. It returns ("", false) when the
// endpoint 404s (production) or any other non-200 status. It returns
// (token, true) only when the server is in dev mode and the request is local.
func fetchDevCreds() (token string, ok bool) {
	resp, err := http.Get("/console/devcreds")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	var dc devCredsResp
	if err := json.Unmarshal(body, &dc); err != nil {
		return "", false
	}
	if dc.AdminToken == "" {
		return "", false
	}
	return dc.AdminToken, true
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
// Sub-views (pure render functions — no hooks, receive plain values + callbacks)
// ---------------------------------------------------------------------------

// homeView renders the landing / hero screen.
// hasToken controls whether the "Open console" button is shown.
func homeView(hasToken bool, onSignIn, onOpenConsole ui.Handler) ui.Node {
	return Div(
		css.Class("home-page"),
		Div(
			css.Class("home-hero"),
			H1(css.Class("home-title"), Text("CashFlux — Operator Console")),
			P(css.Class("home-tagline"), Text("Manage your CashFlux backend: users, subscriptions, usage, and encrypted sync — from one secure interface.")),
			Div(
				css.Class("home-actions"),
				Button(
					Type("button"),
					css.Class("btn btn-primary"),
					Attr("aria-label", "Sign in to the operator console"),
					OnClick(onSignIn),
					Text("Sign in"),
				),
				If(hasToken,
					Button(
						Type("button"),
						css.Class("btn btn-secondary"),
						Attr("aria-label", "Open the console using the stored token"),
						OnClick(onOpenConsole),
						Text("Open console"),
					),
				),
			),
		),
		Div(
			css.Class("feature-grid"),
			Map(featureCards, func(f featureCard) ui.Node {
				return Div(
					css.Class("feature-card"),
					Div(css.Class("feature-icon"), Text(f.icon)),
					Div(
						css.Class("feature-card-body"),
						Div(css.Class("feature-card-title"), Text(f.title)),
						Div(css.Class("feature-card-desc"), Text(f.body)),
					),
				)
			}),
		),
	)
}

// loginView renders the token-entry screen.
// devToken is non-empty only when the server returned a dev-mode token; the
// prefill button is shown only in that case.
func loginView(tokenVal, devToken string, onInput, onSignIn, onBack, onPrefill ui.Handler) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			H1(css.Class("login-title"), Text("Sign in")),
			P(css.Class("login-sub"), Text("Enter the admin token to access the operator console.")),
			If(devToken != "",
				Div(
					css.Class("dev-banner"),
					P(css.Class("dev-hint"), Text("Dev mode — local only")),
					Button(
						Type("button"),
						css.Class("btn btn-dev"),
						Attr("aria-label", "Prefill admin token from dev mode"),
						OnClick(onPrefill),
						Text("Prefill admin (dev)"),
					),
				),
			),
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
				Attr("aria-label", "Sign in with the entered token"),
				OnClick(onSignIn),
				Text("Sign in"),
			),
			Button(
				Type("button"),
				css.Class("btn btn-link"),
				Attr("aria-label", "Back to home"),
				OnClick(onBack),
				Text("Back"),
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
		P(css.Class("error-msg"), Text("Not authorized — check the token.")),
		Button(
			Type("button"),
			css.Class("btn btn-secondary"),
			Attr("aria-label", "Sign out and return to home"),
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
			Attr("aria-label", "Sign out and return to home"),
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
					Attr("aria-label", "Refresh console data"),
					OnClick(onRefresh),
					Text("Refresh"),
				),
				Button(
					Type("button"),
					css.Class("btn btn-danger"),
					Attr("aria-label", "Sign out and return to home"),
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
// Navigation flow: Home → Login → Console (or Home directly to Console when a
// stored token is present).
func App() ui.Node {
	view := ui.UseState(screenHome)
	tokenInput := ui.UseState("")
	devToken := ui.UseState("") // non-empty only in dev mode
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
		view.Set(screenLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			lsSet(tok)
			overview.Set(ov)
			users.Set(us)
			view.Set(screenReady)
		}()
	})

	// handleSignOut clears stored state and returns to the home screen.
	handleSignOut := ui.UseEvent(func() {
		lsRemove()
		overview.Set(nil)
		users.Set(nil)
		tokenInput.Set("")
		devToken.Set("")
		view.Set(screenHome)
	})

	// handleRefresh re-fetches using the stored token.
	handleRefresh := ui.UseEvent(func() {
		tok := lsGet()
		if tok == "" {
			view.Set(screenHome)
			return
		}
		view.Set(screenLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			view.Set(screenReady)
		}()
	})

	// handleGoToLogin transitions to the login screen and fetches dev creds.
	handleGoToLogin := ui.UseEvent(func() {
		view.Set(screenLogin)
		go func() {
			tok, ok := fetchDevCreds()
			if ok {
				devToken.Set(tok)
			}
		}()
	})

	// handleBack returns from login to home.
	handleBack := ui.UseEvent(func() {
		tokenInput.Set("")
		devToken.Set("")
		view.Set(screenHome)
	})

	// handlePrefill fills the token input with the dev-mode token.
	handlePrefill := ui.UseEvent(func() {
		tokenInput.Set(devToken.Get())
	})

	// handleOpenConsole goes straight to the console using the stored token.
	handleOpenConsole := ui.UseEvent(func() {
		tok := lsGet()
		if tok == "" {
			view.Set(screenLogin)
			return
		}
		tokenInput.Set(tok)
		view.Set(screenLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			view.Set(screenReady)
		}()
	})

	// Auto-load any stored token on mount (runs once — no deps). If a valid
	// token exists, bypass home and go straight to the console.
	ui.UseEffect(func() func() {
		tok := lsGet()
		if tok == "" {
			return nil
		}
		tokenInput.Set(tok)
		view.Set(screenLoading)
		go func() {
			ov, us, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				// Token is stored but no longer valid; return to home.
				lsRemove()
				tokenInput.Set("")
				view.Set(screenHome)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			view.Set(screenReady)
		}()
		return nil
	})

	// Render based on current navigation state.
	switch view.Get() {
	case screenLoading:
		return loadingView()
	case screenAuthErr:
		return authErrView(handleSignOut)
	case screenNetErr:
		return netErrView(netErrMsg.Get(), handleSignOut)
	case screenReady:
		ov := overview.Get()
		us := users.Get()
		if ov == nil {
			return loadingView()
		}
		return readyView(ov, us, handleSignOut, handleRefresh)
	case screenLogin:
		return loginView(tokenInput.Get(), devToken.Get(), handleTokenInput, handleSignIn, handleBack, handlePrefill)
	default: // screenHome
		hasToken := lsGet() != ""
		return homeView(hasToken, handleGoToLogin, handleOpenConsole)
	}
}

func main() {
	ui.Render(ui.CreateElement(App), "#app")
	utils.WaitForever()
}
