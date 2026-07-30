// SPDX-License-Identifier: MIT

//go:build js && wasm

// Command cashflux-services owns CashFlux's browser-side gRPC transport inside
// a Dedicated Web Worker.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backendrpc/grpccodec"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serviceWorker struct {
	mu      sync.Mutex
	conns   map[string]*grpc.ClientConn
	cancels map[string]context.CancelFunc
	epoch   uint64
}

const blobStreamChunkBytes = 64 << 10
const aiChunkFlushBytes = 4 << 10
const aiChunkFlushInterval = 16 * time.Millisecond

func newServiceWorker() *serviceWorker {
	return &serviceWorker{
		conns:   make(map[string]*grpc.ClientConn),
		cancels: make(map[string]context.CancelFunc),
	}
}

func (w *serviceWorker) connection(ctx context.Context, endpoint, token string) (*grpc.ClientConn, error) {
	key := strings.TrimRight(strings.TrimSpace(endpoint), "/") + "\x00" + strings.TrimSpace(token)
	w.mu.Lock()
	conn := w.conns[key]
	epoch := w.epoch
	w.mu.Unlock()
	if conn != nil {
		return conn, nil
	}
	conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: endpoint, Token: token})
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	if w.epoch != epoch || ctx.Err() != nil {
		w.mu.Unlock()
		_ = conn.Close()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, context.Canceled
	}
	if existing := w.conns[key]; existing != nil {
		w.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	w.conns[key] = conn
	w.mu.Unlock()
	return conn, nil
}

func (w *serviceWorker) register(id string) (context.Context, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.cancels[id]; exists {
		return nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancels[id] = cancel
	return ctx, true
}

func (w *serviceWorker) finish(id string) {
	w.mu.Lock()
	delete(w.cancels, id)
	w.mu.Unlock()
}

func (w *serviceWorker) cancel(id string) {
	w.mu.Lock()
	cancel := w.cancels[id]
	delete(w.cancels, id)
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (w *serviceWorker) reset() {
	w.mu.Lock()
	cancels := w.cancels
	conns := w.conns
	w.epoch++
	w.cancels = make(map[string]context.CancelFunc)
	w.conns = make(map[string]*grpc.ClientConn)
	w.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (w *serviceWorker) handle(req rpcprotocol.Request) {
	ctx, ok := w.register(req.ID)
	if !ok {
		postFailure(req.ID, &rpcprotocol.Error{
			Code:    "AlreadyExists",
			Message: "an operation with this id is already active",
		})
		return
	}
	go func() {
		defer w.finish(req.ID)
		defer func() {
			if recovered := recover(); recovered != nil {
				postFailure(req.ID, &rpcprotocol.Error{
					Code:    "Internal",
					Message: fmt.Sprintf("services worker operation panicked: %v", recovered),
				})
			}
		}()
		switch req.Kind {
		case rpcprotocol.KindUnary:
			w.handleUnary(ctx, req)
		case rpcprotocol.KindStreamOpen:
			w.handleServerStream(ctx, req)
		case rpcprotocol.KindBlobUpload:
			w.handleBlobUpload(ctx, req)
		case rpcprotocol.KindBlobDownload:
			w.handleBlobDownload(ctx, req)
		default:
			postFailure(req.ID, &rpcprotocol.Error{
				Code:    "Unimplemented",
				Message: fmt.Sprintf("services worker does not handle %q yet", req.Kind),
			})
		}
	}()
}

func (w *serviceWorker) handleBlobUpload(ctx context.Context, req rpcprotocol.Request) {
	var header backendrpc.UploadBlobHeader
	if err := json.Unmarshal(req.Payload, &header); err != nil {
		postFailure(req.ID, &rpcprotocol.Error{Code: "InvalidArgument", Message: err.Error()})
		return
	}
	conn, err := w.connection(ctx, req.Endpoint, req.Token)
	if err != nil {
		postFailure(req.ID, rpcError(err, false))
		return
	}
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true}, req.Method, grpccodec.CallOptions()...)
	if err == nil {
		err = stream.SendMsg(&backendrpc.UploadBlobChunk{Header: &header})
	}
	for offset := 0; err == nil && offset < len(req.Binary); offset += blobStreamChunkBytes {
		end := offset + blobStreamChunkBytes
		if end > len(req.Binary) {
			end = len(req.Binary)
		}
		err = stream.SendMsg(&backendrpc.UploadBlobChunk{Data: req.Binary[offset:end]})
	}
	if err == nil {
		err = stream.CloseSend()
	}
	var response backendrpc.UploadBlobResponse
	if err == nil {
		err = stream.RecvMsg(&response)
	}
	if err != nil {
		postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
		return
	}
	payload, err := json.Marshal(response)
	if err != nil {
		postFailure(req.ID, &rpcprotocol.Error{Code: "Internal", Message: err.Error()})
		return
	}
	postPayload(rpcprotocol.KindReply, req.ID, 0, payload)
}

func (w *serviceWorker) handleBlobDownload(ctx context.Context, req rpcprotocol.Request) {
	var request backendrpc.DownloadBlobRequest
	if err := json.Unmarshal(req.Payload, &request); err != nil {
		postFailure(req.ID, &rpcprotocol.Error{Code: "InvalidArgument", Message: err.Error()})
		return
	}
	conn, err := w.connection(ctx, req.Endpoint, req.Token)
	if err != nil {
		postFailure(req.ID, rpcError(err, false))
		return
	}
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, req.Method, grpccodec.CallOptions()...)
	if err == nil {
		err = stream.SendMsg(&request)
	}
	if err == nil {
		err = stream.CloseSend()
	}
	var body []byte
	for err == nil {
		var chunk backendrpc.DownloadBlobChunk
		recvErr := stream.RecvMsg(&chunk)
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			err = recvErr
			break
		}
		body = append(body, chunk.Data...)
	}
	if err != nil {
		postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
		return
	}
	postBinary(req.ID, nil, body)
}

