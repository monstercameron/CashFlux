// SPDX-License-Identifier: MIT

package i18n

// provisionalReportKeys captions a report period with how settled its figures
// are (C693). Merged via init so this file never touches en.go.
//
// The wording states rather than warns. Nothing is wrong with a month that is
// still running; what is wrong is reading its total as though it were finished,
// and the fix for that is a sentence, not an alarm.
var provisionalReportKeys = Catalog{
	"reports.provisionalThrough":      "These figures are not final. Your statements are reconciled through %s; anything after that is still arriving.",
	"reports.provisionalNoStatements": "These figures are not final. No account has been reconciled to a statement yet, so nothing here has been confirmed against your bank.",
	// Said only when some of the period's balance rests on a checkpoint rather
	// than on transactions — the difference between "you spent less" and "the
	// month is not over".
	"reports.provisionalCheckpointOne":  "%d balance checkpoint accounts for %s of this period, and is left out of the figures above.",
	"reports.provisionalCheckpointMany": "%d balance checkpoints account for %s of this period, and are left out of the figures above.",
}

func init() {
	for k, v := range provisionalReportKeys {
		english[k] = v
	}
}
