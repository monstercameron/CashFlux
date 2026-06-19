package server

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestBackendErrorTaxonomyStableMappings(t *testing.T) {
	want := map[ErrorReason]ErrorTaxonomy{
		ErrorReasonUnauthenticated:     {Reason: ErrorReasonUnauthenticated, GRPC: codes.Unauthenticated, HTTP: http.StatusUnauthorized},
		ErrorReasonPermissionDenied:    {Reason: ErrorReasonPermissionDenied, GRPC: codes.PermissionDenied, HTTP: http.StatusForbidden},
		ErrorReasonInvalidArgument:     {Reason: ErrorReasonInvalidArgument, GRPC: codes.InvalidArgument, HTTP: http.StatusBadRequest},
		ErrorReasonNotFound:            {Reason: ErrorReasonNotFound, GRPC: codes.NotFound, HTTP: http.StatusNotFound},
		ErrorReasonFailedPrecondition:  {Reason: ErrorReasonFailedPrecondition, GRPC: codes.FailedPrecondition, HTTP: http.StatusPreconditionFailed},
		ErrorReasonResourceExhausted:   {Reason: ErrorReasonResourceExhausted, GRPC: codes.ResourceExhausted, HTTP: http.StatusInsufficientStorage},
		ErrorReasonRateLimited:         {Reason: ErrorReasonRateLimited, GRPC: codes.ResourceExhausted, HTTP: http.StatusTooManyRequests},
		ErrorReasonUpstreamUnavailable: {Reason: ErrorReasonUpstreamUnavailable, GRPC: codes.Unavailable, HTTP: http.StatusBadGateway},
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
