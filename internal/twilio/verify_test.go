// SPDX-License-Identifier: MIT

package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoopVerifyClientAlwaysFails(t *testing.T) {
	var c NoopVerifyClient
	if err := c.SendCode(context.Background(), "+15551234567"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("SendCode err = %v, want ErrNotConfigured", err)
	}
	if _, err := c.CheckCode(context.Background(), "+15551234567", "123456"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("CheckCode err = %v, want ErrNotConfigured", err)
	}
}

func TestTwilioVerifyClientNotConfigured(t *testing.T) {
	c := NewTwilioVerifyClient(Config{})
	if err := c.SendCode(context.Background(), "+15551234567"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("SendCode err = %v, want ErrNotConfigured", err)
	}
	if _, err := c.CheckCode(context.Background(), "+15551234567", "123456"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("CheckCode err = %v, want ErrNotConfigured", err)
	}
}

// withTestVerifyServer swaps verifyBaseURL to point at a test server for the
// duration of fn, restoring it afterward.
func withTestVerifyServer(t *testing.T, handler http.HandlerFunc, fn func(cfg Config)) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := verifyBaseURL
	verifyBaseURL = srv.URL
	t.Cleanup(func() { verifyBaseURL = prev })
	fn(Config{AccountSID: "AC_test", AuthToken: "token_test", VerifyServiceSID: "VA_test"})
}

func TestTwilioVerifyClientSendCode(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "success", status: http.StatusCreated, body: `{"status":"pending"}`},
		{name: "twilio rejects", status: http.StatusBadRequest, body: `{"message":"Invalid parameter"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotAuthUser string
			withTestVerifyServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				user, _, _ := r.BasicAuth()
				gotAuthUser = user
				if err := r.ParseForm(); err != nil {
					t.Fatalf("parse form: %v", err)
				}
				if got := r.Form.Get("To"); got != "+15551234567" {
					t.Fatalf("To = %q, want +15551234567", got)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}, func(cfg Config) {
				c := NewTwilioVerifyClient(cfg)
				err := c.SendCode(context.Background(), "+15551234567")
				if tc.wantErr && err == nil {
					t.Fatal("expected error, got nil")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if !strings.Contains(gotPath, "/Services/VA_test/Verifications") {
				t.Fatalf("path = %q, want .../Services/VA_test/Verifications", gotPath)
			}
			if gotAuthUser != "AC_test" {
				t.Fatalf("basic auth user = %q, want AC_test", gotAuthUser)
			}
		})
	}
}

func TestTwilioVerifyClientCheckCode(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantOK     bool
		wantErr    bool
		wantErrNil bool // expired/unknown: (false, nil), not an error
	}{
		{name: "approved", status: http.StatusOK, body: `{"status":"approved"}`, wantOK: true},
		{name: "pending (wrong code)", status: http.StatusOK, body: `{"status":"pending"}`, wantOK: false},
		{name: "expired or unknown verification (404)", status: http.StatusNotFound, body: `{"message":"not found"}`, wantOK: false, wantErrNil: true},
		{name: "server error", status: http.StatusInternalServerError, body: `{"message":"boom"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withTestVerifyServer(t, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/VerificationCheck") {
					t.Fatalf("path = %q, want .../VerificationCheck", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}, func(cfg Config) {
				c := NewTwilioVerifyClient(cfg)
				ok, err := c.CheckCode(context.Background(), "+15551234567", "123456")
				if tc.wantErr && err == nil {
					t.Fatal("expected error, got nil")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if ok != tc.wantOK {
					t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
				}
			})
		})
	}
}

func TestVerifyErrorMessageFallsBackToSnippet(t *testing.T) {
	if got := verifyErrorMessage([]byte(`{"message":"bad request"}`)); got != "bad request" {
		t.Fatalf("verifyErrorMessage = %q, want %q", got, "bad request")
	}
	if got := verifyErrorMessage([]byte(`not json`)); got != "not json" {
		t.Fatalf("verifyErrorMessage = %q, want %q", got, "not json")
	}
	if got := verifyErrorMessage(nil); got != "no error detail" {
		t.Fatalf("verifyErrorMessage(nil) = %q, want %q", got, "no error detail")
	}
}

func TestConfigConfigured(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "complete", cfg: Config{AccountSID: "a", AuthToken: "b", VerifyServiceSID: "c"}, want: true},
		{name: "missing sid", cfg: Config{AuthToken: "b", VerifyServiceSID: "c"}, want: false},
		{name: "empty", cfg: Config{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Configured(); got != tc.want {
				t.Fatalf("Configured() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ensure verifyStatusResponse round-trips through encoding/json as expected by
// doVerify (a smoke check on the tag names, since they're hand-picked to match
// Twilio's documented field name, not Go convention).
func TestVerifyStatusResponseJSONTag(t *testing.T) {
	var out verifyStatusResponse
	if err := json.Unmarshal([]byte(`{"status":"approved"}`), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "approved" {
		t.Fatalf("Status = %q, want approved", out.Status)
	}
}
