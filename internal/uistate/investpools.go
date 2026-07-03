// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"
)

// investPoolsKey is the app-KV key holding the investment-pool config (persisted with the
// rest of the data — cleared by a wipe like any other stored config).
const investPoolsKey = "cashflux:invest:pools"

// InvestPool is a user-defined group of investment accounts, so the investments page can
// show a growth graph for a whole pool (e.g. "Retirement" = 401k + Roth IRA) as well as per
// account. Membership is by account ID; an account belongs to at most one pool.
type InvestPool struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	AccountIDs []string `json:"accountIds,omitempty"`
}

// InvestPools returns the persisted pools (empty when none configured).
func InvestPools() []InvestPool {
	raw := kvGet(investPoolsKey)
	if raw == "" {
		return nil
	}
	var pools []InvestPool
	if err := json.Unmarshal([]byte(raw), &pools); err != nil {
		return nil
	}
	return pools
}

// SetInvestPools persists the full pool set (empty clears it).
func SetInvestPools(pools []InvestPool) {
	if len(pools) == 0 {
		kvSet(investPoolsKey, "")
		return
	}
	if data, err := json.Marshal(pools); err == nil {
		kvSet(investPoolsKey, string(data))
	}
}

// DeleteInvestPool removes the pool with the given id (its accounts become ungrouped).
func DeleteInvestPool(id string) {
	pools := InvestPools()
	out := make([]InvestPool, 0, len(pools))
	for _, p := range pools {
		if p.ID != id {
			out = append(out, p)
		}
	}
	SetInvestPools(out)
}

// AssignAccountToPool moves an account into poolID, removing it from any other pool first.
// A blank poolID leaves the account ungrouped (removed from all pools).
func AssignAccountToPool(accountID, poolID string) {
	if accountID == "" {
		return
	}
	pools := InvestPools()
	for i := range pools {
		filtered := pools[i].AccountIDs[:0:0]
		for _, id := range pools[i].AccountIDs {
			if id != accountID {
				filtered = append(filtered, id)
			}
		}
		pools[i].AccountIDs = filtered
		if pools[i].ID == poolID {
			pools[i].AccountIDs = append(pools[i].AccountIDs, accountID)
		}
	}
	SetInvestPools(pools)
}

// UpsertInvestPool creates or updates a pool with the given id, name, and member accounts.
// Because an account belongs to at most one pool, the selected accounts are removed from any
// other pool first. A blank name is a no-op.
func UpsertInvestPool(id, name string, accountIDs []string) {
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return
	}
	sel := make(map[string]bool, len(accountIDs))
	for _, a := range accountIDs {
		sel[a] = true
	}
	pools := InvestPools()
	found := false
	for i := range pools {
		if pools[i].ID == id {
			pools[i].Name = name
			pools[i].AccountIDs = append([]string(nil), accountIDs...)
			found = true
			continue
		}
		// Strip any of the selected accounts from other pools (one pool per account).
		kept := pools[i].AccountIDs[:0:0]
		for _, aid := range pools[i].AccountIDs {
			if !sel[aid] {
				kept = append(kept, aid)
			}
		}
		pools[i].AccountIDs = kept
	}
	if !found {
		pools = append(pools, InvestPool{ID: id, Name: name, AccountIDs: append([]string(nil), accountIDs...)})
	}
	SetInvestPools(pools)
}

// PoolForAccount returns the id of the pool an account belongs to, or "" when ungrouped.
func PoolForAccount(accountID string) string {
	for _, p := range InvestPools() {
		for _, id := range p.AccountIDs {
			if id == accountID {
				return p.ID
			}
		}
	}
	return ""
}
