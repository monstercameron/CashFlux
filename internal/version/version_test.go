// SPDX-License-Identifier: MIT

package version

import "testing"

func TestLabel(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	cases := []struct{ in, want string }{
		{"0.1.0", "v0.1.0"},  // bare semver gets a v prefix
		{"v1.2.3", "v1.2.3"}, // an injected tag already prefixed is unchanged
		{"1.0.0-rc.1", "v1.0.0-rc.1"},
	}
	for _, c := range cases {
		Version = c.in
		if got := Label(); got != c.want {
			t.Errorf("Label() with Version=%q = %q, want %q", c.in, got, c.want)
		}
	}
}
