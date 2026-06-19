package server

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// ErrorReason is a stable machine-readable backend error reason.
type ErrorReason string

const (
	ErrorReasonUnauthenticated     ErrorReason = "AUTH_UNAUTHENTICATED"
	ErrorReasonPermissionDenied    ErrorReason = "AUTH_PERMISSION_DENIED"
	ErrorReasonInvalidArgument     ErrorReason = "REQUEST_INVALID"
	ErrorReasonNotFound            ErrorReason = "RESOURCE_NOT_FOUND"
	ErrorReasonFailedPrecondition  ErrorReason = "FAILED_PRECONDITION"
	ErrorReasonResourceExhausted   ErrorReason = "RESOURCE_EXHAUSTED"
	ErrorReasonRateLimited         ErrorReason = "RATE_LIMITED"
	ErrorReasonUpstreamUnavailable ErrorReason = "UPSTREAM_UNAVAILABLE"
	ErrorReasonDeadlineExceeded    ErrorReason = "DEADLINE_EXCEEDED"
	ErrorReasonCanceled            ErrorReason = "CANCELED"
	ErrorReasonInternal            ErrorReason = "INTERNAL"
)

// ErrorTaxonomy binds one reason to its gRPC and HTTP transport status.
type ErrorTaxonomy struct {
	Reason ErrorReason
	GRPC   codes.Code
	HTTP   int
}

// BackendErrorTaxonomy is the stable reason/code/status table for backend API errors.
var BackendErrorTaxonomy = []ErrorTaxonomy{
	{Reason: ErrorReasonUnauthenticated, GRPC: codes.Unauthenticated, HTTP: http.StatusUnauthorized},
	{Reason: ErrorReasonPermissionDenied, GRPC: codes.PermissionDenied, HTTP: http.StatusForbidden},
	{Reason: ErrorReasonInvalidArgument, GRPC: codes.InvalidArgument, HTTP: http.StatusBadRequest},
	{Reason: ErrorReasonNotFound, GRPC: codes.NotFound, HTTP: http.StatusNotFound},
	{Reason: ErrorReasonFailedPrecondition, GRPC: codes.FailedPrecondition, HTTP: http.StatusPreconditionFailed},
	{Reason: ErrorReasonResourceExhausted, GRPC: codes.ResourceExhausted, HTTP: http.StatusInsufficientStorage},
	{Reason: ErrorReasonRateLimited, GRPC: codes.ResourceExhausted, HTTP: http.StatusTooManyRequests},
	{Reason: ErrorReasonUpstreamUnavailable, GRPC: codes.Unavailable, HTTP: http.StatusBadGateway},
	{Reason: ErrorReasonDeadlineExceeded, GRPC: codes.DeadlineExceeded, HTTP: http.StatusGatewayTimeout},
	{Reason: ErrorReasonCanceled, GRPC: codes.Canceled, HTTP: 499},
	{Reason: ErrorReasonInternal, GRPC: codes.Internal, HTTP: http.StatusInternalServerError},
}

// LookupErrorTaxonomy returns the transport mapping for a stable reason.
func LookupErrorTaxonomy(reason ErrorReason) (ErrorTaxonomy, bool) {
	for _, row := range BackendErrorTaxonomy {
		if row.Reason == reason {
			return row, true
		}
	}
	return ErrorTaxonomy{}, false
}
