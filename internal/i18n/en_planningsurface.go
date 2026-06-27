// SPDX-License-Identifier: MIT

package i18n

// planningSurfaceKeys holds the English strings for the /planning safe-to-spend
// surface (C141/C142/C168). Defined in their own file and merged via init so this
// does not touch the shared en.go; mirrors the en_setup.go pattern.
var planningSurfaceKeys = Catalog{
	// C141: standalone Safe to spend tile on /planning (matches dashboard terminology).
	"planning.safeToSpend": "Safe to spend",
	// C142: unified label for the affordability card's "available" stat — was "Free to spend".
	"planning.affordAvailableLabel": "Safe to spend",
}

func init() {
	for k, v := range planningSurfaceKeys {
		english[k] = v
	}
}
