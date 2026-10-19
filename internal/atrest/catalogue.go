// SPDX-License-Identifier: MIT

// Package atrest defines the storage sensitivity catalogue and encrypted
// record envelope used by CashFlux local storage.
package atrest

import "strings"

// Class is the storage protection requirement for a key.
type Class uint8

const (
	// MustEncrypt means the value must be encrypted before it is persisted.
	MustEncrypt Class = iota + 1
	// MayStayPlain means the value contains only low-sensitivity chrome or
	// routing metadata and may remain readable at rest.
	MayStayPlain
	// ShouldNotPersist means the value is transient or a secret that must not
	// be written to JavaScript-readable storage.
	ShouldNotPersist
)

// String returns the catalogue label for c.
func (c Class) String() string {
	switch c {
	case MustEncrypt:
		return "MUST-ENCRYPT"
	case MayStayPlain:
		return "MAY-STAY-PLAIN"
	case ShouldNotPersist:
		return "SHOULD-NOT-PERSIST"
	default:
		return "UNKNOWN"
	}
}

// Entry describes one exact key or prefix in the storage sensitivity
// catalogue. A trailing colon or asterisk marks a prefix match.
type Entry struct {
	// Key is an exact key or a prefix ending in a colon or asterisk.
	Key string
	// Class is the required storage treatment.
	Class Class
	// Reason gives the one-line reason for this classification.
	Reason string
}

