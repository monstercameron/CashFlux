// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package rpcworker is the render-thread client for CashFlux's services WASM.
package rpcworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
)

const streamBufferSize = 64

type reply struct {
	payload []byte
	binary  []byte
	err     error
}

type streamReply struct {
	payload []byte
	err     error
	done    bool
	seq     uint64
}

// Client owns one Dedicated Worker and correlates its replies and stream events.
type Client struct {
	worker js.Value

	mu      sync.Mutex
	nextID  uint64
	ready   chan struct{}
	started bool
	fatal   error
	pending map[string]chan reply
	streams map[string]*Stream

	onMessage      js.Func
	onError        js.Func
	onMessageError js.Func
}

// Stream is one server-streaming RPC delivered by services.wasm.
type Stream struct {
	client *Client
	id     string
	events chan streamReply
	done   chan struct{}

	mu     sync.Mutex
	cursor rpcprotocol.StreamCursor
	closed bool
}

var defaultClient *Client

// StartDefault boots the one process-wide RPC worker.
func StartDefault() (*Client, error) {
	if defaultClient != nil {
		return defaultClient, nil
	}
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return nil, errors.New("rpc worker: document is unavailable")
	}
	workerURL := js.Global().Get("URL").New(
		"services-worker.js",
		document.Get("baseURI").String(),
	).Get("href").String()
	client, err := New(workerURL)
	if err != nil {
		return nil, err
	}
	defaultClient = client
	return client, nil
}

// Default returns the process-wide worker client, or nil before StartDefault.
func Default() *Client {
	return defaultClient
}

// New starts a Dedicated Worker at workerURL.
func New(workerURL string) (*Client, error) {
	if workerURL == "" {
		return nil, errors.New("rpc worker: worker URL is required")
	}
	workerCtor := js.Global().Get("Worker")
	if workerCtor.IsUndefined() || workerCtor.Type() != js.TypeFunction {
		return nil, errors.New("rpc worker: Dedicated Workers are unavailable")
	}
	client := &Client{
		worker:  workerCtor.New(workerURL),
		ready:   make(chan struct{}),
		pending: make(map[string]chan reply),
		streams: make(map[string]*Stream),
	}
	setWorkerStatus("starting")
	client.installHandlers()
	return client, nil
}

func setWorkerStatus(status string) {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return
	}
	root := document.Get("documentElement")
	if root.IsUndefined() || root.IsNull() {
		return
	}
	root.Call("setAttribute", "data-services-worker", status)
}

func (c *Client) installHandlers() {
	c.onMessage = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		c.handleMessage(args[0].Get("data"))
		return nil
	})
	c.onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		reason := "services worker failed"
		if len(args) > 0 {
			if message := args[0].Get("message"); message.Truthy() {
				reason = message.String()
			}
		}
		c.failAll(errors.New(reason))
		return nil
	})
	c.onMessageError = js.FuncOf(func(js.Value, []js.Value) any {
		c.failAll(errors.New("services worker sent an unreadable message"))
		return nil
	})
	c.worker.Set("onmessage", c.onMessage)
	c.worker.Set("onerror", c.onError)
	c.worker.Set("onmessageerror", c.onMessageError)
}

func (c *Client) handleMessage(data js.Value) {
	if data.IsUndefined() || data.IsNull() {
		return
	}
	version := data.Get("version").Int()
	kind := rpcprotocol.Kind(data.Get("kind").String())
	if kind == rpcprotocol.KindReady {
		if version != rpcprotocol.Version {
			c.failAll(fmt.Errorf("services worker protocol version %d does not match app version %d; reload required", version, rpcprotocol.Version))
			return
		}
		c.mu.Lock()
		if !c.started {
			c.started = true
			close(c.ready)
		}
		c.mu.Unlock()
		setWorkerStatus("ready")
		return
	}
	if version != rpcprotocol.Version {
		c.failAll(fmt.Errorf("services worker message version %d does not match app version %d", version, rpcprotocol.Version))
		return
	}
	id := data.Get("id").String()
	switch kind {
	case rpcprotocol.KindReply:
		c.deliverReply(id, []byte(data.Get("payload").String()), nil)
	case rpcprotocol.KindFailure:
		c.deliverFailure(id, decodeError(data.Get("error")))
	case rpcprotocol.KindBinaryReply:
		var binary []byte
		if value := data.Get("bytes"); !value.IsUndefined() && !value.IsNull() {
			binary = make([]byte, value.Get("byteLength").Int())
			js.CopyBytesToGo(binary, value)
		}
		c.deliverBinaryReply(id, []byte(data.Get("payload").String()), binary)
	case rpcprotocol.KindStreamEvent:
		c.deliverStream(id, streamReply{
			payload: []byte(data.Get("payload").String()),
			seq:     uint64(data.Get("seq").Float()),
		})
	case rpcprotocol.KindStreamEnd:
		c.deliverStream(id, streamReply{done: true})
	case rpcprotocol.KindFatal:
		c.failAll(decodeError(data.Get("error")))
	}
}

