// Package agent is the pure, in-house tool-calling loop behind CashFlux's agentic
// AI (C82): a typed tool registry and a bounded model→tool-calls→execute→repeat
// loop with step and token-budget caps, cancellation, and a recorded transcript.
// No off-the-shelf Go agent framework fits GOOS=js GOARCH=wasm + local-first, and
// the loop is small, so it's built here on the provider abstraction (C81).
//
// This package is platform-independent and model-agnostic: the Model is an
// interface the wasm/AI layer implements over a real provider, and tools are plain
// Go handlers. No syscall/js; unit-tested on native Go with a scripted fake model.
package agent

import (
	"context"
	"encoding/json"
	"errors"
)

// Tool is one capability the agent can invoke: a name and description the model
// sees, a JSON-schema for its arguments, and a handler that runs it. Handlers are
// plain Go — the wasm layer binds them to appstate (read first, then guarded,
// audited writes) when wiring this up.
type Tool struct {
	Name        string
	Description string
	Params      json.RawMessage // JSON schema for the arguments (sent to the model)
	Handler     func(args json.RawMessage) (string, error)
}

// ToolSpec is the model-facing description of a tool (no handler), the shape sent
// to the provider when offering tools.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Params      json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall is a model's request to run a tool with the given arguments.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"arguments,omitempty"`
}

// ToolResult is the outcome of executing a ToolCall. Err is non-empty when the call
// failed (unknown tool, bad arguments, or a handler error) — it is fed back to the
// model rather than aborting the loop, so the model can recover.
type ToolResult struct {
	CallID  string `json:"callId"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Err     string `json:"error,omitempty"`
}

// Registry holds the tools available to the agent, in registration order.
type Registry struct {
	tools []Tool
	index map[string]int
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{index: map[string]int{}} }

// Register adds or replaces a tool by name.
func (r *Registry) Register(t Tool) {
	if i, ok := r.index[t.Name]; ok {
		r.tools[i] = t
		return
	}
	r.index[t.Name] = len(r.tools)
	r.tools = append(r.tools, t)
}

// Get returns the tool with the given name.
func (r *Registry) Get(name string) (Tool, bool) {
	if i, ok := r.index[name]; ok {
		return r.tools[i], true
	}
	return Tool{}, false
}

// Specs returns the model-facing specs for every registered tool, in registration
// order — what the provider is told it can call.
func (r *Registry) Specs() []ToolSpec {
	out := make([]ToolSpec, len(r.tools))
	for i, t := range r.tools {
		out[i] = ToolSpec{Name: t.Name, Description: t.Description, Params: t.Params}
	}
	return out
}

// execute runs one tool call, turning every failure mode into a ToolResult with Err
// set (never a panic or loop abort), so the model sees and can react to the error.
func (r *Registry) execute(call ToolCall) ToolResult {
	res := ToolResult{CallID: call.ID, Name: call.Name}
	t, ok := r.Get(call.Name)
	if !ok {
		res.Err = "unknown tool: " + call.Name
		return res
	}
	if len(call.Args) > 0 && !json.Valid(call.Args) {
		res.Err = "invalid JSON arguments"
		return res
	}
	out, err := t.Handler(call.Args)
	res.Content = out
	if err != nil {
		res.Err = err.Error()
	}
	return res
}

// Turn is what a Model produces each step: either a final answer (Text, no Calls) or
// a set of tool calls to execute (Calls, Text optional as reasoning). Token counts
// drive the budget cap.
type Turn struct {
	Text         string
	Calls        []ToolCall
	InputTokens  int
	OutputTokens int
}

// Message is one entry in the running conversation the Model reads. Role is
// "system", "user", "assistant", or "tool"; ToolCallID links a tool message to the
// call it answers.
type Message struct {
	Role       string
	Content    string
	ToolCallID string
}

// Model is the agent's view of an LLM: given the conversation so far, produce the
// next Turn. The wasm/AI layer implements this over a real provider; tests use a
// scripted fake.
type Model interface {
	Next(history []Message) (Turn, error)
}

// Options bound the loop. MaxSteps caps model turns (<=0 means a default of 8);
// TokenBudget caps total tokens across turns (0 = unbounded).
type Options struct {
	MaxSteps    int
	TokenBudget int
}

const defaultMaxSteps = 8

// Step records one iteration of the loop for the transcript (explainability rule).
type Step struct {
	Turn    Turn
	Results []ToolResult
}

// Transcript is the full record of a run: every step, the final answer, why it
// stopped, and the tokens spent.
type Transcript struct {
	Steps      []Step
	Final      string
	StopReason string // "done", "max_steps", "budget", "canceled"
	TokensUsed int
}

// ErrModel wraps a model error so callers can distinguish it; the loop returns the
// partial transcript alongside it.
var ErrModel = errors.New("agent: model error")

// Run drives the bounded loop: ask the model, execute any tool calls, append their
// results, and repeat until the model returns a final answer, the step or token cap
// is hit, or the context is canceled. It never aborts on a tool failure — those
// become ToolResults the model can react to. A model error stops the loop and is
// returned with the partial transcript.
func Run(ctx context.Context, model Model, reg *Registry, history []Message, opts Options) (Transcript, error) {
	maxSteps := opts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	convo := append([]Message(nil), history...)
	var tr Transcript

	for step := 0; step < maxSteps; step++ {
		if ctx.Err() != nil {
			tr.StopReason = "canceled"
			return tr, nil
		}
		turn, err := model.Next(convo)
		if err != nil {
			tr.StopReason = "error"
			return tr, errors.Join(ErrModel, err)
		}
		tr.TokensUsed += turn.InputTokens + turn.OutputTokens

		if len(turn.Calls) == 0 {
			tr.Steps = append(tr.Steps, Step{Turn: turn})
			tr.Final = turn.Text
			tr.StopReason = "done"
			return tr, nil
		}

		results := make([]ToolResult, 0, len(turn.Calls))
		convo = append(convo, Message{Role: "assistant", Content: turn.Text})
		for _, call := range turn.Calls {
			res := reg.execute(call)
			results = append(results, res)
			content := res.Content
			if res.Err != "" {
				content = "error: " + res.Err
			}
			convo = append(convo, Message{Role: "tool", Content: content, ToolCallID: res.CallID})
		}
		tr.Steps = append(tr.Steps, Step{Turn: turn, Results: results})

		if opts.TokenBudget > 0 && tr.TokensUsed >= opts.TokenBudget {
			tr.StopReason = "budget"
			return tr, nil
		}
	}
	tr.StopReason = "max_steps"
	return tr, nil
}
