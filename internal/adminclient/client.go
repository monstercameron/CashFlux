// SPDX-License-Identifier: MIT

// Package adminclient owns the client-side admin API contract and acquisition
// cache shared by the app's access probe and the /admin screen.
package adminclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendauth"
)

const OverviewTTL = 30 * time.Second

// Overview mirrors the JSON response from GET /v1/admin/overview.
type Overview struct {
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

// User mirrors one user row from GET /v1/admin/users.
type User struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Email              string `json:"email"`
	CreatedAt          string `json:"createdAt"`
	SubscriptionPlan   string `json:"subscriptionPlan,omitempty"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
}

// UsersResponse mirrors the envelope from GET /v1/admin/users.
type UsersResponse struct {
	Users  []User `json:"users"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// FetchMeta reports when the response was actually acquired and whether this
// call reused a successful response acquired within OverviewTTL.
type FetchMeta struct {
	FetchedAt time.Time
	FromCache bool
	Status    int
}

type OverviewResult struct {
	Overview Overview
	FetchMeta
}

type UsersResult struct {
	Users  []User
	Limit  int
	Offset int
	FetchMeta
}

// Doer is the HTTP seam used by Client and its native tests.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// RequestError reports a redacted failure. Its Error string never includes the
// endpoint, Authorization header, or transport's potentially sensitive text.
type RequestError struct {
	Operation string
	Status    int
	Kind      string
}

func (e *RequestError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("admin %s: HTTP %d", e.Operation, e.Status)
	}
	return fmt.Sprintf("admin %s: %s", e.Operation, e.Kind)
}

var ErrCredentials = errors.New("admin request requires a server endpoint and bearer token")

type cacheKey struct {
	endpoint string
	tokenID  string
	kind     string
	limit    int
	offset   int
}

type overviewEntry struct {
	value OverviewResult
}

type usersEntry struct {
	value UsersResult
}

// Client performs admin requests and holds successful, identity-scoped results
// for a short TTL. Only a one-way token digest is retained in cache keys.
type Client struct {
	doer  Doer
	now   func() time.Time
	ttl   time.Duration
	mu    sync.Mutex
	views map[cacheKey]overviewEntry
	users map[cacheKey]usersEntry
}

func NewClient(doer Doer) *Client {
	return NewClientWithClock(doer, time.Now)
}

// NewClientWithClock creates a Client with a deterministic clock seam. The
// production default uses time.Now; callers should pass a stable clock only for
// tests or explicitly controlled environments.
func NewClientWithClock(doer Doer, now func() time.Time) *Client {
	if doer == nil {
		doer = http.DefaultClient
	}
	if now == nil {
		now = time.Now
	}
	return &Client{
		doer:  doer,
		now:   now,
		ttl:   OverviewTTL,
		views: make(map[cacheKey]overviewEntry),
		users: make(map[cacheKey]usersEntry),
	}
}

// Default is the single WASM client used by the app probe and /admin route.
var Default = NewClient(http.DefaultClient)

// Overview fetches the platform overview. The returned status is preserved so
// callers can keep their existing acceptance policy (the app probe requires
// exactly 200; the screen accepts any 2xx response).
func (c *Client) Overview(ctx context.Context, endpoint, token string) (OverviewResult, error) {
	endpoint, token, key, err := identity(endpoint, token, "overview")
	if err != nil {
		return OverviewResult{}, err
	}
	if cached, ok := c.cachedOverview(key); ok {
		return cached, nil
	}
	var value Overview
	status, err := c.getJSON(ctx, endpoint+"/v1/admin/overview", token, "overview", &value)
	if err != nil {
		return OverviewResult{}, err
	}
	result := OverviewResult{Overview: value, FetchMeta: FetchMeta{FetchedAt: c.now(), Status: status}}
	c.mu.Lock()
	c.views[key] = overviewEntry{value: result}
	c.mu.Unlock()
	return result, nil
}

// Users fetches a page of admin users. Successful pages are cached by endpoint,
// token identity, limit, and offset; returned slices never alias cached storage.
func (c *Client) Users(ctx context.Context, endpoint, token string, limit, offset int) (UsersResult, error) {
	endpoint, token, key, err := identity(endpoint, token, "users")
	if err != nil {
		return UsersResult{}, err
	}
	key.limit, key.offset = limit, offset
	if cached, ok := c.cachedUsers(key); ok {
		return cached, nil
	}
	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("offset", fmt.Sprintf("%d", offset))
	var value UsersResponse
	status, err := c.getJSON(ctx, endpoint+"/v1/admin/users?"+query.Encode(), token, "users", &value)
	if err != nil {
		return UsersResult{}, err
	}
	result := UsersResult{
		Users: cloneUsers(value.Users), Limit: value.Limit, Offset: value.Offset,
		FetchMeta: FetchMeta{FetchedAt: c.now(), Status: status},
	}
	c.mu.Lock()
	c.users[key] = usersEntry{value: cloneUsersResult(result)}
	c.mu.Unlock()
	return result, nil
}

func (c *Client) cachedOverview(key cacheKey) (OverviewResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.views[key]
	if !ok || !c.fresh(entry.value.FetchedAt) {
		if ok {
			delete(c.views, key)
		}
		return OverviewResult{}, false
	}
	result := entry.value
	result.FromCache = true
	return result, true
}

func (c *Client) cachedUsers(key cacheKey) (UsersResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.users[key]
	if !ok || !c.fresh(entry.value.FetchedAt) {
		if ok {
			delete(c.users, key)
		}
		return UsersResult{}, false
	}
	result := cloneUsersResult(entry.value)
	result.FromCache = true
	return result, true
}

func (c *Client) fresh(at time.Time) bool {
	age := c.now().Sub(at)
	return age >= 0 && age < c.ttl
}

func (c *Client) getJSON(ctx context.Context, target, token, operation string, dst any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, &RequestError{Operation: operation, Kind: "invalid request"}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.doer.Do(req)
	if err != nil {
		return 0, &RequestError{Operation: operation, Kind: "network error"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, &RequestError{Operation: operation, Status: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return resp.StatusCode, &RequestError{Operation: operation, Kind: "invalid response"}
	}
	return resp.StatusCode, nil
}

func identity(endpoint, token, kind string) (string, string, cacheKey, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		return "", "", cacheKey{}, ErrCredentials
	}
	if err := backendauth.ValidateServerEndpoint(endpoint); err != nil {
		// Do not include endpoint or token details in transport errors. The
		// identity check runs before either cache access or an HTTP request.
		return "", "", cacheKey{}, &RequestError{Operation: kind, Kind: "invalid server endpoint"}
	}
	digest := sha256.Sum256([]byte(token))
	return endpoint, token, cacheKey{endpoint: endpoint, tokenID: hex.EncodeToString(digest[:]), kind: kind}, nil
}

func cloneUsers(users []User) []User {
	return append([]User(nil), users...)
}

func cloneUsersResult(result UsersResult) UsersResult {
	result.Users = cloneUsers(result.Users)
	return result
}