func (w *serviceWorker) handleUnary(ctx context.Context, req rpcprotocol.Request) {
	conn, err := w.connection(ctx, req.Endpoint, req.Token)
	if err != nil {
		postFailure(req.ID, rpcError(err, false))
		return
	}
	switch req.Method {
	case backendrpc.MethodSyncPutWorkspace:
		var request backendrpc.PutWorkspaceRequest
		if err := json.Unmarshal(req.Payload, &request); err != nil {
			postFailure(req.ID, &rpcprotocol.Error{Code: "InvalidArgument", Message: err.Error()})
			return
		}
		request.Dataset = req.Binary
		var response backendrpc.PutWorkspaceResponse
		if err := conn.Invoke(ctx, req.Method, request, &response, grpccodec.CallOptions()...); err != nil {
			postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
			return
		}
		response.Dataset = nil
		payload, err := json.Marshal(response)
		if err != nil {
			postFailure(req.ID, &rpcprotocol.Error{Code: "Internal", Message: err.Error()})
			return
		}
		postPayload(rpcprotocol.KindReply, req.ID, 0, payload)
	case backendrpc.MethodSyncGetWorkspace:
		var request backendrpc.GetWorkspaceRequest
		if err := json.Unmarshal(req.Payload, &request); err != nil {
			postFailure(req.ID, &rpcprotocol.Error{Code: "InvalidArgument", Message: err.Error()})
			return
		}
		var response backendrpc.GetWorkspaceResponse
		if err := conn.Invoke(ctx, req.Method, request, &response, grpccodec.CallOptions()...); err != nil {
			postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
			return
		}
		body := response.Dataset
		response.Dataset = nil
		payload, err := json.Marshal(response)
		if err != nil {
			postFailure(req.ID, &rpcprotocol.Error{Code: "Internal", Message: err.Error()})
			return
		}
		postBinary(req.ID, payload, body)
	default:
		request := json.RawMessage(req.Payload)
		var response json.RawMessage
		if err := conn.Invoke(ctx, req.Method, request, &response, grpccodec.CallOptions()...); err != nil {
			postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
			return
		}
		postPayload(rpcprotocol.KindReply, req.ID, 0, response)
	}
}

