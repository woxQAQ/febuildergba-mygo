package pages

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/hexeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/records"
)

// Actions connects page intent to the owning desktop window.
type Actions struct {
	Open                  func()
	Save                  func(bool)
	Reload                func()
	Run                   func()
	HistoryContextChanged func()
}

type Workspace struct {
	Editor      *editor.State
	Actions     Actions
	tab         int
	records     *records.Page
	hex         *hexeditor.Page
	textHistory bool
}

func New() *Workspace {
	w := &Workspace{Editor: &editor.State{}}
	w.hex = hexeditor.New(w.Editor)
	w.records = records.New(w.Editor, w.openHex, func(kind game.Kind) { w.tab = slices.Index(game.Kinds, kind) })
	return w
}

func (w *Workspace) SetSession(session *game.Session) {
	w.Editor.SetSession(session)
	w.records.Reset()
	w.hex.Reset()
	w.tab = 0
}

func (w *Workspace) openHex(address, length int) {
	w.tab = len(game.Kinds)
	w.hex.Open(address, length)
}

// UsesTextHistory includes native dialogs, whose fields own their text history.
func (w *Workspace) UsesTextHistory() bool {
	return w.Editor.TextFocused || w.Editor.Busy
}

func (w *Workspace) View(c *ui.Context) {
	state := w.Editor
	state.TextFocused = false
	components.ApplyTheme(c)
	ui.Column(c).Fill().Children(func() {
		components.Header(c, state)
		components.ErrorBanner(c, state)
		if state.Session == nil {
			empty(c)
			return
		}
		components.ExternalChangeBanner(c, state, w.Actions.Reload)
		ui.Tabs(c, &w.tab, locales.Units(), locales.Classes(), locales.Items(), locales.HexEditor())
		if w.tab == len(game.Kinds) {
			w.hex.View(c)
		} else {
			w.records.View(c, w.tab)
		}
		components.StatusBar(c, state, w.Actions.Run)
	})
	if textHistory := w.UsesTextHistory(); textHistory != w.textHistory {
		w.textHistory = textHistory
		if w.Actions.HistoryContextChanged != nil {
			w.Actions.HistoryContextChanged()
		}
	}
	if state.Busy {
		return
	}
	if !state.TextFocused {
		if c.Shortcut(ui.Cmd, ui.KeyZ) {
			state.Undo()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
			state.Redo()
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyO) && w.Actions.Open != nil {
		w.Actions.Open()
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) && state.Session != nil && w.Actions.Save != nil {
		w.Actions.Save(false)
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyS) && state.Session != nil && w.Actions.Save != nil {
		w.Actions.Save(true)
	}
}
