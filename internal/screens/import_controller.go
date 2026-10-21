// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/store"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type documentsController struct {
	draftMetadata                 state.Atom[importdraft.Metadata]
	app                           *appstate.App
	nav                           router.Navigator
	rev                           state.Atom[int]
	importModalOpen               state.Atom[bool]
	memberLens                    []string
	csvText                       ui.State[string]
	stmtText                      ui.State[string]
	msg                           ui.State[string]
	cadenceMsg                    ui.State[string]
	csvPreflightS                 ui.State[*csvPreflightInfo]
	pendingCSV                    ui.State[[]byte]
	accounts                      []domain.Account
	defaultMemberID               string
	defaultAcc                    string
	imageURL                      ui.State[string]
	imageName                     ui.State[string]
	imageSize                     ui.State[int]
	imageDraftAtom                state.Atom[string]
	aiLoading                     ui.State[bool]
	aiErr                         ui.State[string]
	needsKey                      ui.State[bool]
	draft                         state.Atom[[]extract.Row]
	reviewPage                    ui.State[int]
	importAcct                    ui.State[string]
	receiptMode                   ui.State[bool]
	receiptTotal                  ui.State[string]
	receiptMerchant               ui.State[string]
	stage                         ui.State[string]
	setDraft                      func([]extract.Row)
	pdfData                       ui.State[string]
	pdfBytes                      ui.State[[]byte]
	pdfName                       ui.State[string]
	pdfSaveCopy                   ui.State[bool]
	source                        ui.State[string]
	onPickCSV                     ui.Handler
	onPickStmt                    ui.Handler
	onPickPDF                     ui.Handler
	onPickReceipt                 ui.Handler
	onDropPDF                     ui.Handler
	onDropImage                   ui.Handler
	onFileDragOver                ui.Handler
	onPickAmazon                  ui.Handler
	backToTypes                   ui.Handler
	goReview                      ui.Handler
	goInput                       ui.Handler
	goArtifacts                   ui.Handler
	goActivity                    ui.Handler
	wizardVisible                 ui.State[bool]
	wizardHeader                  ui.State[[]string]
	wizardRawRows                 ui.State[[][][]string]
	wizardDate                    ui.State[string]
	wizardDescCol                 ui.State[string]
	wizardAmount                  ui.State[string]
	wizardDebit                   ui.State[string]
	wizardCredit                  ui.State[string]
	dateOrderChoice               ui.State[string]
	numberFormatChoice            ui.State[string]
	statementRaw                  ui.State[string]
	statementAmbiguousDates       ui.State[int]
	statementAmbiguousNumbers     ui.State[int]
	statementInvalidRows          ui.State[int]
	statementFormatOptionsVisible ui.State[bool]
	profileName                   ui.State[string]
	showProfiles                  ui.State[bool]
	onCsv                         ui.Handler
	onStmt                        ui.Handler
	onWizardDate                  func(ui.Event)
	onWizardDesc                  func(ui.Event)
	onWizardAmount                func(ui.Event)
	onWizardDebit                 func(ui.Event)
	onWizardCredit                func(ui.Event)
	onProfileName                 func(string)
	checkpointSaving              ui.State[bool]
	checkpointRun                 func(string, func(string))
	commitCSVImport               func([]byte)
	confirmCSV                    ui.Handler
	cancelCSVPreflight            ui.Handler
	buildCSVPreflight             func([]byte)
	chooseCsvFile                 ui.Handler
	onAcct                        ui.Handler
	onReceiptTotal                ui.Handler
	onReceiptMerchant             ui.Handler
	importCSV                     ui.Handler
	parseStatement                ui.Handler
	onDateOrderChoice             ui.Handler
	onNumberFormatChoice          ui.Handler
	applyStatementFormats         ui.Handler
	applyWizard                   func()
	saveProfile                   func()
	createCadenceReminder         ui.Handler
	chooseImage                   ui.Handler
	settings                      store.Settings
	pr                            prefs.Prefs
	useBackendAI                  bool
	aiModel                       string
	readAI                        ui.Handler
	choosePDF                     ui.Handler
	onTogglePdfSave               ui.Handler
	runPDFAI                      ui.Handler
	extractWithAI                 ui.Handler
	categorizeDraft               ui.Handler
	doImportDraft                 func()
	importDraft                   ui.Handler
	doImportReceipt               func()
	importReceipt                 ui.Handler
	saveModalImport               ui.Handler
	removeDraft                   func(int)
	updateDraft                   func(int, extract.Row)
	rows                          []extract.Row
	reviewStart                   int
	reviewEnd                     int
	currentReviewPage             int
	reviewPageCount               int
	visibleRows                   []extract.Row
	reviewCur                     string
	recBaseCur                    string
	recDec                        int
	receiptCands                  []receiptmatch.Candidate
	attachReceipt                 func(string)
	receiptToggle                 ui.Node
	seenSigs                      map[string]bool
	clearDraft                    ui.Handler
	rollbackImport                func(domain.Document)
	pdfFileLabel                  string
	pdfCard                       ui.Node
	flowScreen                    importFlowScreen
	inReview                      bool
	model                         *importdraft.Model
	job                           *importPanelJob
	batch                         appstate.StagedImport
	summary                       importdraft.Summary
	categorizeAI                  ui.Handler
	pinnedFooter                  bool
}