func decodeError(value js.Value) error {
	if value.IsUndefined() || value.IsNull() {
		return errors.New("RPC failed without an error")
	}
	return &rpcprotocol.Error{
		Code:           value.Get("code").String(),
		Message:        value.Get("message").String(),
		MayHaveApplied: value.Get("mayHaveApplied").Bool(),
	}
}

func (c *Client) deliverReply(id string, payload []byte, err error) {
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- reply{payload: payload, err: err}
	}
}

func (c *Client) deliverBinaryReply(id string, payload, binary []byte) {
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- reply{payload: payload, binary: binary}
	}
}

func (c *Client) deliverFailure(id string, err error) {
	c.mu.Lock()
	_, unary := c.pending[id]
	c.mu.Unlock()
	if unary {
		c.deliverReply(id, nil, err)
		return
	}
	c.deliverStream(id, streamReply{err: err, done: true})
}

func (c *Client) deliverStream(id string, event streamReply) {
	c.mu.Lock()
	stream := c.streams[id]
	c.mu.Unlock()
	if stream == nil {
		return
	}
	if err := stream.accept(event); err != nil {
		c.removeStream(id)
		c.postCancel(id)
		stream.fail(err)
	}
}

func (s *Stream) accept(event streamReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.done {
		if err := s.cursor.Terminate(); err != nil {
			return err
		}
		s.closeLocked()
		s.client.removeStream(s.id)
	} else if err := s.cursor.AcceptEvent(event.seq); err != nil {
		return err
	}
	// Keep one channel slot reserved for the terminal failure/end signal. Without
	// that reserve, an overflow could fill the queue and then drop the very error
	// that tells Recv to stop, leaving the caller blocked after draining it.
	if len(s.events) >= streamBufferSize {
		s.closeLocked()
		return errors.New("rpc worker: stream event buffer overflow")
	}
	s.events <- event
	return nil
}

func (s *Stream) closeLocked() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.done)
}

func (s *Stream) fail(err error) {
	s.mu.Lock()
	s.closeLocked()
	s.mu.Unlock()
	select {
	case s.events <- streamReply{err: err, done: true}:
	default:
	}
}

func (c *Client) failAll(err error) {
	if err == nil {
		err = errors.New("services worker failed")
	}
	c.mu.Lock()
	if c.fatal == nil {
		c.fatal = err
	}
	pending := c.pending
	streams := c.streams
	c.pending = make(map[string]chan reply)
	c.streams = make(map[string]*Stream)
	if !c.started {
		c.started = true
		close(c.ready)
	}
	c.mu.Unlock()
	setWorkerStatus("error")
	for _, ch := range pending {
		ch <- reply{err: err}
	}
	for _, stream := range streams {
		stream.fail(err)
	}
}

func (c *Client) waitReady(ctx context.Context) error {
	if c == nil {
		return errors.New("rpc worker: client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-c.ready:
		c.mu.Lock()
		err := c.fatal
		c.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) newID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return "rpc-" + strconv.FormatUint(c.nextID, 10)
}

func (c *Client) post(kind rpcprotocol.Kind, id, endpoint, token, method string, payload []byte) error {
	c.mu.Lock()
	fatal := c.fatal
	c.mu.Unlock()
	if fatal != nil {
		return fatal
	}
	message := js.Global().Get("Object").New()
	message.Set("version", rpcprotocol.Version)
	message.Set("kind", string(kind))
	message.Set("id", id)
	if endpoint != "" {
		message.Set("endpoint", endpoint)
	}
	if token != "" {
		message.Set("token", token)
	}
	if method != "" {
		message.Set("method", method)
	}
	if payload != nil {
		message.Set("payload", string(payload))
	}
	c.worker.Call("postMessage", message)
	return nil
}

func (c *Client) postBinary(kind rpcprotocol.Kind, id, endpoint, token, method string, payload, binary []byte) error {
	c.mu.Lock()
	fatal := c.fatal
	c.mu.Unlock()
	if fatal != nil {
		return fatal
	}
	message := js.Global().Get("Object").New()
	message.Set("version", rpcprotocol.Version)
	message.Set("kind", string(kind))
	message.Set("id", id)
	message.Set("endpoint", endpoint)
	message.Set("token", token)
	message.Set("method", method)
	if payload != nil {
		message.Set("payload", string(payload))
	}
	bytes := js.Global().Get("Uint8Array").New(len(binary))
	js.CopyBytesToJS(bytes, binary)
	message.Set("bytes", bytes)
	transfer := js.Global().Get("Array").New()
	transfer.Call("push", bytes.Get("buffer"))
	c.worker.Call("postMessage", message, transfer)
	return nil
}

func (c *Client) postCancel(id string) {
	_ = c.post(rpcprotocol.KindStreamCancel, id, "", "", "", nil)
}

// ResetSession cancels active worker operations and closes pooled connections.
// It is used when credentials are rotated or cleared.
func (c *Client) ResetSession() {
	if c == nil {
		return
	}
	_ = c.post(rpcprotocol.KindSessionReset, "", "", "", "", nil)
}

// Invoke performs one unary gRPC request in services.wasm.
func (c *Client) Invoke(ctx context.Context, endpoint, token, method string, request, response any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("rpc worker: encode %s: %w", method, err)
	}
	id := c.newID()
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.post(rpcprotocol.KindUnary, id, endpoint, token, method, payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case result := <-ch:
		if result.err != nil {
			return result.err
		}
		if response == nil {
			return nil
		}
		if err := json.Unmarshal(result.payload, response); err != nil {
			return fmt.Errorf("rpc worker: decode %s: %w", method, err)
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.postCancel(id)
		return ctx.Err()
	}
}

// InvokeWithBinaryRequest performs a unary RPC whose large byte field crosses
// to services.wasm as a transferable ArrayBuffer.
func (c *Client) InvokeWithBinaryRequest(ctx context.Context, endpoint, token, method string, request, response any, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("rpc worker: encode %s: %w", method, err)
	}
	id := c.newID()
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.postBinary(rpcprotocol.KindUnary, id, endpoint, token, method, payload, body); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case result := <-ch:
		if result.err != nil {
			return result.err
		}
		if response != nil {
			if err := json.Unmarshal(result.payload, response); err != nil {
				return fmt.Errorf("rpc worker: decode %s: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.postCancel(id)
		return ctx.Err()
	}
}

// InvokeWithBinaryResponse performs a unary RPC whose large byte field is
// returned by services.wasm as a transferable ArrayBuffer.
func (c *Client) InvokeWithBinaryResponse(ctx context.Context, endpoint, token, method string, request, response any) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("rpc worker: encode %s: %w", method, err)
	}
	id := c.newID()
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.post(rpcprotocol.KindUnary, id, endpoint, token, method, payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		if response != nil && len(result.payload) > 0 {
			if err := json.Unmarshal(result.payload, response); err != nil {
				return nil, fmt.Errorf("rpc worker: decode %s: %w", method, err)
			}
		}
		return result.binary, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.postCancel(id)
		return nil, ctx.Err()
	}
}

