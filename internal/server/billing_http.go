package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const stripeSignatureHeader = "Stripe-Signature"

type stripeEvent struct {
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeSubscriptionObject struct {
	ID               string            `json:"id"`
	Customer         string            `json:"customer"`
	Status           string            `json:"status"`
	CurrentPeriodEnd int64             `json:"current_period_end"`
	TrialEnd         int64             `json:"trial_end"`
	Metadata         map[string]string `json:"metadata"`
	Items            struct {
		Data []struct {
			Price struct {
				ID        string `json:"id"`
				LookupKey string `json:"lookup_key"`
				Nickname  string `json:"nickname"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

type stripeCheckoutSessionObject struct {
	Customer          string            `json:"customer"`
	Subscription      string            `json:"subscription"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
}

type stripeInvoiceObject struct {
	Customer     string `json:"customer"`
	Subscription string `json:"subscription"`
}

func handleStripeWebhook(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		if !cfg.Billing {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "billing is disabled")
			return
		}
		secret := strings.TrimSpace(cfg.StripeWebhookSecret)
		if secret == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "stripe webhook secret is not configured")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "webhook body is invalid")
			return
		}
		if !validStripeSignature(r.Header.Get(stripeSignatureHeader), body, secret, time.Now().UTC()) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "stripe signature is invalid")
			return
		}
		var event stripeEvent
		if err := json.Unmarshal(body, &event); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "stripe event is invalid")
			return
		}
		if err := applyStripeEvent(store, event, time.Now().UTC()); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func validStripeSignature(header string, body []byte, secret string, now time.Time) bool {
	var timestamp, signature string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = strings.TrimSpace(value)
		case "v1":
			signature = strings.TrimSpace(value)
		}
	}
	if timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	eventTime := time.Unix(ts, 0).UTC()
	if now.Sub(eventTime) > 5*time.Minute || eventTime.Sub(now) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

func applyStripeEvent(store *Store, event stripeEvent, now time.Time) error {
	switch strings.TrimSpace(event.Type) {
	case "checkout.session.completed":
		var session stripeCheckoutSessionObject
		if err := json.Unmarshal(event.Data.Object, &session); err != nil {
			return fmt.Errorf("stripe checkout session is invalid")
		}
		userID := metadataValue(session.Metadata, "user_id", "cashflux_user_id")
		if userID == "" {
			userID = strings.TrimSpace(session.ClientReferenceID)
		}
		if userID == "" || strings.TrimSpace(session.Customer) == "" || strings.TrimSpace(session.Subscription) == "" {
			return fmt.Errorf("stripe checkout session is missing subscription identity")
		}
		return store.PutSubscription(Subscription{
			UserID:             userID,
			StripeCustomer:     session.Customer,
			StripeSubscription: session.Subscription,
			Status:             metadataValueDefault(session.Metadata, "trialing", "subscription_status", "status"),
			Plan:               metadataValueDefault(session.Metadata, "unknown", "plan", "price"),
			UpdatedAt:          now,
		})
	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripeSubscriptionObject
		if err := json.Unmarshal(event.Data.Object, &sub); err != nil {
			return fmt.Errorf("stripe subscription is invalid")
		}
		if event.Type == "customer.subscription.deleted" && strings.TrimSpace(sub.Status) == "" {
			sub.Status = "canceled"
		}
		return putStripeSubscription(store, sub, now)
	case "invoice.payment_failed":
		var invoice stripeInvoiceObject
		if err := json.Unmarshal(event.Data.Object, &invoice); err != nil {
			return fmt.Errorf("stripe invoice is invalid")
		}
		existing, ok, err := store.GetSubscriptionByStripeID(invoice.Subscription)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("stripe invoice subscription is unknown")
		}
		existing.Status = "past_due"
		existing.UpdatedAt = now
		if strings.TrimSpace(invoice.Customer) != "" {
			existing.StripeCustomer = invoice.Customer
		}
		return store.PutSubscription(existing)
	default:
		return nil
	}
}

func putStripeSubscription(store *Store, sub stripeSubscriptionObject, now time.Time) error {
	userID := metadataValue(sub.Metadata, "user_id", "cashflux_user_id")
	if userID == "" {
		if existing, ok, err := store.GetSubscriptionByStripeID(sub.ID); err != nil {
			return err
		} else if ok {
			userID = existing.UserID
		}
	}
	if userID == "" || strings.TrimSpace(sub.Customer) == "" || strings.TrimSpace(sub.ID) == "" ||
		strings.TrimSpace(sub.Status) == "" {
		return fmt.Errorf("stripe subscription is missing required fields")
	}
	return store.PutSubscription(Subscription{
		UserID:             userID,
		StripeCustomer:     sub.Customer,
		StripeSubscription: sub.ID,
		Status:             sub.Status,
		Plan:               stripeSubscriptionPlan(sub),
		CurrentPeriodEnd:   unixTime(sub.CurrentPeriodEnd),
		TrialEnd:           unixTime(sub.TrialEnd),
		UpdatedAt:          now,
	})
}

func stripeSubscriptionPlan(sub stripeSubscriptionObject) string {
	if plan := metadataValue(sub.Metadata, "plan", "price"); plan != "" {
		return plan
	}
	if len(sub.Items.Data) == 0 {
		return "unknown"
	}
	price := sub.Items.Data[0].Price
	for _, value := range []string{price.LookupKey, price.Nickname, price.ID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown"
}

func metadataValue(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func metadataValueDefault(metadata map[string]string, fallback string, keys ...string) string {
	if value := metadataValue(metadata, keys...); value != "" {
		return value
	}
	return fallback
}

func unixTime(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}
