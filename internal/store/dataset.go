// Package store persists CashFlux data. This file defines the pure, platform-
// independent core: the Dataset aggregate, Settings, and schema-versioned JSON
// export/import (with migration). The IndexedDB-backed implementation lives in a
// separate, syscall/js file; this core unit-tests on native Go.
package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// PayoffBaseline snapshots the total debt owed when the user started tracking
// payoff progress, so "paid off $X since you started" can be measured against it.
type PayoffBaseline struct {
	TotalOwed int64     `json:"totalOwed"` // minor units, base currency
	Currency  string    `json:"currency"`
	StartedAt time.Time `json:"startedAt"`
}

// SchemaVersion is the current on-disk dataset schema version. Bump it when the
// shape changes and add a migration step in migrate.
const SchemaVersion = 1

// Settings holds app-wide configuration persisted alongside the data.
type Settings struct {
	BaseCurrency       string               `json:"baseCurrency"`
	FXRates            map[string]float64   `json:"fxRates,omitempty"`
	FXUpdatedAt        map[string]time.Time `json:"fxUpdatedAt,omitempty"` // when each FX rate was last set (L4 staleness)
	OpenAIKey          string               `json:"openAiKey,omitempty"`
	OpenAIModel        string               `json:"openAiModel,omitempty"`
	FreshnessOverrides map[string]int       `json:"freshnessOverrides,omitempty"`
	BudgetMethodology  string               `json:"budgetMethodology,omitempty"` // budgeting.Methodology; empty = simple
	PayoffBaseline     *PayoffBaseline      `json:"payoffBaseline,omitempty"`    // debt-payoff progress baseline (L5)
	Music              *MusicState          `json:"music,omitempty"`             // background-music resume point (checkpointed)
}

// MusicState is the background music's durable resume point: the on/off choice,
// volume, and the current track index + position. Checkpointed into the dataset
// (not streamed) so it travels with export/import and backups and resumes on a
// fresh device. The live, high-frequency position lives in localStorage.
type MusicState struct {
	Enabled  bool    `json:"enabled"`
	Volume   float64 `json:"volume"`
	Index    int     `json:"index"`
	Position float64 `json:"position"`
}

// Dataset is the complete CashFlux dataset: every entity plus settings. It is
// the unit of export, import, and (later) sync.
type Dataset struct {
	SchemaVersion             int                               `json:"schemaVersion"`
	Members                   []domain.Member                   `json:"members"`
	Accounts                  []domain.Account                  `json:"accounts"`
	Categories                []domain.Category                 `json:"categories"`
	Transactions              []domain.Transaction              `json:"transactions"`
	Budgets                   []domain.Budget                   `json:"budgets"`
	Goals                     []domain.Goal                     `json:"goals"`
	Tasks                     []domain.Task                     `json:"tasks"`
	CustomFields              []customfields.Def                `json:"customFieldDefs,omitempty"`
	Rules                     []rules.Rule                      `json:"rules,omitempty"`
	Documents                 []domain.Document                 `json:"documents,omitempty"`
	SavedInsights             []domain.SavedInsight             `json:"savedInsights,omitempty"`
	Conversations             []domain.Conversation             `json:"conversations,omitempty"`
	Recurring                 []domain.Recurring                `json:"recurring,omitempty"`
	AllocProfiles             []domain.AllocationProfile        `json:"allocProfiles,omitempty"`
	Formulas                  []domain.Formula                  `json:"formulas,omitempty"`
	Plans                     []domain.Plan                     `json:"plans,omitempty"`
	CustomPages               []domain.CustomPage               `json:"customPages,omitempty"`
	Artifacts                 []domain.Artifact                 `json:"artifacts,omitempty"`
	Workflows                 []workflow.Workflow               `json:"workflows,omitempty"`
	WorkflowRuns              []workflow.Run                    `json:"workflowRuns,omitempty"`
	SharedExpenses            []domain.SharedExpense            `json:"sharedExpenses,omitempty"`
	Settlements               []domain.Settlement               `json:"settlements,omitempty"`
	Earmarks                  []domain.Earmark                  `json:"earmarks,omitempty"`
	SubscriptionCancellations []domain.SubscriptionCancellation `json:"subscriptionCancellations,omitempty"`
	Settings                  Settings                          `json:"settings"`
}

// Export serializes the dataset to indented JSON, stamping the current schema
// version.
func Export(ds Dataset) ([]byte, error) {
	ds.SchemaVersion = SchemaVersion
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: export: %w", err)
	}
	return b, nil
}

// Import parses a dataset from JSON and migrates it to the current schema.
func Import(data []byte) (Dataset, error) {
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return Dataset{}, fmt.Errorf("store: import: %w", err)
	}
	migrated, err := migrate(ds)
	if err != nil {
		return Dataset{}, err
	}
	return migrated, nil
}

// migrate brings a decoded dataset up to the current schema version. Unversioned
// data (version 0) is treated as the initial release. Newer-than-supported data
// is rejected rather than silently mishandled.
func migrate(ds Dataset) (Dataset, error) {
	if ds.SchemaVersion == 0 {
		ds.SchemaVersion = 1
	}
	if ds.SchemaVersion > SchemaVersion {
		return Dataset{}, fmt.Errorf("store: dataset schema version %d is newer than supported version %d", ds.SchemaVersion, SchemaVersion)
	}
	// Future stepwise migrations go here (e.g. if ds.SchemaVersion < 2 { ... }).
	ds.SchemaVersion = SchemaVersion
	return ds, nil
}
