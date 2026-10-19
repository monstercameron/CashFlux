// SPDX-License-Identifier: MIT

package screens

type importFlowStage string
type importFlowSource string
type importFlowScreen string
type importFlowAction string

const (
	importFlowInput  importFlowStage = "input"
	importFlowReview importFlowStage = "review"

	importSourcePicker  importFlowSource = ""
	importSourceCSV     importFlowSource = "csv"
	importSourceStmt    importFlowSource = "stmt"
	importSourcePDF     importFlowSource = "pdf"
	importSourceReceipt importFlowSource = "receipt"
	importSourceAmazon  importFlowSource = "amazon"

	importScreenStart   importFlowScreen = "start"
	importScreenReview  importFlowScreen = "review"
	importScreenCSV     importFlowScreen = "csv"
	importScreenStmt    importFlowScreen = "statement"
	importScreenPDF     importFlowScreen = "pdf"
	importScreenReceipt importFlowScreen = "receipt"
	importScreenOrder   importFlowScreen = "order"

	importActionBack    importFlowAction = "back"
	importActionNext    importFlowAction = "next"
	importActionAddMore importFlowAction = "add-more"
)

type importFlowState struct {
	Stage  importFlowStage
	Source importFlowSource
}

// importFlowScreenFor selects the visible import screen from the flow state.
func importFlowScreenFor(state importFlowState) importFlowScreen {
	if state.Stage == importFlowReview {
		return importScreenReview
	}
	switch state.Source {
	case importSourceCSV:
		return importScreenCSV
	case importSourceStmt:
		return importScreenStmt
	case importSourcePDF:
		return importScreenPDF
	case importSourceReceipt:
		return importScreenReceipt
	case importSourceAmazon:
		return importScreenOrder
	default:
		return importScreenStart
	}
}

// transitionImportFlow applies the existing back/next controls without changing draft data.
func transitionImportFlow(state importFlowState, action importFlowAction) importFlowState {
	switch action {
	case importActionBack:
		if state.Stage == importFlowReview {
			state.Stage = importFlowInput
			state.Source = importSourcePicker
		} else {
			state.Source = importSourcePicker
		}
	case importActionNext:
		state.Stage = importFlowReview
	case importActionAddMore:
		state.Stage = importFlowInput
		state.Source = importSourcePicker
	}
	return state
}