func (w *serviceWorker) handleServerStream(ctx context.Context, req rpcprotocol.Request) {
	conn, err := w.connection(ctx, req.Endpoint, req.Token)
	if err != nil {
		postFailure(req.ID, rpcError(err, false))
		return
	}
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, req.Method, grpccodec.CallOptions()...)
	if err == nil {
		err = stream.SendMsg(json.RawMessage(req.Payload))
	}
	if err == nil {
		err = stream.CloseSend()
	}
	if err != nil {
		postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
		return
	}
	if req.Method == backendrpc.MethodAIChatStream || req.Method == backendrpc.MethodAIVisionStream {
		w.forwardAIStream(req.ID, stream)
		return
	}
	var seq uint64
	for {
		var event json.RawMessage
		err = stream.RecvMsg(&event)
		if err == io.EOF {
			postPayload(rpcprotocol.KindStreamEnd, req.ID, 0, nil)
			return
		}
		if err != nil {
			postFailure(req.ID, rpcError(err, mayHaveApplied(err)))
			return
		}
		postPayload(rpcprotocol.KindStreamEvent, req.ID, seq, event)
		seq++
	}
}

func (w *serviceWorker) forwardAIStream(id string, stream grpc.ClientStream) {
	var (
		seq       uint64
		pending   backendrpc.CompletionChunk
		flushFrom = time.Now()
	)
	flush := func() bool {
		if pending.Content == "" && pending.Usage == (backendrpc.Usage{}) && !pending.Done {
			return true
		}
		payload, err := json.Marshal(pending)
		if err != nil {
			postFailure(id, &rpcprotocol.Error{Code: "Internal", Message: err.Error()})
			return false
		}
		postPayload(rpcprotocol.KindStreamEvent, id, seq, payload)
		seq++
		pending = backendrpc.CompletionChunk{}
		flushFrom = time.Now()
		return true
	}
	for {
		var chunk backendrpc.CompletionChunk
		err := stream.RecvMsg(&chunk)
		if err == io.EOF {
			if flush() {
				postPayload(rpcprotocol.KindStreamEnd, id, 0, nil)
			}
			return
		}
		if err != nil {
			postFailure(id, rpcError(err, mayHaveApplied(err)))
			return
		}
		pending.Content += chunk.Content
		if chunk.Usage != (backendrpc.Usage{}) {
			pending.Usage = chunk.Usage
		}
		pending.Done = pending.Done || chunk.Done
		if pending.Done || len(pending.Content) >= aiChunkFlushBytes || time.Since(flushFrom) >= aiChunkFlushInterval {
			done := pending.Done
			if !flush() {
				return
			}
			if done {
				postPayload(rpcprotocol.KindStreamEnd, id, 0, nil)
				return
			}
		}
	}
}

func rpcError(err error, ambiguous bool) *rpcprotocol.Error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if ok {
		return &rpcprotocol.Error{
			Code:           st.Code().String(),
			Message:        st.Message(),
			MayHaveApplied: ambiguous,
		}
	}
	return &rpcprotocol.Error{
		Code:           "Unavailable",
		Message:        err.Error(),
		MayHaveApplied: ambiguous,
	}
}

func mayHaveApplied(err error) bool {
	switch status.Code(err) {
	case codes.Canceled, codes.DeadlineExceeded, codes.Unknown, codes.Internal, codes.Unavailable:
		return true
	default:
		return false
	}
}

func postPayload(kind rpcprotocol.Kind, id string, seq uint64, payload []byte) {
	message := js.Global().Get("Object").New()
	message.Set("version", rpcprotocol.Version)
	message.Set("kind", string(kind))
	message.Set("id", id)
	if kind == rpcprotocol.KindStreamEvent {
		message.Set("seq", float64(seq))
	}
	if payload != nil {
		message.Set("payload", string(payload))
	}
	js.Global().Call("postMessage", message)
}

func postFailure(id string, rpcErr *rpcprotocol.Error) {
	message := js.Global().Get("Object").New()
	message.Set("version", rpcprotocol.Version)
	message.Set("kind", string(rpcprotocol.KindFailure))
	message.Set("id", id)
	errValue := js.Global().Get("Object").New()
	errValue.Set("code", rpcErr.Code)
	errValue.Set("message", rpcErr.Message)
	errValue.Set("mayHaveApplied", rpcErr.MayHaveApplied)
	message.Set("error", errValue)
	js.Global().Call("postMessage", message)
}