func useDocumentsController() *documentsController {
	c := &documentsController{app: appstate.Default}
	c.useNav()
	c.useGoInput()
	c.useCheckpointRun()
	c.useCsvContext()
	c.useBuildCSVPreflight()
	c.useChooseCsvFile()
	c.useOnAcct()
	c.useImportCSV()
	c.useParseStatement()
	c.useOnDateOrderChoice()
	c.useApplyStatementFormats()
	c.useApplyWizard()
	c.useSaveProfile()
	c.useCreateCadenceReminder()
	c.useChooseImage()
	c.useReadAI()
	c.useChoosePDF()
	c.useOnTogglePdfSave()
	c.useRunPDFAI()
	c.useExtractWithAI()
	c.useCategorizeDraft()
	c.useCategorizeAI()
	c.useDoImportDraft()
	c.useImportDraft()
	c.useDoImportReceipt()
	c.useImportReceipt()
	c.useSaveModalImport()
	c.useRemoveDraft()
	c.useAttachReceipt()
	c.useReceiptToggle()
	c.useSeenSigs()
	c.usePdfCard()
	c.useFlowScreen()
	return c
}

func (c *documentsController) useNav() {
	c.nav = router.UseNavigate()
	c.rev = state.UseAtom("rev:documents", 0)
	c.importModalOpen = uistate.UseImportPanelOpen()
	c.memberLens = uistate.UseMemberLens()
	c.csvText = ui.UseState("")
	c.stmtText = ui.UseState("")
	c.msg = ui.UseState("")
	c.cadenceMsg = ui.UseState("")
	c.csvPreflightS = ui.UseState((*csvPreflightInfo)(nil))

	c.pendingCSV = ui.UseState([]byte(nil))
	c.accounts = ui.UseMemo(c.app.Accounts, c.app, c.app.Rev())
	c.defaultMemberID = ""
	if len(c.memberLens) == 1 {
		c.defaultMemberID = c.memberLens[0]
	}
	c.defaultAcc = ui.UseMemo(func() string {
		return resolveInitialImportAccount(accountselect.ResolveInput{
			Accounts: c.accounts, Transactions: c.app.Transactions(), ActiveMemberID: c.defaultMemberID,
			Members: c.app.Members(), WorkspaceDefaultID: c.app.DefaultAccountID(),
		})
	}, c.app, c.app.Rev(), c.defaultMemberID)
	c.imageURL = ui.UseState("")
	c.imageName = ui.UseState("")
	c.imageSize = ui.UseState(0)
	c.imageDraftAtom = uistate.UseImageDraft()
	ui.UseEffect(func() func() {
		if c.imageURL.Get() == "" {
			if saved := c.imageDraftAtom.Get(); saved != "" {
				c.imageURL.Set(saved)
			}
		}
		return nil
	}, "mount")
	c.useDraftSession()
	c.pdfData = ui.UseState("")
	c.pdfBytes = ui.UseState([]byte(nil))
	c.pdfName = ui.UseState("")
	c.pdfSaveCopy = ui.UseState(false)
	c.source = ui.UseState("")
	c.onPickCSV = ui.UseEvent(Prevent(func() { c.chooseSource("csv") }))
	c.onPickStmt = ui.UseEvent(Prevent(func() { c.chooseSource("stmt") }))
	c.onPickPDF = ui.UseEvent(Prevent(func() {
		c.importAcct.Set("")
		c.chooseSource("pdf")
	}))
	c.onPickReceipt = ui.UseEvent(Prevent(func() { c.chooseSource("receipt") }))
	c.onFileDragOver = ui.UseEvent(func(e ui.Event) { e.PreventDefault() })
	c.onDropPDF = ui.UseEvent(func(e ui.Event) { c.dropImportFile("pdf", e) })
	c.onDropImage = ui.UseEvent(func(e ui.Event) { c.dropImportFile("receipt", e) })
	c.onPickAmazon = ui.UseEvent(Prevent(func() { c.chooseSource("amazon") }))
	c.backToTypes = ui.UseEvent(Prevent(func() { c.chooseSource("") }))
	c.goReview = ui.UseEvent(Prevent(func() {
		if c.busy() {
			return
		}
		c.cancelAI()
		next := transitionImportFlow(importFlowState{Stage: importFlowStage(c.stage.Get()), Source: importFlowSource(c.source.Get())}, importActionNext)
		c.stage.Set(string(next.Stage))
		c.source.Set(string(next.Source))
	}))
}

