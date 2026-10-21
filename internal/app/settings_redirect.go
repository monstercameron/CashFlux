// SPDX-License-Identifier: MIT

package app

// defaultSettingsTabRedirect creates the deferred Settings effect. The current
// path must be read when it runs: a callback from a previous Settings render
// must not replace a newer navigation, including one to another Settings tab.
func defaultSettingsTabRedirect(rawTab string, currentPath func() string, navigate func(string)) func() func() {
	return func() func() {
		if rawTab == "" {
			path := currentPath()
			if path == "/settings" || path == "/settings/" {
				navigate("/settings/household")
			}
		}
		return nil
	}
}
