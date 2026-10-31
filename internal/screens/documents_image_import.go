// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// imageImportCardProps carries the state and handlers the ImageImportCard needs
// from the parent Documents() component.
type imageImportCardProps struct {
	ImageURL   string
	AILoading  bool
	Disabled   bool
	FileLabel  string
	FileSize   int
	Disclosure string
	OnChoose   ui.Handler
	OnReadAI   ui.Handler
	OnDrop     ui.Handler
	OnDragOver ui.Handler
	OnRemove   func()
}

// ImageImportCard renders the receipt/statement image import inputs: choose image, the
// Smart+ "Read with AI" action, a preview, and the key-missing notice / errors. The
// title + description live in the shared importFormHeader, so this returns just the body.
func ImageImportCard(props imageImportCardProps) ui.Node {
	return Div(css.Class("doc-form-body"),
		Div(css.Class(importDropZone), Attr("data-testid", "import-image-dropzone"), OnDragOver(props.OnDragOver), OnDrop(props.OnDrop),
			Div(css.Class(importStack),
				Span(Style(map[string]string{"overflow-wrap": "anywhere"}), IfElse(props.FileLabel != "", Text(props.FileLabel), Text(uistate.T("importux.imageHint")))),
				If(props.ImageURL != "", Span(css.Class(importMuted), IfElse(props.FileSize > 0, Text(importdraft.FileSizeLabel(props.FileSize)), Text(uistate.T("importux.selectedImage"))))),
			),
			Button(
				css.Class("btn"), Attr("data-testid", "import-choose-image"), Disabled(props.Disabled || props.AILoading),
				Type("button"), OnClick(props.OnChoose), uistate.T("importux.chooseFile"),
			),
			If(props.ImageURL != "", Button(css.Class("btn btn-ghost"), Type("button"), Attr("data-testid", "import-remove-image"), OnClick(props.OnRemove), Disabled(props.Disabled || props.AILoading), uistate.T("importux.removeFile"))),
		),
		Div(css.Class(importActionRow),
			Button(
				css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Attr("data-testid", "import-read-image"),
				Type("button"), OnClick(props.OnReadAI), Disabled(props.ImageURL == "" || props.AILoading || props.Disabled),
				uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				IfElse(props.AILoading,
					Text(uistate.T("documents.reading")),
					Text(uistate.T("documents.readAI")),
				),
				KeyGateMark(AIKeyConfigured()),
			),
		),
		// Explain the outbound image transfer beside the action before the user can send it.
		importDisclosure(imageDisclosure(props.Disclosure)+" "+uistate.T("documents.imageCostNote")),
		// Image preview: shown once a file is chosen so the user can verify the scan (C60).
		If(props.ImageURL != "",
			Div(css.Class(tw.Flex, tw.Gap3, tw.ItemsStart),
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
	)
}

func imageDisclosure(disclosure string) string {
	if disclosure != "" {
		return disclosure
	}
	return uistate.T("impc.aiDisclosure")
}