func (c *documentsController) useGoInput() {
	c.goInput = ui.UseEvent(Prevent(func() {
		if c.busy() {
			return
		}
		c.cancelAI()
		next := transitionImportFlow(importFlowState{Stage: importFlowStage(c.stage.Get()), Source: importFlowSource(c.source.Get())}, importActionAddMore)
		c.stage.Set(string(next.Stage))
		c.source.Set(string(next.Source))
	}))
	c.goArtifacts = ui.UseEvent(Prevent(func() {
		c.importModalOpen.Set(false)
		c.nav.Navigate(uistate.RoutePath("/artifacts"))
	}))
	c.goActivity = ui.UseEvent(Prevent(func() {
		c.importModalOpen.Set(false)
		c.nav.Navigate(uistate.RoutePath("/activity"))
	}))
	c.wizardVisible = ui.UseState(false)
	c.wizardHeader = ui.UseState([]string{})
	c.wizardRawRows = ui.UseState([][][]string{})
	c.wizardDate = ui.UseState("-1")
	c.wizardDescCol = ui.UseState("-1")
	c.wizardAmount = ui.UseState("-1")
	c.wizardDebit = ui.UseState("-1")
	c.wizardCredit = ui.UseState("-1")
	c.dateOrderChoice = ui.UseState("")
	c.numberFormatChoice = ui.UseState("auto")
	c.statementRaw = ui.UseState("")
	c.statementAmbiguousDates = ui.UseState(0)
	c.statementAmbiguousNumbers = ui.UseState(0)
	c.statementInvalidRows = ui.UseState(0)
	c.statementFormatOptionsVisible = ui.UseState(false)
	c.profileName = ui.UseState("")
	c.showProfiles = ui.UseState(false)
	c.onCsv = ui.UseEvent(func(v string) {
		if c.busy() {
			return
		}
		c.csvText.Set(v)
		c.invalidateCSV()
	})
	c.onStmt = ui.UseEvent(func(v string) {
		if c.busy() {
			return
		}
		c.cancelAI()
		c.stmtText.Set(v)
	})
	c.onWizardDate = func(e ui.Event) { c.wizardDate.Set(e.GetValue()) }
	c.onWizardDesc = func(e ui.Event) { c.wizardDescCol.Set(e.GetValue()) }
	c.onWizardAmount = func(e ui.Event) { c.wizardAmount.Set(e.GetValue()) }
	c.onWizardDebit = func(e ui.Event) { c.wizardDebit.Set(e.GetValue()) }
	c.onWizardCredit = func(e ui.Event) { c.wizardCredit.Set(e.GetValue()) }
	c.onProfileName = func(v string) { c.profileName.Set(v) }

	c.checkpointSaving = ui.UseState(false)
}

func (c *documentsController) useCheckpointRun() {
	c.checkpointRun = func(label string, operation func(string)) {
		if c.busy() {
			return
		}
		c.cancelAI()
		request := c.model.Begin()
		c.checkpointSaving.Set(true)
		uistate.SaveCheckpointThen(label, func(cpID string, err error) {
			if !c.model.Accept(request) {
				return
			}
			if err != nil {
				c.checkpointSaving.Set(false)
				c.msg.Set(uistate.T("enc34.checkpointFailed"))
				return
			}
			operation(cpID)
		})
	}
}

func (c *documentsController) useCsvContext() {
	c.commitCSVImport = func(_ []byte) {
		info := c.csvPreflightS.Get()
		if info == nil || info.AccountID != c.importAcct.Get() || c.source.Get() != "csv" || c.stage.Get() != "input" {
			return
		}
		c.commitStaged(info.Staged, true)
	}
	c.confirmCSV = ui.UseEvent(func() { c.commitCSVImport(nil) })
	c.cancelCSVPreflight = ui.UseEvent(Prevent(func() {
		if !c.busy() {
			c.invalidateCSV()
		}
	}))
}

