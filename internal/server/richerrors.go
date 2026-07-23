// SPDX-License-Identifier: MIT

package server

import (
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// RichErrorReason extends the ErrorReason taxonomy in errors.go with the
// billing/entitlement-specific reasons this package's gRPC rejections need
// (TODOS.md C428/C433/C434/C435). They are new values in the SAME stable,
// machine-readable string taxonomy — never invent a parallel scheme.
const (
	ErrorReasonBillingLapsed        ErrorReason = "BILLING_LAPSED"
	ErrorReasonAdminSuspended       ErrorReason = "ADMIN_SUSPENDED"
	ErrorReasonPlanTierInsufficient ErrorReason = "PLAN_TIER_INSUFFICIENT"
	ErrorReasonStorageQuotaExceeded ErrorReason = "STORAGE_QUOTA_EXCEEDED"
)

// Domain and reason-group constants for google.rpc.ErrorInfo. Domain is the
// stable reverse-DNS namespace every ErrorInfo on this backend uses, so a
// client can group/log by domain regardless of reason.
const richErrorDomain = "cashflux.dev"

// upgradeHelpURL and storageHelpURL are placeholder portal links: an
// insufficient-plan-tier rejection and a storage-overage rejection point a
// user at two different actions (upgrade vs. free-up-space-or-upgrade-
// storage-addon), so they deliberately are not the same URL. The real
// account-portal routes aren't wired yet (cashflux-portal, see
// docs/CUSTOM_SYNC_TRANSPORT.md) — swap these for the real routes once it is.
const (
	upgradeHelpURL = "https://cashflux.dev/account/upgrade"
	storageHelpURL = "https://cashflux.dev/account/storage"
)

// RichErrorInfo attaches a google.rpc.ErrorInfo detail (stable reason +
// domain + free-form metadata) to a gRPC status. This is the minimum detail
// every rich error in this package carries — reason is one of the
// ErrorReason constants (errors.go or the additions above), never invented
// ad hoc, so client-side handling stays keyed off one closed vocabulary
// whether the transport is REST (ErrorDetail.Reason) or gRPC (ErrorInfo.Reason).
func RichErrorInfo(code codes.Code, reason ErrorReason, message string, metadata map[string]string) error {
	st := status.New(code, message)
	info := &errdetails.ErrorInfo{
		Reason:   string(reason),
		Domain:   richErrorDomain,
		Metadata: metadata,
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		// Attaching details should never fail for a well-formed proto message; if
		// it somehow does, surface the plain status rather than losing the error.
		return st.Err()
	}
	return withDetails.Err()
}

// RichRateLimitError builds a RATE_LIMITED rejection carrying ErrorInfo plus a
// RetryInfo so the client knows how long to back off before retrying.
func RichRateLimitError(message string, retryAfter time.Duration) error {
	st := status.New(codes.ResourceExhausted, message)
	info := &errdetails.ErrorInfo{Reason: string(ErrorReasonRateLimited), Domain: richErrorDomain}
	retry := &errdetails.RetryInfo{RetryDelay: durationpb.New(retryAfter)}
	withDetails, err := st.WithDetails(info, retry)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}

// RichStorageQuotaError builds a STORAGE_QUOTA_EXCEEDED rejection carrying
// ErrorInfo, a QuotaFailure with the used/limit figures, and a Help link to
// the storage-upgrade surface.
func RichStorageQuotaError(message string, usedBytes, limitBytes int64) error {
	st := status.New(codes.ResourceExhausted, message)
	info := &errdetails.ErrorInfo{
		Reason: string(ErrorReasonStorageQuotaExceeded),
		Domain: richErrorDomain,
		Metadata: map[string]string{
			"bytes_used":  strconv.FormatInt(usedBytes, 10),
			"bytes_limit": strconv.FormatInt(limitBytes, 10),
		},
	}
	quota := &errdetails.QuotaFailure{
		Violations: []*errdetails.QuotaFailure_Violation{{
			Subject:     "storage_bytes",
			Description: "account storage quota exceeded",
		}},
	}
	help := &errdetails.Help{
		Links: []*errdetails.Help_Link{{
			Description: "Manage storage or upgrade your plan",
			Url:         storageHelpURL,
		}},
	}
	withDetails, err := st.WithDetails(info, quota, help)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}

// RichPlanTierError builds a PLAN_TIER_INSUFFICIENT rejection carrying
// ErrorInfo and a Help link to the plan-upgrade surface (deliberately a
// different URL than RichStorageQuotaError's — "your plan doesn't include
// this" is a different call to action than "you're out of storage").
func RichPlanTierError(message string) error {
	return richErrorWithHelp(codes.PermissionDenied, ErrorReasonPlanTierInsufficient, message, "Upgrade your plan", upgradeHelpURL)
}

// RichBillingLapsedError builds a BILLING_LAPSED rejection (subscription
// inactive/past-due/canceled) carrying ErrorInfo and a Help link to renew.
func RichBillingLapsedError(message string) error {
	return richErrorWithHelp(codes.PermissionDenied, ErrorReasonBillingLapsed, message, "Renew your subscription", upgradeHelpURL)
}

// RichAdminSuspendedError builds an ADMIN_SUSPENDED rejection (operator
// moderation lever — see IsCloudActive in entitlements.go) carrying ErrorInfo.
// There is deliberately no Help link: a suspended account isn't pointed at a
// self-serve upgrade flow.
func RichAdminSuspendedError(message string) error {
	return RichErrorInfo(codes.PermissionDenied, ErrorReasonAdminSuspended, message, nil)
}

func richErrorWithHelp(code codes.Code, reason ErrorReason, message, linkDescription, url string) error {
	st := status.New(code, message)
	info := &errdetails.ErrorInfo{Reason: string(reason), Domain: richErrorDomain}
	help := &errdetails.Help{
		Links: []*errdetails.Help_Link{{Description: linkDescription, Url: url}},
	}
	withDetails, err := st.WithDetails(info, help)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
