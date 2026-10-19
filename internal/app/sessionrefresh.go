// SPDX-License-Identifier: MIT

package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const refreshCredentialPrefix = "cashflux:cred:refresh:"
const authRefreshModeEncrypted = "encrypted-record"

// refreshScope is selected by boot, never inferred from whichever account last
// wrote the generic auth keys. Endpoint is bound too: a server ID alone must not
// authorize sending a credential to another address.
type refreshScope struct {
	ServerID  string `json:"serverId"`
	AccountID string `json:"accountId"`
	Endpoint  string `json:"endpoint"`
	Mode      string `json:"mode"`
}

func newRefreshScope(serverID, accountID, endpoint, pageOrigin string) (refreshScope, error) {
	u, err := refreshEndpoint(endpoint)
	if err != nil {
		return refreshScope{}, err
	}
	s := refreshScope{ServerID: strings.TrimSpace(serverID), AccountID: strings.TrimSpace(accountID), Endpoint: u.String(), Mode: authRefreshModeEncrypted}
	page, err := refreshEndpoint(pageOrigin)
	if err == nil && u.Scheme == "https" && refreshOrigin(u) == refreshOrigin(page) {
		s.Mode = "cookie"
	}
	return s, s.validate()
}

func refreshEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("session refresh: a valid server address is required")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func refreshOrigin(u *url.URL) string {
	port := u.Port()
	if port == "" || (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	return u.Scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
}

func (s refreshScope) validate() error {
	if s.ServerID == "" || s.AccountID == "" || (s.Mode != "cookie" && s.Mode != authRefreshModeEncrypted) {
		return errors.New("session refresh: a fixed server, account and refresh mode are required")
	}
	u, err := refreshEndpoint(s.Endpoint)
	if err == nil && s.Mode == "cookie" && u.Scheme != "https" {
		return errors.New("session refresh: cookie mode requires HTTPS")
	}
	return err
}

func (s refreshScope) key() string {
	encode := base64.RawURLEncoding.EncodeToString
	return refreshCredentialPrefix + encode([]byte(s.ServerID)) + ":" + encode([]byte(s.AccountID))
}

// refreshCredential deliberately has no access-token or CSRF fields. Cookie
// mode persists only scope metadata; encrypted-record mode persists one refresh
// credential through atrest's MUST-ENCRYPT boundary.
type refreshCredential struct {
	Version int          `json:"version"`
	Scope   refreshScope `json:"scope"`
	Token   string       `json:"refreshToken,omitempty"`
}

func encodeRefreshCredential(scope refreshScope, token string) (string, error) {
	if err := scope.validate(); err != nil {
		return "", err
	}
	token = strings.TrimSpace(token)
	if (scope.Mode == authRefreshModeEncrypted) != (token != "") {
		return "", errors.New("session refresh: credential does not match the selected refresh mode")
	}
	raw, err := json.Marshal(refreshCredential{Version: 1, Scope: scope, Token: token})
	if err != nil {
		return "", fmt.Errorf("encode refresh credential: %w", err)
	}
	return string(raw), nil
}

func decodeRefreshCredential(scope refreshScope, raw string) (refreshCredential, error) {
	var credential refreshCredential
	if err := scope.validate(); err != nil {
		return credential, err
	}
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return credential, fmt.Errorf("decode refresh credential: %w", err)
	}
	if credential.Version != 1 || credential.Scope != scope {
		return refreshCredential{}, errors.New("session refresh: credential belongs to another scope or version")
	}
	if (scope.Mode == authRefreshModeEncrypted) != (strings.TrimSpace(credential.Token) != "") {
		return refreshCredential{}, errors.New("session refresh: stored credential does not match its mode")
	}
	return credential, nil
}
