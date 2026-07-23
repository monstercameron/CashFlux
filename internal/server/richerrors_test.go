// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestRichErrorInfoAttachesReasonAndDomain covers TODOS.md C428/C433: the base
// ErrorInfo helper carries the stable reason string, the fixed domain, and
// caller-supplied metadata.
func TestRichErrorInfoAttachesReasonAndDomain(t *testing.T) {
	err := RichErrorInfo(codes.PermissionDenied, ErrorReasonAdminSuspended, "account suspended", map[string]string{"user_id": "u1"})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected a status error")
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code = %v, want PermissionDenied", st.Code())
	}
	info := findErrorInfo(t, st.Details())
	if info.Reason != string(ErrorReasonAdminSuspended) {
		t.Errorf("Reason = %q, want %q", info.Reason, ErrorReasonAdminSuspended)
	}
	if info.Domain != richErrorDomain {
		t.Errorf("Domain = %q, want %q", info.Domain, richErrorDomain)
	}
	if info.Metadata["user_id"] != "u1" {
		t.Errorf("Metadata[user_id] = %q, want u1", info.Metadata["user_id"])
	}
}

func TestRichRateLimitErrorCarriesRetryInfo(t *testing.T) {
	err := RichRateLimitError("slow down", 30*time.Second)
	st, _ := status.FromError(err)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("code = %v, want ResourceExhausted", st.Code())
	}
	var retry *errdetails.RetryInfo
	for _, d := range st.Details() {
		if r, ok := d.(*errdetails.RetryInfo); ok {
			retry = r
		}
	}
	if retry == nil {
		t.Fatal("expected a RetryInfo detail")
	}
	if got := retry.RetryDelay.AsDuration(); got != 30*time.Second {
		t.Errorf("RetryDelay = %v, want 30s", got)
	}
}

func TestRichStorageQuotaErrorCarriesQuotaFailureAndHelp(t *testing.T) {
	err := RichStorageQuotaError("over quota", 900, 1000)
	st, _ := status.FromError(err)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("code = %v, want ResourceExhausted", st.Code())
	}
	info := findErrorInfo(t, st.Details())
	if info.Reason != string(ErrorReasonStorageQuotaExceeded) {
		t.Errorf("Reason = %q, want %q", info.Reason, ErrorReasonStorageQuotaExceeded)
	}
	if info.Metadata["bytes_used"] != "900" || info.Metadata["bytes_limit"] != "1000" {
		t.Errorf("Metadata = %v, want bytes_used=900 bytes_limit=1000", info.Metadata)
	}
	var help *errdetails.Help
	var quota *errdetails.QuotaFailure
	for _, d := range st.Details() {
		switch v := d.(type) {
		case *errdetails.Help:
			help = v
		case *errdetails.QuotaFailure:
			quota = v
		}
	}
	if quota == nil || len(quota.Violations) == 0 {
		t.Fatal("expected a QuotaFailure detail with at least one violation")
	}
	if help == nil || len(help.Links) == 0 {
		t.Fatal("expected a Help detail with at least one link")
	}
}

// TestRichPlanTierAndStorageErrorsUseDifferentHelpLinks covers the ticket's
// explicit requirement: storage overage and insufficient plan tier must not
// point at the same upgrade URL.
func TestRichPlanTierAndStorageErrorsUseDifferentHelpLinks(t *testing.T) {
	planSt, _ := status.FromError(RichPlanTierError("plan too low"))
	storageSt, _ := status.FromError(RichStorageQuotaError("over quota", 1, 2))

	planURL := helpURL(t, planSt.Details())
	storageURL := helpURL(t, storageSt.Details())
	if planURL == "" || storageURL == "" {
		t.Fatalf("expected both errors to carry a Help link, got plan=%q storage=%q", planURL, storageURL)
	}
	if planURL == storageURL {
		t.Fatalf("plan-tier and storage-quota errors must not share a Help URL, both got %q", planURL)
	}
}

func TestRichAdminSuspendedErrorHasNoHelpLink(t *testing.T) {
	st, _ := status.FromError(RichAdminSuspendedError("suspended"))
	if url := helpURL(t, st.Details()); url != "" {
		t.Fatalf("expected no Help link on an admin-suspended error, got %q", url)
	}
}

func findErrorInfo(t *testing.T, details []any) *errdetails.ErrorInfo {
	t.Helper()
	for _, d := range details {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info
		}
	}
	t.Fatal("expected an ErrorInfo detail")
	return nil
}

func helpURL(t *testing.T, details []any) string {
	t.Helper()
	for _, d := range details {
		if help, ok := d.(*errdetails.Help); ok && len(help.Links) > 0 {
			return help.Links[0].Url
		}
	}
	return ""
}
