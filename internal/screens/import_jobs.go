// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/importai"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type importPanelJob struct {
	model             *importdraft.Model
	cancel            func()
	timer             *time.Timer
	consented, closed bool
	pending           func()
	receiptWarning    string
}

var activeImportPanel *importPanelJob

// CancelImportPanel invalidates pending callbacks before the shell closes the
// import panel, including a checkpoint that has not yet started its write.
func CancelImportPanel() {
	if activeImportPanel != nil {
		activeImportPanel.close()
	}
}

func (job *importPanelJob) close() {
	job.closed = true
	job.model.Close()
	if job.timer != nil {
		job.timer.Stop()
	}
	if job.cancel != nil {
		job.cancel()
		job.cancel = nil
	}
}

func (c *documentsController) cancelAI() {
	c.model.Cancel()
	if c.job.timer != nil {
		c.job.timer.Stop()
		c.job.timer = nil
	}
	if c.job.cancel != nil {
		cancel := c.job.cancel
		c.job.cancel = nil
		cancel()
	}
	c.aiLoading.Set(false)
}

// startAI gates the first send on an explicit disclosure. A request generation
// covers both the consent dialog and the transport; neither may outlive a draft.
func (c *documentsController) startAI(send func(func(string, ai.Usage), func(string)) func(), apply func(string)) {
	if c.busy() || c.aiLoading.Get() || c.job.closed {
		return
	}
	if !AIKeyConfigured() {
		c.needsKey.Set(true)
		return
	}
	if cap := c.app.AISpendCap(); cap > 0 && c.app.AISpendThisMonth().CostUSD >= cap {
		c.aiErr.Set(uistate.T("imb.spendCap"))
		return
	}
	c.aiErr.Set("")
	c.needsKey.Set(false)
	c.aiLoading.Set(true)
	request := c.model.Begin()
	start := func() {
		if !c.model.Accept(request) {
			return
		}
		request = c.model.Begin()
		onError := func(message string) {
			if !c.model.Accept(request) {
				return
			}
			if c.job.timer != nil {
				c.job.timer.Stop()
			}
			c.aiLoading.Set(false)
			c.aiErr.Set(importFriendlyError(message))
		}
		c.job.timer = time.AfterFunc(90*time.Second, func() { c.timeoutAI(request) })

		c.job.cancel = send(func(content string, usage ai.Usage) {
			c.app.RecordAISpend("import", c.aiModel, usage)
			if !c.model.Accept(request) {
				return
			}
			if c.job.timer != nil {
				c.job.timer.Stop()
			}
			c.job.cancel = nil
			c.aiLoading.Set(false)
			apply(content)
		}, onError)
	}
	if c.job.consented {
		start()
		return
	}
	uistate.ConfirmModal(c.disclosure()+" "+uistate.T("imb.consent"), false, func(ok bool) {
		if c.job.closed {
			return
		}
		if !ok {
			c.cancelAI()
			return
		}
		c.job.consented = true
		start()
	})
}

func (c *documentsController) disclosure() string {
	source := c.source.Get()
	if c.stage.Get() == "review" {
		source = "categories"
	}
	if importdraft.Transport(source, c.settings.OpenAIKey, c.useBackendAI) == "proxy" {
		return uistate.T("imb.disclosureProxy", c.pr.ServerURL, c.aiModel)
	}
	return uistate.T("imb.disclosureDirect", c.aiModel)
}

