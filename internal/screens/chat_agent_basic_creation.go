// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
)

type assistantTaskArgs struct {
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Priority string `json:"priority"`
	Due      string `json:"due"`
}

type assistantTaskPlan struct {
	task     domain.Task
	expected []domain.Task
	err      error
}

type assistantAccountArgs struct {
	Name        string          `json:"name"`
	Class       string          `json:"class"`
	Type        string          `json:"type"`
	Currency    string          `json:"currency"`
	Balance     json.RawMessage `json:"balance"`
	APR         json.RawMessage `json:"apr"`
	CreditLimit json.RawMessage `json:"credit_limit"`
	MinPayment  json.RawMessage `json:"min_payment"`
}

type assistantAccountPlan struct {
	account        domain.Account
	expected       []domain.Account
	expectedBase   string
	inputOwed      bool
	providedFields []string
	err            error
}

func assistantStrictJSON(raw json.RawMessage, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("couldn't read the details: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("provide one JSON object")
	}
	return nil
}

func prepareAssistantTask(app *appstate.App, raw json.RawMessage) assistantTaskPlan {
	var a assistantTaskArgs
	if err := assistantStrictJSON(raw, &a); err != nil {
		return assistantTaskPlan{err: err}
	}
	title := strings.TrimSpace(a.Title)
	if title == "" {
		return assistantTaskPlan{err: fmt.Errorf("a task needs a nonblank title")}
	}
	priority := domain.PriorityMedium
	if strings.TrimSpace(a.Priority) != "" {
		priority = domain.TaskPriority(strings.ToLower(strings.TrimSpace(a.Priority)))
		if !priority.Valid() {
			return assistantTaskPlan{err: fmt.Errorf("priority must be low, medium or high")}
		}
	}
	var due time.Time
	if strings.TrimSpace(a.Due) != "" {
		parsed, err := dateutil.ParseDate(strings.TrimSpace(a.Due))
		if err != nil {
			return assistantTaskPlan{err: fmt.Errorf("due date must use YYYY-MM-DD: %w", err)}
		}
		due = parsed
	}
	expected, err := app.Store().ListTasks()
	if err != nil {
		return assistantTaskPlan{err: fmt.Errorf("read current tasks: %w", err)}
	}
	for _, existing := range expected {
		if existing.Status != domain.StatusDone && similarText(existing.Title, title) {
			return assistantTaskPlan{err: fmt.Errorf("a similar open task already exists: %q", existing.Title)}
		}
	}
	task := domain.Task{
		ID: id.New(), Title: title, Notes: a.Notes, Due: due,
		Status: domain.StatusOpen, Priority: priority, Source: domain.SourceAI,
	}
	if issues := validate.ValidateTask(task); !issues.OK() {
		return assistantTaskPlan{err: fmt.Errorf("task is not valid: %w", issues)}
	}
	return assistantTaskPlan{task: task, expected: expected}
}

