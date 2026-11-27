// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v8/css"
	. "github.com/monstercameron/GoWebComponents/v8/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v8/ui"
)

type cloudConflictReviewProps struct {
	Open                                       bool
	HasDiff                                    bool
	Comparison                                 conflictcheckpoint.Comparison
	OnClose, OnMerge, OnKeepLocal, OnUseServer func()
}

func cloudConflictReview(p cloudConflictReviewProps) uic.Node {
	var diffNode uic.Node
	if p.HasDiff && p.Comparison.Available {
		localTx := p.Comparison.Counts[conflictcheckpoint.SideLocal]["transactions"]
		serverTx := p.Comparison.Counts[conflictcheckpoint.SideServer]["transactions"]
		summary := uistate.T("cloud1.diffSummary", localTx, serverTx, len(p.Comparison.Changes))
		items := make([]uic.Node, 0, min(len(p.Comparison.Changes), 6))
		for i, ch := range p.Comparison.Changes {
			if i >= 6 {
				break
			}
			label := ch.AfterLabel
			if label == "" {
				label = ch.BeforeLabel
			}
			items = append(items, Li(css.Class(tw.Text12, tw.TextDim), fmt.Sprintf("%s %s: %s", ch.Kind, ch.Entity, label)))
		}
		diffNode = Div(css.Class("card", tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "sync-conflict-diff"),
			P(css.Class(tw.Text13, tw.FontMedium), summary),
			If(len(items) > 0, Ul(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), items)),
		)
	}
	var mergeSection uic.Node
	if p.OnMerge != nil && p.HasDiff {
		mergeSection = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.mergeBoth")),
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.mergeBothHint")),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.mergeBoth"), TestID: "sync-conflict-merge", OnClick: p.OnMerge}),
		)
	}
	return uiw.Overlay(uiw.OverlayProps{Open: p.Open, Kind: uiw.OverlayModal, Title: uistate.T("cloud1.copiesTitle"), TestID: "cloud-conflict-review", OnClose: p.OnClose,
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4),
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.conflictHint")),
			diffNode,
			mergeSection,
			Div(H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.keepBoth")),
				P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.keepBothHint")),
				uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.keepBoth"), TestID: "sync-conflict-keep-both", OnClick: p.OnClose})),
			uic.CreateElement(cloudDisclosure, cloudDisclosureProps{Title: uistate.T("cloud1.otherCopyChoices"), TestID: "cloud-conflict-other-choices", Body: Fragment(
				P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.replaceHint")),
				uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.replaceServer"), TestID: "sync-conflict-keep-local", Kind: cloudActionDanger, OnClick: p.OnKeepLocal}),
				P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.useServerHint")),
				uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.useServer"), TestID: "sync-conflict-use-server", Kind: cloudActionDanger, OnClick: p.OnUseServer}),
			)}),
		),
		Footer: uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.close"), TestID: "cloud-conflict-close", OnClick: p.OnClose}),
	})
}
