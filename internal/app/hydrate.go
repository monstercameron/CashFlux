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
// imported. Otherwise the sample is seeded ONLY on a true first run — once the app
// has ever been seeded (seededBefore), an empty dataset means the user intentionally
// cleared it, so the clean slate is preserved.
func decideHydrate(datasetRaw string, seededBefore bool) hydrateAction {
	if strings.TrimSpace(datasetRaw) != "" {
		return hydrateImport
	}
	if seededBefore {
		return hydrateEmpty
	}
	return hydrateSeed
}