var catalogue = []Entry{
	// Core records, copies and credentials.
	{"dataset", MustEncrypt, "Contains the complete household financial dataset."},
	{"dataset:gen", MustEncrypt, "Cross-tab generation reveals household activity timing."},
	{"seeded", MayStayPlain, "Only records whether sample data has been initialized."},
	{"sampleActive", MayStayPlain, "Only records whether the sample banner is active."},
	{"checkpoints", MustEncrypt, "Checkpoint index metadata accompanies financial snapshots."},
	{"checkpoint-data:", MustEncrypt, "Contains a complete household snapshot."},
	{"sync-queue", MustEncrypt, "Queued changes contain household data and account linkage."},
	{"sync-conflict:", MustEncrypt, "Conflicts retain the rejected household dataset."},
	{"quarantine:", MustEncrypt, "Unreadable source bytes may contain plaintext financial data."},
	{"workspaces", MustEncrypt, "Workspace names and membership reveal household identity."},
	{"ws-data:", MustEncrypt, "Workspace bundles can contain the complete dataset and filters."},
	{"sync-meta:", MustEncrypt, "Sync bookmarks reveal workspace ownership and activity."},
	{"sync-device-id", MustEncrypt, "A stable device identifier links household activity."},
	{"sync-status", MustEncrypt, "Sync errors and state can disclose account or workspace details."},
	{"dataset-owner", MustEncrypt, "Dataset ownership identifies the household account."},
	{"auth:user-id", MustEncrypt, "Account identity is linkable personal metadata."},
	{"auth:access-token", ShouldNotPersist, "Access tokens belong in short-lived memory, not browser storage."},
	{"auth:refresh-token", ShouldNotPersist, "Refresh tokens must not be exposed to JavaScript storage readers."},
	{"auth:expires-in-seconds", MustEncrypt, "Token lifetime metadata is part of the authentication record."},
	{"auth:refresh-mode", MustEncrypt, "Authentication mode is account-linked metadata."},
	{"auth:csrf", ShouldNotPersist, "CSRF credentials should remain transient or protected by cookie policy."},
	{"prefs", MustEncrypt, "Preferences may contain a server URL and static bearer token."},
	{"openai-key", ShouldNotPersist, "Raw AI API keys must not be persisted in JavaScript-readable storage."},
	{"websearch-key", ShouldNotPersist, "Raw search API keys must not be persisted in JavaScript-readable storage."},
	{"cloud-ai-key-set", MustEncrypt, "AI credential presence reveals configured private services."},
	{"cred:openai", MustEncrypt, "Remembered AI credentials require encryption at rest."},
	{"credvault", MustEncrypt, "Contains bank-login usernames, passwords, and security notes."},
	{"chat-system-prompt", MustEncrypt, "User-authored AI prompts may contain personal information."},
	{"applock", MustEncrypt, "Contains passcode verifier material and lock configuration."},
	{"webauthn-credid", MustEncrypt, "Passkey credential metadata identifies an authenticator."},
	{"webauthn-salt", MustEncrypt, "Passkey derivation salt belongs to the protected keyring metadata."},
	{"webauthn-vault", MustEncrypt, "Contains wrapped key material required to unlock stored data."},
	{"member-pins", MustEncrypt, "Contains member access-control secrets."},
	{"pairing-code", ShouldNotPersist, "Short-lived pairing codes must never be stored in browser storage."},
	{"pairing-code:", ShouldNotPersist, "Short-lived pairing codes must never be stored in browser storage."},
	{"recovery-code", ShouldNotPersist, "One-time recovery codes must never be persisted."},
	{"recovery-code:", ShouldNotPersist, "One-time recovery codes must never be persisted."},
	{"passcode", ShouldNotPersist, "Raw passcodes must remain transient in memory."},
	{"passcode:", ShouldNotPersist, "Raw passcodes must remain transient in memory."},
	{"active-identity", MustEncrypt, "Identifies the selected household member."},
	{"active-member", MustEncrypt, "Identifies the selected household member."},
	{"active-scope", MustEncrypt, "Reveals the active household data scope."},
	{"accounts-class", MustEncrypt, "Account-class selection reveals household account structure."},

	// At-rest media and auxiliary stores.
	{"cf.artifactSalt", MayStayPlain, "A random salt is not secret and contains no household data."},
	{"artifact:", MustEncrypt, "Attachments and receipts can contain financial records and personal data."},
	{"cfv2:log:", MustEncrypt, "Change-log entries contain encrypted household operations."},
	{"cfv2:snapshot:", MustEncrypt, "Snapshots contain household state and must remain ciphertext."},
	{"cfv2:drafts:", MustEncrypt, "Drafts may contain unfinished financial edits."},
	{"cfv2:quarantine:", MustEncrypt, "Quarantine must not retain plaintext source data."},
	{"cfv2:keyring:", MustEncrypt, "Keyring material controls access to workspace data keys."},
	{"cfv2:meta:", MayStayPlain, "Minimal scope and cursor metadata is needed for local routing."},
	{"cfv2:local", MayStayPlain, "This is a local database namespace identifier only."},
	{"cache:cashflux-*", MayStayPlain, "The audited service-worker cache is for static application assets."},

	// Low-sensitivity chrome keys from the ENC0 inventory.
	{"theme", MayStayPlain, "Theme choice is a low-sensitivity display preference."},
	{"fonts", MayStayPlain, "Font selection is display configuration without household facts."},
	{"banner", MayStayPlain, "Banner dismissal is low-sensitivity presentation state."},
	{"active-lang", MayStayPlain, "Language choice is a low-sensitivity display preference."},
	{"languages", MayStayPlain, "Available language selection is display configuration."},
	{"rail-collapsed", MayStayPlain, "Sidebar collapse state is presentation-only."},
	{"rail-tool-groups", MayStayPlain, "Tool-group visibility is presentation-only."},
	{"txn-cols", MayStayPlain, "Transaction column order is a display preference."},
	{"layout", MayStayPlain, "Layout positions are presentation-only."},
	{"layout-mode", MayStayPlain, "Layout mode is a display preference."},
	{"nav-order", MayStayPlain, "Navigation order is presentation-only."},
	{"nav-favorites", MayStayPlain, "Navigation favorites are low-sensitivity display state."},
	{"nav-favorites-set", MayStayPlain, "Navigation favorites are low-sensitivity display state."},
	{"hidden-modules", MayStayPlain, "Module visibility is presentation-only."},
	{"hidden-widgets", MayStayPlain, "Widget visibility is presentation-only."},
	{"budgets:density", MayStayPlain, "Budget density changes presentation without storing budget values."},
	{"notify:browser", MayStayPlain, "Browser notification permission is a presentation preference."},
	{"muzak", MayStayPlain, "Music preference is low-sensitivity presentation state."},
	{"muzak-pos", MayStayPlain, "Music position contains no household financial information."},
	{"muzak-volume", MayStayPlain, "Music volume contains no household financial information."},
	{"muzak-noticed", MayStayPlain, "Music onboarding dismissal is low-sensitivity state."},
	{"onboard-dismissed", MayStayPlain, "Onboarding dismissal is low-sensitivity UI state."},
	{"setup:currencyConfirmed", MayStayPlain, "Setup confirmation is a low-sensitivity UI flag."},
	{"setup:wizardDone", MayStayPlain, "Setup completion is a low-sensitivity UI flag."},
	{"wb-canvas-pos", MayStayPlain, "Canvas position is presentation-only."},
	{"wb-canvas-view", MayStayPlain, "Canvas viewport is presentation-only."},
	{"wb-cards", MayStayPlain, "Canvas card arrangement is presentation-only."},
	{"wb-toggle:", MayStayPlain, "Canvas display toggles are presentation-only."},
	{"dash-preset", MayStayPlain, "Dashboard preset selects a display arrangement."},
	{"dash-daily-nudge", MayStayPlain, "Dashboard nudge dismissal is low-sensitivity UI state."},
	{"version", MayStayPlain, "Build and migration version flags contain no household facts."},
	{"release", MayStayPlain, "Build and migration flags contain no household facts."},

	// The rest of the ENC0 key inventory is personal, behavioral, or financial.
	{"tx-filter", MustEncrypt, "Saved filters can reveal transaction searches and merchants."},
	{"widget-config", MustEncrypt, "Widget configuration can retain financial plans and values."},
	{"period-*", MustEncrypt, "Period settings can encode financial reporting context."},
	{"allocate:", MustEncrypt, "Allocation settings contain household financial targets."},
	{"debt:", MustEncrypt, "Debt settings contain financial assumptions and balances."},
	{"networth:", MustEncrypt, "Net-worth settings contain financial balances and assumptions."},
	{"networth-view", MustEncrypt, "Net-worth view state reveals financial accounts and activity."},
	{"planning:", MustEncrypt, "Planning settings contain household financial plans."},
	{"reports:", MustEncrypt, "Report settings can contain financial filters and snapshots."},
	{"report-*", MustEncrypt, "Report filters and snapshots reveal financial activity."},
	{"saved-reports", MustEncrypt, "Saved reports contain financial views and results."},
	{"retirement-plan", MustEncrypt, "Retirement plans contain financial assumptions and targets."},
	{"tax-plan", MustEncrypt, "Tax plans contain sensitive financial assumptions."},
	{"rebalance-targets", MustEncrypt, "Rebalancing targets contain investment allocations."},
	{"invest:", MustEncrypt, "Investment pools contain financial allocations."},
	{"goals:", MustEncrypt, "Goal settings contain financial targets and progress."},
	{"budgets:", MustEncrypt, "Budget settings contain household financial plans."},
	{"bills:", MustEncrypt, "Bill settings contain payment and household financial data."},
	{"health:", MustEncrypt, "Health trends are sensitive personal information."},
	{"benchmark-series", MustEncrypt, "Benchmark data can expose household financial assumptions."},
	{"notify:delivered", MustEncrypt, "Delivery records may include activity and entity details."},
	{"notify:feed", MustEncrypt, "Notification feed can disclose merchants and amounts."},
	{"notifhistory", MustEncrypt, "Notification history can reveal household activity."},
	{"notify:lastSeen", MustEncrypt, "Notification timing reveals household behavior."},
	{"notify:routes", MustEncrypt, "Notification routes reveal activity and feature usage."},
	{"smart-digest:", MustEncrypt, "Digest state is derived from household activity."},
	{"smart-settings", MustEncrypt, "Smart settings can encode household behavior and decisions."},
	{"subs-*", MustEncrypt, "Subscription detection settings reveal merchant behavior."},
	{"subs:", MustEncrypt, "Confirmed subscriptions reveal merchant and payment behavior."},
	{"rhythm-*", MustEncrypt, "Routine settings reveal household behavioral patterns."},
	{"learn-*", MustEncrypt, "Learning data records merchant and category behavior."},
	{"freshness-dismissals", MustEncrypt, "Dismissals reveal account freshness and behavior."},
	{"review-resolutions", MustEncrypt, "Review decisions can reveal financial data and actions."},
	{"flag-verdicts", MustEncrypt, "Verdicts may include private transaction decisions."},
	{"agent-memory", MustEncrypt, "Agent memory can retain private household context."},
	{"assistant-privacy-default", MustEncrypt, "Assistant privacy choices reveal personal configuration."},
	{"import-wip", MustEncrypt, "Import state can reveal financial document activity."},
	{"reconcile:draft:", MustEncrypt, "Reconciliation drafts contain account balances and dates."},
	{"txntemplates", MustEncrypt, "Transaction templates can expose merchants and amounts."},
	{"todo-views", MustEncrypt, "Task views can expose private household plans."},
	{"tasksuggest:dismissed", MustEncrypt, "Task suggestions derive from household activity."},
	{"wins-seen", MustEncrypt, "Wins history can reveal household progress."},
	{"visit:", MustEncrypt, "Visit timing reveals personal usage behavior."},
	{"wb-graph", MustEncrypt, "Workflow graph can contain household plans and values."},
	{"standing-instructions", MustEncrypt, "Standing instructions can encode financial actions."},
	{"saved-view-threshold-dismissals", MustEncrypt, "Saved-view choices reveal personal financial behavior."},
	{"app:explainSeed", MustEncrypt, "Explanation context can contain household financial data."},
	{"inflation-pct", MustEncrypt, "Inflation assumptions are part of financial planning."},
	{"backupCadence", MustEncrypt, "Backup configuration may reveal household data practices."},
	{"lastBackupAt", MustEncrypt, "Backup timing is household activity metadata."},
	{"cloud-mention-snoozed", MustEncrypt, "AI mention behavior can reveal private workflow context."},
}

// Catalog returns a copy of the complete known-key catalogue.
func Catalog() []Entry {
	return append([]Entry(nil), catalogue...)
}

// Classify returns the protection class for key. Unknown keys default to
// MustEncrypt so new storage cannot silently become plaintext.
func Classify(key string) Class {
	return ClassifyEntry(key).Class
}

// ClassifyEntry returns the matching catalogue entry, or a fail-closed entry
// for an unknown key.
func ClassifyEntry(key string) Entry {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "cashflux:") {
		key = strings.TrimPrefix(key, "cashflux:")
	}
	for _, entry := range catalogue {
		if strings.HasSuffix(entry.Key, ":") || strings.HasSuffix(entry.Key, "*") {
			prefix := strings.TrimSuffix(entry.Key, "*")
			if strings.HasPrefix(key, prefix) {
				return entry
			}
		} else if key == entry.Key {
			return entry
		}
	}
	return Entry{Key: key, Class: MustEncrypt, Reason: "Unknown storage keys are encrypted by default."}
}
