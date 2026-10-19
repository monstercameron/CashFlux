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
	// All ids survive; likely chat/reasoning models precede other model types.
	want := []string{"gpt-5.4-mini", "gpt-5.5", "o4-mini", "chatgpt-4o-latest", "gpt-4o", "dall-e-3", "gpt-3.5-turbo-instruct", "gpt-4o-realtime-preview", "omni-moderation-latest", "text-embedding-3-large", "whisper-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ChatModelIDs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestSortAndGroupModelIDsPreservesEveryID(t *testing.T) {
	ids := []string{"whisper-1", "gpt-4o", "o4-mini", "gpt-5.5", "text-embedding-3-small", "custom-chat"}
	got := SortModelIDs(ids, "whisper-1")
	want := []string{"whisper-1", "gpt-5.5", "o4-mini", "gpt-4o", "custom-chat", "text-embedding-3-small"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortModelIDs = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(ids, []string{"whisper-1", "gpt-4o", "o4-mini", "gpt-5.5", "text-embedding-3-small", "custom-chat"}) {
		t.Fatal("SortModelIDs mutated its input")
	}
	groups := []ModelGroup{GroupModelID("gpt-4o"), GroupModelID("o4-mini"), GroupModelID("text-embedding-3-small"), GroupModelID("custom-chat")}
	wantGroups := []ModelGroup{ModelGroupChat, ModelGroupReasoning, ModelGroupOther, ModelGroupOther}
	if !reflect.DeepEqual(groups, wantGroups) {
		t.Fatalf("groups = %v, want %v", groups, wantGroups)
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

func TestErrorMessageExplainsNonChatModelRejection(t *testing.T) {
	got := ErrorMessage(400, []byte(`{"error":{"message":"This model is not supported in the chat completions endpoint"}}`))
	want := "This model can't be used for chat. Choose a chat-capable model in Settings."
	if got != want {
		t.Fatalf("ErrorMessage = %q, want %q", got, want)
	}
}
