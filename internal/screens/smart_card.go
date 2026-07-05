// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// smartCardProps carries one insight to render. The insight is passed whole; the
// card formats its own amount and tone so callers stay simple. Compact renders a
// flat row (no bordered/rounded box) so the dashboard's Smart digest tile matches
// the flat-row style of every other bento tile, instead of nesting cards in a card.
type smartCardProps struct {
	Ins     smart.Insight
	Compact bool // flat list row (no bordered box) — the Smart strip
	Glance  bool // tightest: truncated, no action footer — the dashboard digest tile
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
	// Capture the notice atom unconditionally so PostNotice works even when this
	// card renders on a page that doesn't call UseNotice itself (e.g. /smart hub).
	_ = uistate.UseNotice()
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
			if ins.Action.Route == "" {
				return
			}
			target := uistate.RoutePath(ins.Action.Route)
			current := router.GetCurrentPath()
			// SMART-SU1 fix: when already on the target page, scroll to the named
			// row instead of issuing a no-op navigation. The subscription name lives
			// in the insight key after the first colon (e.g. "SMART-SU1:netflix").
			// We use the same nameSlug as the subscriptions screen to build the
			// data-testid of the row checkbox, then scroll its parent .sub-row into
			// view and briefly add a highlight outline so the row is easy to spot.
			if current == target && ins.Action.Route == "/subscriptions" {
				parts := strings.SplitN(ins.Key, ":", 2)
				if len(parts) == 2 {
					slug := nameSlug(parts[1])
					doc := js.Global().Get("document")
					// The checkbox is the anchor; the row wrapper is .sub-row.
					anchor := doc.Call("querySelector", `[data-testid="sub-cancel-select-`+slug+`"]`)
					if !anchor.IsNull() && !anchor.IsUndefined() {
						rowEl := anchor.Call("closest", ".sub-row")
						scrollEl := anchor
						if !rowEl.IsNull() && !rowEl.IsUndefined() {
							scrollEl = rowEl
						}
						scrollEl.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "center"})
						// Transient outline highlight for 1.5 s so the user's eye finds the row.
						cl := scrollEl.Get("classList")
						cl.Call("add", "smart-highlight-row")
						var cb js.Func
						cb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
							cl.Call("remove", "smart-highlight-row")
							cb.Release()
							return nil
						})
						js.Global().Call("setTimeout", cb, 1500)
					}
				}
				return
			}
			nav.Navigate(target)
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
			// SMART-SU9 fix: confirm the to-do was added with a toast notice.
			uistate.PostNotice(uistate.T("smart.taskAdded"), false)
			rev.Set(rev.Get() + 1)

		case smart.ActionCreateGoal:
			app := appstate.Default
			if app == nil {
				return
			}
			cur := smartCurrencyOr(ins.Action.GoalCurrency, app.Settings().BaseCurrency)
			g := domain.Goal{
				ID:            id.New(),
				Name:          ins.Action.GoalName,
				OwnerID:       domain.GroupOwnerID, // household-level goal; passes OwnerID validation
				Scope:         domain.ScopeShared,
				TargetAmount:  money.Money{Amount: ins.Action.GoalTarget, Currency: cur},
				IsSinkingFund: ins.Action.GoalIsSinkingFund,
				CategoryID:    ins.Action.GoalCategoryID,
			}
			if err := app.PutGoal(g); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			noticeKey := "smart.goalCreated"
			if ins.Action.GoalIsSinkingFund {
				noticeKey = "smart.sinkingFundCreated"
			}
			uistate.PostNotice(uistate.T(noticeKey), false)
			rev.Set(rev.Get() + 1)
			nav.Navigate(uistate.RoutePath("/goals"))

		case smart.ActionCreateRecurring:
			app := appstate.Default
			if app == nil {
				return
			}
			cur := smartCurrencyOr(ins.Action.RecurringCurrency, app.Settings().BaseCurrency)
			r := domain.Recurring{
				ID:      id.New(),
				Label:   ins.Action.RecurringLabel,
				Amount:  money.Money{Amount: ins.Action.RecurringAmount, Currency: cur},
				Cadence: domain.RecurringCadence(ins.Action.RecurringCadence),
			}
			if err := app.PutRecurring(r); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.PostNotice(uistate.T("smart.recurringCreated"), false)
			rev.Set(rev.Get() + 1)
			nav.Navigate(uistate.RoutePath("/planning"))

		case smart.ActionCancelSubscription:
			app := appstate.Default
			if app == nil {
				return
			}
			if err := app.MarkSubscriptionCancelled(ins.Action.SubscriptionName, time.Now()); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.PostNotice(uistate.T("smart.subscriptionCancelled"), false)
			rev.Set(rev.Get() + 1)
			nav.Navigate(uistate.RoutePath("/subscriptions"))

		case smart.ActionAutomateGoal:
			app := appstate.Default
			if app == nil {
				return
			}
			if _, err := app.CreateWorkflowFromGoal(ins.Action.GoalID, ins.Action.GoalMonthlyAmount); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.PostNotice(uistate.T("smart.automateGoalCreated"), false)
			rev.Set(rev.Get() + 1)
			nav.Navigate(uistate.RoutePath("/planning"))
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

	// Three layouts share one component:
	//   • default — a bordered, rounded card (the /smart hub aesthetic).
	//   • Compact — a flat list row with a hairline divider (the Smart strip).
	//   • Glance  — the tightest variant for the height-constrained dashboard digest
	//     tile: a single title line + a one-line truncated detail and NO action
	//     footer, so each insight is a glanceable line-pair that fits the tile.
	flat := props.Compact || props.Glance
	boxCls := "smart-card " + tw.Fold(tw.Flex, tw.FlexCol, tw.Gap1, tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)
	if flat {
		// Omit the "smart-card" class: its CSS forces a bordered, rounded, shadowed box
		// (with !important) and a severity left-rail. A flat row with a hairline divider
		// matches the surrounding surface; the severity dot still carries the signal.
		boxCls = "smart-row " + tw.Fold(tw.Flex, tw.FlexCol, tw.Gap1, tw.Py15, tw.BorderB, tw.BorderLine)
	}

	detailCls := tw.Fold(tw.Text13, tw.TextDim)
	titleCls := tw.Fold(tw.FontSemibold, tw.Text14)
	if props.Glance {
		detailCls = tw.Fold(tw.Text13, tw.TextDim, tw.LineClamp2) // two lines, word-boundary ellipsis
		titleCls = tw.Fold(tw.FontSemibold, tw.Text14, tw.Truncate)
	}

	children := []any{
		Attr("data-testid", "smart-card"),
		Attr("data-feature", ins.Feature),
		Attr("data-severity", ins.Severity.String()),

		// Title row: severity dot + title on the left, amount on the right.
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsStart, tw.JustifyBetween, tw.Gap2)),
			Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsStart, tw.Gap2, tw.MinW0)),
				Span(ClassStr(tw.ColorClass(tone)), Attr("aria-hidden", "true"), "●"),
				Span(ClassStr(titleCls), ins.Title),
			),
			amountNode,
		),

		// Reason / explanation.
		P(ClassStr(detailCls), ins.Detail),
	}

	// Glance is a passive summary (the digest tile) — no action/dismiss footer; the
	// full controls live on the Smart strip and the /smart hub.
	if !props.Glance {
		children = append(children, Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1)),
			actionNode,
			Button(ClassStr("btn btn-sm btn-ghost "+tw.Fold(tw.MlAuto)), Type("button"),
				Attr("data-testid", "smart-dismiss"),
				Attr("aria-label", uistate.T("smart.dismiss")),
				Attr("title", uistate.T("smart.dismiss")),
				OnClick(onDismiss),
				uistate.T("smart.dismiss"),
			),
		))
	}

	return Article(append([]any{ClassStr(boxCls)}, children...)...)
}

// smartCurrencyOr returns preferred when non-empty, otherwise fallback. It is
// used to resolve the currency for a create-goal or create-recurring action
// where the engine may or may not supply an explicit currency.
func smartCurrencyOr(preferred, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
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

// smartStripList renders insights as flat list rows (Compact) for the Smart strip
// — one panel of insight rows rather than bordered cards nested in the strip card.
func smartStripList(insights []smart.Insight) ui.Node {
	return Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol)),
		MapKeyed(insights,
			func(i smart.Insight) any { return i.Key },
			func(i smart.Insight) ui.Node {
				return ui.CreateElement(smartInsightCard, smartCardProps{Ins: i, Compact: true})
			},
		),
	)
}

// smartDigestList renders insights as tight Glance rows for the dashboard's Smart
// digest tile: one title line + a truncated detail and no action footer, so each
// insight fits the height-constrained bento tile instead of overflowing it.
func smartDigestList(insights []smart.Insight) ui.Node {
	return Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol)),
		MapKeyed(insights,
			func(i smart.Insight) any { return i.Key },
			func(i smart.Insight) ui.Node {
				return ui.CreateElement(smartInsightCard, smartCardProps{Ins: i, Glance: true})
			},
		),
	)
}