func prepareAssistantAccount(app *appstate.App, raw json.RawMessage) assistantAccountPlan {
	var a assistantAccountArgs
	if err := assistantStrictJSON(raw, &a); err != nil {
		return assistantAccountPlan{err: err}
	}
	name := strings.TrimSpace(a.Name)
	if name == "" {
		return assistantAccountPlan{err: fmt.Errorf("an account needs a nonblank name")}
	}
	class := domain.AccountClass(strings.ToLower(strings.TrimSpace(a.Class)))
	if !class.Valid() {
		return assistantAccountPlan{err: fmt.Errorf("class must be asset or liability")}
	}
	accountType := domain.AccountType(strings.ToLower(strings.TrimSpace(a.Type)))
	if accountType == "" {
		if class == domain.ClassAsset {
			accountType = domain.TypeChecking
		} else {
			accountType = domain.TypeLoan
		}
	} else if !assistantSupportedAccountType(accountType) {
		return assistantAccountPlan{err: fmt.Errorf("type must be checking, savings, cash, investment, loan, credit_card, mortgage or line_of_credit")}
	}
	if accountType.Class() != class {
		return assistantAccountPlan{err: fmt.Errorf("%s accounts require the %s class", accountType, accountType.Class())}
	}
	settings, err := app.Store().GetSettings()
	if err != nil {
		return assistantAccountPlan{err: fmt.Errorf("read saved currency settings: %w", err)}
	}
	code := strings.ToUpper(strings.TrimSpace(a.Currency))
	if code == "" {
		code = strings.ToUpper(strings.TrimSpace(settings.BaseCurrency))
		if code == "" {
			code = "USD"
		}
	}
	if !currency.Valid(code) {
		return assistantAccountPlan{err: fmt.Errorf("choose a supported currency or set a supported base currency in Settings")}
	}
	account := domain.Account{
		ID: id.New(), Name: name, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: class, Type: accountType, Currency: code,
		OpeningBalance: money.Zero(code), BalanceAsOf: app.ClockNow(),
	}
	provided := make([]string, 0, 4)
	if len(a.Balance) > 0 {
		minor, err := assistantMinorFromJSON(a.Balance, currency.Decimals(code))
		if err != nil {
			return assistantAccountPlan{err: fmt.Errorf("invalid opening balance in %s: %w", code, err)}
		}
		if class == domain.ClassLiability {
			if minor < 0 {
				return assistantAccountPlan{err: fmt.Errorf("enter a liability balance as a nonnegative amount owed")}
			}
			var negErr error
			minor, negErr = money.Neg(minor)
			if negErr != nil {
				return assistantAccountPlan{err: fmt.Errorf("opening balance: %w", negErr)}
			}
		}
		account.OpeningBalance = money.New(minor, code)
		provided = append(provided, "balance")
	}
	if len(a.APR) > 0 {
		apr, err := assistantFiniteFloat(a.APR)
		if err != nil || apr < 0 {
			return assistantAccountPlan{err: fmt.Errorf("APR must be a finite, nonnegative percentage")}
		}
		if class != domain.ClassLiability {
			return assistantAccountPlan{err: fmt.Errorf("APR is only available for liability accounts")}
		}
		account = account.WithRateAPR(apr)
		provided = append(provided, "apr")
	}
	if len(a.CreditLimit) > 0 {
		minor, err := assistantMinorFromJSON(a.CreditLimit, currency.Decimals(code))
		if err != nil || minor < 0 {
			return assistantAccountPlan{err: fmt.Errorf("credit_limit must be a nonnegative amount in %s", code)}
		}
		if class != domain.ClassLiability {
			return assistantAccountPlan{err: fmt.Errorf("credit_limit is only available for liability accounts")}
		}
		account.CreditLimit = money.New(minor, code)
		provided = append(provided, "credit_limit")
	}
	if len(a.MinPayment) > 0 {
		minor, err := assistantMinorFromJSON(a.MinPayment, currency.Decimals(code))
		if err != nil || minor < 0 {
			return assistantAccountPlan{err: fmt.Errorf("min_payment must be a nonnegative amount in %s", code)}
		}
		if class != domain.ClassLiability {
			return assistantAccountPlan{err: fmt.Errorf("min_payment is only available for liability accounts")}
		}
		account.MinPayment = money.New(minor, code)
		provided = append(provided, "min_payment")
	}
	account = account.WithLiabilitySign()
	if issues := validate.ValidateAccount(account); !issues.OK() {
		return assistantAccountPlan{err: fmt.Errorf("account is not valid: %w", issues)}
	}
	expected, err := app.Store().ListAccounts()
	if err != nil {
		return assistantAccountPlan{err: fmt.Errorf("read current accounts: %w", err)}
	}
	for _, existing := range expected {
		if existing.Archived {
			continue
		}
		if similarText(existing.Name, name) {
			return assistantAccountPlan{err: fmt.Errorf("an account named %q already exists", existing.Name)}
		}
	}
	return assistantAccountPlan{account: account, expected: expected, expectedBase: settings.BaseCurrency,
		inputOwed: class == domain.ClassLiability, providedFields: provided}
}

func assistantSupportedAccountType(t domain.AccountType) bool {
	switch t {
	case domain.TypeChecking, domain.TypeSavings, domain.TypeCash, domain.TypeInvestment,
		domain.TypeLoan, domain.TypeCreditCard, domain.TypeMortgage, domain.TypeLineOfCredit:
		return true
	default:
		return false
	}
}

func assistantMinorFromJSON(raw json.RawMessage, decimals int) (int64, error) {
	text := strings.TrimSpace(string(raw))
	if len(text) > 0 && text[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	}
	return assistantDecimalMinor(text, decimals)
}

func assistantDecimalMinor(text string, decimals int) (int64, error) {
	return money.ParseMinor(text, decimals)
}

func assistantFiniteFloat(raw json.RawMessage) (float64, error) {
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	} else {
		text = string(raw)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid finite number")
	}
	return value, nil
}

func assistantTaskPreview(task domain.Task) string {
	out := fmt.Sprintf("Add a to-do: %q\nStatus: open\nPriority: %s\nAssigned member: unassigned\nCreated by: assistant", task.Title, assistantPriorityLabel(task.Priority))
	if !task.Due.IsZero() {
		out += "\nDue: " + dateutil.FormatDate(task.Due)
	} else {
		out += "\nDue: none"
	}
	if task.Notes != "" {
		out += "\nNotes: " + task.Notes
	} else {
		out += "\nNotes: none"
	}
	return out
}

func assistantPriorityLabel(priority domain.TaskPriority) string {
	if priority == domain.PriorityMedium {
		return "medium"
	}
	return string(priority)
}

