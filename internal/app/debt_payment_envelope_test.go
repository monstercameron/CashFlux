// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestEncryptedDatasetEnvelopeCarriesDebtPaymentRecords(t *testing.T) {
	requireVaultWebCrypto(t)
	links := []domain.DebtPaymentLink{{
		ID: "private-link-id", TransactionIDs: []string{"private-transaction-id"}, DebtAccountID: "private-loan-id",
		Currency: "USD", PrincipalMinor: 9900, Method: domain.DebtPaymentStatement, Source: domain.DebtPaymentConfirmed,
	}}
	rules := []domain.DebtPayeeRule{{
		ID: "private-rule-id", WorkspaceID: "private-workspace-id", Payee: "Private Lender", DebtAccountID: "private-loan-id",
	}}
	plain, err := store.Export(store.Dataset{DebtPaymentLinks: links, DebtPayeeRules: rules})
	if err != nil {
		t.Fatalf("export dataset: %v", err)
	}
	envelopeBytes, err := awaitVaultTest(t, func(done func([]byte, error)) {
		encryptDataset(plain, "debt-envelope-test-passcode", done)
	})
	if err != nil {
		t.Fatalf("encrypt dataset: %v", err)
	}
	if !cryptobox.IsEnvelope(envelopeBytes) {
		t.Fatal("encrypted dataset was not stored as a cryptobox envelope")
	}
	for _, secret := range []string{"private-link-id", "private-rule-id", "Private Lender", "private-transaction-id"} {
		if bytes.Contains(envelopeBytes, []byte(secret)) {
			t.Fatalf("encrypted envelope contains plaintext record data %q", secret)
		}
	}
	env, ok := cryptobox.Parse(envelopeBytes)
	if !ok {
		t.Fatal("encrypted dataset envelope did not parse")
	}
	decrypted, err := awaitVaultTest(t, func(done func([]byte, error)) {
		decryptDataset(env, "debt-envelope-test-passcode", done)
	})
	if err != nil {
		t.Fatalf("decrypt dataset: %v", err)
	}
	if !strings.Contains(string(decrypted), `"debtPaymentLinks"`) || !strings.Contains(string(decrypted), `"debtPayeeRules"`) {
		t.Fatalf("encrypted dataset plaintext does not contain debt-payment fields: %s", decrypted)
	}
	got, err := store.Import(decrypted)
	if err != nil {
		t.Fatalf("import decrypted dataset: %v", err)
	}
	if len(got.DebtPaymentLinks) != 1 || got.DebtPaymentLinks[0].ID != links[0].ID || len(got.DebtPayeeRules) != 1 || got.DebtPayeeRules[0].ID != rules[0].ID {
		t.Fatalf("decrypted debt-payment records = links %#v, rules %#v", got.DebtPaymentLinks, got.DebtPayeeRules)
	}
}
