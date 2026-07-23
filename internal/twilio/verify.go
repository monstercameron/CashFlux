// SPDX-License-Identifier: MIT

// Package twilio talks to Twilio Verify for SMS-based phone verification
// (TODOS.md C420). CashFlux calls Twilio's REST API directly over net/http —
// matching the existing Stripe/PayPal pattern (see internal/server/billing_provider.go)
// of no vendor SDK dependency in go.mod — rather than hand-rolling SMS code
// generation, expiry, replay, and fraud protection.
package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotConfigured is returned by NoopVerifyClient for every call: it is the
// placeholder used until a real, Twilio-backed VerifyClient is wired in
// (TODOS.md C420, lane A).
var ErrNotConfigured = errors.New("twilio: verify service is not configured")

// VerifyClient sends and checks SMS verification codes through Twilio Verify.
// The server package depends on this interface, not on Twilio directly, so
// tests and the pre-Twilio foundation pass can substitute NoopVerifyClient.
type VerifyClient interface {
	// SendCode asks Twilio Verify to text a one-time code to phone.
	SendCode(ctx context.Context, phone string) error
	// CheckCode verifies a code the user entered for phone. It returns
	// (true, nil) only when Twilio confirms the code is correct and unexpired.
	CheckCode(ctx context.Context, phone, code string) (bool, error)
}

// NoopVerifyClient is the placeholder VerifyClient used until lane A adds a
// real Twilio-backed implementation in this same file (TODOS.md C420). Both
// methods fail clearly rather than silently pretending to send or accept a
// code, so a deployment that forgets to configure Twilio fails loudly instead
// of enrolling users against a verification check that never happens.
type NoopVerifyClient struct{}

// SendCode always fails: no Twilio credentials are configured.
func (NoopVerifyClient) SendCode(_ context.Context, phone string) error {
	return fmt.Errorf("twilio: cannot send verification code to %q: %w", phone, ErrNotConfigured)
}

// CheckCode always fails: no Twilio credentials are configured.
func (NoopVerifyClient) CheckCode(_ context.Context, phone, _ string) (bool, error) {
	return false, fmt.Errorf("twilio: cannot check verification code for %q: %w", phone, ErrNotConfigured)
}

// Config carries the Twilio Verify credentials needed to call the REST API. It is
// a small, self-contained struct (not server.Config) so this package never
// imports internal/server — internal/server already imports internal/twilio, and
// a reverse import would be a cycle. The server wires its Config's
// TwilioAccountSID/TwilioAuthToken/TwilioVerifyServiceSID fields into this struct
// at the call site (TODOS.md C420).
type Config struct {
	AccountSID       string
	AuthToken        string
	VerifyServiceSID string
}

// Configured reports whether all three Twilio credentials are present.
func (c Config) Configured() bool {
	return strings.TrimSpace(c.AccountSID) != "" && strings.TrimSpace(c.AuthToken) != "" && strings.TrimSpace(c.VerifyServiceSID) != ""
}

// verifyHTTPClient is a package-level client (like billing_http.go's
// stripeHTTPClient) so every call reuses one connection pool instead of paying a
// fresh TLS handshake per verification.
var verifyHTTPClient = &http.Client{Timeout: 15 * time.Second}

// verifyBaseURL is the Twilio Verify API base; overridable in tests.
var verifyBaseURL = "https://verify.twilio.com/v2"

// TwilioVerifyClient is the real VerifyClient, backed by raw net/http calls to
// Twilio's Verify REST API (TODOS.md C420) — no Twilio SDK dependency, matching
// the existing Stripe/PayPal house style (see internal/server/billing_provider.go,
// billing_http.go).
type TwilioVerifyClient struct {
	cfg Config
}

// NewTwilioVerifyClient builds a TwilioVerifyClient from cfg. The returned client
// fails clearly (ErrNotConfigured) on every call when cfg is incomplete, exactly
// like NoopVerifyClient, so a half-configured deployment fails loudly rather than
// silently pretending to send or accept a code.
func NewTwilioVerifyClient(cfg Config) *TwilioVerifyClient {
	return &TwilioVerifyClient{cfg: cfg}
}

// SendCode asks Twilio Verify to text a one-time code to phone (E.164 format,
// e.g. "+15551234567") via POST .../Verifications.
func (c *TwilioVerifyClient) SendCode(ctx context.Context, phone string) error {
	if !c.cfg.Configured() {
		return fmt.Errorf("twilio: cannot send verification code to %q: %w", phone, ErrNotConfigured)
	}
	form := url.Values{}
	form.Set("To", phone)
	form.Set("Channel", "sms")
	path := fmt.Sprintf("/Services/%s/Verifications", url.PathEscape(c.cfg.VerifyServiceSID))
	var out verifyStatusResponse
	if err := c.doVerify(ctx, path, form, &out); err != nil {
		return fmt.Errorf("twilio: send verification code: %w", err)
	}
	return nil
}

// CheckCode verifies a code the user entered for phone via POST
// .../VerificationCheck. It returns (true, nil) only when Twilio reports the
// check's status as "approved" — any other status (e.g. "pending" for a wrong
// code, or Twilio's 404 for an expired/unknown verification) is a clean
// (false, nil), not an error, since those are ordinary "the code was wrong"
// outcomes rather than a call failure.
func (c *TwilioVerifyClient) CheckCode(ctx context.Context, phone, code string) (bool, error) {
	if !c.cfg.Configured() {
		return false, fmt.Errorf("twilio: cannot check verification code for %q: %w", phone, ErrNotConfigured)
	}
	form := url.Values{}
	form.Set("To", phone)
	form.Set("Code", code)
	path := fmt.Sprintf("/Services/%s/VerificationCheck", url.PathEscape(c.cfg.VerifyServiceSID))
	var out verifyStatusResponse
	if err := c.doVerify(ctx, path, form, &out); err != nil {
		if errors.Is(err, errVerificationNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("twilio: check verification code: %w", err)
	}
	return strings.EqualFold(out.Status, "approved"), nil
}

// verifyStatusResponse is the subset of Twilio's Verification/VerificationCheck
// response bodies this client needs.
type verifyStatusResponse struct {
	Status string `json:"status"`
}

// errVerificationNotFound marks a Twilio 404 (expired/unknown verification SID/
// phone pair) so CheckCode can treat it as "code did not verify" rather than a
// call failure.
var errVerificationNotFound = errors.New("twilio: verification not found")

// doVerify POSTs form to Twilio Verify's path, authenticated with HTTP Basic auth
// (AccountSid:AuthToken, Twilio's documented scheme for the Verify API), and
// decodes a successful response into out.
func (c *TwilioVerifyClient) doVerify(ctx context.Context, path string, form url.Values, out *verifyStatusResponse) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyBaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.cfg.AccountSID, c.cfg.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := verifyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return errVerificationNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio status %d: %s", resp.StatusCode, verifyErrorMessage(data))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode twilio response: %w", err)
	}
	return nil
}

// verifyErrorMessage extracts Twilio's human-readable error message
// ({"message":"..."}) from an error body, truncated, or falls back to a short raw
// snippet. Never returns secrets — Twilio error messages don't echo the auth token.
func verifyErrorMessage(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && strings.TrimSpace(parsed.Message) != "" {
		return strings.TrimSpace(parsed.Message)
	}
	snippet := strings.TrimSpace(string(body))
	const maxSnippet = 200
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet] + "…"
	}
	if snippet == "" {
		return "no error detail"
	}
	return snippet
}
