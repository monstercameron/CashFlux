// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsImage(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/imageroute"
)

// agToolsImage adds the one entry point for a picture dropped into the chat
// (AG12).
//
// CashFlux already had three pipelines that could consume an image — propose a
// transaction, propose a split, file a document — and no way to choose between
// them without asking the user to classify their own photo first. That is
// backwards: the app is the one that just looked at it. This tool takes what the
// vision model read and says where it belongs and why, so the conversation can
// offer the right next step instead of a menu.
//
// It routes, it never files. Every destination lands on an existing preview the
// user approves, which is also why routing wrongly is cheap: the alternatives ride
// along, so a bad guess costs a click rather than a re-upload.
func agToolsImage(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = app
	_ = base
	_ = rates
	return []chatTool{
		{
			spec: ai.FunctionTool("route_image",
				"Decide what to do with a picture the user shared, from what you read in it. Call this "+
					"AFTER describing the image, passing what you found: the merchant, the total, how many "+
					"line items, and how many separate dated rows. It returns where the image belongs — a "+
					"transaction, a split, a statement review, or a document — and how sure that is. Do not "+
					"guess the destination yourself; pass what you saw and use the answer.",
				json.RawMessage(`{"type":"object","properties":{`+
					`"merchant":{"type":"string","description":"the payee, if the image names one"},`+
					`"total":{"type":"number","description":"the document total in major units, e.g. 42.10"},`+
					`"line_items":{"type":"integer","description":"how many purchased items are listed"},`+
					`"rows":{"type":"integer","description":"how many SEPARATE DATED transaction rows appear — a statement has many, a receipt has one"},`+
					`"text":{"type":"string","description":"the text you read, so an explicit request to split can be spotted"},`+
					`"unreadable":{"type":"boolean","description":"true when you could not make sense of the image"}`+
					`}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Merchant   string  `json:"merchant"`
					Total      float64 `json:"total"`
					LineItems  int     `json:"line_items"`
					Rows       int     `json:"rows"`
					Text       string  `json:"text"`
					Unreadable bool    `json:"unreadable"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read what you saw in the image."
				}
				route := imageroute.Decide(imageroute.Reading{
					Merchant: strings.TrimSpace(a.Merchant),
					// The model reports major units; the app works in minor ones.
					// math.Round for the same reason as the scenario tool: the
					// conversion truncates toward zero, so the +0.5 trick rounds
					// negatives the wrong way. Harmless here today (only the sign is
					// read) and a landmine the moment the figure is used.
					TotalMinor: int64(math.Round(a.Total * 100)),
					LineItems:  a.LineItems,
					Rows:       a.Rows,
					Text:       a.Text,
					Unreadable: a.Unreadable,
				})

				var b strings.Builder
				fmt.Fprintf(&b, "Destination: %s\n", route.Destination.Label())
				fmt.Fprintf(&b, "Why: %s\n", route.Why)
				if !route.Sure {
					b.WriteString("This one is a guess — say so, and lead with what you're unsure about.\n")
				}
				b.WriteString("\nIf that's not right, the other options are: ")
				alts := make([]string, 0, 3)
				for _, alt := range route.Destination.Alternatives() {
					alts = append(alts, alt.Label())
				}
				b.WriteString(strings.Join(alts, ", ") + ".\n")
				b.WriteString("\nOffer the destination as the next step and let them approve it. " +
					"Nothing is saved until they do.")
				return b.String()
			},
		},
	}
}
