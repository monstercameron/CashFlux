// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

type accountBalanceWriteArgs struct {
	AccountID string `json:"account_id,omitempty"`
	Account   string `json:"account,omitempty"`
	Balance   string `json:"balance"`
	AsOf      string `json:"as_of,omitempty"`
}

type accountTransferWriteArgs struct {
	FromAccountID string `json:"from_account_id,omitempty"`
	FromAccount   string `json:"from_account,omitempty"`
	ToAccountID   string `json:"to_account_id,omitempty"`
	ToAccount     string `json:"to_account,omitempty"`
	Amount        string `json:"amount"`
	Received      string `json:"received_amount,omitempty"`
	Fee           string `json:"fee,omitempty"`
	Date          string `json:"date,omitempty"`
	Description   string `json:"description,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
}

type accountWritePlan struct {
	balance  *appstate.AccountReconciliationPreview
	transfer *appstate.ExpectedTransferPreview
	err      string
	used     bool
}

// agToolsAccountWrites returns preview-bound account reconciliation and transfer
// tools. Each run consumes its exact approved preview once; the appstate expected
// APIs provide the final fresh-state check and atomic commit.
func agToolsAccountWrites(app *appstate.App) []chatTool {
	plans := map[string]*accountWritePlan{}
	makeBalanceTool := func() chatTool {
		keyOf := func(args accountBalanceWriteArgs) string {
			b, _ := json.Marshal(args)
			return "balance:" + string(b)
		}
		prepare := func(raw json.RawMessage) (accountBalanceWriteArgs, string, *accountWritePlan) {
			args, err := parseAccountBalanceWriteArgs(raw)
			if err != nil {
				return args, "", &accountWritePlan{err: err.Error()}
			}
			account, err := resolveAccountWriteTarget(app, args.AccountID, args.Account)
			if err != nil {
				plan := &accountWritePlan{err: err.Error()}
				plans[keyOf(args)] = plan
				return args, keyOf(args), plan
			}
			minor, err := money.ParseMinor(args.Balance, currencyDecimalsFor(account.Currency))
			if err != nil {
				plan := &accountWritePlan{err: "balance must be an exact decimal amount in " + account.Currency}
				plans[keyOf(args)] = plan
				return args, keyOf(args), plan
			}
			if account.Class == domain.ClassLiability {
				if minor < 0 {
					plan := &accountWritePlan{err: "liability balance must be a nonnegative amount still owed"}
					plans[keyOf(args)] = plan
					return args, keyOf(args), plan
				}
				if minor == math.MinInt64 {
					plan := &accountWritePlan{err: "balance is outside the supported range"}
					plans[keyOf(args)] = plan
					return args, keyOf(args), plan
				}
				minor = -minor // tool convention is positive owed; native preview uses stated balance.
			}
			asOf, err := parseOptionalAccountWriteDate(args.AsOf)
			if err != nil {
				plan := &accountWritePlan{err: err.Error()}
				plans[keyOf(args)] = plan
				return args, keyOf(args), plan
			}
			preview, err := app.PreviewAccountReconciliation(account.ID, minor, asOf)
			plan := &accountWritePlan{}
			if err != nil {
				plan.err = err.Error()
			} else {
				plan.balance = &preview
			}
			key := keyOf(args)
			plans[key] = plan
			return args, key, plan
		}
		schema := json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string","description":"stable account ID; use this or an exact unique account name"},"account":{"type":"string","description":"exact unique account name; use this or account_id"},"balance":{"type":"string","description":"exact decimal amount in the account currency; for liabilities enter the positive amount still owed"},"as_of":{"type":"string","description":"optional exact YYYY-MM-DD date the statement balance was true"}},"required":["balance"],"additionalProperties":false}`)
		return chatTool{
			spec:    ai.FunctionTool("update_account_balance", "Reconcile one account to a statement balance. A liability balance is the positive amount still owed. This records an excluded balance checkpoint and does not rewrite the opening balance.", schema),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				_, _, plan := prepare(raw)
				if plan.err != "" {
					return "Can't prepare this change: " + plan.err
				}
				return accountBalanceWritePreview(*plan.balance)
			},
			run: func(raw json.RawMessage) string {
				args, err := parseAccountBalanceWriteArgs(raw)
				if err != nil {
					return "No change made: " + err.Error()
				}
				plan, ok := plans[keyOf(args)]
				if !ok {
					return "No change made: no matching approval preview is available. Request a new preview first."
				}
				if plan.err != "" {
					return "No change made: " + plan.err
				}
				if plan.used {
					return "This approved change was already processed; no further change was made."
				}
				plan.used = true
				checkpoint, err := app.CommitAccountReconciliation(*plan.balance, "Assistant balance checkpoint")
				if err != nil {
					return "No change made: " + err.Error()
				}
				if !plan.balance.NeedsAdjustment {
					return fmt.Sprintf("%s already matches the entered balance; no ledger change was needed.", plan.balance.AccountName)
				}
				if checkpoint.ID == "" {
					return fmt.Sprintf("Removed the prior balance checkpoint for %s as of %s; the account now matches the approved statement balance.",
						plan.balance.AccountName, plan.balance.AsOf.Format("2006-01-02"))
				}
				return fmt.Sprintf("Recorded the approved balance checkpoint for %s as of %s (%s). It is excluded from income and spending reports; the opening balance was left unchanged.%s",
					plan.balance.AccountName, plan.balance.AsOf.Format("2006-01-02"), fmtMoney(checkpoint.Amount), openLink("/transactions", checkpoint.ID))
			},
		}
	}

	makeTransferTool := func() chatTool {
		keyOf := func(args accountTransferWriteArgs) string {
			b, _ := json.Marshal(args)
			return "transfer:" + string(b)
		}
		prepare := func(raw json.RawMessage) (accountTransferWriteArgs, string, *accountWritePlan) {
			args, err := parseAccountTransferWriteArgs(raw)
			if err != nil {
				return args, "", &accountWritePlan{err: err.Error()}
			}
			from, err := resolveAccountWriteTarget(app, args.FromAccountID, args.FromAccount)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			to, err := resolveAccountWriteTarget(app, args.ToAccountID, args.ToAccount)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			amount, err := money.ParseMinor(args.Amount, currencyDecimalsFor(from.Currency))
			if err != nil || amount <= 0 {
				return args, "", storeAccountWritePlanError(plans, keyOf(args), "amount must be a positive exact decimal amount in the source currency")
			}
			received, err := parseOptionalTransferAmount(args.Received, to.Currency, false)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			fee, err := parseOptionalTransferAmount(args.Fee, from.Currency, true)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			date, err := parseOptionalAccountWriteDate(args.Date)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			p := appstate.TransferParams{FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: amount,
				ReceivedMinor: received, FeeMinor: fee, Date: date, Desc: strings.TrimSpace(args.Description),
				OperationID: args.OperationID, Source: domain.TxnSourceAssistant}
			preview, err := app.PreviewExpectedTransfer(p)
			if err != nil {
				return args, "", storeAccountWriteError(plans, keyOf(args), err)
			}
			plan := &accountWritePlan{transfer: &preview}
			key := keyOf(args)
			plans[key] = plan
			return args, key, plan
		}
		schema := json.RawMessage(`{"type":"object","properties":{"from_account_id":{"type":"string","description":"stable source account ID; use this or an exact unique source name"},"from_account":{"type":"string","description":"exact unique source account name; use this or from_account_id"},"to_account_id":{"type":"string","description":"stable destination account ID; use this or an exact unique destination name"},"to_account":{"type":"string","description":"exact unique destination account name; use this or to_account_id"},"amount":{"type":"string","description":"positive exact decimal amount in the source account currency"},"received_amount":{"type":"string","description":"optional positive exact amount received in the destination currency; required when a saved FX rate is unavailable"},"fee":{"type":"string","description":"optional exact nonnegative fee in the source currency"},"date":{"type":"string","description":"optional exact YYYY-MM-DD transfer date"},"description":{"type":"string","description":"optional transfer description"},"operation_id":{"type":"string","description":"optional stable idempotency key of at most 64 letters, numbers, hyphens, or underscores"}},"required":["amount"],"additionalProperties":false}`)
		return chatTool{
			spec:    ai.FunctionTool("add_transfer", "Move money between two accounts as one matched transfer. For different currencies, the preview uses the current saved exchange rate or the exact received amount you provide.", schema),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				_, _, plan := prepare(raw)
				if plan.err != "" {
					return "Can't prepare this change: " + plan.err
				}
				return accountTransferWritePreview(*plan.transfer)
			},
			run: func(raw json.RawMessage) string {
				args, err := parseAccountTransferWriteArgs(raw)
				if err != nil {
					return "No change made: " + err.Error()
				}
				plan, ok := plans[keyOf(args)]
				if !ok {
					return "No change made: no matching approval preview is available. Request a new preview first."
				}
				if plan.err != "" {
					return "No change made: " + plan.err
				}
				if plan.used {
					return "This approved change was already processed; no further change was made."
				}
				plan.used = true
				outID, _, err := app.CreateTransferPairExpected(*plan.transfer)
				if err != nil {
					return "No change made: " + err.Error()
				}
				return fmt.Sprintf("Recorded the approved transfer.%s", openLink("/transactions", outID))
			},
		}
	}
	return []chatTool{makeBalanceTool(), makeTransferTool()}
}

