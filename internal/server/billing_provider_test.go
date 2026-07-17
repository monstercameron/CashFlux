// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestPaymentProviderRegistry(t *testing.T) {
	if p, ok := paymentProvider(""); !ok || p.Name() != "stripe" {
		t.Fatalf("empty provider = %v/%v, want stripe", p, ok)
	}
	if p, ok := paymentProvider("STRIPE"); !ok || p.Name() != "stripe" {
		t.Fatalf("case-insensitive stripe = %v/%v", p, ok)
	}
	if p, ok := paymentProvider("paypal"); !ok || p.Name() != "paypal" {
		t.Fatalf("paypal provider = %v/%v", p, ok)
	}
	if _, ok := paymentProvider("venmo"); ok {
		t.Fatal("unknown provider resolved")
	}
	// Configured gates: stripe needs a secret key, paypal needs client id+secret.
	if (stripeProvider{}).Configured(Config{}) {
		t.Fatal("stripe reported configured with no secret key")
	}
	if !(stripeProvider{}).Configured(Config{StripeSecretKey: "sk_test"}) {
		t.Fatal("stripe not configured with a secret key")
	}
	if (paypalProvider{}).Configured(Config{PayPalClientID: "id"}) {
		t.Fatal("paypal configured with only a client id")
	}
}

func TestStripeProviderVerifyWebhook(t *testing.T) {
	cfg := Config{StripeWebhookSecret: "whsec_test"}
	now := time.Now().UTC()
	body := []byte(`{"id":"evt_123","type":"customer.subscription.updated","data":{"object":{}}}`)

	header := http.Header{}
	header.Set(stripeSignatureHeader, testStripeSignature(t, body, cfg.StripeWebhookSecret, now))
	ev, err := (stripeProvider{}).VerifyWebhook(context.Background(), cfg, header, body, now)
	if err != nil {
		t.Fatalf("verify valid webhook: %v", err)
	}
	if ev.ID != "evt_123" || ev.Type != "customer.subscription.updated" {
		t.Fatalf("verified event = %+v", ev)
	}

	// A tampered body under the same signature must be rejected.
	bad := http.Header{}
	bad.Set(stripeSignatureHeader, testStripeSignature(t, body, cfg.StripeWebhookSecret, now))
	if _, err := (stripeProvider{}).VerifyWebhook(context.Background(), cfg, bad, append(body, ' '), now); err == nil {
		t.Fatal("tampered webhook body verified")
	}
}
