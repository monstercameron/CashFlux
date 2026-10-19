// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

type cloudConflictReviewProps struct {
	Open                              bool
	OnClose, OnKeepLocal, OnUseServer func()
}

func cloudConflictReview(p cloudConflictReviewProps) uic.Node {
	return uiw.Overlay(uiw.OverlayProps{Open: p.Open, Kind: uiw.OverlayModal, Title: uistate.T("cloud1.copiesTitle"), TestID: "cloud-conflict-review", OnClose: p.OnClose,
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4),
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.conflictHint")),
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