// UploadBlob transfers one blob body to services.wasm, where the gRPC client
// stream is opened and chunked. The body crosses the worker boundary as a
// transferable ArrayBuffer rather than base64 JSON.
func (c *Client) UploadBlob(ctx context.Context, endpoint, token, method string, header, response any, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return err
	}
	payload, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("rpc worker: encode blob header: %w", err)
	}
	id := c.newID()
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.postBinary(rpcprotocol.KindBlobUpload, id, endpoint, token, method, payload, body); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case result := <-ch:
		if result.err != nil {
			return result.err
		}
		if response != nil {
			if err := json.Unmarshal(result.payload, response); err != nil {
				return fmt.Errorf("rpc worker: decode blob upload: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.postCancel(id)
		return ctx.Err()
	}
}

// DownloadBlob asks services.wasm to assemble one gRPC server stream and
// transfers the completed bytes back as an ArrayBuffer.
func (c *Client) DownloadBlob(ctx context.Context, endpoint, token, method string, request any) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("rpc worker: encode blob request: %w", err)
	}
	id := c.newID()
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.post(rpcprotocol.KindBlobDownload, id, endpoint, token, method, payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case result := <-ch:
		return result.binary, result.err
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.postCancel(id)
		return nil, ctx.Err()
	}
}

// OpenServerStream starts one server-streaming gRPC request in services.wasm.
func (c *Client) OpenServerStream(ctx context.Context, endpoint, token, method string, request any) (*Stream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("rpc worker: encode %s: %w", method, err)
	}
	id := c.newID()
	stream := &Stream{
		client: c,
		id:     id,
		events: make(chan streamReply, streamBufferSize+1),
		done:   make(chan struct{}),
	}
	c.mu.Lock()
	c.streams[id] = stream
	c.mu.Unlock()
	if err := c.post(rpcprotocol.KindStreamOpen, id, endpoint, token, method, payload); err != nil {
		c.removeStream(id)
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			stream.Cancel()
		case <-stream.done:
		}
	}()
	return stream, nil
}

// Recv decodes the next stream event. It returns io.EOF after the worker's one
// terminal event.
func (s *Stream) Recv(target any) error {
	if s == nil {
		return errors.New("rpc worker: stream is nil")
	}
	event := <-s.events
	if event.err != nil {
		return event.err
	}
	if event.done {
		return io.EOF
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(event.payload, target); err != nil {
		return fmt.Errorf("rpc worker: decode stream event: %w", err)
	}
	return nil
}

// Cancel cancels the worker-side gRPC context and releases app correlation
// state. It is safe to call repeatedly.
func (s *Stream) Cancel() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closeLocked()
	s.mu.Unlock()
	s.client.removeStream(s.id)
	s.client.postCancel(s.id)
	select {
	case s.events <- streamReply{err: context.Canceled, done: true}:
	default:
	}
}

func (c *Client) removeStream(id string) {
	c.mu.Lock()
	delete(c.streams, id)
	c.mu.Unlock()
}

// InFlight reports active unary and streaming operations.
func (c *Client) InFlight() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending) + len(c.streams)
}
