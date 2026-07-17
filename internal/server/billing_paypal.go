// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// paypalProvider implements PaymentProvider against PayPal's Subscriptions API.
// This file currently carries the seam and configuration gate; the live API
// calls (create subscription, webhook verification, event mapping) are filled in
// by the PayPal integration phase. Until configured it reports Configured=false,
// so it is offered to nobody and every method fails closed.
type paypalProvider struct{}

func (paypalProvider) Name() string { return "paypal" }

func (paypalProvider) Configured(cfg Config) bool {
	return strings.TrimSpace(cfg.PayPalClientID) != "" &&
		strings.TrimSpace(cfg.PayPalClientSecret) != ""
}

func (paypalProvider) Checkout(_ context.Context, _ Config, _, _ string) (string, string, error) {
	return "", "", fmt.Errorf("paypal is not configured")
}

func (paypalProvider) Portal(_ context.Context, _ Config, _ Subscription) (string, error) {
	return "", fmt.Errorf("paypal is not configured")
}

func (paypalProvider) VerifyWebhook(_ context.Context, _ Config, _ http.Header, _ []byte, _ time.Time) (WebhookEvent, error) {
	return WebhookEvent{}, fmt.Errorf("paypal is not configured")
}

func (paypalProvider) ApplyWebhook(_ *Store, _ WebhookEvent, _ time.Time, _ *Metrics) error {
	return fmt.Errorf("paypal is not configured")
}
