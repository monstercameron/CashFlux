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
	"net/url"
	"strings"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
	"github.com/monstercameron/GoWebComponents/v5/utils"
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
	Username           string `json:"username,omitempty"`
	Role               string `json:"role"`
	CreatedAt          string `json:"createdAt"`
	SubscriptionPlan   string `json:"subscriptionPlan,omitempty"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
	// Identity/sync columns (C698): which of these rows is the browser in front
	// of you, and is it actually working.
	Workspaces     int    `json:"workspaces"`
	Devices        int    `json:"devices"`
	LastSyncAt     string `json:"lastSyncAt,omitempty"`
	PendingDevices int    `json:"pendingDevices,omitempty"`
}

// adminUsersResp mirrors server.AdminUsersResponse.
type adminUsersResp struct {
	Users   []adminUserRow `json:"users"`
	HasMore bool           `json:"hasMore"`
	Query   string         `json:"query,omitempty"`
}

// usersPageSize is the operator-console page size for the users table.
const usersPageSize = 25

// auditEvent mirrors server.AuditEvent — one row of the append-only security log.
type auditEvent struct {
	ID         int64  `json:"id"`
	Timestamp  string `json:"timestamp"`
	ActorID    string `json:"actorId"`
	Action     string `json:"action"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	IP         string `json:"ip,omitempty"`
}

// devCredsResp mirrors server.devCredsResponse.
type devCredsResp struct {
	AdminToken string `json:"adminToken"`
}

type adminBrowserSession struct {
	Authenticated bool   `json:"authenticated"`
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	ExpiresIn     int64  `json:"expiresIn"`
}

type adminSetupStatus struct {
	Required bool `json:"required"`
}

