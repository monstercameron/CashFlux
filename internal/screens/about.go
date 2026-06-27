// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens contains all top-level routed screens. This file owns the
// /about screen (C290 / C293): a dedicated About & Privacy page with honest,
// plain-English disclosure of what CashFlux is, what data stays on device,
// what cloud sync sends, and how AI features use the user's own key.
package screens

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// AboutScreen is the dedicated /about route (C290 / C293). It covers:
//   - App identity and tagline.
//   - Privacy & local-first data-handling statement.
//   - Cloud-sync disclosure: what leaves the device and when (C291).
//   - AI-key disclosure: bring-your-own-key, stored locally, sent to OpenAI only on demand (C292).
//   - Version and changelog link.
func AboutScreen() ui.Node {
	return Div(
		css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		Attr("id", "about-page"),
		aboutIdentityCard(),
		aboutPrivacyCard(),
		aboutCloudSyncCard(),
		aboutAICard(),
		aboutVersionCard(),
	)
}

// aboutIdentityCard renders the app name, tagline, and one-line description.
func aboutIdentityCard() ui.Node {
	return uiw.Card(uiw.CardProps{
		Title: uistate.T("about.headingIdentity"),
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.tagline")),
		),
	})
}

// aboutPrivacyCard renders the local-first privacy commitment (C290, C293).
func aboutPrivacyCard() ui.Node {
	return uiw.Card(uiw.CardProps{
		Title:  uistate.T("about.headingPrivacy"),
		TestID: "about-privacy-card",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.privacyLocalFirst")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.privacyExport")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.privacyNoTracking")),
		),
	})
}

// aboutCloudSyncCard discloses what data leaves the device when cloud sync is
// enabled, that it is off by default, and that the user controls it (C291).
func aboutCloudSyncCard() ui.Node {
	return uiw.Card(uiw.CardProps{
		Title:  uistate.T("about.headingCloudSync"),
		TestID: "about-cloudsync-card",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.cloudSyncOff")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.cloudSyncOn")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.cloudSyncControl")),
		),
	})
}

// aboutAICard discloses the bring-your-own-key model, local key storage, and
// when prompts/data are sent to OpenAI (C292).
func aboutAICard() ui.Node {
	settingsLink := A(
		Attr("href", uistate.RoutePath("/settings")),
		css.Class(tw.Underline, tw.HoverTextFg),
		uistate.T("about.aiKeySettings"),
	)
	return uiw.Card(uiw.CardProps{
		Title:  uistate.T("about.headingAI"),
		TestID: "about-ai-card",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyOwnKey")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyStorage")),
			P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyUsage")),
			P(css.Class("t-body", tw.TextDim), settingsLink),
		),
	})
}

// aboutVersionCard renders the current version and links to the changelog,
// license, and source repository.
func aboutVersionCard() ui.Node {
	return uiw.Card(uiw.CardProps{
		Title:  uistate.T("about.headingVersion"),
		TestID: "about-version-card",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			P(css.Class("t-body", tw.TextDim),
				uistate.T("about.versionLabel")+": "+version.Label(),
			),
			P(css.Class("t-caption", tw.Mt1),
				A(Attr("href", uistate.T("about.changelogHref")),
					Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
					css.Class(tw.Underline, tw.HoverTextFg),
					uistate.T("about.changelogLink"),
				),
			),
			P(css.Class("t-caption"),
				A(Attr("href", uistate.T("about.sourceHref")),
					Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
					css.Class(tw.Underline, tw.HoverTextFg),
					uistate.T("about.sourceLink"),
				),
			),
			P(css.Class("t-caption", tw.TextFaint),
				uistate.T("about.licenseNote")+" ",
				A(Attr("href", uistate.T("about.licenseHref")),
					Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
					css.Class(tw.Underline, tw.HoverTextFg),
					uistate.T("about.licenseLink"),
				),
			),
		),
	})
}
