// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

type transactionWriteArgs struct {
	Target        string `json:"target"`
	TransactionID string `json:"transaction_id,omitempty"`
	Amount        string `json:"amount,omitempty"`
	Date          string `json:"date,omitempty"`
}

type transactionWritePlan struct {
	rows       []domain.Transaction
	survivorID string
	err        string
	used       bool
}

// agToolsTransactionWrites returns preview-bound transaction removal tools.
// Transfers are deliberately excluded: callers must use the existing transfer
// tools so paired-leg behavior stays in the appstate transfer engine.
func agToolsTransactionWrites(app *appstate.App) []chatTool {
	plans := map[string]*transactionWritePlan{}
	makeTool := func(name, description string, merge bool) chatTool {
		planKey := func(args transactionWriteArgs) string {
			b, _ := json.Marshal(args)
			return name + ":" + string(b)
		}
		prepare := func(raw json.RawMessage) (transactionWriteArgs, string, *transactionWritePlan) {
			args, err := parseTransactionWriteArgs(raw)
			if err != nil {
				return args, "", &transactionWritePlan{err: err.Error()}
			}
			key := planKey(args)
			plan := resolveTransactionWritePlan(app.Transactions(), args, merge)
			plans[key] = plan
			return args, key, plan
		}
		schema := json.RawMessage(`{"type":"object","properties":{"target":{"type":"string","description":"required payee or description phrase"},"transaction_id":{"type":"string","description":"optional exact transaction id to disambiguate"},"amount":{"type":"string","description":"optional exact absolute decimal amount, for example 38.49"},"date":{"type":"string","description":"optional exact calendar date in YYYY-MM-DD form"}},"required":["target"],"additionalProperties":false}`)
		return chatTool{
			spec:    ai.FunctionTool(name, description, schema),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				_, _, plan := prepare(raw)
				if plan.err != "" {
					return "Can't prepare this change: " + plan.err
				}
				return transactionWritePreview(plan, merge)
			},
			run: func(raw json.RawMessage) string {
				args, err := parseTransactionWriteArgs(raw)
				if err != nil {
					return "No change made: " + err.Error()
				}
				key := planKey(args)
				plan, exists := plans[key]
				if !exists {
					return "No change made: no matching approval preview is available. Request a new preview first."
				}
				if plan.err != "" {
					return "No change made: " + plan.err
				}
				if plan.used {
					return "This approved change was already processed; no further change was made."
				}
				if len(plan.rows) == 0 {
					return "No change made: " + plan.err
				}
				// Preview and execution are separate user-visible moments. Resolve the
				// exact target again from live state, and require the same rows before
				// entering the atomic expected-before appstate operation.
				fresh := resolveTransactionWritePlan(app.Transactions(), args, merge)
				if fresh.err != "" || !sameTransactionRows(plan.rows, fresh.rows) || fresh.survivorID != plan.survivorID {
					plan.used = true
					return "No change made: the matching transactions changed after approval was requested. Review the current matches and approve again."
				}
				plan.used = true
				var writeErr error
				if merge {
					writeErr = app.MergeDuplicateTransactionsExpected(plan.rows, plan.survivorID)
				} else {
					writeErr = app.DeleteTransactionExpected(plan.rows[0].ID, plan.rows)
				}
				if writeErr != nil {
					return "No change made: " + writeErr.Error()
				}
				if merge {
					return fmt.Sprintf("Merged the approved duplicate group, keeping transaction %s and removing %d duplicate entr%s.", plan.survivorID, len(plan.rows)-1, ifStr(len(plan.rows)-1 == 1, "y", "ies"))
				}
				return fmt.Sprintf("Deleted the approved transaction %s.", plan.rows[0].ID)
			},
		}
	}
	return []chatTool{
		makeTool("delete_transaction", "Delete one exact non-transfer transaction. `target` is required; supply an exact transaction_id, amount string, and/or YYYY-MM-DD date to disambiguate. Transfers must be handled with the transfer tools.", false),
		makeTool("merge_duplicate_transactions", "Merge one exact duplicate group: keep the displayed survivor and remove the extras. `target` is required; optional exact transaction_id, amount string, and YYYY-MM-DD date narrow the group.", true),
	}
}

func parseTransactionWriteArgs(raw json.RawMessage) (transactionWriteArgs, error) {
	var args transactionWriteArgs
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return args, fmt.Errorf("arguments must be one JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return args, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return args, fmt.Errorf("arguments must contain exactly one JSON object")
	}
	args.Target = strings.TrimSpace(args.Target)
	args.TransactionID = strings.TrimSpace(args.TransactionID)
	args.Amount = strings.TrimSpace(args.Amount)
	args.Date = strings.TrimSpace(args.Date)
	if args.Target == "" {
		return args, fmt.Errorf("target is required")
	}
	if args.Date != "" {
		parsed, err := time.Parse("2006-01-02", args.Date)
		if err != nil || parsed.Format("2006-01-02") != args.Date {
			return args, fmt.Errorf("date must be an exact YYYY-MM-DD calendar date")
		}
	}
	if args.Amount != "" {
		if strings.HasPrefix(args.Amount, "-") || strings.HasPrefix(args.Amount, "+") {
			return args, fmt.Errorf("amount must be an unsigned exact decimal string")
		}
		if amount, err := money.ParseMinor(args.Amount, 6); err != nil {
			return args, fmt.Errorf("amount must be an exact decimal string: %w", err)
		} else if amount == 0 {
			return args, fmt.Errorf("amount must be greater than zero")
		}
	}
	return args, nil
}

