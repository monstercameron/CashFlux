package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// scripted is a fake Model that returns a fixed sequence of turns (repeating the
// last), optionally erroring on the 1-based call number errAt.
type scripted struct {
	turns []Turn
	errAt int
	n     int
}

func (s *scripted) Next(_ []Message) (Turn, error) {
	i := s.n
	s.n++
	if s.errAt > 0 && i == s.errAt-1 {
		return Turn{}, errors.New("boom")
	}
	if i < len(s.turns) {
		return s.turns[i], nil
	}
	return s.turns[len(s.turns)-1], nil
}

func call(name string) ToolCall { return ToolCall{ID: name + "-1", Name: name} }

func testRegistry() *Registry {
	r := NewRegistry()
	r.Register(Tool{Name: "add", Description: "add a thing", Handler: func(json.RawMessage) (string, error) { return "added", nil }})
	r.Register(Tool{Name: "fail", Description: "always fails", Handler: func(json.RawMessage) (string, error) { return "", errors.New("nope") }})
	return r
}

func TestRunMultiStepDone(t *testing.T) {
	model := &scripted{turns: []Turn{
		{Calls: []ToolCall{call("add")}, OutputTokens: 10},
		{Calls: []ToolCall{call("add")}, OutputTokens: 10},
		{Text: "all done", OutputTokens: 5},
	}}
	tr, err := Run(context.Background(), model, testRegistry(), nil, Options{MaxSteps: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.StopReason != "done" || tr.Final != "all done" {
		t.Errorf("stop=%q final=%q, want done/all done", tr.StopReason, tr.Final)
	}
	if len(tr.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(tr.Steps))
	}
	if r := tr.Steps[0].Results; len(r) != 1 || r[0].Content != "added" || r[0].Err != "" {
		t.Errorf("step 0 result = %+v, want one 'added' with no error", r)
	}
	if tr.TokensUsed != 25 {
		t.Errorf("tokens = %d, want 25", tr.TokensUsed)
	}
}

func TestRunStopsAtMaxSteps(t *testing.T) {
	model := &scripted{turns: []Turn{{Calls: []ToolCall{call("add")}}}} // always wants another call
	tr, _ := Run(context.Background(), model, testRegistry(), nil, Options{MaxSteps: 2})
	if tr.StopReason != "max_steps" {
		t.Errorf("stop = %q, want max_steps", tr.StopReason)
	}
	if len(tr.Steps) != 2 {
		t.Errorf("got %d steps, want 2", len(tr.Steps))
	}
}

func TestRunStopsOnTokenBudget(t *testing.T) {
	model := &scripted{turns: []Turn{{Calls: []ToolCall{call("add")}, InputTokens: 40, OutputTokens: 30}}}
	tr, _ := Run(context.Background(), model, testRegistry(), nil, Options{MaxSteps: 8, TokenBudget: 50})
	if tr.StopReason != "budget" {
		t.Errorf("stop = %q, want budget", tr.StopReason)
	}
	if len(tr.Steps) != 1 || tr.TokensUsed != 70 {
		t.Errorf("steps=%d tokens=%d, want 1 step / 70 tokens", len(tr.Steps), tr.TokensUsed)
	}
}

func TestRunToolErrorContinues(t *testing.T) {
	model := &scripted{turns: []Turn{
		{Calls: []ToolCall{call("fail")}},
		{Text: "recovered"},
	}}
	tr, err := Run(context.Background(), model, testRegistry(), nil, Options{})
	if err != nil {
		t.Fatalf("a tool error must not abort the loop: %v", err)
	}
	if tr.StopReason != "done" || tr.Final != "recovered" {
		t.Errorf("stop=%q final=%q, want done/recovered", tr.StopReason, tr.Final)
	}
	if got := tr.Steps[0].Results[0].Err; got != "nope" {
		t.Errorf("tool error = %q, want nope", got)
	}
}

func TestRunUnknownTool(t *testing.T) {
	model := &scripted{turns: []Turn{{Calls: []ToolCall{call("ghost")}}, {Text: "ok"}}}
	tr, _ := Run(context.Background(), model, testRegistry(), nil, Options{})
	if got := tr.Steps[0].Results[0].Err; got != "unknown tool: ghost" {
		t.Errorf("err = %q, want 'unknown tool: ghost'", got)
	}
}

func TestRunCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model := &scripted{turns: []Turn{{Calls: []ToolCall{call("add")}}}}
	tr, err := Run(ctx, model, testRegistry(), nil, Options{})
	if err != nil {
		t.Fatalf("cancellation is not an error: %v", err)
	}
	if tr.StopReason != "canceled" || len(tr.Steps) != 0 {
		t.Errorf("stop=%q steps=%d, want canceled / 0 steps", tr.StopReason, len(tr.Steps))
	}
}

func TestRunModelError(t *testing.T) {
	model := &scripted{turns: []Turn{{Text: "x"}}, errAt: 1}
	tr, err := Run(context.Background(), model, testRegistry(), nil, Options{})
	if !errors.Is(err, ErrModel) {
		t.Errorf("err = %v, want it to wrap ErrModel", err)
	}
	if tr.StopReason != "error" {
		t.Errorf("stop = %q, want error", tr.StopReason)
	}
}

func TestRegistry(t *testing.T) {
	r := testRegistry()
	if _, ok := r.Get("add"); !ok {
		t.Error("add should be registered")
	}
	specs := r.Specs()
	if len(specs) != 2 || specs[0].Name != "add" || specs[1].Name != "fail" {
		t.Errorf("specs = %+v, want add then fail in registration order", specs)
	}
	// Re-registering by name replaces, not duplicates.
	r.Register(Tool{Name: "add", Description: "v2"})
	if specs := r.Specs(); len(specs) != 2 || specs[0].Description != "v2" {
		t.Errorf("re-register should replace in place: %+v", specs)
	}
}

func TestExecuteInvalidJSONArgs(t *testing.T) {
	r := testRegistry()
	res := r.execute(ToolCall{ID: "x", Name: "add", Args: json.RawMessage("{not json")})
	if res.Err != "invalid JSON arguments" {
		t.Errorf("err = %q, want 'invalid JSON arguments'", res.Err)
	}
}
