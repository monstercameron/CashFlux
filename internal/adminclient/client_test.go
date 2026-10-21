// SPDX-License-Identifier: MIT

package adminclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOverviewCacheIsScopedToEndpointAndTokenAndReportsFreshness(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/admin/overview" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalUsers":7,"day":"2026-10-21"}`)
	}))
	defer server.Close()

	client := NewClient(server.Client())
	first, err := client.Overview(context.Background(), server.URL+"/", " token-a ")
	if err != nil {
		t.Fatal(err)
	}
	if first.FromCache || first.Status != http.StatusOK || first.Overview.TotalUsers != 7 || first.FetchedAt.IsZero() {
		t.Fatalf("first result = %+v", first)
	}
	second, err := client.Overview(context.Background(), " "+server.URL, "token-a")
	if err != nil {
		t.Fatal(err)
	}
	if !second.FromCache || !second.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("second result did not preserve original fetch metadata: first=%+v second=%+v", first, second)
	}
	if _, err := client.Overview(context.Background(), server.URL, "token-b"); err != nil {
		t.Fatal(err)
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalUsers":9}`)
	}))
	defer other.Close()
	otherResult, err := client.Overview(context.Background(), other.URL, "token-a")
	if err != nil {
		t.Fatal(err)
	}
	if otherResult.Overview.TotalUsers != 9 || otherResult.FromCache {
		t.Fatalf("other endpoint reused prior identity: %+v", otherResult)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("same endpoint requests = %d, want 2 (cache hit plus different token)", got)
	}
}

func TestOverviewTTLAndSuccessOnlyCaching(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalUsers":2}`)
	}))
	defer server.Close()

	client := NewClient(server.Client())
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }
	if _, err := client.Overview(context.Background(), server.URL, "token"); err == nil {
		t.Fatal("first forbidden response succeeded")
	} else if got, ok := err.(*RequestError); !ok || got.Status != http.StatusForbidden {
		t.Fatalf("first error = %#v, want HTTP 403 RequestError", err)
	}
	first, err := client.Overview(context.Background(), server.URL, "token")
	if err != nil || first.FromCache {
		t.Fatalf("retry after error = %+v, %v", first, err)
	}
	now = now.Add(OverviewTTL - time.Nanosecond)
	within, err := client.Overview(context.Background(), server.URL, "token")
	if err != nil || !within.FromCache {
		t.Fatalf("within-TTL read = %+v, %v", within, err)
	}
	now = now.Add(2 * time.Nanosecond)
	after, err := client.Overview(context.Background(), server.URL, "token")
	if err != nil || after.FromCache {
		t.Fatalf("expired read = %+v, %v", after, err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("network calls = %d, want 3", got)
	}
}

func TestUsersCacheReturnsDefensiveCopiesAndKeysPagination(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("limit") == "10" {
			_, _ = fmt.Fprint(w, `{"users":[{"id":"ten","email":"ten@example.test"}],"limit":10,"offset":0}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"users":[{"id":"one","email":"one@example.test"}],"limit":1,"offset":0}`)
	}))
	defer server.Close()
	client := NewClient(server.Client())

	first, err := client.Users(context.Background(), server.URL, "token", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	first.Users[0].Email = "mutated@example.test"
	second, err := client.Users(context.Background(), server.URL, "token", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !second.FromCache || second.Users[0].Email != "one@example.test" {
		t.Fatalf("cache result was aliased or not reused: %+v", second)
	}
	otherPage, err := client.Users(context.Background(), server.URL, "token", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if otherPage.FromCache || otherPage.Users[0].ID != "ten" {
		t.Fatalf("pagination reused wrong cache entry: %+v", otherPage)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("network calls = %d, want 2", got)
	}
}

func TestInvalidIdentityAndHTTPErrorDoNotExposeToken(t *testing.T) {
	client := NewClient(nil)
	if _, err := client.Overview(context.Background(), "", "secret-token"); err == nil {
		t.Fatal("missing endpoint accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()
	_, err := client.Overview(context.Background(), server.URL, "secret-token")
	if err == nil {
		t.Fatal("unauthorized response accepted")
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error leaks credential or endpoint: %q", err)
	}
	if status, ok := err.(*RequestError); !ok || status.Status != http.StatusUnauthorized {
		t.Fatalf("error = %#v, want HTTP 401 RequestError", err)
	}
}

func TestOverviewPreservesSuccessfulStatusForCallerPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"totalUsers":1}`)
	}))
	defer server.Close()
	result, err := NewClient(server.Client()).Overview(context.Background(), server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusCreated {
		t.Fatalf("status = %d, want 201 for the caller to apply its own acceptance policy", result.Status)
	}
}
