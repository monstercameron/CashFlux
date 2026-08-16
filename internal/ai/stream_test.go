// SPDX-License-Identifier: MIT

package ai

import (
	"strings"
	"testing"
)

// feedAll runs a whole stream through the decoder in one go.
func feedAll(t *testing.T, s string) []StreamEvent {
	t.Helper()
	var d StreamDecoder
	return d.Feed([]byte(s))
}

func TestDecoderYieldsTextDeltasInOrder(t *testing.T) {
	stream := "event: response.output_text.delta\n" +
		`data: {"type":"response.output_text.delta","delta":"You spent "}` + "\n\n" +
		`data: {"type":"response.output_text.delta","delta":"$312."}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 2 {
		t.Fatalf("events = %d: %+v", len(got), got)
	}
	if got[0].TextDelta != "You spent " || got[1].TextDelta != "$312." {
		t.Fatalf("deltas = %q, %q", got[0].TextDelta, got[1].TextDelta)
	}
	if !got[0].IsDelta() || got[0].IsFinal() {
		t.Fatalf("a delta event classified wrongly: %+v", got[0])
	}
}

func TestAnEventSplitAcrossChunksIsHeldUntilComplete(t *testing.T) {
	// This is the case that matters: network chunks do not respect message
	// boundaries, and a decoder that parses whatever arrived loses text.
	var d StreamDecoder
	if got := d.Feed([]byte(`data: {"type":"response.output_text.`)); len(got) != 0 {
		t.Fatalf("emitted %d events from half a payload", len(got))
	}
	if got := d.Feed([]byte(`delta","delta":"hello"}` + "\n")); len(got) != 0 {
		t.Fatalf("emitted before the blank line that ends the event: %+v", got)
	}
	got := d.Feed([]byte("\n"))
	if len(got) != 1 || got[0].TextDelta != "hello" {
		t.Fatalf("events = %+v", got)
	}
}

func TestASingleJSONPayloadSplitMidStringSurvives(t *testing.T) {
	var d StreamDecoder
	var text strings.Builder
	whole := `data: {"type":"response.output_text.delta","delta":"a long answer with, commas"}` + "\n\n"
	// One byte at a time — the most hostile chunking there is.
	for i := 0; i < len(whole); i++ {
		for _, ev := range d.Feed([]byte{whole[i]}) {
			text.WriteString(ev.TextDelta)
		}
	}
	if text.String() != "a long answer with, commas" {
		t.Fatalf("text = %q", text.String())
	}
}

func TestCRLFFramingIsUnderstood(t *testing.T) {
	stream := "event: x\r\n" + `data: {"type":"response.output_text.delta","delta":"hi"}` + "\r\n\r\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].TextDelta != "hi" {
		t.Fatalf("events = %+v", got)
	}
}

func TestLeadingSpacesInADeltaAreNotEaten(t *testing.T) {
	// SSE strips ONE space after "data:", no more. A delta of "   indented" must
	// survive with its indentation, or streamed Markdown code blocks break.
	stream := `data: {"type":"response.output_text.delta","delta":"   indented"}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].TextDelta != "   indented" {
		t.Fatalf("delta = %q", got[0].TextDelta)
	}
}

func TestMultiLineDataFieldsAreJoined(t *testing.T) {
	stream := "data: {\"type\":\"response.output_text.delta\",\n" +
		"data: \"delta\":\"joined\"}\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].TextDelta != "joined" {
		t.Fatalf("events = %+v", got)
	}
}

func TestCommentsAndKeepalivesAreIgnored(t *testing.T) {
	stream := ": keepalive\n\n" +
		`data: {"type":"response.output_text.delta","delta":"x"}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].TextDelta != "x" {
		t.Fatalf("events = %+v", got)
	}
}

func TestCompletedEventCarriesTheWholeResponse(t *testing.T) {
	stream := `data: {"type":"response.completed","response":{"status":"completed",` +
		`"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"final"}]}],` +
		`"usage":{"input_tokens":10,"output_tokens":3}}}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || !got[0].IsFinal() {
		t.Fatalf("events = %+v", got)
	}
	// The completed payload feeds the SAME parser the non-streaming path uses, so
	// the authoritative answer never comes from reassembled deltas.
	msg, usage, err := ParseResponsesChat(got[0].Response)
	if err != nil {
		t.Fatalf("ParseResponsesChat: %v", err)
	}
	if msg.Content != "final" || usage.TotalTokens != 13 {
		t.Fatalf("msg = %+v usage = %+v", msg, usage)
	}
}

func TestToolCallArgumentDeltasAreNotShownAsText(t *testing.T) {
	// Half a JSON argument on screen is noise; the completed event carries the
	// whole call.
	stream := `data: {"type":"response.function_call_arguments.delta","delta":"{\"mon"}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 {
		t.Fatalf("events = %+v", got)
	}
	if got[0].TextDelta != "" {
		t.Fatalf("an argument fragment leaked into the answer text: %q", got[0].TextDelta)
	}
}

func TestAnErrorEventCarriesItsMessage(t *testing.T) {
	stream := `data: {"type":"error","error":{"message":"the key was rejected"}}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].Err != "the key was rejected" {
		t.Fatalf("events = %+v", got)
	}
}

func TestAFailedResponseExplainsItself(t *testing.T) {
	stream := `data: {"type":"response.failed","response":{"error":{"message":"ran out of room"}}}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 1 || got[0].Err != "ran out of room" {
		t.Fatalf("events = %+v", got)
	}
	// A failed response with no reason still says something usable.
	bare := feedAll(t, `data: {"type":"response.failed","response":{}}`+"\n\n")
	if len(bare) != 1 || bare[0].Err == "" {
		t.Fatalf("a bare failure produced no message: %+v", bare)
	}
}

func TestTheTerminatorStopsTheDecoder(t *testing.T) {
	var d StreamDecoder
	got := d.Feed([]byte("data: [DONE]\n\n" + `data: {"type":"response.output_text.delta","delta":"late"}` + "\n\n"))
	if len(got) != 0 {
		t.Fatalf("events after the terminator = %+v", got)
	}
	if more := d.Feed([]byte(`data: {"type":"response.output_text.delta","delta":"later"}` + "\n\n")); len(more) != 0 {
		t.Fatalf("decoder kept going after [DONE]: %+v", more)
	}
}

func TestMalformedPayloadDoesNotKillTheStream(t *testing.T) {
	stream := "data: not json at all\n\n" +
		`data: {"type":"response.output_text.delta","delta":"still here"}` + "\n\n"
	got := feedAll(t, stream)
	if len(got) != 2 {
		t.Fatalf("events = %d: %+v", len(got), got)
	}
	if got[0].TextDelta != "" || got[0].Err != "" {
		t.Fatalf("a malformed payload produced content: %+v", got[0])
	}
	if got[1].TextDelta != "still here" {
		t.Fatalf("the stream did not recover: %+v", got[1])
	}
}

func TestEmptyDataBlocksAreSkipped(t *testing.T) {
	got := feedAll(t, "data:\n\ndata:  \n\n")
	if len(got) != 0 {
		t.Fatalf("events = %+v", got)
	}
}
