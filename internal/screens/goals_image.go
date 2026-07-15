// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goalImageBanner renders a goal's vision image (GL6) as a small banner atop its
// card. It resolves the artifact bytes the same way receipt thumbnails do (TX5),
// producing a data URL. When the goal has no image it renders nothing; when the
// image is referenced but its blob is missing it shows a calm placeholder rather
// than a broken image. object-cover + rounded corners keep it tidy at any size.
func goalImageBanner(app *appstate.App, g domain.Goal) ui.Node {
	if g.GoalImageArtifactID == "" {
		return Fragment()
	}
	var dataURL string
	if app != nil {
		for _, a := range app.Artifacts() {
			if a.ID == g.GoalImageArtifactID && len(a.Bytes) > 0 {
				dataURL = artifacts.DataURL(a.MIME, a.Bytes)
				break
			}
		}
	}
	alt := uistate.T("goals.photoAlt", g.Name)
	if dataURL == "" {
		// Referenced but unavailable: a quiet placeholder, never a broken <img>.
		return Div(css.Class("goal-card-photo", "is-missing"), Attr("data-testid", "goal-photo-"+g.ID),
			Attr("role", "img"), Attr("aria-label", uistate.T("goals.photoMissing")),
			Span(css.Class("muted"), uistate.T("goals.photoMissing")))
	}
	return Div(css.Class("goal-card-photo"), Attr("data-testid", "goal-photo-"+g.ID),
		Img(Attr("src", dataURL), Attr("alt", alt), Attr("loading", "lazy"),
			Attr("style", "width:100%;height:88px;object-fit:cover;border-radius:8px;display:block;")))
}

// attachGoalImage opens the shared image picker, stores the chosen file as an
// Artifact (mirroring the receipt attach flow, TX5/L29), points the goal at it
// via GoalImageArtifactID, and persists. It operates on the stored goal so it
// survives independently of unsaved edits in the form. Bumps the data revision
// so the card's banner updates immediately.
func attachGoalImage(goalID string, onErr func(string)) {
	app := appstate.Default
	if app == nil {
		return
	}
	pickFile("image/*", func(name, mime string, data []byte) {
		art := domain.Artifact{ID: id.New(), Name: name, Kind: "image", MIME: mime, Bytes: data, Size: len(data), CreatedAt: time.Now()}
		if err := app.PutArtifact(art); err != nil {
			if onErr != nil {
				onErr(err.Error())
			}
			return
		}
		for _, g := range app.Goals() {
			if g.ID != goalID {
				continue
			}
			g.GoalImageArtifactID = art.ID
			if err := app.PutGoal(g); err != nil {
				if onErr != nil {
					onErr(err.Error())
				}
				return
			}
			break
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("goals.photoAdded"), false)
	})
}

// removeGoalImage clears a goal's vision-image reference and persists. The
// underlying artifact is left to the blob GC (internal/artifactref no longer
// counts it once unreferenced), matching how detaching a receipt behaves.
func removeGoalImage(goalID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, g := range app.Goals() {
		if g.ID != goalID {
			continue
		}
		g.GoalImageArtifactID = ""
		if err := app.PutGoal(g); err == nil {
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("goals.photoRemoved"), false)
		}
		return
	}
}