func resolveTransactionWritePlan(txns []domain.Transaction, args transactionWriteArgs, merge bool) *transactionWritePlan {
	plan := &transactionWritePlan{}
	if merge {
		byID := make(map[string]domain.Transaction, len(txns))
		for _, txn := range txns {
			byID[txn.ID] = txn
		}
		var matches []dedupe.Group
		for _, group := range dedupe.FindDuplicates(txns) {
			if !groupMatchesWriteArgs(group, byID, args) {
				continue
			}
			matches = append(matches, group)
		}
		if len(matches) == 0 {
			plan.err = "no complete duplicate group matches those details"
			return plan
		}
		if len(matches) > 1 {
			plan.err = "more than one duplicate group matches; add an exact transaction_id, amount, or date"
			return plan
		}
		group := matches[0]
		plan.survivorID = group.IDs[0]
		for _, id := range group.IDs {
			plan.rows = append(plan.rows, byID[id])
		}
		return plan
	}
	for _, txn := range txns {
		if txn.IsTransfer() || !transactionMatchesWriteArgs(txn, args) {
			continue
		}
		plan.rows = append(plan.rows, txn)
	}
	sort.Slice(plan.rows, func(i, j int) bool { return plan.rows[i].ID < plan.rows[j].ID })
	if len(plan.rows) == 0 {
		plan.err = "no non-transfer transaction matches those details"
	} else if len(plan.rows) > 1 {
		plan.err = fmt.Sprintf("%d transactions match; add an exact transaction_id, amount, or date", len(plan.rows))
		plan.rows = nil
	}
	return plan
}

func groupMatchesWriteArgs(group dedupe.Group, byID map[string]domain.Transaction, args transactionWriteArgs) bool {
	if args.TransactionID != "" && !containsTransactionID(group.IDs, args.TransactionID) {
		return false
	}
	if args.Date != "" && group.Date != args.Date {
		return false
	}
	if args.Amount != "" {
		minor, err := money.ParseMinor(args.Amount, currencyDecimalsFor(group.Currency))
		if err != nil || minor == 0 {
			return false
		}
		abs, err := money.Abs(group.Amount)
		if err != nil || abs != minor {
			return false
		}
	}
	for _, id := range group.IDs {
		if transactionTextMatches(byID[id], args.Target) {
			return true
		}
	}
	return false
}

func transactionMatchesWriteArgs(txn domain.Transaction, args transactionWriteArgs) bool {
	if txn.IsTransfer() || !transactionTextMatches(txn, args.Target) {
		return false
	}
	if args.TransactionID != "" && txn.ID != args.TransactionID {
		return false
	}
	if args.Date != "" && txn.Date.Format("2006-01-02") != args.Date {
		return false
	}
	if args.Amount != "" {
		minor, err := money.ParseMinor(args.Amount, currencyDecimalsFor(txn.Amount.Currency))
		if err != nil || minor == 0 {
			return false
		}
		abs, err := money.Abs(txn.Amount.Amount)
		if err != nil || abs != minor {
			return false
		}
	}
	return true
}

func transactionTextMatches(txn domain.Transaction, target string) bool {
	return strings.Contains(strings.ToLower(txn.Payee+" "+txn.Desc), strings.ToLower(target))
}

func currencyDecimalsFor(code string) int { return currency.Decimals(code) }

func containsTransactionID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func transactionWritePreview(plan *transactionWritePlan, merge bool) string {
	if len(plan.rows) == 0 {
		return "No transaction selected: " + plan.err + ". No data will change."
	}
	var b strings.Builder
	if merge {
		fmt.Fprintf(&b, "Merge %d exact duplicate transactions; keep survivor %s and remove the extras:\n", len(plan.rows), plan.survivorID)
	} else {
		fmt.Fprintf(&b, "Delete this exact transaction (transfers are excluded):\n")
	}
	for _, row := range plan.rows {
		fmt.Fprintf(&b, "• %s — %s, %s, %s, account %s, ID %s\n", row.Date.Format("2006-01-02"), row.Payee, fmtMoney(money.New(row.Amount.Amount, row.Amount.Currency)), row.Desc, row.AccountID, row.ID)
	}
	return strings.TrimRight(b.String(), "\n")
}

func sameTransactionRows(a, b []domain.Transaction) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]domain.Transaction(nil), a...)
	right := append([]domain.Transaction(nil), b...)
	sort.Slice(left, func(i, j int) bool { return left[i].ID < left[j].ID })
	sort.Slice(right, func(i, j int) bool { return right[i].ID < right[j].ID })
	for i := range left {
		if left[i].ID != right[i].ID || !reflect.DeepEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}
