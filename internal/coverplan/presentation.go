// SPDX-License-Identifier: MIT

package coverplan

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Translate resolves existing catalog keys at the display edge.
type Translate func(string, ...any) string

// FormatMoney formats a minor-unit amount using the current user preferences.
type FormatMoney func(money.Money) string

// Display holds all label, hint and enable decisions for the form's renderers.
type Display struct {
	Intro, ApplyLabel, ImpactKey string
	NextMonthHint                bool
	Disabled                     bool
}

// CheckpointText names the existing pre-cover checkpoint using the displayed count.
func CheckpointText(model Model, t Translate) string {
	count := t("budgets.countMany", len(model.Overs))
	if len(model.Overs) == 1 {
		count = t("budgets.countOne")
	}
	return t("ckpt.beforeCoverAll", count)
}

// Present preserves the precedence of overflow and currency messages and the
// original explicit-selection enable policy. Formatting itself stays at the edge.
func Present(model Model, chosen map[string]string, saving bool, t Translate, format FormatMoney) Display {
	var total int64
	var totalErr error
	for _, over := range model.Overs {
		if over.Error != nil {
			totalErr = over.Error
			break
		}
		total, totalErr = money.Add(total, over.Shortfall.Amount)
		if totalErr != nil {
			break
		}
	}
	intro := t("coverAll.intro", len(model.Overs), format(money.New(total, model.Base)))
	if len(model.Overs) == 1 {
		intro = t("coverAll.introOne", format(model.Overs[0].Shortfall))
	}
	if model.CurrencyError {
		intro = t("coverAll.currencyError")
	}
	selected, _, selectedErr := SelectedTotalChecked(model.Overs, chosen)
	if totalErr != nil || selectedErr != nil {
		intro = t("budgets.incomplete")
	}
	label := t("bv7g.cover.applyEmpty")
	if selected > 0 {
		label = t("bv7g.cover.apply", format(money.New(selected, model.Base)))
	}
	hasNext, funded := HasNextMonthSelection(chosen), false
	for _, source := range chosen {
		if source != "" && source != NextSource {
			funded = true
		}
	}
	impact := "budgets.impactReachThisPeriod"
	if hasNext {
		impact = "budgets.impactReachThisAndNext"
	}
	if funded {
		impact += "Funded"
	}
	return Display{Intro: intro, ApplyLabel: label, ImpactKey: impact, NextMonthHint: hasNext,
		Disabled: saving || !HasDonor(chosen) || totalErr != nil || selectedErr != nil || model.CurrencyError}
}

// ProblemText formats the existing validation message without UI dependencies.
func ProblemText(problem *Problem, t Translate, format FormatMoney, errorText func(error) string) string {
	if problem.Err != nil {
		return errorText(problem.Err)
	}
	switch problem.Key {
	case "coverAll.rolloverBorrowBlocked":
		return t(problem.Key, problem.Name)
	case "coverAll.sourceShort":
		return t(problem.Key, problem.Name, format(problem.Amount))
	default:
		return t(problem.Key)
	}
}

// CompletionText retains the original single-row/multiple-row notice policy,
// including its use of the first displayed row for a single applied request.
func CompletionText(model Model, chosen map[string]string, applied int, t Translate, format FormatMoney) string {
	if applied == 1 && len(model.Overs) > 0 {
		over := model.Overs[0]
		source := chosen[over.ID]
		if source != "" && source != NextSource {
			return t("br3dlg.coveredFrom", format(over.Shortfall), sourceIndex(model.Sources)[source].Name)
		}
		return t("coverAll.doneOne")
	}
	return t("coverAll.doneMany", applied)
}

// Completion describes whether a draft has work and the existing outcome text.
type Completion struct {
	Empty bool
	Text  string
}

// Complete resolves the request count and empty/success message for a draft.
// Callers publish success only after the application's write boundary succeeds.
func Complete(model Model, chosen map[string]string, requests []budgeting.CoverRequest, t Translate, format FormatMoney) Completion {
	if len(requests) == 0 {
		return Completion{Empty: true, Text: t("coverAll.doneNone")}
	}
	return Completion{Text: CompletionText(model, chosen, len(requests), t, format)}
}
