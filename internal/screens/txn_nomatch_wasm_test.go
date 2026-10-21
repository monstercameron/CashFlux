// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
	if message != "No posted transactions. Scheduled items are listed below." {
		t.Errorf("future empty-state message = %q, want the short line above the scheduled rows", message)
	}
	links := f.AllByTag("a")
	if len(links) != 1 || links[0].Attr("href") != "/recurring" || links[0].Text() != "View bills and repeating payments" {
		t.Errorf("future empty-state schedule link = %#v, want one link to /recurring", links)
	}
}
