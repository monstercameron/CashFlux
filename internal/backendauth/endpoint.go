// SPDX-License-Identifier: MIT

package backendauth

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ValidateServerEndpoint prevents credentials and financial data from being
// sent over an unencrypted connection to a server outside this device.
func ValidateServerEndpoint(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return errors.New("enter a full server address")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("use a server address without a password, query, or fragment")
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "wss":
		return nil
	case "http", "ws":
		host := strings.ToLower(u.Hostname())
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return errors.New("use HTTPS for a server outside this device")
	default:
		return errors.New("server address must start with HTTPS")
	}
}
