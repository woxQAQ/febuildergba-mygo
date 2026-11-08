package components

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
)

// TextDraft declares native text history for the wrapped input while it holds
// the focus: undo and redo edit the input's text, never the ROM document. The
// declaration replaces any inner claim, so a caller can keep native text
// history around a control that commits immediately.
func TextDraft(state *editor.State, input ui.Element) ui.Element {
	if input.FocusWithin() {
		state.HistoryClaim = editor.HistoryClaimText
	}
	return input
}

// DocumentField declares document history for a text-taking control whose
// OnTextKey already routes undo and redo to State.Undo and State.Redo: its
// values commit immediately, so undo belongs to the ROM document even while
// the control holds the focus. An outer TextDraft claim wins where a caller
// explicitly keeps native text history.
func DocumentField(state *editor.State, input ui.Element) ui.Element {
	if input.FocusWithin() {
		state.HistoryClaim = editor.HistoryClaimDocument
	}
	return input
}
