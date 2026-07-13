// SPDX-License-Identifier: MIT

package ai

import (
	"reflect"
	"testing"
)

func TestChatModelIDs(t *testing.T) {
	body := []byte(`{"object":"list","data":[
		{"id":"gpt-5.5","object":"model"},
		{"id":"gpt-5.4-mini","object":"model"},
		{"id":"o4-mini","object":"model"},
		{"id":"text-embedding-3-large","object":"model"},
		{"id":"whisper-1","object":"model"},
		{"id":"gpt-4o-realtime-preview","object":"model"},
		{"id":"dall-e-3","object":"model"},
		{"id":"omni-moderation-latest","object":"model"},
		{"id":"gpt-4o","object":"model"},
		{"id":"gpt-3.5-turbo-instruct","object":"model"},
		{"id":"chatgpt-4o-latest","object":"model"},
		{"id":"gpt-5.5","object":"model"}
	]}`)
	got, err := ChatModelIDs(body)
	if err != nil {
		t.Fatalf("ChatModelIDs error: %v", err)
	}
	// Chat-capable, de-duplicated, sorted descending. Excludes embeddings, whisper,
	// realtime, dall-e, moderation, and -instruct completions models.
	want := []string{"o4-mini", "gpt-5.5", "gpt-5.4-mini", "gpt-4o", "chatgpt-4o-latest"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ChatModelIDs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestChatModelIDsBadBody(t *testing.T) {
	if _, err := ChatModelIDs([]byte("not json")); err == nil {
		t.Error("expected an error for a non-JSON body")
	}
	// A well-formed but empty list yields no models and no error.
	got, err := ChatModelIDs([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no models, got %v", got)
	}
}

func TestIsChatModelID(t *testing.T) {
	chat := []string{"gpt-5.5", "gpt-4o", "o4-mini", "o1-preview", "chatgpt-4o-latest"}
	for _, id := range chat {
		if !isChatModelID(id) {
			t.Errorf("isChatModelID(%q) = false, want true", id)
		}
	}
	notChat := []string{"", "text-embedding-3-small", "whisper-1", "tts-1", "dall-e-3",
		"gpt-4o-audio-preview", "gpt-3.5-turbo-instruct", "omni-moderation-latest", "davinci-002"}
	for _, id := range notChat {
		if isChatModelID(id) {
			t.Errorf("isChatModelID(%q) = true, want false", id)
		}
	}
}
