// SPDX-License-Identifier: MIT

package i18n

// transferPairKeys holds the strings for transfer-pair health findings (C686).
// Merged via init so this file never touches en.go.
var transferPairKeys = Catalog{
	// Distinct from an orphan on purpose. Telling somebody to go looking for a
	// missing transaction that is sitting in front of them wastes the trip, so
	// this says the far side EXISTS and states what the two sides fail to cancel
	// by — which is the number they will be reconciling against.
	"health.transferLegsDisagree": "“%s” is a transfer whose two sides do not cancel — together they move %s that has no source. One leg was probably edited without the other.",
}

func init() {
	for k, v := range transferPairKeys {
		english[k] = v
	}
}
