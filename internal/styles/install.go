// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

import "syscall/js"

// Register builds the app design system (the typed rules in rules_gen.go) and injects
// it into a dedicated <style id="cf-app-css"> in <head>. Call it ONCE, as the first
// thing in app.Run() — before any rendering — so:
//   - the design system is present before the app paints, and
//   - it registers before the css utility engine creates its own <style id="gwc-css">
//     during render, preserving the cascade (design system first, utilities after, so
//     equal-specificity utilities still win — exactly as when the CSS was static).
//
// The boot splash (web/index.html's small inline boot-critical style) covers the
// window before the wasm loads and this runs.
func Register() {
	resetSheet()
	registerGenerated()
	registerRecurringSurface()
	registerRecurTasksSurface()
	registerReportsSurface()
	registerNetWorthSurface()
	registerHealthSurface()
	registerAssistantSurface()
	registerCreditSurface()
	registerStudioSurface()
	registerSmartSurface()
	registerFieldsSurface()
	registerStudioTabs()
	registerWorkflowsSurface()
	registerHouseholdSurface()
	registerCategoriesSurface()
	registerRulesSurface()
	registerVaultSurface()
	registerRecordSurface()
	registerSystemSurface()
	registerBudgetsSurface()
	registerBudgetFlexSurface()
	registerCoverAllSurface()
	registerAnnualGridSurface()
	registerGoalStatesWidget()
	registerGoalHealthTones()
	registerGoalOrder()
	registerReportsAnnual()
	registerReportsSummary()
	registerReportsVitals()
	registerBudgetTargets()
	registerTierSystem()
	registerImportWizard()
	registerTxnToolbar()
	registerTxnEditSurface()
	registerTxnCalendar()
	registerSavedViews()
	registerAccountsSurface()
	registerGoalsSurface()
	registerGoalTrajectorySurface()
	registerCalendarSurface()
	registerTodoCalSurface()
	registerTodoBoardSurface()
	registerTxnTemplatesSurface()
	registerNotifyHistorySurface()
	registerMerchantTrendSurface()
	registerPayeeCleanSurface()
	registerRecapSurface()
	registerPeriodBadge()
	registerMobileShell()
	registerHeaderBalance()
	registerLane3Mobile()
	registerTxnUpcoming()
	registerReviewInboxSurface()
	registerTxcFieldsSurface()
	registerNotifySurface()
	registerTodoPrioSurface()
	registerTodoPolish()
	registerDashHeroSurface()
	registerR4Surface()
	registerLane2Dashboard()
	registerLane6Fixes()
	registerBgPolish()
	registerDtxPolish()
	registerNotifTriage()
	registerDashTodo()
	registerNotifAsst()
	registerAcctxn()
	registerBudgetRefine()
	registerDesignPolish()
	registerDpType()
	registerDpColor()
	registerDpSerif()
	registerDpBorders()
	registerDpHeader()
	registerDpAlign()
	registerDpLinks()
	registerDpRadius()
	registerDpControls()
	registerAcctDetails()
	registerUxbatch3()
	registerUxbatch4()
	inject(Build())
}

// inject writes the rendered stylesheet into the managed <style id="cf-app-css">,
// creating it on first call.
func inject(cssText string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	style := doc.Call("getElementById", "cf-app-css")
	if !style.Truthy() {
		style = doc.Call("createElement", "style")
		style.Set("id", "cf-app-css")
		head := doc.Get("head")
		if !head.Truthy() {
			head = doc.Get("documentElement")
		}
		if head.Truthy() {
			head.Call("appendChild", style)
		}
	}
	style.Set("textContent", cssText)
}