func parseAccountBalanceWriteArgs(raw json.RawMessage) (accountBalanceWriteArgs, error) {
	var args accountBalanceWriteArgs
	if err := decodeStrictAccountWriteArgs(raw, &args); err != nil {
		return args, err
	}
	args.AccountID, args.Account, args.Balance, args.AsOf = strings.TrimSpace(args.AccountID), strings.TrimSpace(args.Account), strings.TrimSpace(args.Balance), strings.TrimSpace(args.AsOf)
	if args.AccountID == "" && args.Account == "" {
		return args, fmt.Errorf("provide account_id or an exact account name")
	}
	if args.Balance == "" {
		return args, fmt.Errorf("balance is required as an exact decimal string")
	}
	if _, err := parseOptionalAccountWriteDate(args.AsOf); err != nil {
		return args, err
	}
	return args, nil
}

func parseAccountTransferWriteArgs(raw json.RawMessage) (accountTransferWriteArgs, error) {
	var args accountTransferWriteArgs
	if err := decodeStrictAccountWriteArgs(raw, &args); err != nil {
		return args, err
	}
	args.FromAccountID, args.FromAccount = strings.TrimSpace(args.FromAccountID), strings.TrimSpace(args.FromAccount)
	args.ToAccountID, args.ToAccount = strings.TrimSpace(args.ToAccountID), strings.TrimSpace(args.ToAccount)
	args.Amount, args.Received, args.Fee = strings.TrimSpace(args.Amount), strings.TrimSpace(args.Received), strings.TrimSpace(args.Fee)
	args.Date, args.Description = strings.TrimSpace(args.Date), strings.TrimSpace(args.Description)
	args.OperationID = strings.TrimSpace(args.OperationID)
	if (args.FromAccountID == "" && args.FromAccount == "") || (args.ToAccountID == "" && args.ToAccount == "") {
		return args, fmt.Errorf("provide a stable ID or exact unique name for both accounts")
	}
	if args.Amount == "" {
		return args, fmt.Errorf("amount is required as an exact decimal string")
	}
	if args.OperationID != "" && !validTransferOperationID(args.OperationID) {
		return args, fmt.Errorf("operation ID must be at most 64 letters, numbers, hyphens, or underscores")
	}
	if _, err := parseOptionalAccountWriteDate(args.Date); err != nil {
		return args, err
	}
	return args, nil
}

