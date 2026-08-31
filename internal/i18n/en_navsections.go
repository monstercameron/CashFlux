// SPDX-License-Identifier: MIT

package i18n

// Rail section names.
//
// Each is the question a household is asking when they open it, in their words
// rather than the app's. The set they replace named what the software does —
// "Understand", "Build", "Data & people" — which reads as a table of contents for
// the codebase, and split the one question people ask most ("how am I doing?")
// across three separate places.
func init() {
	for k, v := range map[string]string{
		// What actually happened to the money. The daily loop.
		"nav.sectionDaily": "Day to day",
		// Money already spoken for, before it moves.
		"nav.sectionPlan": "Plan",
		// What the numbers add up to.
		"nav.sectionInsights": "Insights",
		// Anything waiting on a person.
		"nav.sectionNext": "Needs you",
		// How the household and the app are configured.
		"nav.sectionSetup": "Setup",
		// The household glance, now on the workspace control rather than in a
		// four-line footer. Plural because "1 members" is what it said before.
		"household.summaryOne":  "%d member · %s base",
		"household.summaryMany": "%d members · %s base",
	} {
		english[k] = v
	}
}
