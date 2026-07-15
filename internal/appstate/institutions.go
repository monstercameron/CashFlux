// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/emergencypack"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// docExpiryTaskKey is the Task.Custom key under which a document-expiry reminder
// task records its stable docexpiry.Reminder key, so the reconcile can find and
// auto-resolve it later (AC17 ⇄ XC8).
const docExpiryTaskKey = "docExpiryKey"

// --- AC10: institution directory ---

// Institutions returns every institution in the directory. The institution layer
// grounds the ★★ Multi-Institution Analytics feature with a real entity instead of
// matching on the free-text Account.Institution string.
func (a *App) Institutions() []domain.Institution {
	v, err := a.store.ListInstitutions()
	a.logErr("institutions", err)
	return v
}

// PutInstitution upserts an institution (needs an ID and a name).
func (a *App) PutInstitution(in domain.Institution) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("appstate: institution needs an id")
	}
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("appstate: institution needs a name")
	}
	if err := a.store.PutInstitution(in); err != nil {
		return err
	}
	a.log.Info("institution saved", "id", in.ID)
	return nil
}

// DeleteInstitution removes an institution and reassigns every account that
// referenced it back to no-institution (AC10 reassign-on-delete), persisting only
// the accounts that actually changed.
func (a *App) DeleteInstitution(instID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	for _, acc := range domain.ReassignAccountsOnInstitutionDelete(a.Accounts(), instID) {
		if err := a.store.PutAccount(acc); err != nil {
			return fmt.Errorf("appstate: reassign account %q on institution delete: %w", acc.ID, err)
		}
	}
	return a.del("institution", instID, a.store.DeleteInstitution)
}

// --- AC13: projected balance on the row ---

// ProjectAccount projects one account's balance over the next `horizonDays` days
// from today's balance plus every recurring cash flow scoped to that account
// (paychecks, bills, transfers). It returns the running low point and the dated
// drivers behind it, so the row can show "$2,340 today → ~$1,150 low on the 28th"
// and expand to explain "rent −$1,400 on the 1st". A missing account yields a flat
// projection at zero.
func (a *App) ProjectAccount(accountID string, asOf time.Time, horizonDays int) acctproject.Projection {
	var acc domain.Account
	var found bool
	for _, x := range a.Accounts() {
		if x.ID == accountID {
			acc, found = x, true
			break
		}
	}
	if !found {
		return acctproject.Project(0, asOf, nil, horizonDays)
	}
	bal, err := ledger.Balance(acc, a.Transactions())
	a.logErr("project account balance", err)

	var drivers []acctproject.Driver
	end := asOf.AddDate(0, 0, horizonDays)
	for _, r := range a.Recurring() {
		if r.AccountID != accountID || r.Amount.IsZero() {
			continue
		}
		due := r.NextDue
		// Walk occurrences from NextDue up to the horizon end; cap iterations so a
		// misconfigured cadence can never spin.
		for i := 0; i < 400 && !due.After(end); i++ {
			if !due.Before(asOf) {
				drivers = append(drivers, acctproject.Driver{
					Label:  r.Label,
					Date:   due,
					Amount: r.Amount.Amount,
				})
			}
			nextDue := r.Cadence.Next(due)
			if !nextDue.After(due) {
				break // non-advancing cadence guard
			}
			due = nextDue
		}
	}
	return acctproject.Project(bal.Amount, asOf, drivers, horizonDays)
}

// --- AC17: document expiry reminders (reconciled against tasks) ---