func (c *documentsController) useBuildCSVPreflight() {
	c.buildCSVPreflight = func(data []byte) {
		clean, err := importdraft.CleanCSV(data)
		if err != nil {
			c.msg.Set(uistate.T("imb.csvInvalid"))
			return
		}
		data = clean
		if c.busy() {
			return
		}
		if len(data) > maxImportCSVBytes || validateCSVRowLimit(data, maxImportRows) != nil {
			c.msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
			return
		}
		info, empty, err := prepareCSVPreflight(c.app, c.importAcct.Get(), data)
		if err != nil {
			c.msg.Set(uistate.T("documents.csvError", strings.TrimPrefix(err.Error(), "store: ")))
			return
		}
		if empty {
			c.msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		c.csvText.Set(string(data))
		c.pendingCSV.Set(append([]byte(nil), data...))

		c.msg.Set("")
		c.csvPreflightS.Set(info)
	}
}

func (c *documentsController) useChooseCsvFile() {
	c.chooseCsvFile = ui.UseEvent(func() {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		c.cancelAI()
		request := c.model.Begin()
		pickFileLimited(".csv,text/csv", maxImportCSVBytes, func(_, limit int) {
			if !c.model.Accept(request) {
				return
			}
			c.msg.Set(uistate.T("impc.fileTooLarge", "CSV", fileLimitLabel(limit)))
		}, func(_, _ string, data []byte) {
			if !c.model.Accept(request) {
				return
			}
			if len(data) == 0 {
				c.msg.Set(uistate.T("documents.csvFileEmpty"))
				return
			}
			if err := validateCSVRowLimit(data, maxImportRows); err != nil {
				c.msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
				return
			}
			c.buildCSVPreflight(data)
		})
	})
}

func (c *documentsController) useOnAcct() {
	c.onAcct = ui.UseEvent(func(e ui.Event) {
		if c.busy() {
			return
		}
		c.cancelAI()
		c.model.Cancel()
		c.importAcct.Set(e.GetValue())
		c.invalidateCSV()
	})
	c.onReceiptTotal = ui.UseEvent(func(v string) {
		if !c.busy() {
			c.receiptTotal.Set(v)
		}
	})
	c.onReceiptMerchant = ui.UseEvent(func(v string) {
		if !c.busy() {
			c.receiptMerchant.Set(v)
		}
	})
}

func (c *documentsController) useImportCSV() {
	c.importCSV = ui.UseEvent(Prevent(func() {
		if c.busy() {
			return
		}
		data := []byte(strings.TrimSpace(c.csvText.Get()))
		if len(data) == 0 {
			c.msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		c.buildCSVPreflight(data)
	}))
}

func (c *documentsController) useParseStatement() {
	c.parseStatement = ui.UseEvent(Prevent(func() {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		c.cancelAI()
		data := strings.TrimSpace(c.stmtText.Get())
		if data == "" {
			c.msg.Set(uistate.T("documents.stmtEmpty"))
			return
		}
		isStructured := importdraft.StructuredSource(data)
		if !isStructured {
			if err := validateCSVRowLimit([]byte(data), maxImportRows); err != nil {
				c.msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
				return
			}
		}
		// Parse amounts at the import account's currency precision. ParseAny
		// auto-detects the format — OFX 1.x/2.x (bank/card statements) as well as the
		// delimited CSV/TSV/pipe path — so a pasted .ofx/.qfx statement now imports
		// through the same review pipeline (C74 Tier 2).
		dec := currency.Decimals(reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get()))
		c.statementRaw.Set(data)
		c.statementFormatOptionsVisible.Set(true)
		options := statementParseOptions(c.dateOrderChoice.Get(), c.numberFormatChoice.Get())
		var st statement.Statement
		var stErr error
		var stRows []statement.Row
		if isStructured {
			stRows, stErr = statement.ParseAny(strings.NewReader(data), dec)
		} else {
			st, stErr = statement.ParseWithOptions(data, dec, options)
			stRows = st.Rows
			c.statementAmbiguousDates.Set(st.AmbiguousDates)
			c.statementAmbiguousNumbers.Set(st.AmbiguousNumbers)
			c.statementInvalidRows.Set(len(st.Errors))
			if st.DateOrder != "" {
				c.dateOrderChoice.Set(string(st.DateOrder))
			}
			if st.NumberFormat.DecimalSep == ',' {
				c.numberFormatChoice.Set("comma")
			} else if st.NumberFormat.DecimalSep == '.' && st.NumberFormat.GroupSep == ',' {
				c.numberFormatChoice.Set("dot")
			}
		}
		err := stErr
		if err != nil {
			c.showStatementFailure(data, dec, err)
			return
		}
		rows := statementRowsForDraft(stRows, dec)
		if len(rows) == 0 {
			c.msg.Set(uistate.T("documents.stmtNoneFound"))
			return
		}
		// Successful parse — hide wizard, populate draft.
		c.wizardVisible.Set(false)
		c.setDraft(rows)
		c.msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")) + statementImportCounts(c.statementAmbiguousDates.Get(), c.statementAmbiguousNumbers.Get(), c.statementInvalidRows.Get()))
		c.rev.Set(c.rev.Get() + 1)
	}))
}

func (c *documentsController) useOnDateOrderChoice() {
	c.onDateOrderChoice = ui.UseEvent(func(e ui.Event) { c.dateOrderChoice.Set(e.GetValue()) })
	c.onNumberFormatChoice = ui.UseEvent(func(e ui.Event) { c.numberFormatChoice.Set(e.GetValue()) })
}

func (c *documentsController) useApplyStatementFormats() {
	c.applyStatementFormats = ui.UseEvent(Prevent(func() {
		data := c.statementRaw.Get()
		if data == "" {
			return
		}
		dec := currency.Decimals(reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get()))
		options := statementParseOptions(c.dateOrderChoice.Get(), c.numberFormatChoice.Get())
		st, err := statement.ParseWithOptions(data, dec, options)
		if err != nil {
			c.msg.Set(uistate.T("documents.stmtError", strings.TrimPrefix(err.Error(), "statement: ")))
			return
		}
		c.statementAmbiguousDates.Set(st.AmbiguousDates)
		c.statementAmbiguousNumbers.Set(st.AmbiguousNumbers)
		c.statementInvalidRows.Set(len(st.Errors))
		c.statementFormatOptionsVisible.Set(true)
		rows := statementRowsForDraft(st.Rows, dec)
		if len(rows) == 0 {
			c.msg.Set(uistate.T("impc.noValidRows", len(st.Errors)))
			return
		}
		c.confirmReplace(rows)
		c.msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")) + statementImportCounts(st.AmbiguousDates, st.AmbiguousNumbers, len(st.Errors)))
	}))
}