func assistantAccountPreview(plan assistantAccountPlan) string {
	a := plan.account
	balance := fmtMoney(a.OpeningBalance)
	if plan.inputOwed {
		owed, err := a.OpeningBalance.AbsChecked()
		if err != nil {
			balance = "unavailable " + a.Currency
		} else {
			balance = fmtMoney(owed) + " owed"
		}
	}
	out := fmt.Sprintf("Create a %s %s account %q\nCurrency: %s\nOpening balance: %s\nOwner: household group\nScope: shared",
		a.Class, a.Type, a.Name, a.Currency, balance)
	out += "\nBalance as of: " + a.BalanceAsOf.Format(time.RFC3339)
	if plan.inputOwed && a.LiabilitySign == domain.LiabilityNegativeOwed {
		out += "\nStored liability convention: owed balances are negative"
	} else if plan.inputOwed {
		out += "\nStored liability convention: owed balances are positive"
	}
	if apr, ok := a.RateAPR(); ok {
		out += fmt.Sprintf("\nAPR: %s%%", strconv.FormatFloat(apr, 'f', -1, 64))
	} else {
		out += "\nAPR: unset"
	}
	if a.CreditLimit.Currency != "" {
		out += "\nCredit limit: " + fmtMoney(a.CreditLimit)
	} else {
		out += "\nCredit limit: unset"
	}
	if a.MinPayment.Currency != "" {
		out += "\nMinimum payment: " + fmtMoney(a.MinPayment)
	} else {
		out += "\nMinimum payment: unset"
	}
	if len(plan.providedFields) > 0 {
		out += "\nFields supplied: " + strings.Join(plan.providedFields, ", ")
	}
	return out
}

// agToolsBasicCreation builds strict, preview-bound task and account tools.
func agToolsBasicCreation(app *appstate.App) []chatTool {
	preparedTasks := map[string]assistantTaskPlan{}
	preparedAccounts := map[string]assistantAccountPlan{}
	taskSchema := json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"notes":{"type":"string"},"priority":{"type":"string","enum":["low","medium","high"]},"due":{"type":"string","description":"YYYY-MM-DD"}},"required":["title"],"additionalProperties":false}`)
	accountSchema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"class":{"type":"string","enum":["asset","liability"]},"type":{"type":"string","enum":["checking","savings","cash","investment","loan","credit_card","mortgage","line_of_credit"]},"currency":{"type":"string","description":"Optional supported ISO currency; defaults to saved base currency, or USD when unset"},"balance":{"type":["number","string"],"description":"Plain exact decimal opening amount in major units, without exponent notation; for liabilities enter a nonnegative amount owed"},"apr":{"type":["number","string"],"description":"Finite nonnegative percentage as a plain decimal"},"credit_limit":{"type":["number","string"],"description":"Nonnegative exact decimal amount in account currency, without exponent notation"},"min_payment":{"type":["number","string"],"description":"Nonnegative exact decimal amount in account currency, without exponent notation"}},"required":["name","class"],"additionalProperties":false}`)
	return []chatTool{
		{
			spec:    ai.FunctionTool("add_task", "Add a to-do task. The approval preview shows the complete title, notes, priority and due date; invalid values and stale previews are rejected.", taskSchema),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				plan := prepareAssistantTask(app, raw)
				preparedTasks[string(raw)] = plan
				if plan.err != nil {
					return "Can't add that task: " + plan.err.Error()
				}
				return assistantTaskPreview(plan.task)
			},
			run: func(raw json.RawMessage) string {
				plan, ok := preparedTasks[string(raw)]
				delete(preparedTasks, string(raw))
				if !ok {
					return "Review the task preview before adding it."
				}
				if plan.err != nil {
					return "Can't add that task: " + plan.err.Error()
				}
				task, err := app.CreateTaskExpected(plan.task, plan.expected)
				if err != nil {
					return "Couldn't add the task: " + err.Error()
				}
				return "Added to-do: " + task.Title + "." + openLink("/todo", task.ID)
			},
		},
		{
			spec:    ai.FunctionTool("add_account", "Create an asset or liability account. Account class and type must agree. Enter liability balances as positive amounts owed. The preview shows all written fields, owner, scope and currency; invalid values and stale previews are rejected.", accountSchema),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				plan := prepareAssistantAccount(app, raw)
				preparedAccounts[string(raw)] = plan
				if plan.err != nil {
					return "Can't create that account: " + plan.err.Error()
				}
				return assistantAccountPreview(plan)
			},
			run: func(raw json.RawMessage) string {
				plan, ok := preparedAccounts[string(raw)]
				delete(preparedAccounts, string(raw))
				if !ok {
					return "Review the account preview before creating it."
				}
				if plan.err != nil {
					return "Can't create that account: " + plan.err.Error()
				}
				account, err := app.CreateAccountExpected(plan.account, plan.expected, plan.expectedBase)
				if err != nil {
					return "Couldn't create the account: " + err.Error()
				}
				return fmt.Sprintf("Created %s account %q with balance %s.%s", account.Class, account.Name, fmtMoney(account.OpeningBalance), openLink("/accounts", account.ID))
			},
		},
	}
}
