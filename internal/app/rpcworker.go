// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/rpcworker"
)

func invokeWorkerRPC(ctx context.Context, endpoint, token, method string, request, response any) error {
	client := rpcworker.Default()
	if client == nil {
		var err error
		client, err = rpcworker.StartDefault()
		if err != nil {
			return err
		}
	}
	switch method {
	case backendrpc.MethodSyncPutWorkspace:
		var typed backendrpc.PutWorkspaceRequest
		switch value := request.(type) {
		case backendrpc.PutWorkspaceRequest:
			typed = value
		case *backendrpc.PutWorkspaceRequest:
			if value == nil {
				return errors.New("services worker: nil PutWorkspace request")
			}
			typed = *value
		default:
			return errors.New("services worker: unexpected PutWorkspace request type")
		}
		dataset := typed.Dataset
		typed.Dataset = nil
		return client.InvokeWithBinaryRequest(ctx, endpoint, token, method, typed, response, dataset)
	case backendrpc.MethodSyncGetWorkspace:
		typed, ok := response.(*backendrpc.GetWorkspaceResponse)
		if !ok || typed == nil {
			return errors.New("services worker: unexpected GetWorkspace response type")
		}
		dataset, err := client.InvokeWithBinaryResponse(ctx, endpoint, token, method, request, typed)
		if err != nil {
			return err
		}
		typed.Dataset = dataset
		return nil
	case backendrpc.MethodBlobSyncPutSnapshot:
		var typed backendrpc.PutSnapshotRequest
		switch value := request.(type) {
		case backendrpc.PutSnapshotRequest:
			typed = value
		case *backendrpc.PutSnapshotRequest:
			if value == nil {
				return errors.New("services worker: nil PutSnapshot request")
			}
			typed = *value
		default:
			return errors.New("services worker: unexpected PutSnapshot request type")
		}
		blob := typed.Blob
		typed.Blob = nil
		return client.InvokeWithBinaryRequest(ctx, endpoint, token, method, typed, response, blob)
	case backendrpc.MethodBlobSyncGetSnapshot:
		typed, ok := response.(*backendrpc.GetSnapshotResponse)
		if !ok || typed == nil {
			return errors.New("services worker: unexpected GetSnapshot response type")
		}
		blob, err := client.InvokeWithBinaryResponse(ctx, endpoint, token, method, request, typed)
		if err != nil {
			return err
		}
		typed.Blob = blob
		return nil
	default:
		return client.Invoke(ctx, endpoint, token, method, request, response)
	}
}

func openWorkerRPCStream(ctx context.Context, endpoint, token, method string, request any) (*rpcworker.Stream, error) {
	client := rpcworker.Default()
	if client == nil {
		var err error
		client, err = rpcworker.StartDefault()
		if err != nil {
			return nil, err
		}
	}
	if client == nil {
		return nil, errors.New("services worker is unavailable")
	}
	return client.OpenServerStream(ctx, endpoint, token, method, request)
}

func uploadWorkerBlob(ctx context.Context, endpoint, token, method string, header, response any, body []byte) error {
	client := rpcworker.Default()
	if client == nil {
		var err error
		client, err = rpcworker.StartDefault()
		if err != nil {
			return err
		}
	}
	return client.UploadBlob(ctx, endpoint, token, method, header, response, body)
}

func downloadWorkerBlob(ctx context.Context, endpoint, token, method string, request any) ([]byte, error) {
	client := rpcworker.Default()
	if client == nil {
		var err error
		client, err = rpcworker.StartDefault()
		if err != nil {
			return nil, err
		}
	}
	return client.DownloadBlob(ctx, endpoint, token, method, request)
}