func (c *documentsController) useApplyWizard() {
	c.applyWizard = func() {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		c.cancelAI()
		dec := currency.Decimals(reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get()))
		p := c.wizardSelection().Profile("manual", dec)
		rows, rowErrors, rowWarnings := importdraft.ApplyProfile(p, c.wizardRawRows.Get())
		if len(rows) == 0 {
			c.msg.Set(uistate.T("documents.wizardNoRows", csvSkipDetail(mapImportErrors(rowErrors))))
			return
		}
		c.wizardVisible.Set(false)
		c.setDraft(rows)
		c.msg.Set(importdraft.ParsedMessage(len(rows), rowErrors, rowWarnings, uistate.T))
		c.rev.Set(c.rev.Get() + 1)
	}
}

func (c *documentsController) useSaveProfile() {
	c.saveProfile = func() {
		name := strings.TrimSpace(c.profileName.Get())
		if name == "" {
			return
		}
		p := c.wizardSelection().Profile(name, currency.Decimals(reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get())))
		saved, err := c.app.SaveImportProfile(importmap.SavedProfile{Profile: p}, activeImportProfileWorkspaceID())
		if err != nil {
			c.msg.Set(uistate.T("tc4n6.profileSaveFailed", err.Error()))
			return
		}
		c.profileName.Set("")
		c.msg.Set(uistate.T("documents.profileSaved", saved.Profile.Name))
		c.rev.Set(c.rev.Get() + 1)
	}
}

func (c *documentsController) useCreateCadenceReminder() {
	c.createCadenceReminder = ui.UseEvent(func() {
		acctName := importdraft.AccountName(c.accounts, c.importAcct.Get())
		label := uistate.T("imb.reminder")
		if acctName != "" {
			label = uistate.T("imb.accountReminder", acctName)
		}
		t := importdraft.Reminder(id.New(), label, time.Now())
		if err := c.app.PutTask(t); err != nil {
			c.cadenceMsg.Set(uistate.T("tc4n6.reminderFailed", err.Error()))
			return
		}
		// C18: confirm right next to the button (not in the far-away top message).
		c.cadenceMsg.Set(uistate.T("documents.cadenceCreated", t.Due.Format("Jan 2, 2006")))
		c.rev.Set(c.rev.Get() + 1)
	})
}

func (c *documentsController) useChooseImage() {
	c.chooseImage = ui.UseEvent(func() { c.chooseImportFile("receipt") })
	c.settings = c.app.Settings()
	c.pr = uistate.UsePrefs().Get().Normalize()
	c.useBackendAI = uistate.BackendConnectionActive(c.pr)
	c.aiModel = c.settings.OpenAIModel
	if c.aiModel == "" {
		c.aiModel = "gpt-5.4-mini"
	}
}

