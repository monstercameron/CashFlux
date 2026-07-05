// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// imageImportCardProps carries the state and handlers the ImageImportCard needs
// from the parent Documents() component.
type imageImportCardProps struct {
	ImageURL  string
	AILoading bool
	AIErr     string
	NeedsKey  bool
	OnChoose  ui.Handler
	OnReadAI  ui.Handler
	Nav       router.Navigator
}

// ImageImportCard renders the receipt/statement image import section: choose
// image, call AI vision, show preview, surface key-missing notice and errors.
func ImageImportCard(props imageImportCardProps) ui.Node {
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("documents.imageTitle"),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("documents.imageDesc")),
			// G14 §4: "Choose image" is primary until an image is selected; once
			// imageURL is set the user is ready to run AI — swap weights so the
			// affordance tracks the user's current action step.
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				Button(
					css.Class(chooseImageBtnClass(props.ImageURL)),
					Type("button"), OnClick(props.OnChoose), uistate.T("documents.chooseImage"),
				),
				Button(
					css.Class(readAIBtnClass(props.ImageURL), tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
					Type("button"), OnClick(props.OnReadAI), Disabled(props.AILoading),
					uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					IfElse(props.AILoading,
						Text(uistate.T("documents.reading")),
						Text(uistate.T("documents.readAI")),
					),
				),
			),
			// C99: muted cost note so BYOK users understand each vision call has a
			// small credit cost before they click "Read with AI".
			P(css.Class("muted", tw.Text12, tw.Mt1), uistate.T("documents.imageCostNote")),
			// Image preview: shown once a file is chosen so the user can verify the
			// scan result against the original receipt (C60).
			If(props.ImageURL != "",
				Div(css.Class(tw.Mt2, tw.Flex, tw.Gap3, tw.ItemsStart),
					Img(
						Attr("src", props.ImageURL),
						Attr("alt", uistate.T("documents.imagePreviewAlt")),
						Attr("data-testid", "doc-image-preview"),
						css.Class(tw.MaxWFull, tw.ObjectContain, tw.Rounded, tw.BorderLine70),
						Style(map[string]string{
							"border-width": "1px",
							"border-style": "solid",
							"max-width":    "200px",
							"max-height":   "160px",
						}),
					),
				),
			),
			If(props.NeedsKey,
				Div(css.Class("notice notice-warn", tw.Mt1, tw.Flex, tw.ItemsCenter, tw.Gap2),
					Span(uistate.T("documents.needKey")),
					Button(css.Class("btn btn-sm"), Type("button"),
						OnClick(func() { uistate.OpenGlobalSettingsAt("ai") }),
						uistate.T("documents.goToSettings"),
					),
				),
			),
			If(props.AIErr != "", P(css.Class("err"), Attr("role", "alert"), props.AIErr)),
		),
	})
}

// chooseImageBtnClass returns the button class for "Choose image": primary
// when no image is chosen yet (first step), secondary after one is loaded.
func chooseImageBtnClass(imageURL string) string {
	if imageURL == "" {
		return "btn btn-primary"
	}
	return "btn"
}

// readAIBtnClass returns the button class for "Read with AI": secondary until
// an image is loaded, then primary (user's next logical step).
func readAIBtnClass(imageURL string) string {
	if imageURL != "" {
		return "btn btn-primary"
	}
	return "btn"
}
