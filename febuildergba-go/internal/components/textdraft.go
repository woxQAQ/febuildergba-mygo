package components

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
)

// TextDraft keeps text editing commands with the focused, unapplied draft.
func TextDraft(state *editor.State, input ui.Element) ui.Element {
	state.TextFocused = state.TextFocused || input.Focused()
	return input
}
