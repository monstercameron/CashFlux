// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsReference(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/supportbundle"
	"github.com/monstercameron/CashFlux/internal/taxref"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// agToolsReference adds the two tools that keep the assistant honest about things
// it would otherwise answer from memory: dated tax limits (PS9) and a diagnostic
// bundle for a bug report (PS10).
func agToolsReference(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = base
	_ = rates
	return []chatTool{
		{
			spec: ai.FunctionTool("tax_limits",
				"Look up US federal contribution limits and thresholds for a tax year — 401(k), IRA, HSA, "+
					"standard deduction, Social Security wage base and claim ages. ALWAYS call this rather "+
					"than answering from memory: these change every year and your recollection of them is "+
					"frozen. Quote the year with every figure. If the year isn't covered, say so — do not "+
					"use a nearby year.",
				json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","description":"the tax year, e.g. 2026; defaults to the most recent year available"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Year int `json:"year"`
				}
				_ = json.Unmarshal(raw, &a)
				if a.Year == 0 {
					l := taxref.Latest()
					if l.Year == 0 {
						return "No tax figures are available."
					}
					return l.Summary()
				}
				l, ok := taxref.For(a.Year)
				if !ok {
					years := taxref.Years()
					list := make([]string, 0, len(years))
					for _, y := range years {
						list = append(list, fmt.Sprintf("%d", y))
					}
					return fmt.Sprintf("We don't have figures for %d. We have: %s. "+
						"Tell them that plainly — do not quote a nearby year's numbers as though they were %d's.",
						a.Year, strings.Join(list, ", "), a.Year)
				}
				return l.Summary()
			},
		},
		{
			spec: ai.FunctionTool("build_diagnostic",
				"Build a diagnostic the user can paste into a bug report: app version, browser, how much "+
					"data they have, which settings are on, and recent errors. It contains NO transactions, "+
					"names, notes or amounts. Use when the user reports something broken and wants to report "+
					"it. Show them the result and tell them it is safe to paste publicly.",
				json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				return supportbundle.Build(diagnosticInput(app))
			},
		},
	}
}

// diagnosticInput gathers the bundle's contents. Every value here is a count, a
// version, or a flag — the shape of the household's setup rather than any of its
// content — and the errors go through Redact on the way in.
func diagnosticInput(app *appstate.App) supportbundle.Input {
	in := supportbundle.Input{
		AppVersion: uistate.T("about.version"),
		UserAgent:  browserUserAgent(),
		At:         time.Now(),
		Counts:     map[string]int{},
		Flags:      map[string]string{},
	}
	if app == nil {
		return in
	}
	in.Counts["accounts"] = len(app.Accounts())
	in.Counts["transactions"] = len(app.Transactions())
	in.Counts["categories"] = len(app.Categories())
	in.Counts["budgets"] = len(app.Budgets())
	in.Counts["goals"] = len(app.Goals())
	in.Counts["recurring"] = len(app.Recurring())
	in.Counts["conversations"] = len(app.Conversations())

	settings := app.Settings()
	in.Flags["baseCurrency"] = settings.BaseCurrency
	in.Flags["aiKey"] = yesNo(strings.TrimSpace(settings.OpenAIKey) != "")
	in.Flags["aiModel"] = settings.OpenAIModel
	in.Flags["customEndpoint"] = yesNo(strings.TrimSpace(settings.OpenAIBaseURL) != "")
	in.Flags["theme"] = string(uistate.LoadPrefs().Theme)

	// Only the app's own ERRORS ride along. Info-level lines are a trace of what
	// somebody did, which is both less useful for diagnosis and closer to their
	// private activity than a fault report has any business carrying.
	if ring := app.LogRing(); ring != nil {
		entries := ring.Entries()
		for i := len(entries) - 1; i >= 0 && len(in.RecentErrors) < supportBundleErrorCount; i-- {
			if entries[i].Level < slog.LevelError {
				continue
			}
			in.RecentErrors = append(in.RecentErrors, supportbundle.Redact(entries[i].Message))
		}
	}
	return in
}

// supportBundleErrorCount is how many error lines are pulled. The bundle trims to
// its own smaller limit afterwards; pulling a few more here means a
// redacted-to-nothing line does not cost a useful one.
const supportBundleErrorCount = 20

// browserUserAgent reads the browser string, which is where rendering faults live.
func browserUserAgent() string {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return ""
	}
	return nav.Get("userAgent").String()
}

// yesNo renders a flag the way a bug report reads best.
func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
