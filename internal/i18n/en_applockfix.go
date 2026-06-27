// SPDX-License-Identifier: MIT

package i18n

// applockFixKeys holds the English string for the C287 weak-passcode rejection.
// Defined in its own file and merged via init so this does not touch the
// user-WIP en.go; mirrors the en_setup.go pattern.
var applockFixKeys = Catalog{
	"applock.tooWeak": "That passcode is too easy to guess — avoid repeated or sequential digits like 000000 or 123456.",
}

func init() {
	for k, v := range applockFixKeys {
		english[k] = v
	}
}
