// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestUXT6ReviewIntroUsesQuietCategoryCopy(t *testing.T) {
	f := render.New(t)
	f.Render(reviewIntro(247, 9))
	if got := f.Text(); !strings.Contains(got, "Review categories") || !strings.Contains(got, "247 charges from 9 merchants") {
		t.Fatalf("review intro text is missing its heading or count line: %q", got)
	}
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == "review-intro" && strings.Contains(node.Text(), "→") {
			t.Fatalf("review intro contains an arrow: %q", node.Text())
		}
	}
}
