// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestBackendErrorTaxonomyStableMappings(t *testing.T) {
	want := map[ErrorReason]ErrorTaxonomy{
		ErrorReasonUnauthenticated:     {Reason: ErrorReasonUnauthenticated, GRPC: codes.Unauthenticated, HTTP: http.StatusUnauthorized},
		ErrorReasonPermissionDenied:    {Reason: ErrorReasonPermissionDenied, GRPC: codes.PermissionDenied, HTTP: http.StatusForbidden},
		ErrorReasonInvalidArgument:     {Reason: ErrorReasonInvalidArgument, GRPC: codes.InvalidArgument, HTTP: http.StatusBadRequest},
		ErrorReasonPayloadTooLarge:     {Reason: ErrorReasonPayloadTooLarge, GRPC: codes.ResourceExhausted, HTTP: http.StatusRequestEntityTooLarge},
		ErrorReasonUnsupportedMedia:    {Reason: ErrorReasonUnsupportedMedia, GRPC: codes.InvalidArgument, HTTP: http.StatusUnsupportedMediaType},
		ErrorReasonNotFound:            {Reason: ErrorReasonNotFound, GRPC: codes.NotFound, HTTP: http.StatusNotFound},
		ErrorReasonFailedPrecondition:  {Reason: ErrorReasonFailedPrecondition, GRPC: codes.FailedPrecondition, HTTP: http.StatusPreconditionFailed},
		ErrorReasonResourceExhausted:   {Reason: ErrorReasonResourceExhausted, GRPC: codes.ResourceExhausted, HTTP: http.StatusInsufficientStorage},
		ErrorReasonRateLimited:         {Reason: ErrorReasonRateLimited, GRPC: codes.ResourceExhausted, HTTP: http.StatusTooManyRequests},
		ErrorReasonUpstreamUnavailable: {Reason: ErrorReasonUpstreamUnavailable, GRPC: codes.Unavailable, HTTP: http.StatusBadGateway},
		ErrorReasonServerUnavailable:   {Reason: ErrorReasonServerUnavailable, GRPC: codes.Unavailable, HTTP: http.StatusServiceUnavailable},
		ErrorReasonDeadlineExceeded:    {Reason: ErrorReasonDeadlineExceeded, GRPC: codes.DeadlineExceeded, HTTP: http.StatusGatewayTimeout},
		ErrorReasonCanceled:            {Reason: ErrorReasonCanceled, GRPC: codes.Canceled, HTTP: 499},
		ErrorReasonInternal:            {Reason: ErrorReasonInternal, GRPC: codes.Internal, HTTP: http.StatusInternalServerError},
	}
	if len(BackendErrorTaxonomy) != len(want) {
		t.Fatalf("taxonomy length = %d, want %d", len(BackendErrorTaxonomy), len(want))
	}
	seen := map[ErrorReason]bool{}
	for _, row := range BackendErrorTaxonomy {
		if seen[row.Reason] {
			t.Fatalf("duplicate reason %s", row.Reason)
		}
		seen[row.Reason] = true
		if got := row; got != want[row.Reason] {
			t.Fatalf("taxonomy[%s] = %+v, want %+v", row.Reason, got, want[row.Reason])
		}
		if _, ok := LookupErrorTaxonomy(row.Reason); !ok {
			t.Fatalf("LookupErrorTaxonomy(%s) missing", row.Reason)
		}
	}
	if _, ok := LookupErrorTaxonomy("UNKNOWN"); ok {
		t.Fatal("unknown reason mapped")
	}
}

func TestWriteErrorJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorJSON(rr, ErrorReasonInvalidArgument, "bad input")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Reason != ErrorReasonInvalidArgument || body.Error.Message != "bad input" {
		t.Fatalf("body = %+v", body)
	}

	rr = httptest.NewRecorder()
	writeErrorJSON(rr, "UNKNOWN", "secret stack detail")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("unknown status = %d, want 500", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode unknown error: %v", err)
	}
	if body.Error.Reason != ErrorReasonInternal || body.Error.Message != "internal server error" {
		t.Fatalf("unknown body = %+v", body)
	}
}