func (c *documentsController) useReadAI() {
	c.readAI = ui.UseEvent(func() { c.requestExtraction("receipt") })
}

func (c *documentsController) useChoosePDF() {
	c.choosePDF = ui.UseEvent(func() { c.chooseImportFile("pdf") })
}

func (c *documentsController) useOnTogglePdfSave() {
	c.onTogglePdfSave = ui.UseEvent(func() { c.pdfSaveCopy.Set(!c.pdfSaveCopy.Get()) })
}

func (c *documentsController) useRunPDFAI() {
	c.runPDFAI = ui.UseEvent(func() { c.requestExtraction("pdf") })
}

func (c *documentsController) useExtractWithAI() {
	c.extractWithAI = ui.UseEvent(func() { c.requestExtraction("stmt") })
}

func (c *documentsController) useCategorizeDraft() {
	c.categorizeDraft = ui.UseEvent(func() {
		if c.busy() {
			return
		}
		c.cancelAI()
		c.model.Suggest(c.app.Rules(), c.app.Categories(), c.app.Transactions(), c.recDec)
		c.syncDraft()
		c.msg.Set(uistate.T("imb.suggested"))
	})
}

func (c *documentsController) useDoImportDraft() {
	c.doImportDraft = func() {
		if c.stage.Get() != "review" || len(c.model.Rows) == 0 || c.busy() {
			return
		}
		if c.summary.Invalid > 0 || c.summary.Overflow || len(c.model.Errors) > 0 {
			c.msg.Set(uistate.T("imb.fixRows"))
			return
		}
		c.commitStaged(c.batch, false)
	}
}

func (c *documentsController) useImportDraft() {
	c.importDraft = ui.UseEvent(Prevent(func() { c.doImportDraft() }))
}

func (c *documentsController) useDoImportReceipt() {
	c.doImportReceipt = func() { c.commitReceipt("") }
}

func (c *documentsController) useImportReceipt() {
	c.importReceipt = ui.UseEvent(Prevent(func() { c.doImportReceipt() }))
}

func (c *documentsController) useSaveModalImport() {
	c.saveModalImport = ui.UseEvent(Prevent(func() {
		action := importdraft.FooterAction(c.stage.Get(), c.source.Get(), len(c.model.Rows), c.csvPreflightS.Get() != nil, c.busy() || c.aiLoading.Get())
		switch action {
		case "review":
			if c.receiptMode.Get() {
				c.doImportReceipt()
			} else {
				c.doImportDraft()
			}
		case "csv-confirm":
			c.commitCSVImport(nil)
		case "csv-preview":
			c.buildCSVPreflight([]byte(strings.TrimSpace(c.csvText.Get())))
		case "show-review":
			c.stage.Set("review")
		case "done":
			c.importModalOpen.Set(false)
		}
	}))
}

func (c *documentsController) useRemoveDraft() {
	c.removeDraft = func(i int) {
		if c.busy() || c.aiLoading.Get() || i < 0 || i >= len(c.model.Rows) {
			return
		}
		c.model.Remove(c.model.Rows[i].ID)
		c.syncDraft()
	}
	c.updateDraft = func(_ int, row extract.Row) {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		c.model.Replace(row.ID, row)
		c.syncDraft()
	}
	c.rows = c.model.Rows
	c.reviewStart, c.reviewEnd, c.currentReviewPage, c.reviewPageCount = draftReviewWindow(len(c.rows), c.reviewPage.Get())
	c.visibleRows = c.rows[c.reviewStart:c.reviewEnd]
	c.reviewCur = reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get())
	c.recBaseCur = c.reviewCur
	c.recDec = currency.Decimals(c.reviewCur)
	receiptMatchDate := importdraft.ReceiptMatchDate(c.rows, time.Now())
	c.receiptCands = ui.UseMemo(func() []receiptmatch.Candidate {
		if !c.receiptMode.Get() {
			return nil
		}
		return importdraft.ReceiptCandidates(c.rows, c.receiptMerchant.Get(), c.receiptTotal.Get(), c.recDec, c.app.Transactions(), receiptMatchDate)
	}, c.app, c.app.Rev(), c.model.Revision, len(c.rows), c.receiptMode.Get(), c.receiptTotal.Get(), c.receiptMerchant.Get(), c.reviewCur, receiptMatchDate)
	c.batch = ui.UseMemo(func() appstate.StagedImport {
		batch, err := c.app.Stage(c.documentKind(), c.rows, c.importAcct.Get())
		if err != nil {
			return appstate.StagedImport{}
		}
		return batch
	}, c.app, c.app.Rev(), c.model.Revision, len(c.rows), c.importAcct.Get())
	c.summary = ui.UseMemo(func() importdraft.Summary { return importdraft.Summarize(c.batch.Batch) }, c.app.Rev(), c.model.Revision, len(c.batch.Rows), c.batch.Rows)
}

