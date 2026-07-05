// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// billingStatus mirrors the server's GET /v1/billing/status response.
type billingStatus struct {
	Status           string `json:"status"`
	Plan             string `json:"plan,omitempty"`
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`
	TrialEnd         string `json:"trialEnd,omitempty"`
}

// fetchBillingStatus GETs the caller's subscription state from the backend. Runs
// off-thread; onDone receives the parsed status (or is not called on failure, so
// the banner simply stays hidden — a status fetch must never disrupt the app).
func fetchBillingStatus(endpoint, token string, onDone func(billingStatus)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/billing/status", nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return
		}
		var bs billingStatus
		if err := json.NewDecoder(resp.Body).Decode(&bs); err != nil {
			return
		}
		onDone(bs)
	}()
}

// daysUntil returns whole days from now to an RFC3339 timestamp (>=0), and whether
// the timestamp parsed.
func daysUntil(rfc3339 string) (int, bool) {
	if rfc3339 == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return 0, false
	}
	d := int(time.Until(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d, true
}

// subBannerFace maps a subscription status to its banner copy + class. Returns
// ok=false for states that need no banner (active, disabled, none, "").
func subBannerFace(bs billingStatus) (text, cls string, ok bool) {
	switch bs.Status {
	case "trialing":
		if d, has := daysUntil(bs.TrialEnd); has {
			return uistate.T("cloud.bannerTrialDays", d), "sub-banner sub-trial", true
		}
		return uistate.T("cloud.bannerTrial"), "sub-banner sub-trial", true
	case "past_due":
		return uistate.T("cloud.bannerPastDue"), "sub-banner sub-pastdue", true
	case "canceled":
		return uistate.T("cloud.bannerCanceled"), "sub-banner sub-canceled", true
	default:
		return "", "", false
	}
}

// SubscriptionBanner shows the Cloud account/subscription state end-to-end (§7.11):
// a trial-days-left note, a past-due grace banner, or a canceled → downgrade-to-local
// notice. It fetches the live status on mount and self-hides for active/free/local
// users. The local app never stops working regardless of state (graceful downgrade
// is inherent to the local-first architecture). Clicking opens Cloud settings.
func SubscriptionBanner() uic.Node {
	prefsAtom := uistate.UsePrefs()
	status := uic.UseState(billingStatus{})

	pr := prefsAtom.Get().Normalize()
	endpoint, token := pr.ServerURL, pr.ServerToken

	uic.UseEffect(func() func() {
		if strings.TrimSpace(endpoint) != "" && strings.TrimSpace(token) != "" {
			fetchBillingStatus(endpoint, token, func(bs billingStatus) { status.Set(bs) })
		}
		return nil
	}, endpoint+"\x00"+token)

	text, cls, ok := subBannerFace(status.Get())
	if !ok {
		return Fragment()
	}
	onClick := uic.UseEvent(func() { uistate.OpenGlobalSettingsAt("cloud") })
	return Button(ClassStr(cls+" "+tw.Fold(tw.WFull, tw.TextLeft)), Type("button"),
		Attr("role", "status"), OnClick(onClick), text)
}