type adminSetupResponse struct {
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	RecoveryCode string `json:"recoveryCode"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// View state — top-level navigation
// ---------------------------------------------------------------------------

// screen is the top-level navigation state for the SPA.
type screen int

const (
	screenHome          screen = iota // landing / hero
	screenLogin                       // token-entry form
	screenLoading                     // skeleton while fetching data
	screenAuthErr                     // 401/403 from the API
	screenNetErr                      // network / other error
	screenReady                       // data loaded, console visible
	screenManage                      // managing a single user (detail + actions)
	screenCreate                      // creating a username/password user
	screenAudit                       // the global security audit log
	screenSetup                       // one-time first-owner registration
	screenSetupRecovery               // one-time recovery-code acknowledgement
)

// ---------------------------------------------------------------------------
// Feature highlight data (static — no interactivity, safe to range)
// ---------------------------------------------------------------------------

type featureCard struct {
	num   string
	title string
	body  string
}

var featureCards = []featureCard{
	{num: "01", title: "See your whole money picture", body: "Net worth, cash flow, budgets and goals on one calm dashboard — so you always know what you have and what's coming."},
	{num: "02", title: "Budget the way you think", body: "Give every dollar a job, or carry envelopes forward. Weekly, monthly or quarterly — with a gentle nudge before you overspend."},
	{num: "03", title: "Plan ahead with confidence", body: "Forecast your cash flow, pay down debt with snowball or avalanche, and watch every savings goal get closer."},
	{num: "04", title: "Private by default", body: "Your money stays on your device, encrypted. No account to create, nothing sold, and it never phones home."},
	{num: "05", title: "Every number, explained", body: "Tap any figure — a budget, a forecast, a balance — and see the exact transactions behind it. No black boxes."},
	{num: "06", title: "Yours to keep, forever", body: "Export everything to CSV or JSON in one click. No lock-in, no proprietary trap — leave any time with all your data."},
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

var adminCSRF string

func adminCookieValue(name string) string {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return ""
	}
	for _, part := range strings.Split(doc.Get("cookie").String(), ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && key == name {
			return value
		}
	}
	return ""
}

func currentAdminCSRF() string {
	if strings.TrimSpace(adminCSRF) != "" {
		return adminCSRF
	}
	return adminCookieValue("cashflux_admin_csrf")
}

func captureAdminCSRF(resp *http.Response) {
	if resp == nil {
		return
	}
	if token := strings.TrimSpace(resp.Header.Get("X-CashFlux-CSRF")); token != "" {
		adminCSRF = token
	}
}

func applyAdminRequestAuth(req *http.Request, token string, mutation bool) {
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mutation {
		if csrf := currentAdminCSRF(); csrf != "" {
			req.Header.Set("X-CashFlux-CSRF", csrf)
		}
	}
}

func signInAdminCredentials(username, password string) (*adminBrowserSession, bool, error) {
	body, _ := json.Marshal(map[string]string{"username": strings.TrimSpace(username), "password": password})
	req, err := http.NewRequest(http.MethodPost, "/v1/admin/session/login", strings.NewReader(string(body)))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	captureAdminCSRF(resp)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("sign in: HTTP %d", resp.StatusCode)
	}
	var session adminBrowserSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, false, err
	}
	return &session, false, nil
}

func fetchAdminSetupStatus() (bool, error) {
	code, body, err := adminDo("", http.MethodGet, "/v1/admin/setup", "")
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("owner setup status: HTTP %d", code)
	}
	var status adminSetupStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return false, err
	}
	return status.Required, nil
}

func createInitialOwner(setupKey, username, password string) (*adminSetupResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"setupKey": strings.TrimSpace(setupKey),
		"username": strings.TrimSpace(username),
		"password": password,
	})
	code, responseBody, err := adminDo("", http.MethodPost, "/v1/admin/setup", string(body))
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		var response apiErrorResponse
		if json.Unmarshal(responseBody, &response) == nil && strings.TrimSpace(response.Error.Message) != "" {
			return nil, fmt.Errorf("%s", response.Error.Message)
		}
		return nil, fmt.Errorf("owner setup: HTTP %d", code)
	}
	var response adminSetupResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func refreshAdminSession() bool {
	req, err := http.NewRequest(http.MethodPost, "/v1/admin/session/refresh", nil)
	if err != nil {
		return false
	}
	applyAdminRequestAuth(req, "", true)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	captureAdminCSRF(resp)
	return resp.StatusCode == http.StatusOK
}

func restoreAdminSession() bool {
	code, _, err := adminDo("", http.MethodGet, "/v1/admin/session", "")
	return err == nil && code == http.StatusOK
}

func signOutAdminSession() {
	req, err := http.NewRequest(http.MethodPost, "/v1/admin/session/logout", nil)
	if err == nil {
		applyAdminRequestAuth(req, "", true)
		if resp, doErr := http.DefaultClient.Do(req); doErr == nil {
			resp.Body.Close()
		}
	}
	adminCSRF = ""
}

// fetchAdminData fetches the overview and first users page. A non-empty token
// is the explicit break-glass path; an empty token uses the HttpOnly owner
// session cookies and transparently refreshes once on an expired access token.
func fetchAdminData(token string) (ov *adminOverview, users []adminUserRow, hasMore bool, authErr bool, err error) {
	code, body, e := adminDo(token, http.MethodGet, "/v1/admin/overview", "")
	if e != nil {
		return nil, nil, false, false, e
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return nil, nil, false, true, nil
	}
	if code != http.StatusOK {
		return nil, nil, false, false, fmt.Errorf("overview: HTTP %d", code)
	}
	var o adminOverview
	if e := json.Unmarshal(body, &o); e != nil {
		return nil, nil, false, false, e
	}

	// first page of users
	us, more, _, e := fetchUsers(token, "", 0)
	if e != nil {
		return &o, nil, false, false, e
	}
	return &o, us, more, false, nil
}

// fetchUsers loads one page of the users table, optionally filtered by an email
// substring. authErr is true on 401/403. Returns the page rows and whether a
// further page exists (from the server's has-more probe).
func fetchUsers(token, query string, offset int) (users []adminUserRow, hasMore bool, authErr bool, err error) {
	u := fmt.Sprintf("/v1/admin/users?limit=%d&offset=%d", usersPageSize, offset)
	if q := strings.TrimSpace(query); q != "" {
		u += "&q=" + url.QueryEscape(q)
	}
	code, body, e := adminDo(token, http.MethodGet, u, "")
	if e != nil {
		return nil, false, false, e
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return nil, false, true, nil
	}
	if code != http.StatusOK {
		return nil, false, false, fmt.Errorf("users: HTTP %d", code)
	}
	var ur adminUsersResp
	if e := json.Unmarshal(body, &ur); e != nil {
		return nil, false, false, e
	}
	return ur.Users, ur.HasMore, false, nil
}

// fetchAudit loads the global security audit log (admin-scoped server-side). The
// endpoint streams newline-delimited JSON (one AuditEvent per line), newest last;
// this parses each line and returns them newest-first for display.
func fetchAudit(token string, limit int) (events []auditEvent, authErr bool, err error) {
	code, body, e := adminDo(token, http.MethodGet, fmt.Sprintf("/v1/audit?limit=%d", limit), "")
	if e != nil {
		return nil, false, e
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return nil, true, nil
	}
	if code != http.StatusOK {
		return nil, false, fmt.Errorf("audit: HTTP %d", code)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev auditEvent
		if json.Unmarshal([]byte(line), &ev) == nil {
			events = append(events, ev)
		}
	}
	// Reverse to newest-first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, false, nil
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

// brandMark renders the gradient CashFlux wordmark. A non-empty tag appends a
// muted descriptor (e.g. "Operator Console") beside the name.
func brandMark(tag string) ui.Node {
	return Div(
		css.Class("brand"),
		Div(css.Class("brand-mark"), Text("C")),
		Span(css.Class("brand-name"), Text("CashFlux")),
		If(tag != "", Span(css.Class("brand-tag"), Text(tag))),
	)
}

// trustItem renders one check-marked reassurance line under the hero CTA.
func trustItem(label string) ui.Node {
	return Span(
		css.Class("trust-item"),
		Span(css.Class("chk"), Text("✓")),
		Text(label),
	)
}

// statPill renders one cell of the hero stats band.
func statPill(num, caption string) ui.Node {
	return Div(
		css.Class("stat-pill"),
		Div(css.Class("num"), Text(num)),
		Div(css.Class("cap"), Text(caption)),
	)
}

// shotFrame wraps a product screenshot in a browser-style chrome (traffic-light
// dots + a faux URL) so the landing demonstrates the real app.
func shotFrame(urlLabel, src, alt string) ui.Node {
	return Div(
		css.Class("frame"),
		Div(
			css.Class("frame-bar"),
			Span(css.Class("frame-dot")),
			Span(css.Class("frame-dot")),
			Span(css.Class("frame-dot")),
			Span(css.Class("frame-url"), Text(urlLabel)),
		),
		Img(Attr("src", src), Attr("alt", alt), Attr("loading", "lazy")),
	)
}

// homeView renders the marketing landing screen: a sticky nav, a gradient hero,
// a stats band, the feature grid, a closing call-to-action, and a footer.
// hasToken controls whether the "Open console" shortcut is offered (a valid
// token is already stored from a previous session).
func homeView(hasToken bool, onSignIn, onOpenConsole ui.Handler) ui.Node {
	return Div(
		// Sticky top navigation.
		Div(
			css.Class("nav"),
			Div(
				css.Class("nav-inner"),
				brandMark("Operator Console"),
				Div(
					css.Class("nav-actions"),
					If(hasToken,
						Button(
							Type("button"),
							css.Class("btn btn-ghost btn-sm"),
							Attr("aria-label", "Open the console using the stored token"),
							OnClick(onOpenConsole),
							Text("Open console"),
						),
					),
					Button(
						Type("button"),
						css.Class("btn btn-primary btn-sm"),
						Attr("aria-label", "Sign in to the operator console"),
						OnClick(onSignIn),
						Text("Sign in"),
					),
				),
			),
		),
		// Hero.
		Div(
			css.Class("wrap"),
			Div(
				css.Class("hero"),
				Span(
					css.Class("eyebrow fade d1"),
					Span(css.Class("dot")),
					Text("Private money management · yours, on your device"),
				),
				H1(css.Class("hero-title fade d2"), Text("Finally know where your money goes.")),
				P(
					css.Class("hero-sub fade d3"),
					Text("CashFlux brings your accounts, budgets, goals and bills into one calm dashboard — so you always know what you have, what's coming, and where it went. No bank logins. No ads. No account required."),
				),
				Div(
					css.Class("hero-actions fade d4"),
					Button(
						Type("button"),
						css.Class("btn btn-primary btn-lg"),
						Attr("aria-label", "Get started with CashFlux"),
						OnClick(onSignIn),
						Text("Get started — free"),
					),
					A(Attr("href", "#features"), css.Class("btn btn-secondary btn-lg"), Text("See how it works")),
				),
				Div(
					css.Class("hero-trust fade d5"),
					trustItem("No bank logins, ever"),
					trustItem("No ads or trackers"),
					trustItem("Export anytime"),
				),
			),
			Div(
				css.Class("shot-hero fade d5"),
				shotFrame("cashflux · your dashboard", "img/dashboard.png", "The CashFlux dashboard: net worth, income, spending, budgets and a net-worth trend chart"),
			),
		),
		// Stats band.
		Div(
			css.Class("wrap"),
			Div(
				css.Class("stats-band fade d3"),
				statPill("$0", "To get started"),
				statPill("100%", "On your device"),
				statPill("Zero", "Ads · trackers · resold data"),
				statPill("1-click", "Export all your data"),
			),
		),
		// Features.
		Div(
			css.Class("wrap"),
			Div(
				ID("features"),
				css.Class("section"),
				Div(
					css.Class("section-head"),
					Div(css.Class("section-eyebrow"), Text("Why CashFlux")),
					H2(css.Class("section-title"), Text("Everything your money needs. Nothing it doesn't.")),
					P(
						css.Class("section-desc"),
						Text("A complete picture of your finances — accounts, budgets, goals, bills and forecasts — that stays private, stays simple, and always shows its work."),
					),
				),
				Div(
					css.Class("feature-grid"),
					Map(featureCards, func(f featureCard) ui.Node {
						return Div(
							css.Class("feature-card fade"),
							Div(css.Class("feat-num"), Text(f.num)),
							Div(css.Class("feature-card-title"), Text(f.title)),
							P(css.Class("feature-card-desc"), Text(f.body)),
						)
					}),
				),
			),
		),
		// Product screenshots.
		Div(
			css.Class("wrap"),
			Div(
				css.Class("section"),
				Div(
					css.Class("section-head"),
					Div(css.Class("section-eyebrow"), Text("See it in action")),
					H2(css.Class("section-title"), Text("Clarity at a glance")),
				),
				Div(
					css.Class("shots-grid"),
					Div(
						shotFrame("cashflux · reports", "img/reports.png", "CashFlux reports: net, income, spending, savings rate and spending by category"),
						Div(
							css.Class("shot-cap"),
							H3(Text("Reports that actually explain")),
							P(Text("Net, income, savings rate and cash runway — plus plain-English insights like “Transit is 200% above its usual.”")),
						),
					),
					Div(
						shotFrame("cashflux · transactions", "img/transactions.png", "CashFlux transactions ledger with categories, accounts and tags"),
						Div(
							css.Class("shot-cap"),
							H3(Text("Every transaction, organized")),
							P(Text("Search, filter, tag and auto-categorize. Reconcile against your statement, then export whenever you like.")),
						),
					),
				),
			),
		),
		// Closing call-to-action.
		Div(
			css.Class("wrap"),
			Div(
				css.Class("cta-band"),
				H2(Text("Take control of your money today.")),
				P(Text("It's free, private, and yours to keep. Pick up right where you left off.")),
				Button(
					Type("button"),
					css.Class("btn btn-primary btn-lg"),
					Attr("aria-label", "Get started with CashFlux"),
					OnClick(onSignIn),
					Text("Get started"),
				),
			),
		),
		// Footer.
		Div(
			css.Class("wrap"),
			Div(
				css.Class("footer"),
				Div(
					css.Class("footer-inner"),
					Span(Attr("style", "color:var(--text-faint);font-size:13px;"), Text("© 2026 CashFlux · Operator Console")),
					Div(
						css.Class("footer-links"),
						A(Attr("href", "/v1/version"), Text("API")),
						A(Attr("href", "/status"), Text("Status")),
						A(Attr("href", "/legal/privacy"), Text("Privacy")),
					),
				),
			),
		),
	)
}

// credentialLoginView is the production operator door. The static server
// token is retained only behind the explicit break-glass disclosure.
func credentialLoginView(
	username, password, tokenVal, devToken string,
	advanced bool,
	onUsername, onPassword, onCredentialSignIn, onToggleAdvanced,
	onToken, onTokenSignIn, onBack, onPrefill ui.Handler,
) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			Div(css.Class("login-brand"), brandMark("")),
			H1(css.Class("login-title"), Text("Sign in")),
			P(css.Class("login-sub"), Text("Use your CashFlux owner account to open the operator console.")),
			Label(For("admin-username"), css.Class("login-label"), Text("Username")),
			Input(
				ID("admin-username"),
				Type("text"),
				Attr("autocomplete", "username"),
				Attr("data-testid", "admin-username"),
				css.Class("login-input"),
				Placeholder("Username"),
				Value(username),
				OnInput(onUsername),
			),
			Label(For("admin-password"), css.Class("login-label"), Text("Password")),
			Input(
				ID("admin-password"),
				Type("password"),
				Attr("autocomplete", "current-password"),
				Attr("data-testid", "admin-password"),
				css.Class("login-input"),
				Placeholder("Password"),
				Value(password),
				OnInput(onPassword),
			),
			Button(
				Type("button"),
				css.Class("btn btn-primary"),
				Attr("data-testid", "admin-credential-signin"),
				Attr("aria-label", "Sign in with owner credentials"),
				OnClick(onCredentialSignIn),
				Text("Sign in"),
			),
			Button(
				Type("button"),
				css.Class("btn btn-link"),
				Attr("data-testid", "admin-break-glass-toggle"),
				Attr("aria-expanded", fmt.Sprintf("%t", advanced)),
				OnClick(onToggleAdvanced),
				Text("Use a break-glass token"),
			),
			If(advanced, Div(
				css.Class("dev-banner"),
				P(css.Class("dev-hint"), Text("Advanced recovery access")),
				If(devToken != "",
					Button(
						Type("button"),
						css.Class("btn btn-dev"),
						Attr("aria-label", "Prefill admin token from dev mode"),
						OnClick(onPrefill),
						Text("Prefill admin (dev)"),
					),
				),
				Label(For("admin-token"), css.Class("login-label"), Text("Administrator token")),
				Input(
					ID("admin-token"),
					Type("password"),
					Attr("data-testid", "admin-break-glass-token"),
					css.Class("login-input"),
					Placeholder("Bearer token"),
					Value(tokenVal),
					OnInput(onToken),
				),
				Button(
					Type("button"),
					css.Class("btn btn-secondary"),
					Attr("data-testid", "admin-break-glass-signin"),
					Attr("aria-label", "Sign in with the administrator token"),
					OnClick(onTokenSignIn),
					Text("Use token"),
				),
			)),
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

func ownerSetupView(
	setupKey, username, password, confirm, message string,
	onSetupKey, onUsername, onPassword, onConfirm, onSubmit ui.Handler,
) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			Div(css.Class("login-brand"), brandMark("")),
			H1(css.Class("login-title"), Text("Create owner account")),
			P(css.Class("login-sub"), Text("Finish this server's one-time setup. You will use this account to approve access and manage users.")),
			Div(
				css.Class("setup-note"),
				Text("Enter the break-glass key configured on this server. Setup closes permanently after the first owner is created."),
			),
			Label(For("setup-key"), css.Class("login-label"), Text("Break-glass setup key")),
			Input(
				ID("setup-key"),
				Type("password"),
				Attr("autocomplete", "off"),
				Attr("data-testid", "admin-setup-key"),
				css.Class("login-input"),
				Placeholder("Server setup key"),
				Value(setupKey),
				OnInput(onSetupKey),
			),
			Label(For("setup-username"), css.Class("login-label"), Text("Owner username")),
			Input(
				ID("setup-username"),
				Type("text"),
				Attr("autocomplete", "username"),
				Attr("data-testid", "admin-setup-username"),
				css.Class("login-input"),
				Placeholder("Username"),
				Value(username),
				OnInput(onUsername),
			),
			Label(For("setup-password"), css.Class("login-label"), Text("Password")),
			Input(
				ID("setup-password"),
				Type("password"),
				Attr("autocomplete", "new-password"),
				Attr("data-testid", "admin-setup-password"),
				css.Class("login-input"),
				Placeholder("At least 8 characters"),
				Value(password),
				OnInput(onPassword),
			),
			Label(For("setup-password-confirm"), css.Class("login-label"), Text("Confirm password")),
			Input(
				ID("setup-password-confirm"),
				Type("password"),
				Attr("autocomplete", "new-password"),
				Attr("data-testid", "admin-setup-password-confirm"),
				css.Class("login-input"),
				Placeholder("Repeat password"),
				Value(confirm),
				OnInput(onConfirm),
			),
			If(message != "", P(
				css.Class("form-error"),
				Attr("role", "alert"),
				Attr("data-testid", "admin-setup-message"),
				Text(message),
			)),
			Button(
				Type("button"),
				css.Class("btn btn-primary"),
				Attr("data-testid", "admin-setup-submit"),
				Attr("aria-label", "Create the first CashFlux owner account"),
				OnClick(onSubmit),
				Text("Create owner account"),
			),
		),
	)
}

func ownerSetupRecoveryView(recoveryCode, message string, onContinue ui.Handler) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			Div(css.Class("login-brand"), brandMark("")),
			H1(css.Class("login-title"), Text("Save your recovery code")),
			P(css.Class("login-sub"), Text("Your owner account is ready. Store this one-time code somewhere safe before continuing.")),
			Div(
				css.Class("recovery-code"),
				Attr("data-testid", "admin-setup-recovery-code"),
				Attr("aria-label", "One-time owner recovery code"),
				Text(recoveryCode),
			),
			P(css.Class("setup-note"), Text("CashFlux stores only a protected hash. This code cannot be shown again.")),
			If(message != "", P(
				css.Class("form-error"),
				Attr("role", "status"),
				Attr("data-testid", "admin-setup-message"),
				Text(message),
			)),
			Button(
				Type("button"),
				css.Class("btn btn-primary"),
				Attr("data-testid", "admin-setup-continue"),
				Attr("aria-label", "Confirm the recovery code is saved and open the console"),
				OnClick(onContinue),
				Text("I saved it — open console"),
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
		P(css.Class("error-msg"), Text("Not authorized — sign in with a CashFlux owner account.")),
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

// readyViewControls bundles the console dashboard's callbacks and users-table
// search/pagination state so the signature stays readable.
type readyViewControls struct {
	token         string
	search        string
	offset        int
	hasMore       bool
	onSignOut     ui.Handler
	onRefresh     ui.Handler
	onCreateUser  ui.Handler
	onOpenUser    func(string)
	onOpenAudit   ui.Handler
	onSearchInput ui.Handler
	onSearchGo    ui.Handler
	onPrev        ui.Handler
	onNext        ui.Handler
}

func readyView(ov *adminOverview, users []adminUserRow, c readyViewControls) ui.Node {
	return Div(
		css.Class("console-page"),
		// Header bar
		Div(
			css.Class("console-header"),
			H1(
				css.Class("console-title"),
				Div(css.Class("brand-mark"), Text("C")),
				Text("Operator Console"),
			),
			Div(
				css.Class("header-actions"),
				Button(
					Type("button"),
					css.Class("btn btn-secondary"),
					Attr("aria-label", "View the security audit log"),
					OnClick(c.onOpenAudit),
					Text("Audit log"),
				),
				Button(
					Type("button"),
					css.Class("btn btn-secondary"),
					Attr("aria-label", "Refresh console data"),
					OnClick(c.onRefresh),
					Text("Refresh"),
				),
				Button(
					Type("button"),
					css.Class("btn btn-primary"),
					Attr("aria-label", "Create a user account"),
					OnClick(c.onCreateUser),
					Text("Create user"),
				),
				Button(
					Type("button"),
					css.Class("btn btn-danger"),
					Attr("aria-label", "Sign out and return to home"),
					OnClick(c.onSignOut),
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
		ui.CreateElement(pendingApprovals, pendingApprovalsProps{token: c.token}),
		// Users toolbar: identity search + page controls, then the clickable table
		// (usersTable lives in manage.go; rows are their own components so each can
		// own an OnClick hook).
		Div(css.Class("users-toolbar"),
			Input(
				Type("search"),
				css.Class("users-search"),
				Attr("placeholder", "Search by username or email…"),
				Attr("aria-label", "Search users by username or email"),
				Value(c.search),
				OnInput(c.onSearchInput),
				OnChange(c.onSearchInput),
			),
			Button(Type("button"), css.Class("btn btn-secondary"), OnClick(c.onSearchGo), Text("Search")),
			Div(css.Class("users-pager"),
				Button(Type("button"), css.Class("btn btn-secondary"), Attr("aria-label", "Previous page"),
					Disabled(c.offset <= 0), OnClick(c.onPrev), Text("← Prev")),
				Span(css.Class("users-page-label"), Text(pageLabel(c.offset, len(users)))),
				Button(Type("button"), css.Class("btn btn-secondary"), Attr("aria-label", "Next page"),
					Disabled(!c.hasMore), OnClick(c.onNext), Text("Next →")),
			),
		),
		usersTable(users, c.onOpenUser),
	)
}

// auditView renders the global security audit log (newest first) as a read-only
// table. Server-side the endpoint is admin-scoped, so a non-admin never reaches
// the "Audit log" button's data.
func auditView(events []auditEvent, onClose ui.Handler) ui.Node {
	var body ui.Node
	if len(events) == 0 {
		body = Div(css.Class("usage-empty"), Text("No audit events recorded yet."))
	} else {
		body = Div(css.Class("table-wrap"),
			Table(css.Class("users-table"),
				Thead(Tr(
					Th(Text("When")),
					Th(Text("Actor")),
					Th(Text("Action")),
					Th(Text("Target")),
					Th(Text("IP")),
				)),
				Tbody(Map(events, func(e auditEvent) ui.Node {
					target := e.TargetType
					if e.TargetID != "" {
						target += " · " + e.TargetID
					}
					return Tr(
						Td(Text(trimDateTime(e.Timestamp))),
						Td(Text(shortActor(e.ActorID))),
						Td(Text(e.Action)),
						Td(Text(target)),
						Td(Text(e.IP)),
					)
				})),
			),
		)
	}
	return Div(css.Class("console-page"),
		Div(css.Class("console-header"),
			H1(css.Class("console-title"),
				Div(css.Class("brand-mark"), Text("C")),
				Text("Audit log"),
			),
			Div(css.Class("header-actions"),
				Button(Type("button"), css.Class("btn btn-secondary"), Attr("aria-label", "Back to console"), OnClick(onClose), Text("← Back")),
			),
		),
		Div(css.Class("table-section"),
			Div(css.Class("table-hint"), Text("Every security-relevant backend event, newest first. Append-only and hash-chained.")),
			body,
		),
	)
}

// trimDateTime shortens an RFC3339 timestamp to "YYYY-MM-DD HH:MM" for display.
func trimDateTime(s string) string {
	if len(s) >= 16 {
		return strings.Replace(s[:16], "T", " ", 1)
	}
	return s
}

// shortActor trims a synthetic token actor id for readability, leaving real user
// ids intact.
func shortActor(id string) string {
	if len(id) > 20 {
		return id[:20] + "…"
	}
	return id
}

// pageLabel renders the current 1-based row range for the users pager.
func pageLabel(offset, count int) string {
	if count == 0 {
		return "No users"
	}
	return fmt.Sprintf("%d–%d", offset+1, offset+count)
}

// ---------------------------------------------------------------------------
// Root App component
// ---------------------------------------------------------------------------

// App is the root component for the CashFlux operator console SPA.
// Navigation flow: loading → credential login → console. A valid owner cookie
// session is restored on mount; a stored token is accepted only for the
// explicit break-glass compatibility path.
func App() ui.Node {
	view := ui.UseState(screenLoading)
	usernameInput := ui.UseState("")
	passwordInput := ui.UseState("")
	setupKeyInput := ui.UseState("")
	setupPasswordConfirm := ui.UseState("")
	setupMessage := ui.UseState("")
	setupRecoveryCode := ui.UseState("")
	tokenInput := ui.UseState("")
	devToken := ui.UseState("") // non-empty only in dev mode
	breakGlassOpen := ui.UseState(false)
	overview := ui.UseState[*adminOverview](nil)
	users := ui.UseState[[]adminUserRow](nil)
	netErrMsg := ui.UseState("")
	manageUserID := ui.UseState("") // target of the user-management view
	userSearch := ui.UseState("")   // live email-search box value
	userOffset := ui.UseState(0)    // current users-table page offset
	usersHasMore := ui.UseState(false)
	auditEvents := ui.UseState[[]auditEvent](nil) // global audit log (screenAudit)

	// showOwnerEntry selects the only valid unauthenticated door: first-owner
	// setup for an empty installation, normal sign-in after setup is complete.
	showOwnerEntry := func() {
		go func() {
			required, err := fetchAdminSetupStatus()
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			if required {
				view.Set(screenSetup)
				return
			}
			view.Set(screenLogin)
		}()
	}

	// reloadUsers refetches just the users table for the current search + offset,
	// leaving the overview stats untouched. Used by search/prev/next.
	reloadUsers := func(query string, offset int) {
		tok := lsGet()
		go func() {
			us, more, authErr, err := fetchUsers(tok, query, offset)
			if authErr || err != nil {
				return // keep the current page; a full refresh surfaces auth errors
			}
			users.Set(us)
			usersHasMore.Set(more)
			userOffset.Set(offset)
			userSearch.Set(query)
		}()
	}

	// handleTokenInput captures the typed value from the password input.
	handleTokenInput := ui.UseEvent(func(v string) {
		tokenInput.Set(v)
	})
	handleUsernameInput := ui.UseEvent(func(v string) { usernameInput.Set(v) })
	handlePasswordInput := ui.UseEvent(func(v string) { passwordInput.Set(v) })
	handleSetupKeyInput := ui.UseEvent(func(v string) { setupKeyInput.Set(v) })
	handleSetupConfirmInput := ui.UseEvent(func(v string) { setupPasswordConfirm.Set(v) })
	handleToggleBreakGlass := ui.UseEvent(func() { breakGlassOpen.Set(!breakGlassOpen.Get()) })

	handleOwnerSetup := ui.UseEvent(func() {
		username := strings.TrimSpace(usernameInput.Get())
		password := passwordInput.Get()
		switch {
		case strings.TrimSpace(setupKeyInput.Get()) == "":
			setupMessage.Set("Enter the server's break-glass setup key.")
			return
		case username == "":
			setupMessage.Set("Choose an owner username.")
			return
		case len(password) < 8:
			setupMessage.Set("Password must be at least 8 characters.")
			return
		case password != setupPasswordConfirm.Get():
			setupMessage.Set("Passwords do not match.")
			return
		}
		setupMessage.Set("Creating owner account…")
		go func() {
			response, err := createInitialOwner(setupKeyInput.Get(), username, password)
			if err != nil {
				setupMessage.Set(err.Error())
				return
			}
			setupKeyInput.Set("")
			setupPasswordConfirm.Set("")
			setupRecoveryCode.Set(response.RecoveryCode)
			setupMessage.Set("")
			view.Set(screenSetupRecovery)
		}()
	})

	handleFinishOwnerSetup := ui.UseEvent(func() {
		username := strings.TrimSpace(usernameInput.Get())
		password := passwordInput.Get()
		setupMessage.Set("Signing in…")
		go func() {
			_, isAuthErr, err := signInAdminCredentials(username, password)
			if isAuthErr {
				setupMessage.Set("The owner was created, but sign-in failed. Return to this page and try the same username and password.")
				return
			}
			if err != nil {
				setupMessage.Set("The owner was created. Sign-in can be retried: " + err.Error())
				return
			}
			ov, us, more, isAuthErr, err := fetchAdminData("")
			if isAuthErr {
				setupMessage.Set("The owner was created, but the console session could not be opened.")
				return
			}
			if err != nil {
				setupMessage.Set("The owner was created. Loading the console can be retried: " + err.Error())
				return
			}
			passwordInput.Set("")
			setupRecoveryCode.Set("")
			setupMessage.Set("")
			overview.Set(ov)
			users.Set(us)
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	})

	// The primary sign-in path establishes an owner-only HttpOnly session.
	handleCredentialSignIn := ui.UseEvent(func() {
		username := strings.TrimSpace(usernameInput.Get())
		password := passwordInput.Get()
		if username == "" || password == "" {
			return
		}
		view.Set(screenLoading)
		go func() {
			_, isAuthErr, err := signInAdminCredentials(username, password)
			if isAuthErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			lsRemove()
			ov, us, more, isAuthErr, err := fetchAdminData("")
			if isAuthErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			passwordInput.Set("")
			overview.Set(ov)
			users.Set(us)
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	})

	// The advanced sign-in path validates the explicit break-glass token.
	handleTokenSignIn := ui.UseEvent(func() {
		tok := strings.TrimSpace(tokenInput.Get())
		if tok == "" {
			return
		}
		view.Set(screenLoading)
		go func() {
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
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
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	})

	// handleSignOut clears stored state and returns to the home screen.
	handleSignOut := ui.UseEvent(func() {
		go signOutAdminSession()
		lsRemove()
		overview.Set(nil)
		users.Set(nil)
		usernameInput.Set("")
		passwordInput.Set("")
		tokenInput.Set("")
		devToken.Set("")
		breakGlassOpen.Set(false)
		view.Set(screenLogin)
	})

	// handleRefresh re-fetches using the stored token.
	handleRefresh := ui.UseEvent(func() {
		tok := lsGet()
		view.Set(screenLoading)
		go func() {
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
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
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	})

	// handleGoToLogin transitions to the login screen and fetches dev creds.
	handleGoToLogin := ui.UseEvent(func() {
		view.Set(screenLoading)
		showOwnerEntry()
		go func() {
			tok, ok := fetchDevCreds()
			if ok {
				devToken.Set(tok)
			}
		}()
	})

	// handleBack returns from login to home.
	handleBack := ui.UseEvent(func() {
		usernameInput.Set("")
		passwordInput.Set("")
		tokenInput.Set("")
		devToken.Set("")
		breakGlassOpen.Set(false)
		view.Set(screenHome)
	})

	// handlePrefill fills the token input with the dev-mode token.
	handlePrefill := ui.UseEvent(func() {
		tokenInput.Set(devToken.Get())
		breakGlassOpen.Set(true)
	})

	// handleOpenConsole goes straight to the console using the stored token.
	handleOpenConsole := ui.UseEvent(func() {
		tok := lsGet()
		tokenInput.Set(tok)
		view.Set(screenLoading)
		go func() {
			if tok == "" && !restoreAdminSession() {
				showOwnerEntry()
				return
			}
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
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
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	})

	// handleOpenUser opens the management view for one user.
	handleOpenUser := func(id string) {
		manageUserID.Set(id)
		view.Set(screenManage)
	}
	handleOpenCreate := ui.UseEvent(func() { view.Set(screenCreate) })
	// handleCloseUser leaves the management view and refreshes the console so any
	// change (deleted account, new plan) is reflected in the list.
	handleCloseUser := func() {
		manageUserID.Set("")
		tok := lsGet()
		view.Set(screenLoading)
		go func() {
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
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
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
	}

	// Auto-load any stored token on mount. The constant deps key makes this run
	// exactly once (mount) instead of on every render — without it the effect
	// re-fired each render, re-fetching admin data ~continuously and replaying
	// the entrance animations (visible page flicker + request spam).
	ui.UseEffect(func() func() {
		tok := lsGet()
		tokenInput.Set(tok)
		view.Set(screenLoading)
		go func() {
			if tok == "" && !restoreAdminSession() {
				showOwnerEntry()
				return
			}
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
			if isAuthErr {
				// Token is stored but no longer valid; return to home.
				lsRemove()
				tokenInput.Set("")
				showOwnerEntry()
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			overview.Set(ov)
			users.Set(us)
			usersHasMore.Set(more)
			userOffset.Set(0)
			userSearch.Set("")
			view.Set(screenReady)
		}()
		return nil
	}, "admin-autoload")

	// Users-table search + pagination handlers.
	onUserSearchInput := ui.UseEvent(func(v string) { userSearch.Set(v) })
	onUserSearchSubmit := ui.UseEvent(func() { reloadUsers(strings.TrimSpace(userSearch.Get()), 0) })
	onUsersPrev := ui.UseEvent(func() {
		off := userOffset.Get() - usersPageSize
		if off < 0 {
			off = 0
		}
		reloadUsers(userSearch.Get(), off)
	})
	onUsersNext := ui.UseEvent(func() { reloadUsers(userSearch.Get(), userOffset.Get()+usersPageSize) })

	// Audit-log open/close.
	handleOpenAudit := ui.UseEvent(func() {
		tok := lsGet()
		view.Set(screenLoading)
		go func() {
			evs, authErr, err := fetchAudit(tok, 200)
			if authErr {
				view.Set(screenAuthErr)
				return
			}
			if err != nil {
				netErrMsg.Set(err.Error())
				view.Set(screenNetErr)
				return
			}
			auditEvents.Set(evs)
			view.Set(screenAudit)
		}()
	})
	handleCloseAudit := ui.UseEvent(func() { view.Set(screenReady) })

	// Render based on current navigation state.
	switch view.Get() {
	case screenLoading:
		return loadingView()
	case screenAuthErr:
		return authErrView(handleSignOut)
	case screenNetErr:
		return netErrView(netErrMsg.Get(), handleSignOut)
	case screenManage:
		return ui.CreateElement(manageView, manageProps{token: lsGet(), userID: manageUserID.Get(), onClose: handleCloseUser})
	case screenCreate:
		return ui.CreateElement(createUserView, createUserProps{token: lsGet(), onClose: handleCloseUser})
	case screenAudit:
		return auditView(auditEvents.Get(), handleCloseAudit)
	case screenSetup:
		return ownerSetupView(
			setupKeyInput.Get(), usernameInput.Get(), passwordInput.Get(), setupPasswordConfirm.Get(), setupMessage.Get(),
			handleSetupKeyInput, handleUsernameInput, handlePasswordInput, handleSetupConfirmInput, handleOwnerSetup,
		)
	case screenSetupRecovery:
		return ownerSetupRecoveryView(setupRecoveryCode.Get(), setupMessage.Get(), handleFinishOwnerSetup)
	case screenReady:
		ov := overview.Get()
		us := users.Get()
		if ov == nil {
			return loadingView()
		}
		return readyView(ov, us, readyViewControls{
			token:         lsGet(),
			search:        userSearch.Get(),
			offset:        userOffset.Get(),
			hasMore:       usersHasMore.Get(),
			onSignOut:     handleSignOut,
			onRefresh:     handleRefresh,
			onCreateUser:  handleOpenCreate,
			onOpenUser:    handleOpenUser,
			onOpenAudit:   handleOpenAudit,
			onSearchInput: onUserSearchInput,
			onSearchGo:    onUserSearchSubmit,
			onPrev:        onUsersPrev,
			onNext:        onUsersNext,
		})
	case screenLogin:
		return credentialLoginView(
			usernameInput.Get(), passwordInput.Get(), tokenInput.Get(), devToken.Get(),
			breakGlassOpen.Get(),
			handleUsernameInput, handlePasswordInput, handleCredentialSignIn, handleToggleBreakGlass,
			handleTokenInput, handleTokenSignIn, handleBack, handlePrefill,
		)
	default: // screenHome
		hasToken := lsGet() != ""
		return homeView(hasToken, handleGoToLogin, handleOpenConsole)
	}
}

func main() {
	ui.Render(ui.CreateElement(App), "#app")
	utils.WaitForever()
}
