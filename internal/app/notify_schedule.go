// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/unusual"
)

type notifyCandidateJob struct {
	generators         []func() []notify.Candidate
	position           int
	candidates         []notify.Candidate
	config             notify.RuleConfig
	app                *appstate.App
	now                time.Time
	txns               []domain.Transaction
	lastID             string
	loaded             bool
	err                error
	merchant           *unusual.MerchantJob
	merchantDone       bool
	merchantPosition   int
	merchantCandidates []notify.Candidate
}

func newNotifyCandidateJob(app *appstate.App, now time.Time, config notify.RuleConfig) *notifyCandidateJob {
	return &notifyCandidateJob{app: app, now: now, config: config}
}

// Step executes one producer; no producer runs in the transaction-write observer.
func (j *notifyCandidateJob) Step() bool {
	if !j.loaded {
		page, err := j.app.Store().TransactionsPage(j.lastID, 64)
		if err != nil {
			j.err = err
			return true
		}
		j.txns = append(j.txns, page...)
		if len(page) > 0 {
			j.lastID = page[len(page)-1].ID
		}
		if len(page) < 64 {
			j.loaded = true
			base := j.app.Settings().BaseCurrency
			if base == "" {
				base = "USD"
			}
			var floor int64
			for _, rule := range notify.DefaultRules() {
				if rule.ID == "default-unusual" {
					floor = notify.EffectiveThreshold(rule.ID, j.config, int64(rule.Threshold))
				}
			}
			j.merchant = unusual.NewMerchantJob(j.txns, floor, j.now.AddDate(0, 0, -30), currency.Rates{Base: base, Rates: j.app.Settings().FXRates})
		}
		return false
	}

	if !j.merchantDone {
		if !j.merchant.Step(64) {
			return false
		}
		findings := j.merchant.Findings()
		if j.merchant.Err() != nil {
			findings = nil
		}
		end := min(j.merchantPosition+32, len(findings))
		for ; j.merchantPosition < end; j.merchantPosition++ {
			j.merchantCandidates = append(j.merchantCandidates, unusualNotificationCandidate(findings[j.merchantPosition], j.app.Settings().BaseCurrency))
		}
		if j.merchantPosition < len(findings) {
			return false
		}
		j.merchantDone = true
		j.generators = notifyCandidateGenerators(j.app, j.txns, j.merchantCandidates, j.now, j.config)
		return false
	}
	if j.position == len(j.generators) {
		return true
	}
	candidates := j.generators[j.position]()
	j.position++
	for i := range candidates {
		candidates[i].MemberID = j.config.MemberFor(candidates[i].RuleID)
	}
	j.candidates = append(j.candidates, candidates...)
	return j.position == len(j.generators)
}

var notifySchedule struct {
	epoch uint64
	timer *time.Timer
}

func scheduleNotifyCatchUp() {
	notifySchedule.epoch++
	epoch := notifySchedule.epoch
	if notifySchedule.timer != nil {
		notifySchedule.timer.Stop()
	}
	app := appstate.Default
	if app == nil {
		return
	}
	revision, workspace := app.Rev(), recordWorkspace()
	now := time.Now()
	config := notify.UnmarshalRuleConfig(uistate.SettingKVGet(notify.RuleConfigKey()))
	var job *notifyCandidateJob
	var step func()
	step = func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				app.Log().Error("notification job failed", "panic", recovered)
				uistate.PostNotice(uistate.T("notify.catchUpError"), true)
			}
		}()
		if epoch != notifySchedule.epoch || appstate.Default != app || recordWorkspace() != workspace || suspendAutosave {
			return
		}
		if app.Rev() != revision {
			scheduleNotifyCatchUp()
			return
		}
		if job == nil {
			job = newNotifyCandidateJob(app, now, config)
		}
		start := time.Now()
		done := job.Step()
		if notifySliceObserve != nil {
			notifySliceObserve(time.Since(start))
		}
		if epoch != notifySchedule.epoch || recordWorkspace() != workspace {
			return
		}
		if app.Rev() != revision {
			scheduleNotifyCatchUp()
			return
		}
		if !done {
			notifySchedule.timer = time.AfterFunc(time.Millisecond, step)
			return
		}
		if job.err != nil {
			app.Log().Warn("notification candidates could not be read", "err", job.err)
			return
		}
		app.RequestDurable(func(result appstate.DurableResult) {
			if result.Err != nil || epoch != notifySchedule.epoch || recordWorkspace() != workspace {
				return
			}
			if app.Rev() != revision {
				scheduleNotifyCatchUp()
				return
			}
			if notifyPublishObserve != nil {
				notifyPublishObserve(revision)
			}
			publishNotifyCatchUp(app, now, config, job.candidates)
		})
	}
	notifySchedule.timer = time.AfterFunc(20*time.Millisecond, step)
}

var notifySliceObserve func(time.Duration)

var notifyPublishObserve func(uint64)
