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

	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
	"github.com/monstercameron/GoWebComponents/v4/utils"
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
	screenManage                // managing a single user (detail + actions)
	screenAudit                 // the global security audit log
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

// fetchAdminData fetches GET /v1/admin/overview and GET /v1/admin/users?limit=100
// using a bearer token. Same-origin relative URLs are used so the SPA works
// regardless of hostname.
func fetchAdminData(token string) (ov *adminOverview, users []adminUserRow, hasMore bool, authErr bool, err error) {
	// overview
	req, e := http.NewRequest("GET", "/v1/admin/overview", nil)
	if e != nil {
		return nil, nil, false, false, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, nil, false, false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, nil, false, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, nil, false, false, fmt.Errorf("overview: HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, nil, false, false, e
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
	req, e := http.NewRequest("GET", u, nil)
	if e != nil {
		return nil, false, false, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, false, false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, false, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, false, false, fmt.Errorf("users: HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, false, false, e
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
	req, e := http.NewRequest("GET", fmt.Sprintf("/v1/audit?limit=%d", limit), nil)
	if e != nil {
		return nil, false, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("audit: HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, false, e
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

// loginView renders the token-entry screen.
// devToken is non-empty only when the server returned a dev-mode token; the
// prefill button is shown only in that case.
func loginView(tokenVal, devToken string, onInput, onSignIn, onBack, onPrefill ui.Handler) ui.Node {
	return Div(
		css.Class("login-page"),
		Div(
			css.Class("login-card"),
			Div(css.Class("login-brand"), brandMark("")),
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

// readyViewControls bundles the console dashboard's callbacks and users-table
// search/pagination state so the signature stays readable.
type readyViewControls struct {
	search        string
	offset        int
	hasMore       bool
	onSignOut     ui.Handler
	onRefresh     ui.Handler
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
		// Users toolbar: email search + page controls, then the clickable table
		// (usersTable lives in manage.go; rows are their own components so each can
		// own an OnClick hook).
		Div(css.Class("users-toolbar"),
			Input(
				Type("search"),
				css.Class("users-search"),
				Attr("placeholder", "Search by email…"),
				Attr("aria-label", "Search users by email"),
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
// Navigation flow: Home → Login → Console (or Home directly to Console when a
// stored token is present).
func App() ui.Node {
	view := ui.UseState(screenHome)
	tokenInput := ui.UseState("")
	devToken := ui.UseState("") // non-empty only in dev mode
	overview := ui.UseState[*adminOverview](nil)
	users := ui.UseState[[]adminUserRow](nil)
	netErrMsg := ui.UseState("")
	manageUserID := ui.UseState("") // target of the user-management view
	userSearch := ui.UseState("")   // live email-search box value
	userOffset := ui.UseState(0)    // current users-table page offset
	usersHasMore := ui.UseState(false)
	auditEvents := ui.UseState[[]auditEvent](nil) // global audit log (screenAudit)

	// reloadUsers refetches just the users table for the current search + offset,
	// leaving the overview stats untouched. Used by search/prev/next.
	reloadUsers := func(query string, offset int) {
		tok := lsGet()
		if tok == "" {
			return
		}
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

	// handleSignIn validates, sets loading, fetches, and transitions state.
	handleSignIn := ui.UseEvent(func() {
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
	// handleCloseUser leaves the management view and refreshes the console so any
	// change (deleted account, new plan) is reflected in the list.
	handleCloseUser := func() {
		manageUserID.Set("")
		tok := lsGet()
		if tok == "" {
			view.Set(screenHome)
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
		if tok == "" {
			return nil
		}
		tokenInput.Set(tok)
		view.Set(screenLoading)
		go func() {
			ov, us, more, isAuthErr, err := fetchAdminData(tok)
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
		if tok == "" {
			view.Set(screenHome)
			return
		}
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
	case screenAudit:
		return auditView(auditEvents.Get(), handleCloseAudit)
	case screenReady:
		ov := overview.Get()
		us := users.Get()
		if ov == nil {
			return loadingView()
		}
		return readyView(ov, us, readyViewControls{
			search:        userSearch.Get(),
			offset:        userOffset.Get(),
			hasMore:       usersHasMore.Get(),
			onSignOut:     handleSignOut,
			onRefresh:     handleRefresh,
			onOpenUser:    handleOpenUser,
			onOpenAudit:   handleOpenAudit,
			onSearchInput: onUserSearchInput,
			onSearchGo:    onUserSearchSubmit,
			onPrev:        onUsersPrev,
			onNext:        onUsersNext,
		})
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
