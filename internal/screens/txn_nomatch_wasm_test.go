// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTxnNoMatchFuturePeriodShowsOneMessageAndScheduleLink(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnNoMatch, txnNoMatchProps{
		FuturePeriod: true, ScheduledCount: 1,
	}))

	if got := len(f.AllByTag("p")); got != 1 {
		t.Fatalf("future empty state rendered %d message paragraphs, want 1", got)
	}
	message := f.AllByTag("p")[0].Text()
	if !strings.Contains(message, "No transactions have been posted for this future period") ||
		!strings.Contains(message, "1 scheduled item") {
		t.Errorf("future empty-state message = %q, want one message mentioning the empty period and scheduled item", message)
	}
	links := f.AllByTag("a")
	if len(links) != 1 || links[0].Attr("href") != "/recurring" || links[0].Text() != "View Bills & recurring" {
		t.Errorf("future empty-state schedule link = %#v, want one link to /recurring", links)
	}
}
