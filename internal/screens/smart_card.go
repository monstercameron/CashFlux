// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// smartCardProps carries one insight to render. The insight is passed whole; the
// card formats its own amount and tone so callers stay simple.
type smartCardProps struct {
	Ins smart.Insight
}

// severityTone maps an insight severity to a color token understood by
// tw.ColorClass — the glanceable "how serious is this" signal.
func severityTone(s smart.Severity) string {
	switch s {
	case smart.SeverityAlert:
		return "text-down"
	case smart.SeverityWarn:
		return "text-warn"
	case smart.SeverityNudge:
		return "text-up"
	default:
		return "text-dim"
	}
}

// smartInsightCard renders one glanceable insight: a severity dot + title, an
// optional headline amount, a plain-English reason, and a footer with the
// optional one-tap action and a dismiss control. It is its own component so its
// event hooks sit at stable positions (the On*-hooks-in-loops rule).
func smartInsightCard(props smartCardProps) ui.Node {
	ins := props.Ins
	nav := router.UseNavigate()
	rev := uistate.UseDataRevision()
	tone := severityTone(ins.Severity)

	onDismiss := ui.UseEvent(func() {
		uistate.DismissSmartInsight(ins.Key)
		rev.Set(rev.Get() + 1)
	})

	// The action handler is declared unconditionally (stable hooks); it no-ops when
	// the insight carries no action.
	onAction := ui.UseEvent(func() {
		if ins.Action == nil {
			return
		}
		switch ins.Action.Kind {
		case smart.ActionNavigate:
			if ins.Action.Route != "" {
				nav.Navigate(uistate.RoutePath(ins.Action.Route))
			}
		case smart.ActionCreateTask:
			app := appstate.Default
			if app == nil {
				return
			}
			t := domain.Task{
				ID:       id.New(),
				Title:    ins.Action.TaskTitle,
				Notes:    ins.Action.TaskNotes,
				Status:   domain.StatusOpen,
				Priority: domain.PriorityMedium,
				Source:   domain.SourceAI,
				Due:      time.Time{},
			}
			if ins.Action.RelatedID != "" {
				t.RelatedID = ins.Action.RelatedID
			}
			if err := app.PutTask(t); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.PostNotice(uistate.T("smart.taskAdded"), false)
			rev.Set(rev.Get() + 1)
		}
	})

	// Headline amount (right-aligned, toned), only when the insight carries one.
	var amountNode ui.Node = Fragment()
	if ins.HasAmount {
		amountNode = Span(ClassStr(tw.Fold(tw.FontSemibold, tw.Text14)+" "+tw.ColorClass(tone)),
			fmtMoney(ins.Amount),
		)
	}

	// Footer action button, only when the insight offers one.
	var actionNode ui.Node = Fragment()
	if ins.Action != nil && ins.Action.Label != "" {
		actionNode = Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "smart-action-"+ins.Feature),
			Attr("aria-label", ins.Action.Label),
			OnClick(onAction),
			ins.Action.Label,
		)
	}

	return Article(ClassStr("smart-card "+tw.Fold(tw.Flex, tw.FlexCol, tw.Gap1, tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
		Attr("data-testid", "smart-card"),
		Attr("data-feature", ins.Feature),
		Attr("data-severity", ins.Severity.String()),

		// Title row: severity dot + title on the left, amount on the right.
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsStart, tw.JustifyBetween, tw.Gap2)),
			Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsStart, tw.Gap2, tw.MinW0)),
				Span(ClassStr(tw.ColorClass(tone)), Attr("aria-hidden", "true"), "●"),
				Span(ClassStr(tw.Fold(tw.FontSemibold, tw.Text14)), ins.Title),
			),
			amountNode,
		),

		// Reason / explanation.
		P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)), ins.Detail),

		// Footer: optional action + dismiss.
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1)),
			actionNode,
			Button(ClassStr("btn btn-sm btn-ghost "+tw.Fold(tw.MlAuto)), Type("button"),
				Attr("data-testid", "smart-dismiss"),
				Attr("aria-label", uistate.T("smart.dismiss")),
				Attr("title", uistate.T("smart.dismiss")),
				OnClick(onDismiss),
				uistate.T("smart.dismiss"),
			),
		),
	)
}

// smartInsightList renders a list of insight cards, each keyed by its stable
// Key so dismissing one doesn't disturb the others' component identity.
func smartInsightList(insights []smart.Insight) ui.Node {
	return Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol, tw.Gap2)),
		MapKeyed(insights,
			func(i smart.Insight) any { return i.Key },
			func(i smart.Insight) ui.Node { return ui.CreateElement(smartInsightCard, smartCardProps{Ins: i}) },
		),
	)
}
