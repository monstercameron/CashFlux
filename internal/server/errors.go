package server

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
)

// ErrorReason is a stable machine-readable backend error reason.
type ErrorReason string

const (
	ErrorReasonUnauthenticated     ErrorReason = "AUTH_UNAUTHENTICATED"
	ErrorReasonPermissionDenied    ErrorReason = "AUTH_PERMISSION_DENIED"
	ErrorReasonInvalidArgument     ErrorReason = "REQUEST_INVALID"
	ErrorReasonPayloadTooLarge     ErrorReason = "REQUEST_TOO_LARGE"
	ErrorReasonUnsupportedMedia    ErrorReason = "REQUEST_UNSUPPORTED_MEDIA"
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

// ErrorResponse is the machine-readable HTTP error body.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes a backend error without leaking internal details.
type ErrorDetail struct {
	Reason  ErrorReason `json:"reason"`
	Message string      `json:"message"`
}

// BackendErrorTaxonomy is the stable reason/code/status table for backend API errors.
var BackendErrorTaxonomy = []ErrorTaxonomy{
	{Reason: ErrorReasonUnauthenticated, GRPC: codes.Unauthenticated, HTTP: http.StatusUnauthorized},
	{Reason: ErrorReasonPermissionDenied, GRPC: codes.PermissionDenied, HTTP: http.StatusForbidden},
	{Reason: ErrorReasonInvalidArgument, GRPC: codes.InvalidArgument, HTTP: http.StatusBadRequest},
	{Reason: ErrorReasonPayloadTooLarge, GRPC: codes.ResourceExhausted, HTTP: http.StatusRequestEntityTooLarge},
	{Reason: ErrorReasonUnsupportedMedia, GRPC: codes.InvalidArgument, HTTP: http.StatusUnsupportedMediaType},
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

func writeErrorJSON(w http.ResponseWriter, reason ErrorReason, message string) {
	row, ok := LookupErrorTaxonomy(reason)
	if !ok {
		row = ErrorTaxonomy{Reason: ErrorReasonInternal, HTTP: http.StatusInternalServerError}
		reason = ErrorReasonInternal
		message = "internal server error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(row.HTTP)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorDetail{Reason: reason, Message: message}}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
