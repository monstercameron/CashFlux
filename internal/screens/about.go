// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens contains all top-level routed screens. This file owns the
// /about screen (C290 / C293): a dedicated About & Privacy page with honest,
// plain-English disclosure of what CashFlux is, what data stays on device,
// what cloud sync sends, and how AI features use the user's own key —
// presented in the Understand-surface language (hero + chips + takeaway +
// serif sections) like the Data & People pages.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// AboutScreen is the dedicated /about route (C290 / C293). It covers:
//   - App identity and tagline (the hero).
//   - Privacy & local-first data-handling statement.
//   - Cloud-sync disclosure: what leaves the device and when (C291).
//   - AI-key disclosure: bring-your-own-key, stored locally, sent to OpenAI only on demand (C292).
//   - Version and changelog link.
func AboutScreen() ui.Node {
	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-about-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("about.eyebrow")),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("about.headingVersion")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), version.Label()),
			),
		),
		Div(css.Class("debt-chips"),
			rptChip(uistate.T("about.chipStorage"), uistate.T("about.chipStorageVal"), rptToneCls("pos")),
			rptChip(uistate.T("about.chipTracking"), uistate.T("about.chipTrackingVal"), ""),
			rptChip(uistate.T("about.chipCloud"), uistate.T("about.chipCloudVal"), ""),
		),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "about-takeaway"), uistate.T("about.tagline")),
	)

	prose := func(keys ...string) ui.Node {
		args := []any{css.Class("sys-prose")}
		for _, k := range keys {
			args = append(args, P(css.Class("t-body", tw.TextDim), uistate.T(k)))
		}
		return Div(args...)
	}

	return Div(css.Class("bento bento-sys"), Attr("id", "about-page"),
		rptTile("about-hero", "1 / span 4", rptSection("", uistate.T("about.headingIdentity"), nil, heroBody)),
		rptTile("about-privacy", "span 2",
			Div(Attr("data-testid", "about-privacy-card"),
				rptSection("sec-about-privacy", uistate.T("about.headingPrivacy"), nil,
					prose("about.privacyLocalFirst", "about.privacyExport", "about.privacyNoTracking")))),
		rptTile("about-cloud", "span 2",
			Div(Attr("data-testid", "about-cloudsync-card"),
				rptSection("sec-about-cloud", uistate.T("about.headingCloudSync"), nil,
					prose("about.cloudSyncOff", "about.cloudSyncOn", "about.cloudSyncControl")))),
		rptTile("about-ai", "span 2",
			Div(Attr("data-testid", "about-ai-card"),
				rptSection("sec-about-ai", uistate.T("about.headingAI"), nil, Div(css.Class("sys-prose"),
					P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyOwnKey")),
					P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyStorage")),
					P(css.Class("t-body", tw.TextDim), uistate.T("about.aiKeyUsage")),
					P(css.Class("t-body", tw.TextDim), Button(
						Type("button"),
						css.Class("btn-link", tw.Underline, tw.HoverTextFg),
						OnClick(Prevent(func() { uistate.OpenGlobalSettings() })),
						uistate.T("about.aiKeySettings"),
					)),
				)))),
		rptTile("about-version", "span 2",
			Div(Attr("data-testid", "about-version-card"),
				rptSection("sec-about-version", uistate.T("about.headingVersion"), nil, Div(css.Class("sys-prose"),
					P(css.Class("t-body", tw.TextDim), uistate.T("about.versionLabel")+": "+version.Label()),
					P(css.Class("t-caption", tw.Mt1),
						A(Attr("href", uistate.T("about.changelogHref")),
							Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
							css.Class(tw.Underline, tw.HoverTextFg),
							uistate.T("about.changelogLink"),
						)),
					P(css.Class("t-caption"),
						A(Attr("href", uistate.T("about.sourceHref")),
							Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
							css.Class(tw.Underline, tw.HoverTextFg),
							uistate.T("about.sourceLink"),
						)),
					P(css.Class("t-caption", tw.TextFaint),
						uistate.T("about.licenseNote")+" ",
						A(Attr("href", uistate.T("about.licenseHref")),
							Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
							css.Class(tw.Underline, tw.HoverTextFg),
							uistate.T("about.licenseLink"),
						)),
				)))),
	)
}
