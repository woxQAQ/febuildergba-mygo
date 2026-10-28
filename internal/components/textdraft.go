package components

import (
	"github.com/egoist/mygo/ui"
)

// TextDraft keeps text editing commands with the focused, unapplied draft.
func TextDraft(state *State, input ui.Element) ui.Element {
	state.TextFocused = state.TextFocused || input.FocusWithin()
	return input
}
