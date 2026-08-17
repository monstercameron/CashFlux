// SPDX-License-Identifier: MIT

// Package methodology is the catalog of "how this is computed" notes for the
// annual report's sections (C385).
//
// The reviewer's complaint was broad benchmark language with no benchmark source
// shown inline: the report says a savings rate is healthy without ever saying
// what it is being measured against or where that number came from. A figure
// with an unstated standard behind it asks the reader to trust a comparison they
// cannot check.
//
// Three things go in a note, and all three matter for different reasons:
//
//   - The FORMULA, so a figure that looks wrong can be reasoned about instead of
//     merely disbelieved.
//   - The EXCLUSIONS, because what a number leaves out is the single most common
//     source of "that isn't what I spent" — transfers especially.
//   - The BENCHMARKS with their sources, because "healthy" is a claim, and a
//     claim with no attribution is the thing the reviewer actually objected to.
//
// The notes are catalog KEYS, not English: this is exactly the copy that must
// translate. The package is data plus a lookup so the catalog is testable and
// cannot silently lose a section.
package methodology

// Benchmark is one comparison value the report judges a figure against, with
// where the number comes from. Source is never omitted — an unsourced benchmark
// is the problem this package exists to fix, not a shortcut it may take.
type Benchmark struct {
	// LabelKey names what is being compared (e.g. "a healthy savings rate").
	LabelKey string
	// ValueKey is the threshold or range as text ("20% or more").
	ValueKey string
	// SourceKey says where the number comes from — including, honestly, when the
	// answer is "a convention this app chose".
	SourceKey string
}

// Note is one section's methodology.
type Note struct {
	// SectionID matches the report section's DOM id ("rpta-01").
	SectionID string
	// FormulaKeys are the calculations, one line each.
	FormulaKeys []string
	// ExclusionKeys are what the figures deliberately leave out.
	ExclusionKeys []string
	// Benchmarks are the standards the section judges against, if any.
	Benchmarks []Benchmark
}

// HasContent reports whether a note has anything worth opening a drawer for.
func (n Note) HasContent() bool {
	return len(n.FormulaKeys) > 0 || len(n.ExclusionKeys) > 0 || len(n.Benchmarks) > 0
}

// transfersExcluded is the exclusion that applies to every money-flow figure in
// the report and is the most common cause of "that isn't what I spent".
const transfersExcluded = "method.exclTransfers"

// excludedFlagged is the per-transaction opt-out.
const excludedFlagged = "method.exclFlagged"

// All returns the catalog, rebuilt per call so a caller mutating a returned
// slice cannot corrupt it for everyone else.
func All() []Note {
	return []Note{
		{
			SectionID:   "rpta-01",
			FormulaKeys: []string{"method.f.health", "method.f.savingsRate"},
			Benchmarks: []Benchmark{
				{LabelKey: "method.b.savingsRate", ValueKey: "method.b.savingsRateValue", SourceKey: "method.b.savingsRateSource"},
				{LabelKey: "method.b.healthBands", ValueKey: "method.b.healthBandsValue", SourceKey: "method.b.ownConvention"},
			},
			ExclusionKeys: []string{transfersExcluded, excludedFlagged},
		},
		{
			SectionID:     "rpta-02",
			FormulaKeys:   []string{"method.f.income", "method.f.expense", "method.f.net"},
			ExclusionKeys: []string{transfersExcluded, excludedFlagged, "method.exclFx"},
		},
		{
			SectionID:     "rpta-03",
			FormulaKeys:   []string{"method.f.monthBuckets", "method.f.inProgress"},
			ExclusionKeys: []string{transfersExcluded},
		},
		{
			SectionID:     "rpta-04",
			FormulaKeys:   []string{"method.f.catTotal", "method.f.catShare", "method.f.catYoY"},
			ExclusionKeys: []string{transfersExcluded, "method.exclUncategorized"},
		},
		{
			SectionID:     "rpta-05",
			FormulaKeys:   []string{"method.f.payeeTotal"},
			ExclusionKeys: []string{transfersExcluded, "method.exclPayeeBlank"},
		},
		{
			SectionID:   "rpta-06",
			FormulaKeys: []string{"method.f.goalCoverage", "method.f.goalProjection"},
			ExclusionKeys: []string{
				"method.exclArchivedGoals", "method.exclNonFinancialGoals",
			},
		},
		{
			SectionID:     "rpta-07",
			FormulaKeys:   []string{"method.f.budgetUsed", "method.f.budgetPace"},
			ExclusionKeys: []string{transfersExcluded, "method.exclNoBudget"},
		},
		{
			SectionID:   "rpta-08",
			FormulaKeys: []string{"method.f.creditUtil", "method.f.creditScore"},
			Benchmarks: []Benchmark{
				{LabelKey: "method.b.util", ValueKey: "method.b.utilValue", SourceKey: "method.b.utilSource"},
				{LabelKey: "method.b.emergency", ValueKey: "method.b.emergencyValue", SourceKey: "method.b.emergencySource"},
			},
			ExclusionKeys: []string{"method.exclNotBureau"},
		},
		{
			SectionID:     "rpta-09",
			FormulaKeys:   []string{"method.f.unusual", "method.f.subsDrift"},
			ExclusionKeys: []string{"method.exclThinHistory"},
		},
		{
			SectionID:     "rpta-10",
			FormulaKeys:   []string{"method.f.planTrim", "method.f.planProject"},
			ExclusionKeys: []string{"method.exclProjectionAssumes"},
		},
	}
}

// For returns the note for a report section id.
func For(sectionID string) (Note, bool) {
	for _, n := range All() {
		if n.SectionID == sectionID {
			return n, true
		}
	}
	return Note{}, false
}
