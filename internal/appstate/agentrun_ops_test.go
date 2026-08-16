// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"
	"time"
)

// withScheduledAgentApp builds an app with an AI key configured, so scheduled
// questions are allowed to run.
func withScheduledAgentApp(t *testing.T, key string) *App {
	t.Helper()
	a := newTestAppAt(t, time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC))
	s := a.Settings()
	s.OpenAIKey = key
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	return a
}

func TestAScheduledQuestionBecomesAConversationBeforeItIsAnswered(t *testing.T) {
	// The conversation is created up front so the run is visible even if the
	// answer never arrives. A run that left no trace at all is unrecoverable.
	a := withScheduledAgentApp(t, "sk-test")
	prior := AgentRunner
	AgentRunner = nil
	defer func() { AgentRunner = prior }()

	if !a.RunScheduledAgentPrompt("Summarise my week and flag anything weird") {
		t.Fatal("the run was refused")
	}
	convs := a.Conversations()
	if len(convs) != 1 {
		t.Fatalf("conversations = %d, want 1", len(convs))
	}
	if len(convs[0].Messages) != 1 || convs[0].Messages[0].Role != "user" {
		t.Fatalf("messages = %+v, want just the question", convs[0].Messages)
	}
	if !strings.Contains(convs[0].Title, "Summarise my week") {
		t.Fatalf("title = %q — a list of scheduled runs must be readable", convs[0].Title)
	}
}

func TestTheAnswerIsFiledAgainstTheRunThatAskedIt(t *testing.T) {
	a := withScheduledAgentApp(t, "sk-test")
	prior := AgentRunner
	AgentRunner = func(prompt string, deliver func(string)) { deliver("You spent $312 this week.") }
	defer func() { AgentRunner = prior }()

	if !a.RunScheduledAgentPrompt("How was my week?") {
		t.Fatal("the run was refused")
	}
	convs := a.Conversations()
	if len(convs) != 1 || len(convs[0].Messages) != 2 {
		t.Fatalf("conversation = %+v", convs)
	}
	if convs[0].Messages[1].Role != "assistant" || !strings.Contains(convs[0].Messages[1].Text, "$312") {
		t.Fatalf("answer = %+v", convs[0].Messages[1])
	}
}

func TestAnEmptyAnswerIsNotFiled(t *testing.T) {
	// An empty assistant turn is the failure this whole overhaul started from; it
	// must not be written into a saved conversation either.
	a := withScheduledAgentApp(t, "sk-test")
	prior := AgentRunner
	AgentRunner = func(prompt string, deliver func(string)) { deliver("   ") }
	defer func() { AgentRunner = prior }()

	a.RunScheduledAgentPrompt("How was my week?")
	if got := len(a.Conversations()[0].Messages); got != 1 {
		t.Fatalf("messages = %d, want the empty answer left out", got)
	}
}

func TestWithoutAProviderTheRunSaysSoRatherThanSkippingSilently(t *testing.T) {
	// A schedule that quietly does nothing every Friday is worse than one that
	// says why: the user set it up expecting something and has no way to find out.
	a := newTestAppAt(t, time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC))
	var notices []string
	a.Notifier = func(msg string) { notices = append(notices, msg) }

	if a.RunScheduledAgentPrompt("How was my week?") {
		t.Fatal("a run with no provider reported success")
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "no AI key") {
		t.Fatalf("notices = %v, want one explaining the skip", notices)
	}
	if len(a.Conversations()) != 0 {
		t.Fatal("a skipped run left a conversation behind")
	}
}

func TestAnEmptyPromptIsRefused(t *testing.T) {
	a := withScheduledAgentApp(t, "sk-test")
	for _, p := range []string{"", "   ", "\t\n"} {
		if a.RunScheduledAgentPrompt(p) {
			t.Fatalf("an empty prompt (%q) was accepted", p)
		}
	}
	if len(a.Conversations()) != 0 {
		t.Fatal("an empty prompt created a conversation")
	}
}

func TestScheduledConversationTitlesStayShortAndDated(t *testing.T) {
	at := time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC)
	long := strings.Repeat("summarise everything about my finances ", 5)
	got := scheduledConversationTitle(long, at)
	if len([]rune(got)) > 60 {
		t.Fatalf("title is %d runes: %q", len([]rune(got)), got)
	}
	if !strings.Contains(got, "16 Aug") {
		t.Fatalf("title = %q, want it dated so repeated runs are distinguishable", got)
	}
}