func decodeStrictAccountWriteArgs(raw json.RawMessage, dst any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("arguments must be one JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("arguments must contain exactly one JSON object")
	}
	return nil
}

func resolveAccountWriteTarget(app *appstate.App, accountID, name string) (domain.Account, error) {
	accounts, err := app.Store().ListAccounts()
	if err != nil {
		return domain.Account{}, fmt.Errorf("couldn't read accounts")
	}
	var byID *domain.Account
	if accountID != "" {
		for i := range accounts {
			if accounts[i].ID == accountID {
				byID = &accounts[i]
				break
			}
		}
		if byID == nil {
			return domain.Account{}, fmt.Errorf("no account has ID %q", accountID)
		}
	}
	if name != "" {
		var matches []domain.Account
		for _, account := range accounts {
			if strings.EqualFold(strings.TrimSpace(account.Name), name) {
				matches = append(matches, account)
			}
		}
		if len(matches) != 1 {
			if len(matches) == 0 {
				return domain.Account{}, fmt.Errorf("no account has the exact name %q", name)
			}
			return domain.Account{}, fmt.Errorf("the exact account name %q is ambiguous; provide its account_id", name)
		}
		if byID != nil && byID.ID != matches[0].ID {
			return domain.Account{}, fmt.Errorf("account_id and account name identify different accounts")
		}
		byID = &matches[0]
	}
	if byID == nil {
		return domain.Account{}, fmt.Errorf("provide account_id or an exact account name")
	}
	if byID.Archived {
		return domain.Account{}, fmt.Errorf("archived accounts cannot be used")
	}
	return *byID, nil
}

func parseOptionalAccountWriteDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	date, err := time.Parse("2006-01-02", raw)
	if err != nil || date.Format("2006-01-02") != raw {
		return time.Time{}, fmt.Errorf("date must be an exact YYYY-MM-DD calendar date")
	}
	return date, nil
}

func parseOptionalTransferAmount(raw, code string, zeroAllowed bool) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	minor, err := money.ParseMinor(raw, currencyDecimalsFor(code))
	if err != nil || minor < 0 || (!zeroAllowed && minor == 0) {
		return 0, fmt.Errorf("optional amount must be an exact %sdecimal in %s", ifStr(zeroAllowed, "nonnegative ", "positive "), code)
	}
	return minor, nil
}

func accountBalanceWritePreview(p appstate.AccountReconciliationPreview) string {
	current := fmtMoney(money.New(p.CurrentStatedMinor, p.Currency))
	target := fmtMoney(money.New(p.TargetStatedMinor, p.Currency))
	if !p.NeedsAdjustment {
		return fmt.Sprintf("%s already matches %s as of %s. No ledger change or timestamp update is needed.", p.AccountName, target, p.AsOf.Format("2006-01-02"))
	}
	change := fmtMoney(money.New(p.AdjustmentStatedMinor, p.Currency))
	action := "add a balance checkpoint " + fmtMoney(money.New(p.CheckpointAmountMinor, p.Currency))
	switch p.CheckpointAction {
	case "replace":
		action = fmt.Sprintf("replace %d prior checkpoint(s) with a balance checkpoint %s", p.CheckpointCount, fmtMoney(money.New(p.CheckpointAmountMinor, p.Currency)))
	case "remove":
		action = fmt.Sprintf("remove %d prior checkpoint(s) and add no new entry", p.CheckpointCount)
	}
	return fmt.Sprintf("Reconcile %s: current %s → target %s, adjustment %s, as of %s. This will %s. Checkpoints are excluded from income and spending reports; the opening balance stays unchanged.",
		p.AccountName, current, target, change, p.AsOf.Format("2006-01-02"), action)
}

func accountTransferWritePreview(p appstate.ExpectedTransferPreview) string {
	params, balances := p.Params, p.Balances
	fromAmount := fmtMoney(money.New(params.AmountMinor, balances.FromBefore.Currency))
	fee := ""
	if params.FeeMinor > 0 {
		fee = fmt.Sprintf("; fee %s", fmtMoney(money.New(params.FeeMinor, balances.FromBefore.Currency)))
	}
	return fmt.Sprintf("Transfer %s from %s (signed leg %s; balance %s → %s) to %s (signed leg %s; balance %s → %s)%s, on %s, description %q. Neither leg is income or spending.",
		fromAmount, p.FromAccountName, fmtMoney(p.FromAmount), fmtMoney(balances.FromBefore), fmtMoney(balances.FromAfter),
		p.ToAccountName, fmtMoney(p.ToAmount), fmtMoney(balances.ToBefore), fmtMoney(balances.ToAfter), fee,
		params.Date.Format("2006-01-02"), params.Desc)
}

func storeAccountWriteError(plans map[string]*accountWritePlan, key string, err error) *accountWritePlan {
	plan := &accountWritePlan{err: err.Error()}
	plans[key] = plan
	return plan
}

func storeAccountWritePlanError(plans map[string]*accountWritePlan, key, message string) *accountWritePlan {
	plan := &accountWritePlan{err: message}
	plans[key] = plan
	return plan
}
