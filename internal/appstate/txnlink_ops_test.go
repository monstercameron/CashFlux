// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPutTxnLinkValidation(t *testing.T) {
	a := newApp(t, false)

	tests := []struct {
		name    string
		link    domain.TxnLink
		wantErr bool
	}{
		{"unknown kind", domain.TxnLink{Kind: "mystery", TxnIDs: []string{"a", "b"}}, true},
		{"group too few", domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a"}}, true},
		{"group ok", domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b", "c"}}, false},
		{"pair wrong count", domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"x", "y", "z"}}, true},
		{"pair ok", domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"x", "y"}}, false},
		{"empty member", domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"x", ""}}, true},
		{"dup member", domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "a"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := a.PutTxnLink(tc.link)
			if (err != nil) != tc.wantErr {
				t.Fatalf("PutTxnLink err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPutTxnLinkOneGroupPerTxn(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b"}}); err != nil {
		t.Fatalf("first group: %v", err)
	}
	// A second group reusing member "b" must be rejected.
	err := a.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"b", "c"}})
	if err == nil {
		t.Fatal("expected rejection: txn already in another group")
	}
	// A refund pair reusing "b" is allowed (different kind, different invariant).
	if err := a.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"b", "d"}}); err != nil {
		t.Fatalf("refund pair reusing grouped txn: %v", err)
	}
}

func TestDeleteTxnLinkReleases(t *testing.T) {
	a := newApp(t, false)
	l := domain.TxnLink{ID: "g1", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b"}}
	if err := a.PutTxnLink(l); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := a.DeleteTxnLink("g1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := a.TxnLinks(); len(got) != 0 {
		t.Fatalf("link not released: %+v", got)
	}
}
