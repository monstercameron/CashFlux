// SPDX-License-Identifier: MIT

package screens

import "github.com/monstercameron/CashFlux/internal/importdraft"

type importFlowStage = importdraft.FlowStage
type importFlowSource = importdraft.FlowSource
type importFlowScreen = importdraft.FlowScreen
type importFlowAction = importdraft.FlowAction
type importFlowState = importdraft.FlowState

const (
	importActionAddMore = importdraft.ActionAddMore
	importActionBack    = importdraft.ActionBack
	importActionNext    = importdraft.ActionNext
	importFlowInput     = importdraft.FlowInput
	importFlowReview    = importdraft.FlowReview
	importScreenCSV     = importdraft.ScreenCSV
	importScreenOrder   = importdraft.ScreenOrder
	importScreenPDF     = importdraft.ScreenPDF
	importScreenReceipt = importdraft.ScreenReceipt
	importScreenReview  = importdraft.ScreenReview
	importScreenStart   = importdraft.ScreenStart
	importScreenStmt    = importdraft.ScreenStmt
	importSourceAmazon  = importdraft.SourceAmazon
	importSourceCSV     = importdraft.SourceCSV
	importSourcePDF     = importdraft.SourcePDF
	importSourcePicker  = importdraft.SourcePicker
	importSourceReceipt = importdraft.SourceReceipt
	importSourceStmt    = importdraft.SourceStmt
)

func importFlowScreenFor(state importFlowState) importFlowScreen {
	return importdraft.FlowScreenFor(state)
}

func transitionImportFlow(state importFlowState, action importFlowAction) importFlowState {
	return importdraft.TransitionFlow(state, action)
}
