// SPDX-License-Identifier: MIT

package importdraft

// FlowStage names one part of the import navigation state.
type FlowStage string

// FlowSource names one part of the import navigation state.
type FlowSource string

// FlowScreen names one part of the import navigation state.
type FlowScreen string

// FlowAction names one part of the import navigation state.
type FlowAction string

const (
	// FlowInput is an import navigation value.
	FlowInput FlowStage = "input"
	// FlowReview is an import navigation value.
	FlowReview FlowStage = "review"

	// SourcePicker is an import navigation value.
	SourcePicker FlowSource = ""
	// SourceCSV is an import navigation value.
	SourceCSV FlowSource = "csv"
	// SourceStmt is an import navigation value.
	SourceStmt FlowSource = "stmt"
	// SourcePDF is an import navigation value.
	SourcePDF FlowSource = "pdf"
	// SourceReceipt is an import navigation value.
	SourceReceipt FlowSource = "receipt"
	// SourceAmazon is an import navigation value.
	SourceAmazon FlowSource = "amazon"

	// ScreenStart is an import navigation value.
	ScreenStart FlowScreen = "start"
	// ScreenReview is an import navigation value.
	ScreenReview FlowScreen = "review"
	// ScreenCSV is an import navigation value.
	ScreenCSV FlowScreen = "csv"
	// ScreenStmt is an import navigation value.
	ScreenStmt FlowScreen = "statement"
	// ScreenPDF is an import navigation value.
	ScreenPDF FlowScreen = "pdf"
	// ScreenReceipt is an import navigation value.
	ScreenReceipt FlowScreen = "receipt"
	// ScreenOrder is an import navigation value.
	ScreenOrder FlowScreen = "order"

	// ActionBack is an import navigation value.
	ActionBack FlowAction = "back"
	// ActionNext is an import navigation value.
	ActionNext FlowAction = "next"
	// ActionAddMore is an import navigation value.
	ActionAddMore FlowAction = "add-more"
)

// FlowState names one part of the import navigation state.
type FlowState struct {
	Stage  FlowStage
	Source FlowSource
}

// FlowScreenFor selects the visible import screen from the flow state.
func FlowScreenFor(state FlowState) FlowScreen {
	if state.Stage == FlowReview {
		return ScreenReview
	}
	switch state.Source {
	case SourceCSV:
		return ScreenCSV
	case SourceStmt:
		return ScreenStmt
	case SourcePDF:
		return ScreenPDF
	case SourceReceipt:
		return ScreenReceipt
	case SourceAmazon:
		return ScreenOrder
	default:
		return ScreenStart
	}
}

// TransitionFlow applies the existing back/next controls without changing draft data.
func TransitionFlow(state FlowState, action FlowAction) FlowState {
	switch action {
	case ActionBack:
		if state.Stage == FlowReview {
			state.Stage = FlowInput
			state.Source = SourcePicker
		} else {
			state.Source = SourcePicker
		}
	case ActionNext:
		state.Stage = FlowReview
	case ActionAddMore:
		state.Stage = FlowInput
		state.Source = SourcePicker
	}
	return state
}
