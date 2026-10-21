// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type loadedArtifactImage struct {
	app  *appstate.App
	id   string
	mime string
	size int
	hash string
	url  string
}

// useArtifactImageURL resolves a file from the encrypted local blob store after
// render. Call it at a stable position inside a component, never inside a loop.
// An inline legacy artifact remains visible while it awaits migration.
func useArtifactImageURL(app *appstate.App, art domain.Artifact) string {
	loaded := ui.UseState(loadedArtifactImage{})
	hash := ""
	if art.BlobRef != nil {
		hash = art.BlobRef.Hash
	}
	needsBlob := app != nil && art.ID != "" && art.Kind == artifacts.KindImage &&
		len(art.Bytes) == 0 && (art.Size > 0 || art.BlobRef != nil)
	ui.UseEffect(func() func() {
		if !needsBlob {
			return nil
		}
		cancelled := false
		go func() {
			data, err := app.GetBlobForArtifact(art.ID)
			if !cancelled && err == nil && len(data) > 0 && (art.Size == 0 || len(data) == art.Size) {
				loaded.Set(loadedArtifactImage{app: app, id: art.ID, mime: art.MIME,
					size: art.Size, hash: hash, url: artifacts.DataURL(art.MIME, data)})
			}
		}()
		return func() { cancelled = true }
	}, app, art.ID, art.MIME, art.Size, hash, needsBlob)
	if len(art.Bytes) > 0 {
		return artifacts.DataURL(art.MIME, art.Bytes)
	}
	value := loaded.Get()
	if value.app == app && value.id == art.ID && value.mime == art.MIME &&
		value.size == art.Size && value.hash == hash && needsBlob {
		return value.url
	}
	return ""
}