func (c *documentsController) requestExtraction(source string) {
	if c.busy() || c.aiLoading.Get() {
		return
	}
	data := strings.TrimSpace(c.stmtText.Get())
	if source == "receipt" {
		data = c.imageURL.Get()
	}
	if source == "pdf" {
		data = c.pdfData.Get()
	}
	if data == "" {
		c.aiErr.Set(uistate.T("imb.sourceRequired"))
		return
	}
	if source == "stmt" && len(data) > importai.MaxSourceBytes {
		c.aiErr.Set(uistate.T("imb.textLimit", importai.MaxSourceBytes))
		return
	}
	// The current proxy exposes structured vision only. Other sources require
	// the user's key until its schema/options contract is available.
	if importdraft.Transport(source, c.settings.OpenAIKey, c.useBackendAI) == "unavailable" {
		c.needsKey.Set(true)
		c.aiErr.Set(uistate.T("imb.directKeyRequired"))
		return
	}
	catNames := c.categoryNames()
	c.startAI(func(result func(string, ai.Usage), fail func(string)) func() {
		opts := importai.RequestOptions(c.aiModel)
		schema := importai.ExtractionSchema()
		system := statementSystemPrompt(catNames)
		if source == "receipt" {
			system = visionSystemPrompt + " Use existing categories: " + strings.Join(catNames, ", ")
		}
		switch source {
		case "receipt":
			if c.useBackendAI {
				return ai.SendProxyStructuredVisionChat(c.pr.ServerURL, uistate.EffectiveServerToken(c.pr.ServerToken), c.aiModel, system, "Extract the receipt lines.", data, 0, "transactions", schema, result, fail)
			}
			return ai.SendStructuredVisionChatWithOptions(c.settings.OpenAIKey, ai.DefaultBaseURL, c.aiModel, system, "Extract the receipt lines.", data, opts, "transactions", schema, result, fail)
		case "pdf":
			return ai.SendStructuredFileChatWithOptions(c.settings.OpenAIKey, ai.DefaultBaseURL, c.aiModel, system, "Extract the statement transactions.", firstNonEmpty(c.pdfName.Get(), "statement.pdf"), data, opts, "transactions", schema, result, fail)
		default:
			messages, err := importai.ExtractionMessages(data)
			if err != nil {
				fail(err.Error())
				return func() {}
			}
			return ai.SendStructuredChat(c.settings.OpenAIKey, ai.DefaultBaseURL, c.aiModel, messages, opts, "transactions", schema, result, fail)
		}
	}, c.acceptExtraction)
}

func (c *documentsController) acceptExtraction(content string) {
	rows, err := importai.DecodeExtraction(content)
	if err != nil {
		c.aiErr.Set(uistate.T("imb.invalidReply"))
		return
	}
	if len(rows) == 0 {
		c.aiErr.Set(uistate.T("documents.noneFound"))
		return
	}
	validated := importdraft.ValidateExtraction(rows, c.accounts, reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get()), time.Now())
	for i := range validated {
		rows[i] = validated[i].Row
	}
	before := len(c.model.Rows)
	c.appendRows(rows)
	if len(c.model.Rows) != before+len(rows) {
		return
	}
	for i, v := range validated {
		if !v.Valid {
			c.model.Errors[c.model.Rows[before+i].ID] = uistate.T("imb.invalidRow")
		}
	}
	c.msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")))
}

func (c *documentsController) categoryNames() []string {
	return importdraft.CategoryNames(c.app.Categories())
}

func (c *documentsController) useCategorizeAI() {
	c.categorizeAI = ui.UseEvent(func() {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		if importdraft.Transport("categories", c.settings.OpenAIKey, c.useBackendAI) == "unavailable" {
			c.needsKey.Set(true)
			return
		}
		inputs, ids := importdraft.CategoryInputs(c.model.Rows)
		if len(ids) == 0 {
			c.msg.Set(uistate.T("imb.suggested"))
			return
		}
		names := c.categoryNames()
		messages, schema, err := importai.CategoryMessages(inputs, names)
		if err != nil {
			c.aiErr.Set(uistate.T("imb.suggestionLimit", importai.MaxSuggestionItems))
			return
		}
		c.startAI(func(result func(string, ai.Usage), fail func(string)) func() {
			return ai.SendStructuredChat(c.settings.OpenAIKey, ai.DefaultBaseURL, c.aiModel, messages, importai.RequestOptions(c.aiModel), "categories", schema, result, fail)
		}, func(content string) {
			suggestions, err := importai.DecodeCategorySuggestions(content, len(ids), names)
			if err != nil {
				c.aiErr.Set(uistate.T("imb.invalidReply"))
				return
			}
			categories := map[string]string{}
			for _, suggestion := range suggestions {
				categories[ids[suggestion.Index]] = suggestion.Category
			}
			c.model.ApplyCategories(categories)
			c.syncDraft()
			c.msg.Set(uistate.T("imb.suggested"))
		})
	})
}

func (c *documentsController) timeoutAI(request importdraft.Request) {
	if !c.model.Accept(request) {
		return
	}
	if c.job.cancel != nil {
		c.job.cancel()
	}
	c.aiLoading.Set(false)
	c.aiErr.Set(uistate.T("imb.timeout"))
}

func (c *documentsController) canSend(source string) bool {
	return importdraft.Transport(source, c.settings.OpenAIKey, c.useBackendAI) != "unavailable"
}
