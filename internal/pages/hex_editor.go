package pages

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/hexeditor"
)

// HexEditor owns a tool window's byte draft and history focus. Its document
// state is shared with the owning workspace.
type HexEditor struct {
	Editor           *editor.State
	Reload           func()
	MenuStateChanged func()
	page             *hexeditor.Page
	session          *game.Session
	menuState        struct{ text, undo, redo, busy bool }
}

func NewHexEditor(source *editor.State) *HexEditor {
	state := source.NewView()
	return &HexEditor{Editor: state, page: hexeditor.New(state)}
}

func (viewer *HexEditor) syncSession() {
	if viewer.session != viewer.Editor.Session {
		viewer.session = viewer.Editor.Session
		viewer.page.Reset()
	}
}

func (viewer *HexEditor) Open(address, length int) {
	viewer.syncSession()
	viewer.page.Open(address, length)
}

func (viewer *HexEditor) View(c *ui.Context) {
	viewer.syncSession()
	state := viewer.Editor
	state.HistoryClaim = editor.HistoryClaimNone
	state.DraftHistory = nil
	components.ApplyTheme(c)
	ui.Column(c.Key("hex-window")).Fill().Children(func() {
		components.ErrorBanner(c, state)
		components.ExternalChangeBanner(c, state, viewer.Reload)
		viewer.page.View(c)
		if state.Session != nil {
			components.StatusBar(c, state, nil, nil)
		}
	})
	state.ResolveHistoryFocus(c.FocusTakesText())
	history := state.History()
	context := struct{ text, undo, redo, busy bool }{
		text: state.TextFocused, undo: history.Undo != nil,
		redo: history.Redo != nil, busy: state.Busy,
	}
	if context != viewer.menuState {
		viewer.menuState = context
		if viewer.MenuStateChanged != nil {
			viewer.MenuStateChanged()
		}
	}
	if !state.Busy && !state.TextFocused {
		c.OnShortcut(ui.Cmd, ui.KeyZ, state.Undo)
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, state.Redo)
	}
}
