// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsTrust(app, base, rates)...) in buildChatTools

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/agentmemory"
	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// agToolsTrust adds the transparent-memory tools (AG19): the agent can propose
// remembering a durable fact the user states ("paid biweekly", "don't suggest
// cutting eating out") and can read back what it already remembers. Capture is
// never silent — remember_fact is a mutating tool, so it runs only after the user
// approves the preview, and every stored fact is visible and editable in Settings.
//
// base and rates are unused here (the memory tools carry no money) but kept in the
// signature so registration matches the other agTools* groups.
func agToolsTrust(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = base
	_ = rates
	boundStore := app.Store()
	activeDataset := func() bool { return appstate.Default == app && app.Store() == boundStore }
	var previewState trustMemoryPreview
	return []chatTool{
		{
			spec: ai.FunctionTool("remember_fact",
				"Save a durable fact the user tells you to remember for future conversations (e.g. \"I'm paid biweekly\", \"don't suggest cutting eating out\"). Use this ONLY when the user asks you to remember something or clearly states a standing preference — the user approves before it's stored, and every remembered fact is visible and editable in Settings. Do not store one-off values, secrets, or anything the user didn't mean to keep.",
				json.RawMessage(`{"type":"object","properties":{"fact":{"type":"string","description":"the fact to remember, in a short first-person or descriptive sentence"}},"required":["fact"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				previewState = trustMemoryPreview{}
				fact, err := parseRememberFactArgs(raw)
				if err != nil {
					return "Can't prepare memory change: " + err.Error()
				}
				if !activeDataset() {
					return "Can't prepare memory change: the active dataset changed."
				}
				memory, rawMemory, exists, err := readTrustMemory(boundStore)
				if err != nil {
					return "Can't prepare memory change: " + err.Error()
				}
				previewState = trustMemoryPreview{fact: fact, raw: rawMemory, exists: exists, ready: true}
				next, added := memory.Add(fact)
				if !added {
					return "Already remembered something equivalent to “" + fact + "” — nothing to add."
				}
				return "Remember: “" + next.Facts[len(next.Facts)-1] + "”"
			},
			run: func(raw json.RawMessage) string {
				fact, err := parseRememberFactArgs(raw)
				if err != nil {
					return "Can't remember fact: " + err.Error()
				}
				if !previewState.ready || previewState.fact != fact {
					return "Can't remember fact: the approved input does not match its preview. Ask the user to try again."
				}
				approvedPreview := previewState
				previewState.ready = false
				if !activeDataset() {
					return "Can't remember fact: the active dataset changed. Ask the user to try again."
				}
				memory, rawMemory, exists, err := readTrustMemory(boundStore)
				if err != nil {
					return "Can't remember fact: " + err.Error()
				}
				if rawMemory != approvedPreview.raw || exists != approvedPreview.exists {
					return "Can't remember fact: transparent memory changed after the preview. Ask the user to review it again."
				}
				next, added := memory.Add(fact)
				if !added {
					return "Already remembered something equivalent to “" + fact + "” — nothing to add."
				}
				if !activeDataset() {
					return "Can't remember fact: the active dataset changed. Ask the user to try again."
				}
				if err := uistate.WriteAgentMemoryToStore(boundStore, next); err != nil {
					return "Can't remember fact: " + err.Error()
				}
				uistate.RequestPersist()
				return "Remembered: “" + next.Facts[len(next.Facts)-1] + "”. You can view or edit this anytime in Settings → AI."
			},
		},
		{
			spec: ai.FunctionTool("list_remembered_facts",
				"List the durable facts the user has asked you to remember (the agent's transparent memory). Call this to check what standing preferences or facts already apply before you propose remembering a new one or give advice.",
				json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(raw json.RawMessage) string {
				if err := validateEmptyToolArgs(raw); err != nil {
					return "Can't read remembered facts: " + err.Error()
				}
				if !activeDataset() {
					return "Can't read remembered facts: the active dataset changed."
				}
				mem, _, _, err := readTrustMemory(boundStore)
				if err != nil {
					return "Can't read remembered facts: " + err.Error()
				}
				if mem.Len() == 0 {
					return "No remembered facts yet."
				}
				var b strings.Builder
				b.WriteString("Remembered facts:\n")
				for _, f := range mem.Facts {
					b.WriteString("- " + f + "\n")
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
	}
}

type trustMemoryPreview struct {
	fact   string
	raw    string
	exists bool
	ready  bool
}

func parseRememberFactArgs(raw json.RawMessage) (string, error) {
	var args struct {
		Fact string `json:"fact"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", fmt.Errorf("arguments must contain one JSON object")
	}
	if strings.TrimSpace(args.Fact) == "" {
		return "", fmt.Errorf("give a fact to remember")
	}
	normalized, added := (agentmemory.Store{}).Add(args.Fact)
	if !added {
		return "", fmt.Errorf("give a fact to remember")
	}
	return normalized.Facts[0], nil
}

func validateEmptyToolArgs(raw json.RawMessage) error {
	var args map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("arguments must contain one JSON object")
	}
	if args == nil || len(args) != 0 {
		return fmt.Errorf("this tool takes no arguments")
	}
	return nil
}

func readTrustMemory(s *store.SQLiteStore) (agentmemory.Store, string, bool, error) {
	memory, raw, exists, err := uistate.ReadAgentMemoryFromStore(s)
	if err != nil {
		return agentmemory.Store{}, "", false, fmt.Errorf("storage read failed: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return memory, raw, exists, nil
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &shape); err != nil {
		return agentmemory.Store{}, "", false, fmt.Errorf("stored transparent memory is malformed")
	}
	if len(shape) > 0 {
		facts, ok := shape["facts"]
		if !ok {
			return agentmemory.Store{}, "", false, fmt.Errorf("stored transparent memory has no facts list")
		}
		var decoded []string
		if err := json.Unmarshal(facts, &decoded); err != nil {
			return agentmemory.Store{}, "", false, fmt.Errorf("stored transparent memory facts are malformed")
		}
	}
	return memory, raw, exists, nil
}

// assistantWriteResult reports the tool output and the exact captured change, if
// the approved call produced a nonempty filtered undo point.
type assistantWriteResult struct {
	Text              string
	Changed           bool
	UndoID            string
	VerificationError string
}

// runAssistantWriteResult runs one approved mutating tool under the assistant's
// identity and returns the receipt for its exact captured change (C389 / AG20).
//
// The changeset path (AG1) already did this, but a tool the model called directly
// did not: the write landed in the audit log attributed to the household, and the
// Activity screen could not tell a change the assistant made from one somebody
// typed. That is precisely the attribution the trust story rests on, so it is done
// here for every approved write, whichever path proposed it.
//
// The actor is cleared on the way out even if the tool panics, because a leaked
// assistant tag would mis-attribute every subsequent hand-made edit in the session
// — a worse failure than the one being fixed.
func runAssistantWriteResult(tool string, run func() string) (result assistantWriteResult) {
	previousActor := auditview.SessionActor()
	auditview.SetSessionActor("")
	if err := auditview.CaptureAssistantBaselineChecked(); err != nil {
		auditview.SetSessionActor(previousActor)
		return assistantWriteResult{Text: uistate.T("assistant.writeBaselineUnavailable", err.Error()), VerificationError: err.Error()}
	}
	auditview.SetSessionActor(auditview.ActorAssistant)
	// A tool runs with arguments the MODEL wrote — unvalidated JSON shaped by
	// something outside this program. An unrecovered panic in a Go/wasm goroutine
	// takes down the whole page, not just the chat, so one malformed argument
	// reaching one nil map would black out the entire app and cost the user
	// everything unsaved. The tool's failure is reported to the model as a tool
	// result instead, which is a thing the conversation already knows how to
	// handle.
	defer func() {
		if r := recover(); r != nil {
			result.Text = fmt.Sprintf("The %s tool failed: %v", tool, r)
		}
		// Capture real partial writes even after a panic. Empty/filtered diffs
		// have no receipt and cannot create a phantom Assistant action.
		var captureErr error
		result.UndoID, result.Changed, captureErr = auditview.CaptureAssistantReceiptChecked()
		if captureErr == nil && result.Changed && result.UndoID == "" {
			captureErr = fmt.Errorf("capture reported a change without an undo receipt")
		}
		if captureErr == nil && !result.Changed && result.UndoID != "" {
			captureErr = fmt.Errorf("capture returned an undo receipt for an unchanged operation")
		}
		if captureErr != nil {
			result.UndoID = ""
			result.Changed = false
			result.VerificationError = captureErr.Error()
			if result.Text != "" {
				result.Text += "\n"
			}
			result.Text += "A change may have been made, but undo verification is unavailable. Check Activity before retrying."
		}
		if result.Changed {
			uistate.NoteAssistantAction(tool, result.UndoID)
		}
		auditview.SetSessionActor(previousActor)
	}()
	result.Text = run()
	return result
}

// runAssistantWrite preserves the original string result for older callers. New
// callers use runAssistantWriteResult so tallies can honor its Changed field.
func runAssistantWrite(tool string, run func() string) string {
	return runAssistantWriteResult(tool, run).Text
}

// runReadTool runs one read-only tool with the same panic guard as a write, minus
// the audit and undo bookkeeping a read does not need.
//
// Read tools take model-written arguments too, so they crash the page just as
// readily as a write does — the guard belongs on both paths, not only the one that
// happens to change something.
func runReadTool(tool string, run func() string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("The %s tool failed: %v", tool, r)
		}
	}()
	return run()
}
