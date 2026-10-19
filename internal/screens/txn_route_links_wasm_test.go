// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTransactionsPageDestinationsRenderAsRouterAnchors(t *testing.T) {
	tests := []struct {
		name string
		node func() ui.Node
		id   string
		href string
	}{
		{name: "follow-up list", node: func() ui.Node { return txnFollowUpTodoLink("txn-1") }, id: "txn-followup-pop-link-txn-1", href: "/todo"},
		{name: "upcoming schedule", node: func() ui.Node { return txnUpcomingScheduleLink() }, id: "txn-upcoming-schedule", href: "/recurring"},
		{name: "AI settings", node: func() ui.Node { return smartCatConnectLink(nil) }, id: "smartcat-connect", href: "/settings/ai"},
		{name: "review categories", node: func() ui.Node {
			return reviewDestinationLink("review-goto-categories", "/categories", "Categories", "Leave Review: Categories", nil, nil)
		}, id: "review-goto-categories", href: "/categories"},
		{name: "review rules", node: func() ui.Node {
			return reviewDestinationLink("review-goto-rules", "/rules", "Rules", "Leave Review: Rules", nil, nil)
		}, id: "review-goto-rules", href: "/rules"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(tt.node())
			links := f.AllByTag("a")
			if len(links) != 1 {
				t.Fatalf("rendered %d anchors, want one", len(links))
			}
			if got := links[0].Attr("data-testid"); got != tt.id {
				t.Errorf("data-testid = %q, want %q", got, tt.id)
			}
			if got := links[0].Attr("href"); got != tt.href {
				t.Errorf("href = %q, want %q", got, tt.href)
			}
		})
	}
}