func (c *documentsController) useAttachReceipt() {
	c.attachReceipt = func(txnID string) { c.commitReceipt(txnID) }
}

func (c *documentsController) useReceiptToggle() {
	c.receiptToggle = uiw.ToggleRow(uiw.ToggleRowProps{
		Label: uistate.T("documents.receiptToggle"), On: c.receiptMode.Get(),
		OnChange: func(on bool) {
			if c.busy() || c.aiLoading.Get() {
				return
			}
			c.receiptTotal.Set(importdraft.ReceiptTotal(c.model.Rows, c.receiptTotal.Get(), c.recDec, on))
			c.receiptMode.Set(on)
		},
	})
}

func (c *documentsController) useSeenSigs() {
	c.seenSigs = nil
	c.clearDraft = ui.UseEvent(func() {
		if c.busy() || c.aiLoading.Get() {
			return
		}
		uistate.ConfirmModal(uistate.T("imb.startOverConfirm"), true, func(ok bool) {
			if ok && !c.busy() {
				c.cancelAI()
				c.model.Reset()
				c.syncDraft()
			}
		})
	})
	c.rollbackImport = func(d domain.Document) {
		uistate.ConfirmModal(uistate.T("documents.rollbackConfirm"), true, func(ok bool) {
			if !ok || c.busy() {
				return
			}
			if uistate.RestoreCheckpoint(d.CheckpointID) {
				c.rev.Set(c.rev.Get() + 1)
			} else {
				c.msg.Set(uistate.T("ckpt.restoreErr"))
			}
		})
	}
}

func (c *documentsController) usePdfCard() {
	remove := ui.UseEvent(func() { c.removeSourceFile("pdf") })
	processing := c.aiLoading.Get()
	buttonStyle := map[string]string{}
	if processing {
		buttonStyle["opacity"] = "1"
		buttonStyle["cursor"] = "progress"
	}
	c.pdfFileLabel = firstNonEmpty(c.pdfName.Get(), uistate.T("importux.pdfHint"))
	c.pdfCard = Fragment(
		importFormHeader(true, uiw.Icon(icon.FileText, css.Class(tw.W5, tw.H5)),
			uistate.T("documents.typePdfTitle"), uistate.T("documents.pdfFormDesc")),
		Div(css.Class("doc-form-body"),
			Div(css.Class(importDropZone), Attr("data-testid", "statementimport-dropzone"), OnDragOver(c.onFileDragOver), OnDrop(c.onDropPDF),
				Div(css.Class(importStack),
					Span(css.Class("statement-file"), Style(map[string]string{"overflow-wrap": "anywhere"}), Attr("data-testid", "statementimport-filename"), c.pdfFileLabel),
					Span(css.Class(importMuted), IfElse(c.pdfData.Get() == "", Text(uistate.T("importux.pdfLimit")), Text(importdraft.FileSizeLabel(len(c.pdfBytes.Get())))))),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "statementimport-choose"), Disabled(c.busy() || c.aiLoading.Get()), OnClick(c.choosePDF),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("importux.chooseFile"))),
				If(c.pdfData.Get() != "", Button(css.Class("btn btn-ghost"), Type("button"), Attr("data-testid", "statementimport-remove"), Disabled(c.busy() || c.aiLoading.Get()), OnClick(remove), uistate.T("importux.removeFile")))),
			Div(css.Class(importActionRow), buttonWithDisabled(c.pdfData.Get() == "" || processing || c.busy(),
				[]any{css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Style(buttonStyle), Type("button"), Attr("aria-busy", strconv.FormatBool(processing)), Attr("data-testid", "statementimport-run"), OnClick(c.runPDFAI)},
				IfElse(processing,
					Span(css.Class("dt-spin dt-spin-sm"), Style(map[string]string{"border-color": "rgba(255,255,255,0.4)", "border-top-color": "#fff"}), Attr("aria-hidden", "true"), Attr("data-testid", "statementimport-spinner")),
					uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				IfElse(processing, Text(uistate.T("statementimport.reading")), Text(uistate.T("importux.readStatement"))), KeyGateMark(c.canSend("pdf")))),
			If(c.pdfData.Get() != "", Label(css.Class("statement-savedoc", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "statementimport-savedoc"), OnChange(c.onTogglePdfSave)}, checkedAttr(c.pdfSaveCopy.Get())...)...),
				Div(css.Class("row-main"),
					Span(uistate.T("statementimport.saveDoc")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("statementimport.saveDocHint"))))),
		),
	)
}

