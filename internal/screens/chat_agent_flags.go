// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

type assistantFlagEffect struct {
	Key, Title, Detail string
}

type assistantFlagPlan struct {
	match   string
	effects []assistantFlagEffect
	err     error
}

func assistantFlagMatch(raw json.RawMessage) (string, error) {
	var args struct {
		Match json.RawMessage `json:"match"`
	}
	if err := assistantObjectArgs(raw, &args); err != nil {
		return "", err
	}
	if len(args.Match) == 0 {
		return "", nil
	}
	var match string
	if args.Match[0] != '"' {
		return "", fmt.Errorf("match must be a string when supplied")
	}
	if err := json.Unmarshal(args.Match, &match); err != nil {
		return "", fmt.Errorf("invalid match: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(match)), nil
}

func assistantFlagEffects(app *appstate.App, match string) []assistantFlagEffect {
	var effects []assistantFlagEffect
	for _, finding := range runAnomalyDetectors(app, uistate.LoadPrefs().WeekStartWeekday()) {
		if match == "" || strings.Contains(strings.ToLower(finding.Title+" "+finding.Detail), match) {
			effects = append(effects, assistantFlagEffect{finding.Key, finding.Title, finding.Detail})
		}
	}
	sort.Slice(effects, func(i, j int) bool { return effects[i].Key < effects[j].Key })
	return effects
}

// agToolsFlags previews exact dismissals and rejects a changed matching set.
// Only the approved keys can be hidden; a retry never sweeps newly detected flags.
func agToolsFlags(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	plans := map[string]assistantFlagPlan{}
	prepare := func(raw json.RawMessage) assistantFlagPlan {
		match, err := assistantFlagMatch(raw)
		value := assistantFlagPlan{match: match, err: err}
		if err == nil {
			value.effects = assistantFlagEffects(app, match)
		}
		plans[string(raw)] = value
		return value
	}
	return []chatTool{{
		spec: ai.FunctionTool("dismiss_flagged_activity",
			"Dismiss the exact currently flagged activities shown in the approval preview. Omit match to select all current flags, or supply a title/detail phrase. This hides flags through dismissal preferences; it never deletes or edits transactions. If matching flags change before approval is applied, review the new set first.",
			json.RawMessage(`{"type":"object","properties":{"match":{"type":"string"}},"additionalProperties":false}`)),
		mutates: true,
		preview: func(raw json.RawMessage) string {
			value := prepare(raw)
			if value.err != nil {
				return "Can't dismiss flagged activity: " + value.err.Error()
			}
			if len(value.effects) == 0 {
				return "No flagged activities to dismiss."
			}
			out := fmt.Sprintf("Dismiss exactly %d flagged activities by saving dismissal preferences. No transactions are deleted or edited.", len(value.effects))
			for _, effect := range value.effects {
				out += fmt.Sprintf("\n- %s (key %s): %s", effect.Title, effect.Key, effect.Detail)
			}
			return out
		},
		run: func(raw json.RawMessage) string {
			value, ok := plans[string(raw)]
			if !ok {
				value = prepare(raw)
			}
			if value.err != nil {
				return "Can't dismiss flagged activity: " + value.err.Error()
			}
			// Dismissals use the active dataset's settings bridge. Never apply an
			// old approval to a different dataset after the operator switches it.
			if appstate.Default != app {
				return "The active dataset changed; review flagged activity again. Nothing changed."
			}
			settings := uistate.LoadSmartSettings()
			allDismissed := len(value.effects) > 0
			for _, effect := range value.effects {
				allDismissed = allDismissed && settings.IsDismissed(effect.Key)
			}
			if allDismissed {
				return "Those flagged activities are already dismissed; nothing changed."
			}
			live := assistantFlagEffects(app, value.match)
			if !reflect.DeepEqual(live, value.effects) {
				return "The matching flagged activities changed after the preview; review them again. Nothing changed."
			}
			if len(value.effects) == 0 {
				return "No flagged activities to dismiss."
			}
			keys := make([]string, 0, len(value.effects))
			for _, effect := range value.effects {
				keys = append(keys, effect.Key)
			}
			uistate.DismissAllSmartInsights(keys)
			saved := uistate.LoadSmartSettings()
			for _, key := range keys {
				if !saved.IsDismissed(key) {
					return "Couldn't save the flag dismissals. Review them and retry."
				}
			}
			return fmt.Sprintf("Dismissed exactly %d flagged activities. No transactions were deleted or edited.", len(keys))
		},
	}}
}
