// SPDX-License-Identifier: MIT

// Server-sent-event decoding for the Responses API's streaming mode. This file has
// no build tags so it compiles on native Go and the framing rules are unit-tested
// without a browser — the byte-level work (events split across network chunks,
// multi-line payloads, CRLF, the terminator) is exactly where a hand-rolled reader
// goes wrong, and exactly what a browser test would not pin down.
package ai

import (
	"bytes"
	"encoding/json"
	"strings"
)

// StreamEvent is one decoded event from a Responses stream.
type StreamEvent struct {
	// Type is the event's own type field ("response.output_text.delta",
	// "response.completed", …). Empty for a payload that carried none.
	Type string
	// TextDelta is the next fragment of the answer, on a delta event.
	TextDelta string
	// Response is the complete response object, on a terminal event. It is the
	// same shape ParseResponsesChat reads, so the authoritative message, tool
	// calls and usage come from one parser rather than from reassembled deltas.
	Response json.RawMessage
	// Err is a plain-English failure carried by an error event.
	Err string
}

// IsDelta reports whether the event carries more of the answer's text.
func (e StreamEvent) IsDelta() bool { return e.TextDelta != "" }

// IsFinal reports whether the event carries the completed response.
func (e StreamEvent) IsFinal() bool { return len(e.Response) > 0 }

// StreamDecoder turns a byte stream into events. It is stateful because network
// chunks do not respect message boundaries: a single `data:` line can arrive in
// three pieces, and an event's payload can be split mid-JSON. Feed it whatever
// arrives and it yields only whole events.
//
// The zero value is ready to use.
type StreamDecoder struct {
	buf []byte
	// done is set once the stream's terminator has been seen, so anything after
	// it is ignored rather than parsed as a truncated event.
	done bool
}

// errStreamUnsupported is reported when the browser gives no readable body for a
// streamed response, so the caller can fall back to the whole-answer path rather
// than showing nothing.
const errStreamUnsupported = "This browser can't stream the answer."

// streamDone is the sentinel payload that ends an SSE stream.
const streamDone = "[DONE]"

// Feed appends a chunk and returns every event that is now complete. A chunk that
// completes no event returns nothing, which is the normal case for most chunks.
func (d *StreamDecoder) Feed(chunk []byte) []StreamEvent {
	if d.done {
		return nil
	}
	d.buf = append(d.buf, chunk...)
	var out []StreamEvent
	for {
		// Events are separated by a blank line. Both line endings appear in the
		// wild, and a stream can mix them, so both are looked for and the earlier
		// one wins.
		idx, sepLen := nextBlankLine(d.buf)
		if idx < 0 {
			return out
		}
		raw := d.buf[:idx]
		d.buf = d.buf[idx+sepLen:]
		ev, ok, finished := decodeSSEBlock(raw)
		if finished {
			d.done = true
			return out
		}
		if ok {
			out = append(out, ev)
		}
	}
}

// nextBlankLine finds the end of the first complete event block and the length of
// the separator, or -1 when no complete block is buffered yet.
func nextBlankLine(buf []byte) (idx, sepLen int) {
	lf := bytes.Index(buf, []byte("\n\n"))
	crlf := bytes.Index(buf, []byte("\r\n\r\n"))
	switch {
	case lf < 0 && crlf < 0:
		return -1, 0
	case crlf < 0 || (lf >= 0 && lf < crlf):
		return lf, 2
	default:
		return crlf, 4
	}
}

// decodeSSEBlock reads one event block into an event. finished is true when the
// block is the stream's terminator.
func decodeSSEBlock(raw []byte) (ev StreamEvent, ok, finished bool) {
	var data strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		// Comment/keepalive lines start with ':' and carry nothing.
		if strings.HasPrefix(line, ":") {
			continue
		}
		payload, isData := strings.CutPrefix(line, "data:")
		if !isData {
			// `event:` and `id:` lines are ignored: the payload carries its own
			// type field, and trusting one source rather than two removes a way
			// for them to disagree.
			continue
		}
		// An SSE field's value has at most one leading space stripped — stripping
		// all whitespace would corrupt a text delta that legitimately begins with
		// several spaces.
		data.WriteString(strings.TrimPrefix(payload, " "))
	}
	body := data.String()
	if strings.TrimSpace(body) == "" {
		return StreamEvent{}, false, false
	}
	if strings.TrimSpace(body) == streamDone {
		return StreamEvent{}, false, true
	}
	return decodeStreamPayload([]byte(body)), true, false
}

// streamPayload is the part of a Responses stream event this app reads.
type streamPayload struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta"`
	Response json.RawMessage `json:"response"`
	Error    *APIError       `json:"error"`
}

// decodeStreamPayload reads one event's JSON. A payload that will not parse yields
// an event with only its raw text discarded rather than an error: a single
// malformed keepalive should not fail an answer that is otherwise arriving fine.
func decodeStreamPayload(body []byte) StreamEvent {
	var p streamPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return StreamEvent{}
	}
	ev := StreamEvent{Type: p.Type}
	if p.Error != nil {
		ev.Err = p.Error.Message
		return ev
	}
	switch {
	case strings.HasSuffix(p.Type, ".delta") && p.Delta != "":
		// Only TEXT deltas are surfaced. Argument deltas for a tool call stream
		// under their own type and are deliberately ignored: half a JSON argument
		// on screen is noise, and the completed event carries the whole call.
		if strings.Contains(p.Type, "output_text") {
			ev.TextDelta = p.Delta
		}
	case p.Type == "response.completed" || p.Type == "response.incomplete":
		ev.Response = p.Response
	case p.Type == "response.failed":
		ev.Response = p.Response
		ev.Err = failedResponseMessage(p.Response)
	}
	return ev
}

// failedResponseMessage digs the reason out of a failed response object, falling
// back to a plain sentence when it carries none.
func failedResponseMessage(raw json.RawMessage) string {
	var r struct {
		Error *APIError `json:"error"`
	}
	if len(raw) > 0 && json.Unmarshal(raw, &r) == nil && r.Error != nil && strings.TrimSpace(r.Error.Message) != "" {
		return r.Error.Message
	}
	return "OpenAI stopped part-way through the answer. Try asking again."
}
