// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/emergencypack"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/store"
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
	if a.ready() != nil {
		return nil
	}
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
// referenced it back to no-institution (AC10 reassign-on-delete). Only the accounts that
// actually changed are written, in the same commit as the institution's removal, so a
// failure leaves both the accounts and the institution as they were.
func (a *App) DeleteInstitution(instID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	accounts, err := a.AccountsChecked()
	if err != nil {
		return fmt.Errorf("appstate: delete institution: %w", err)
	}
	original := make(map[string]domain.Account, len(accounts))
	for _, acc := range accounts {
		original[acc.ID] = acc
	}
	var changes []store.RecordChange
	for _, acc := range domain.ReassignAccountsOnInstitutionDelete(accounts, instID) {
		before, err := json.Marshal(original[acc.ID])
		if err != nil {
			return fmt.Errorf("appstate: encode account %q: %w", acc.ID, err)
		}
		after, err := json.Marshal(acc)
		if err != nil {
			return fmt.Errorf("appstate: encode reassigned account %q: %w", acc.ID, err)
		}
		changes = append(changes, store.RecordChange{Table: "accounts", ID: acc.ID, Before: before, After: after})
	}
	reassigned := len(changes)
	inst, found, err := a.store.GetInstitution(instID)
	if err != nil {
		return fmt.Errorf("appstate: read institution %q: %w", instID, err)
	}
	if found {
		before, err := json.Marshal(inst)
		if err != nil {
			return fmt.Errorf("appstate: encode institution %q: %w", instID, err)
		}
		changes = append(changes, store.RecordChange{Table: "institutions", ID: instID, Before: before})
	}
	if err := a.store.ReplayChanges(changes); err != nil {
		return fmt.Errorf("appstate: delete institution %q: %w", instID, err)
	}
	a.log.Info("entity deleted", "entity", "institution", "id", instID, "existed", found, "reassigned", reassigned)
	return nil
}

// --- AC13: projected balance on the row ---

// ProjectAccount projects one account's balance over the next `horizonDays` days
// from today's exact ledger balance and its recurring cash flows. Recurring amounts are cash flows
// (negative leaves the account), so the projection runs in the stated convention: a liability's
// stored balance is normalised with its own liability sign first, and the result reads
// negative-means-owed. Ledger, currency, and arithmetic errors are returned; a missing account
// remains a valid empty projection at zero.
func (a *App) ProjectAccount(accountID string, asOf time.Time, horizonDays int) (acctproject.Projection, error) {
	if err := a.ready(); err != nil {
		return acctproject.Projection{}, err
	}
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
	if err != nil {
		return acctproject.Projection{}, fmt.Errorf("appstate: project account %s balance: %w", accountID, err)
	}
	return accountview.PrepareProjectionInputs(a.Recurring()).ProjectFor(acc, bal, asOf, horizonDays)
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

// institutionAccountsKey is the key the emergency pack groups an institution's accounts under: the
// cleaned name, so accounts that only carry the name as text land with the directory entry, or the
// entry's ID when the name is blank. The placeholder shown for a blank name is never a key.
func institutionAccountsKey(in domain.Institution) string {
	if name := in.CleanName(); name != "" {
		return name
	}
	return "id:" + in.ID
}

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
	if a.ready() != nil {
		return emergencypack.Pack{GeneratedAt: now, OwnerName: ownerName, VaultHasEntries: vaultHasEntries, Contacts: contacts, ClosingNote: closingNote}
	}
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
		instName, instKey := acc.Institution, acc.Institution
		if in, ok := instByID[acc.InstitutionID]; ok {
			instName, instKey = in.TrimmedName(), institutionAccountsKey(in)
		}
		if instKey != "" {
			instAccounts[instKey] = append(instAccounts[instKey], acc.Name)
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
			Accounts:     instAccounts[institutionAccountsKey(in)],
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
