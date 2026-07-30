// SPDX-License-Identifier: MIT

package app

import "strings"

// hydrateAction is hydrateDataset's boot decision, factored out of the js/wasm
// persist.go so it can be unit-tested natively.
type hydrateAction int

const (
	// hydrateSeed: a genuine first run (nothing saved, never seeded) — load the sample.
	hydrateSeed hydrateAction = iota
	// hydrateImport: a saved dataset exists — load it.
	hydrateImport
	// hydrateEmpty: set up before but the dataset is now empty (e.g. the user wiped
	// it) — stay empty instead of re-seeding a stranger's household (L6).
	hydrateEmpty
)

// decideHydrate chooses what to do on boot. A non-blank saved dataset is always
// imported. Otherwise the sample is seeded only on a true first run when the
// distribution permits it. Server-hosted builds pass allowSample=false so their
// empty store stays empty until the authoritative server workspace is checked.
func decideHydrate(datasetRaw string, seededBefore, allowSample bool) hydrateAction {
	if strings.TrimSpace(datasetRaw) != "" {
		return hydrateImport
	}
	if seededBefore || !allowSample {
		return hydrateEmpty
	}
	return hydrateSeed
}