func postBinary(id string, payload, body []byte) {
	message := js.Global().Get("Object").New()
	message.Set("version", rpcprotocol.Version)
	message.Set("kind", string(rpcprotocol.KindBinaryReply))
	message.Set("id", id)
	if payload != nil {
		message.Set("payload", string(payload))
	}
	bytes := js.Global().Get("Uint8Array").New(len(body))
	js.CopyBytesToJS(bytes, body)
	message.Set("bytes", bytes)
	transfer := js.Global().Get("Array").New()
	transfer.Call("push", bytes.Get("buffer"))
	js.Global().Call("postMessage", message, transfer)
}

func requiredStringField(value js.Value, name string) (string, error) {
	field := value.Get(name)
	if field.IsUndefined() || field.IsNull() || field.Type() != js.TypeString {
		return "", fmt.Errorf("rpc protocol: %s must be a string", name)
	}
	return field.String(), nil
}

func decodeRequest(data js.Value) (rpcprotocol.Request, error) {
	if data.IsUndefined() || data.IsNull() || data.Type() != js.TypeObject {
		return rpcprotocol.Request{}, fmt.Errorf("rpc protocol: request must be an object")
	}
	versionValue := data.Get("version")
	if versionValue.IsUndefined() || versionValue.IsNull() || versionValue.Type() != js.TypeNumber {
		return rpcprotocol.Request{}, fmt.Errorf("rpc protocol: version must be a number")
	}
	kind, err := requiredStringField(data, "kind")
	if err != nil {
		return rpcprotocol.Request{}, err
	}
	req := rpcprotocol.Request{Version: versionValue.Int(), Kind: rpcprotocol.Kind(kind)}
	if req.Kind != rpcprotocol.KindSessionReset {
		req.ID, err = requiredStringField(data, "id")
		if err != nil {
			return rpcprotocol.Request{}, err
		}
	}
	if req.Kind != rpcprotocol.KindStreamCancel && req.Kind != rpcprotocol.KindSessionReset {
		if req.Endpoint, err = requiredStringField(data, "endpoint"); err != nil {
			return rpcprotocol.Request{}, err
		}
		if req.Token, err = requiredStringField(data, "token"); err != nil {
			return rpcprotocol.Request{}, err
		}
		if req.Method, err = requiredStringField(data, "method"); err != nil {
			return rpcprotocol.Request{}, err
		}
		if payload := data.Get("payload"); !payload.IsUndefined() && !payload.IsNull() {
			if payload.Type() != js.TypeString {
				return rpcprotocol.Request{}, fmt.Errorf("rpc protocol: payload must be a string")
			}
			req.Payload = json.RawMessage(payload.String())
		}
		if req.Kind == rpcprotocol.KindBlobUpload || req.Kind == rpcprotocol.KindUnary {
			if value := data.Get("bytes"); !value.IsUndefined() && !value.IsNull() {
				byteLength := value.Get("byteLength")
				if byteLength.IsUndefined() || byteLength.IsNull() || byteLength.Type() != js.TypeNumber {
					return rpcprotocol.Request{}, fmt.Errorf("rpc protocol: bytes must be an ArrayBuffer view")
				}
				req.Binary = make([]byte, value.Get("byteLength").Int())
				js.CopyBytesToGo(req.Binary, value)
			}
		}
	}
	if err := rpcprotocol.ValidateRequest(req); err != nil {
		return rpcprotocol.Request{}, err
	}
	return req, nil
}

func main() {
	worker := newServiceWorker()
	js.Global().Set("onmessage", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		req, err := decodeRequest(args[0].Get("data"))
		if err != nil {
			id := ""
			if len(args) > 0 {
				if value := args[0].Get("data").Get("id"); !value.IsUndefined() && !value.IsNull() {
					id = value.String()
				}
			}
			postFailure(id, &rpcprotocol.Error{Code: "InvalidArgument", Message: err.Error()})
			return nil
		}
		if req.Kind == rpcprotocol.KindStreamCancel {
			worker.cancel(req.ID)
			return nil
		}
		if req.Kind == rpcprotocol.KindSessionReset {
			worker.reset()
			return nil
		}
		worker.handle(req)
		return nil
	}))

	ready := js.Global().Get("Object").New()
	ready.Set("version", rpcprotocol.Version)
	ready.Set("kind", string(rpcprotocol.KindReady))
	js.Global().Call("postMessage", ready)
	select {}
}
