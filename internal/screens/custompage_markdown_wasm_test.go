// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCustomPageTextWidgetRegistersMarkdownEngine(t *testing.T) {
	f := render.New(t)
	defer func() { f.Render(ui.Fragment()); f.Stabilize(); f.Cleanup() }()
	f.Render(widgetBody(domain.PageWidget{
		Type:   widgetspec.TypeText,
		Config: map[string]string{"text": "# Household plan\n\n- **Save** each month\n- [Details](https://example.com/plan)"},
	}, pageCtx{}))
	f.Stabilize()
	headings := f.AllByTag("h1")
	links := f.AllByTag("a")
	if len(headings) != 1 || headings[0].Text() != "Household plan" || len(links) != 1 ||
		links[0].Attr("href") != "https://example.com/plan" || len(f.AllByTag("strong")) != 1 || len(f.AllByTag("li")) != 2 {
		t.Fatalf("text widget did not render Markdown structure: %s", f.Text())
	}
	if strings.Contains(f.Text(), "**Save**") || strings.Contains(f.Text(), "# Household plan") {
		t.Fatalf("Markdown source was rendered without its engine: %s", f.Text())
	}
}
