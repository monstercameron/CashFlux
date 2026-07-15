// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsAuditor(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/audit"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// runFullAudit sweeps every SMART Free detector family over the live data (fee
// bleed, idle cash, price creep, dormant accounts, budget true-up, unbudgeted
// spending, earmark integrity, duplicate/anomaly detectors) and ranks the combined
// findings by dollar impact via the pure audit package. All Free features are
// force-enabled so the audit is complete regardless of the user's per-feature
// opt-ins; dismissed insights are still honored (they've been explicitly cleared).
func runFullAudit(app *appstate.App, base string) audit.Report {
	in := buildSmartInput(app, uistate.LoadPrefs().WeekStartWeekday())
	settings := smart.EnableFreeOnly(uistate.LoadSmartSettings())
	insights := smartengine.Run(in, settings)
	return audit.Audit(insights, base)
}

// agToolsAuditor exposes the background auditor (AG6) as an assistant tool: one
// on-demand deep sweep that returns a prioritized, evidence-carrying findings list
// where each row names its one-tap fix. It reads only; the fixes themselves run
// through the existing mutating tools (per-family) on the user's approval.
func agToolsAuditor(app *appstate.App, base string, rates currency.Rates) []chatTool {
	fmtM := func(minor int64) string { return fmtMoney(money.New(minor, base)) }
	return []chatTool{
		{
			spec: ai.FunctionTool("run_audit",
				"Run a full background audit: sweep every detector family (fees, idle cash, price creep, dormant accounts, budget drift, unbudgeted spending, duplicate/anomaly flags, earmark breaches) over the user's live data and return the findings ranked by yearly dollar impact, each with its evidence and a one-tap fix. Use this for 'find me money', 'audit my finances', or a proactive review. Then offer to apply the top fixes (each waits for approval).",
				json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","description":"max findings to return (default 12)"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Limit int `json:"limit"`
				}
				_ = json.Unmarshal(raw, &a)
				if a.Limit <= 0 || a.Limit > 30 {
					a.Limit = 12
				}
				rep := runFullAudit(app, base)
				if len(rep.Findings) == 0 {
					return "Audit complete — no findings. Nothing is bleeding money right now."
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Audit found %d issue(s), roughly %s in total impact (%d with a one-tap fix). Ranked by dollar impact:\n",
					len(rep.Findings), fmtM(rep.TotalImpactMinor), rep.OneTapCount())
				for i, f := range rep.Findings {
					if i >= a.Limit {
						fmt.Fprintf(&b, "… %d more — raise limit to see them.\n", len(rep.Findings)-a.Limit)
						break
					}
					impact := ""
					if f.ImpactMinor > 0 {
						impact = "  (" + fmtM(f.ImpactMinor) + ")"
					}
					fmt.Fprintf(&b, "%d. [%s] %s%s\n   %s\n", i+1, f.Family, f.Insight.Title, impact, f.Insight.Detail)
					if f.Fix.OneTap && f.Fix.Label != "" {
						fmt.Fprintf(&b, "   Fix: %s\n", f.Fix.Label)
					}
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
	}
}

// AuditFindingsCardProps carries a computed audit report for the findings card.
type AuditFindingsCardProps struct {
	Report audit.Report
}

// AuditFindingsCard renders the prioritized audit findings as a glanceable,
// controllable card (the AG6 surface): the total-impact headline plus one row per
// finding — its family, title, dollar impact, evidence, and one-tap fix label — so
// the agent's audit power is VISIBLE, not buried in chat text. Drop it into the
// assistant surface when a run_audit result is available.
func AuditFindingsCard(p AuditFindingsCardProps) ui.Node {
	return ui.CreateElement(auditFindingsCardComp, p)
}

func auditFindingsCardComp(p AuditFindingsCardProps) ui.Node {
	rep := p.Report
	if len(rep.Findings) == 0 {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("audit.empty"))})
	}
	rowsInner := []any{css.Class(tw.FlexCol, tw.Gap2)}
	for _, f := range rep.Findings {
		rowsInner = append(rowsInner, auditFindingRow(auditFindingRowProps{Finding: f, Base: rep.Base}))
	}
	head := Div(css.Class("audit-head", tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
		Span(css.Class(tw.FontSemibold), uistate.T("audit.title")),
		Span(css.Class(tw.TextXs), uistate.T("audit.total", fmtMoney(rep.TotalImpact()), rep.OneTapCount())),
	)
	return uiw.Card(uiw.CardProps{
		TestID: "audit-findings",
		Attrs:  []any{Attr("role", "status")},
		Body:   Div(css.Class("audit-card", tw.FlexCol, tw.Gap2), head, Div(rowsInner...)),
	})
}

type auditFindingRowProps struct {
	Finding audit.Finding
	Base    string
}

// auditFindingRow renders one finding. Own component so any interactive hook stays
// at a stable render position within the variable-length findings list.
func auditFindingRow(p auditFindingRowProps) ui.Node { return ui.CreateElement(auditFindingRowComp, p) }

func auditFindingRowComp(p auditFindingRowProps) ui.Node {
	f := p.Finding
	impact := ""
	if f.ImpactMinor > 0 {
		impact = fmtMoney(money.New(f.ImpactMinor, p.Base))
	}
	var fix ui.Node
	if f.Fix.OneTap && f.Fix.Label != "" {
		fix = Span(css.Class("audit-fix", tw.TextXs), f.Fix.Label)
	}
	return Div(css.Class("audit-row", tw.FlexCol, tw.Gap1), Attr("data-testid", "audit-finding"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2),
			Span(css.Class(tw.FontMedium), f.Insight.Title),
			Span(css.Class("audit-impact", tw.TextXs), impact),
		),
		Div(css.Class("audit-meta", tw.TextXs, tw.TextFaint), Span("["+f.Family+"] "), Span(f.Insight.Detail)),
		fix,
	)
}