// ReconcileDocExpiryTasks brings the document-renewal reminder tasks in line with
// the account documents as of `now`: it creates a to-do for any due, not-yet-tracked
// document expiry (AC17), and auto-resolves (completes) any existing reminder whose
// document has been renewed by a newer same-label document or had its expiry cleared
// (the XC8 self-resolving pattern). It is safe to call on every document mutation;
// it never spawns duplicates. Returns the number of tasks created and resolved.
func (a *App) ReconcileDocExpiryTasks(now time.Time, leadDays int) (created, resolved int, err error) {
	active := docexpiry.ActiveKeys(a.Accounts(), leadDays, now)

	existing := map[string]domain.Task{}
	for _, t := range a.Tasks() {
		if k, _ := t.Custom[docExpiryTaskKey].(string); k != "" {
			existing[k] = t
		}
	}

	// Resolve reminders whose document no longer needs one (renewed or expiry cleared).
	for k, t := range existing {
		if _, ok := active[k]; ok {
			continue
		}
		if t.Status == domain.StatusDone {
			continue
		}
		if e := a.CompleteTask(t.ID, id.New(), now); e != nil {
			return created, resolved, fmt.Errorf("appstate: resolve doc-expiry task: %w", e)
		}
		resolved++
	}

	// Create tasks for newly-due reminders we are not already tracking.
	for k, r := range active {
		if _, ok := existing[k]; ok {
			continue
		}
		title := "Renew " + r.Label
		if strings.TrimSpace(r.Label) == "" {
			title = "Renew a document"
		}
		t := domain.Task{
			ID:          id.New(),
			Title:       title,
			Notes:       "This document is coming up for renewal. It will clear itself when you file a newer version.",
			Due:         r.ExpiresAt,
			Status:      domain.StatusOpen,
			Priority:    domain.PriorityMedium,
			Source:      domain.SourceNudge,
			RelatedType: domain.RelatedAccount,
			RelatedID:   r.AccountID,
			Custom:      map[string]any{docExpiryTaskKey: k},
		}
		if e := a.PutTask(t); e != nil {
			return created, resolved, fmt.Errorf("appstate: create doc-expiry task: %w", e)
		}
		created++
	}
	return created, resolved, nil
}

// --- AC16: estate emergency pack ---

// BuildEmergencyPack assembles the "in case of emergency" pack (AC16) from the
// local dataset: accounts (with balances, beneficiary/TOD notes and filed
// documents), institutions and their support contacts, and any people-to-contact.
// It NEVER reads or includes credentials — vaultHasEntries is a plain boolean the
// caller supplies so the pack can point the reader to the encrypted vault without
// exposing it. ownerName and closingNote are the user's own words; contacts are
// supplied by the caller (there is no contacts entity yet). The returned Pack is a
// pure value the emergencypack renderers turn into printable text or HTML — no data
// leaves the machine.
func (a *App) BuildEmergencyPack(ownerName, closingNote string, contacts []emergencypack.Contact, vaultHasEntries bool, now time.Time) emergencypack.Pack {
	instByID := domain.InstitutionByID(a.Institutions())
	txns := a.Transactions()
	artByID := map[string]domain.Artifact{}
	for _, art := range a.Artifacts() {
		artByID[art.ID] = art
	}

	instAccounts := map[string][]string{}
	var accEntries []emergencypack.AccountEntry
	for _, acc := range a.Accounts() {
		if acc.Archived {
			continue
		}
		instName := acc.Institution
		if in, ok := instByID[acc.InstitutionID]; ok {
			instName = in.TrimmedName()
		}
		if instName != "" {
			instAccounts[instName] = append(instAccounts[instName], acc.Name)
		}
		bal, err := ledger.Balance(acc, txns)
		a.logErr("emergency pack balance", err)

		var docs []emergencypack.DocEntry
		for _, d := range domain.SortDocRefsByDate(acc.DocRefs) {
			docs = append(docs, emergencypack.DocEntry{
				Label:      d.DisplayLabel(artByID[d.ArtifactID].Name),
				AttachedOn: d.AttachedAt,
				ExpiresOn:  d.ExpiresAt,
			})
		}
		accEntries = append(accEntries, emergencypack.AccountEntry{
			Name:            acc.Name,
			Institution:     instName,
			Type:            acc.Type.String(),
			Balance:         bal.String(),
			BeneficiaryNote: acc.BeneficiaryNote,
			Notes:           acc.Notes,
			Documents:       docs,
		})
	}
	emergencypack.SortAccountsForPack(accEntries)

	var instEntries []emergencypack.InstitutionEntry
	for _, in := range a.Institutions() {
		instEntries = append(instEntries, emergencypack.InstitutionEntry{
			Name:         in.TrimmedName(),
			SupportPhone: in.SupportPhone,
			SupportURL:   in.SupportURL,
			Note:         in.Note,
			Accounts:     instAccounts[in.TrimmedName()],
		})
	}

	return emergencypack.Pack{
		GeneratedAt:     now,
		OwnerName:       ownerName,
		VaultHasEntries: vaultHasEntries,
		Accounts:        accEntries,
		Institutions:    instEntries,
		Contacts:        contacts,
		ClosingNote:     closingNote,
	}
}