func (c *documentsController) useFlowScreen() {
	c.flowScreen = importFlowScreenFor(importFlowState{Stage: importFlowStage(c.stage.Get()), Source: importFlowSource(c.source.Get())})
	c.inReview = c.flowScreen == importScreenReview
}

// wizardSelection reads controls; profile building and mapping stay in importdraft.
func (c *documentsController) wizardSelection() importdraft.WizardSelection {
	return importdraft.WizardSelection{Date: c.wizardDate.Get(), Description: c.wizardDescCol.Get(), Amount: c.wizardAmount.Get(), Debit: c.wizardDebit.Get(), Credit: c.wizardCredit.Get(), NumberFormat: c.numberFormatChoice.Get(), DateOrder: c.dateOrderChoice.Get()}
}

func (c *documentsController) showStatementFailure(data string, dec int, err error) {
	// Tier 3: scanned/image-only or encrypted PDF — don't show the wizard,
	// show a helpful message directing the user to Extract with AI or image import.
	if importdraft.StatementFailure(err) == "scanned" {
		c.msg.Set(uistate.T("documents.scannedPdf"))
		return
	}
	// If the error is about missing columns, offer the wizard instead of a
	// bare error message (C74 wizard path). Also try to populate the wizard
	// header from a raw Parse so the user can see column names.
	if importdraft.StatementFailure(err) == "mapping" {
		// Try a raw Parse to extract header/column info for the wizard.
		raw, rerr := statement.Parse(data, dec)
		// Even if rerr is non-nil, raw.Delimiter is set from the first line,
		// which is enough to populate the wizard column list.
		header, rawRecs := extractRawHeaderAndRows(data, raw.Delimiter)
		_ = rerr
		if len(rawRecs) > maxImportRows {
			c.msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
			return
		}
		c.wizardHeader.Set(header)
		c.wizardRawRows.Set(rawRecs)
		// Pre-fill wizard fields from whatever was auto-detected, with a
		// name-based fallback (C15): if auto-detect found -1 for a field,
		// scan header names for case-insensitive keyword matches so that
		// common header names like "Date", "Amount", "Description" are
		// pre-selected rather than left as "— not present —".
		c.wizardDate.Set(guessWizardField(header, []string{"date", "posted", "trans"}, raw.Columns.Date))
		c.wizardDescCol.Set(guessWizardField(header, []string{"desc", "memo", "narr", "detail", "note", "ref"}, raw.Columns.Description))
		c.wizardAmount.Set(guessWizardField(header, []string{"amount", "value", "amt", "sum"}, raw.Columns.Amount))
		c.wizardDebit.Set(guessWizardField(header, []string{"debit", "withdrawal", "dr"}, raw.Columns.Debit))
		c.wizardCredit.Set(guessWizardField(header, []string{"credit", "deposit", "cr"}, raw.Columns.Credit))
		c.wizardVisible.Set(true)
		c.msg.Set(uistate.T("imb.mappingNeeded"))
		return
	}
	c.msg.Set(uistate.T("documents.stmtError", strings.TrimPrefix(err.Error(), "statement: ")))
	return
}

// useDraftSession installs the session hooks at one fixed position in useNav.
func (c *documentsController) useDraftSession() {
	c.aiLoading = ui.UseState(false)
	c.aiErr = ui.UseState("")
	c.needsKey = ui.UseState(false)
	c.draft = uistate.UseImportDraftRows()
	c.draftMetadata = state.UseAtom("documents:draft-metadata", importdraft.Metadata{})
	c.reviewPage = ui.UseState(1)
	c.importAcct = ui.UseState(c.defaultAcc)
	c.receiptMode = ui.UseState(false)
	c.receiptTotal = ui.UseState("")
	c.receiptMerchant = ui.UseState("")
	c.stage = ui.UseState("input")
	c.job = ui.UseMemo(func() *importPanelJob {
		model := importdraft.New(c.draft.Get())
		model.RestoreMetadata(c.draftMetadata.Get())
		return &importPanelJob{model: model}
	}, "import-session")
	c.model = c.job.model
	c.setDraft = c.appendRows
	ui.UseEffect(func() func() {
		c.syncDraft()
		activeImportPanel = c.job
		return func() {
			c.job.close()
			if activeImportPanel == c.job {
				activeImportPanel = nil
			}
		}
	}, "import-lifetime")
	ui.UseEffect(func() func() {
		if len(c.draft.Get()) > 0 {
			c.stage.Set("review")
		}
		return nil
	}, "resume-mount")
}
