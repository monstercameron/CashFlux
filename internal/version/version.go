// SPDX-License-Identifier: MIT

// Package version is the single source of truth for the user-facing product
// version. It is deliberately tiny and dependency-free so any layer (UI, exports,
// logs, bug reports) can read one value.
package version

// Version is the product version shown in the UI. It is a var, not a const, so a
// release build can inject the git tag without code changes, e.g.:
//
//	go build -ldflags "-X github.com/monstercameron/CashFlux/internal/version.Version=$(git describe --tags)"
//
// The default applies for local/dev builds where no tag is injected.
var Version = "1.0.40"

// Label returns the version prefixed with "v" for display (e.g. "v0.1.0"). An
// injected tag that already starts with "v" is returned unchanged.
func Label() string {
	if len(Version) > 0 && Version[0] == 'v' {
		return Version
	}
	return "v" + Version
}
