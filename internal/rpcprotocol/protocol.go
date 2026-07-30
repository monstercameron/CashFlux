// SPDX-License-Identifier: MIT

// Package rpcprotocol defines the transport-neutral contract between CashFlux's
// render WASM and its GWC 5 services WASM.
package rpcprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Version changes whenever app.wasm and services.wasm would interpret a
// message differently.
const Version = 1

// Kind identifies one worker protocol message shape.
type Kind string

const (
	KindReady         Kind = "ready"
	KindUnary         Kind = "unary"
	KindStreamOpen    Kind = "stream-open"
	KindStreamCancel  Kind = "stream-cancel"
	KindReply         Kind = "reply"
	KindStreamEvent   Kind = "stream-event"
	KindStreamEnd     Kind = "stream-end"
	KindFailure       Kind = "failure"
	KindFatal         Kind = "fatal"
	KindSessionUpdate Kind = "session-update"
	KindSessionReset  Kind = "session-reset"
	KindBlobUpload    Kind = "blob-upload"
	KindBlobDownload  Kind = "blob-download"
	KindBinaryReply   Kind = "binary-reply"
)

// Request is the JSON/control portion of an app-to-worker operation. Large
// bytes travel beside it as a transferable ArrayBuffer.
type Request struct {
	Version  int             `json:"version"`
	Kind     Kind            `json:"kind"`
	ID       string          `json:"id"`
	Endpoint string          `json:"endpoint,omitempty"`
	Token    string          `json:"token,omitempty"`
	Method   string          `json:"method,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Binary   []byte          `json:"-"`
}

// Response is the JSON/control portion of a worker-to-app message.
type Response struct {
	Version int             `json:"version"`
	Kind    Kind            `json:"kind"`
	ID      string          `json:"id,omitempty"`
	Seq     uint64          `json:"seq,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a gRPC-independent failure crossing the worker boundary.
type Error struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	MayHaveApplied bool   `json:"mayHaveApplied,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Code) != "" {
		return e.Code
	}
	return "RPC failed"
}

// IsCode reports whether err is a worker RPC error with the requested stable
// gRPC code name.
func IsCode(err error, code string) bool {
	var rpcErr *Error
	return errors.As(err, &rpcErr) && strings.EqualFold(rpcErr.Code, code)
}

// ValidateRequest rejects malformed or version-skewed operations before they
// can enter the worker's operation table.
func ValidateRequest(req Request) error {
	if req.Version != Version {
		return fmt.Errorf("rpc protocol: unsupported version %d (want %d)", req.Version, Version)
	}
	switch req.Kind {
	case KindUnary, KindStreamOpen, KindBlobUpload, KindBlobDownload:
		if strings.TrimSpace(req.ID) == "" {
			return errors.New("rpc protocol: operation id is required")
		}
		if strings.TrimSpace(req.Endpoint) == "" {
			return errors.New("rpc protocol: endpoint is required")
		}
		if strings.TrimSpace(req.Token) == "" {
			return errors.New("rpc protocol: token is required")
		}
		if strings.TrimSpace(req.Method) == "" {
			return errors.New("rpc protocol: method is required")
		}
	case KindStreamCancel:
		if strings.TrimSpace(req.ID) == "" {
			return errors.New("rpc protocol: cancellation id is required")
		}
	case KindSessionReset:
		return nil
	default:
		return fmt.Errorf("rpc protocol: unsupported request kind %q", req.Kind)
	}
	return nil
}

// ValidateResponse rejects malformed worker messages before correlation state
// is mutated.
func ValidateResponse(resp Response) error {
	if resp.Version != Version {
		return fmt.Errorf("rpc protocol: unsupported version %d (want %d)", resp.Version, Version)
	}
	switch resp.Kind {
	case KindReady:
		return nil
	case KindReply, KindStreamEvent, KindStreamEnd, KindFailure, KindBinaryReply:
		if strings.TrimSpace(resp.ID) == "" {
			return errors.New("rpc protocol: response id is required")
		}
	case KindFatal:
		if resp.Error == nil {
			return errors.New("rpc protocol: fatal response needs an error")
		}
	case KindSessionUpdate:
		if len(resp.Payload) == 0 {
			return errors.New("rpc protocol: session update needs a payload")
		}
	default:
		return fmt.Errorf("rpc protocol: unsupported response kind %q", resp.Kind)
	}
	if resp.Kind == KindFailure && resp.Error == nil {
		return errors.New("rpc protocol: failure response needs an error")
	}
	return nil
}

// StreamCursor validates exactly-once, monotonically ordered stream delivery.
type StreamCursor struct {
	next       uint64
	terminated bool
}

// AcceptEvent records an event sequence number.
func (c *StreamCursor) AcceptEvent(seq uint64) error {
	if c.terminated {
		return errors.New("rpc protocol: event arrived after stream termination")
	}
	if seq != c.next {
		return fmt.Errorf("rpc protocol: stream sequence %d arrived, want %d", seq, c.next)
	}
	c.next++
	return nil
}

// Terminate records the stream's one terminal message.
func (c *StreamCursor) Terminate() error {
	if c.terminated {
		return errors.New("rpc protocol: stream terminated more than once")
	}
	c.terminated = true
	return nil
}
