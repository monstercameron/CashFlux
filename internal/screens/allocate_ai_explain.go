//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// aiExplainCardProps carries the AI-explain state and callbacks.
type aiExplainCardProps struct {
	HasRanked      bool
	AiResult       string
	AiLoading      bool
	AiErr          string
	NeedKeyMsg     string // the i18n value of "allocate.needKey", for comparison
	OnExplain      any
	OnGoToSettings any
}

// AiExplainCard renders the "Why this ranking?" AI explanation card. It is a
// pure rendering function — all hooks live in Allocate().
func AiExplainCard(p aiExplainCardProps) ui.Node {
	if !p.HasRanked {
		return Fragment()
	}
	return Section(css.Class("card"),
		H2(css.Class("card-title"), uistate.T("allocate.whyTitle")),
		Button(css.Class("btn"), Type("button"), OnClick(p.OnExplain), IfElse(p.AiLoading, Text(uistate.T("allocate.thinking")), Text(uistate.T("allocate.explainAI")))),
		If(p.AiErr != "", Div(css.Class("err"), Attr("role", "alert"),
			Text(p.AiErr),
			If(p.AiErr == p.NeedKeyMsg,
				Button(css.Class("btn"), Type("button"),
					Attr("aria-label", "Open Settings to add your AI key"),
					Style(map[string]string{"margin-left": "0.5rem"}),
					OnClick(p.OnGoToSettings),
					"Open Settings",
				),
			),
		)),
		If(p.AiResult != "", P(css.Class("muted"), p.AiResult)),
	)
}
