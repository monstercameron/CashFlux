// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// PaymentProvider abstracts a subscription payment backend (Stripe, PayPal) behind
// one seam so the billing routes and the entitlement layer stay provider-neutral.
// A provider owns its own checkout/portal redirect flow and its own webhook
// signature scheme and event shape; the generic handler orchestrates auth,
// idempotency, and replay dedupe around these methods.
type PaymentProvider interface {
	// Name is the stable provider id ("stripe" | "paypal") stored on subscriptions
	// and used to route webhooks.
	Name() string
	// Configured reports whether this provider has the secrets/ids it needs to run,
	// so an unconfigured provider is offered to nobody.
	Configured(cfg Config) bool
	// Checkout starts a subscription purchase for the given plan interval
	// ("annual"|"monthly", empty = annual) and returns a redirect URL plus the
	// resolved plan id recorded on the pending subscription.
	Checkout(ctx context.Context, cfg Config, userID, interval string) (redirectURL, plan string, err error)
	// Portal returns a manage/cancel URL for an existing subscriber.
	Portal(ctx context.Context, cfg Config, sub Subscription) (redirectURL string, err error)
	// VerifyWebhook authenticates a raw webhook against the provider's signature
	// scheme and returns the parsed event. A non-nil error means "reject".
	VerifyWebhook(ctx context.Context, cfg Config, header http.Header, body []byte, now time.Time) (WebhookEvent, error)
	// ApplyWebhook maps a verified event to a subscription mutation and applies it
	// to the store. A no-op event returns nil without touching the store.
	ApplyWebhook(store *Store, ev WebhookEvent, now time.Time, metrics *Metrics) error
}

// WebhookEvent is a provider-verified webhook: a stable ID for replay dedupe, the
// provider's event type, and the raw provider payload the provider knows how to
// map in ApplyWebhook.
type WebhookEvent struct {
	ID   string
	Type string
	Raw  json.RawMessage
}

// paymentProviders builds the registry of configured providers keyed by name.
// Stripe is always registered (it may be unconfigured — Configured gates use);
// PayPal is added once its config is present.
func paymentProviders() map[string]PaymentProvider {
	providers := map[string]PaymentProvider{
		"stripe": stripeProvider{},
		"paypal": paypalProvider{},
	}
	return providers
}

// ConfiguredPaymentProviders returns the names of providers with complete
// configuration (billing only — the caller gates on cfg.Billing separately),
// sorted, so the client's version discovery offers only working buttons. Only
// billing-enabled deployments have any.
func (c Config) ConfiguredPaymentProviders() []string {
	if !c.Billing {
		return nil
	}
	var names []string
	for name, p := range paymentProviders() {
		if p.Configured(c) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// paymentProvider resolves a provider by name, defaulting to Stripe when the name
// is empty. ok is false for an unknown name.
func paymentProvider(name string) (PaymentProvider, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "stripe"
	}
	p, ok := paymentProviders()[name]
	return p, ok
}

// --- Stripe provider (delegates to the existing, tested Stripe helpers) --------

type stripeProvider struct{}

func (stripeProvider) Name() string { return "stripe" }

func (stripeProvider) Configured(cfg Config) bool {
	return strings.TrimSpace(cfg.StripeSecretKey) != ""
}

func (stripeProvider) Checkout(ctx context.Context, cfg Config, userID, interval string) (string, string, error) {
	price, plan, err := stripePriceForIntervalValue(cfg, interval)
	if err != nil {
		return "", "", err
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("success_url", strings.TrimSpace(cfg.StripeSuccessURL))
	form.Set("cancel_url", strings.TrimSpace(cfg.StripeCancelURL))
	form.Set("client_reference_id", userID)
	form.Set("line_items[0][price]", price)
	form.Set("line_items[0][quantity]", "1")
	form.Set("metadata[user_id]", userID)
	form.Set("metadata[plan]", plan)
	form.Set("subscription_data[metadata][user_id]", userID)
	form.Set("subscription_data[metadata][plan]", plan)
	form.Set("allow_promotion_codes", "true")
	sessionURL, err := createStripeSession(ctx, cfg, "/checkout/sessions", form)
	if err != nil {
		return "", "", err
	}
	return sessionURL, plan, nil
}

func (stripeProvider) Portal(ctx context.Context, cfg Config, sub Subscription) (string, error) {
	if strings.TrimSpace(sub.ProviderCustomer) == "" {
		return "", fmt.Errorf("stripe customer is not configured")
	}
	form := url.Values{}
	form.Set("customer", sub.ProviderCustomer)
	form.Set("return_url", strings.TrimSpace(cfg.StripePortalReturnURL))
	return createStripeSession(ctx, cfg, "/billing_portal/sessions", form)
}

func (stripeProvider) VerifyWebhook(_ context.Context, cfg Config, header http.Header, body []byte, now time.Time) (WebhookEvent, error) {
	secret := strings.TrimSpace(cfg.StripeWebhookSecret)
	if secret == "" {
		return WebhookEvent{}, fmt.Errorf("stripe webhook secret is not configured")
	}
	if !validStripeSignature(header.Get(stripeSignatureHeader), body, secret, now) {
		return WebhookEvent{}, fmt.Errorf("stripe signature is invalid")
	}
	var event stripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return WebhookEvent{}, fmt.Errorf("stripe event is invalid")
	}
	return WebhookEvent{ID: strings.TrimSpace(event.ID), Type: strings.TrimSpace(event.Type), Raw: body}, nil
}

func (stripeProvider) ApplyWebhook(store *Store, ev WebhookEvent, now time.Time, metrics *Metrics) error {
	var event stripeEvent
	if err := json.Unmarshal(ev.Raw, &event); err != nil {
		return fmt.Errorf("stripe event is invalid")
	}
	return applyStripeEvent(store, event, now, metrics)
}

// stripePriceForIntervalValue is the non-HTTP twin of stripePriceForInterval: it
// resolves the (price, plan) for an interval and returns an error instead of
// writing an HTTP response, so the provider seam stays transport-free.
func stripePriceForIntervalValue(cfg Config, interval string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "", "annual", "yearly":
		price := strings.TrimSpace(cfg.StripePriceAnnual)
		if price == "" {
			return "", "", fmt.Errorf("annual stripe price is not configured")
		}
		return price, "personal_annual", nil
	case "monthly":
		price := strings.TrimSpace(cfg.StripePriceMonthly)
		if price == "" {
			return "", "", fmt.Errorf("monthly stripe price is not configured")
		}
		return price, "personal_monthly", nil
	default:
		return "", "", fmt.Errorf("billing interval is invalid")
	}
}
