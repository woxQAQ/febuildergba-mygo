package pages

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/recordform"
	"github.com/woxQAQ/febuildergba-mygo/locales"
	"github.com/woxQAQ/febuildergba-mygo/pages/hexeditor"
	"github.com/woxQAQ/febuildergba-mygo/pages/records"
	"github.com/woxQAQ/febuildergba-mygo/pages/texteditor"
)

// Actions connects page intent to the owning desktop window.
type Actions struct {
	Open                  func()
	Save                  func(bool)
	Reload                func()
	Run                   func()
	ImportPNG             func(func([]byte, error))
	ExportPNG             func(game.AssetRef)
	HistoryContextChanged func()
}

type Workspace struct {
	Editor      *components.State
	Actions     Actions
	tab         int
	records     *records.Page
	hex         *hexeditor.Page
	texts       *texteditor.Page
	textHistory bool
}

func New() *Workspace {
	w := &Workspace{Editor: &components.State{}}
	w.hex = hexeditor.New(w.Editor)
	w.texts = texteditor.New(
		w.Editor,
		func(kind game.Kind, id int) { w.records.Open(kind, id) },
	)
	w.records = records.New(
		w.Editor,
		w.openHex,
		w.openText,
		func(kind game.Kind) { w.tab = slices.Index(game.Kinds, kind) },
		recordform.AssetActions{
			Import: func(done func([]byte, error)) {
				if w.Actions.ImportPNG != nil {
					w.Actions.ImportPNG(done)
				}
			},
			Export: func(ref game.AssetRef) {
				if w.Actions.ExportPNG != nil {
					w.Actions.ExportPNG(ref)
				}
			},
		},
	)
	return w
}

func (w *Workspace) SetSession(session *game.Session) {
	w.Editor.SetSession(session)
	w.records.Reset()
	w.hex.Reset()
	w.texts.Reset()
	w.tab = 0
}

func (w *Workspace) openHex(address, length int) {
	w.tab = len(game.Kinds) + 1
	w.hex.Open(address, length)
}

func (w *Workspace) openText(id uint16) {
	w.tab = len(game.Kinds)
	w.texts.Open(id)
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
		labels := make([]string, 0, len(game.Kinds)+2)
		for _, kind := range game.Kinds {
			labels = append(labels, locales.Label(string(kind)))
		}
		labels = append(labels, locales.TextEditor(), locales.HexEditor())
		ui.Tabs(c, &w.tab, labels...)
		switch w.tab {
		case len(game.Kinds):
			w.texts.View(c)
		case len(game.Kinds) + 1:
			w.hex.View(c)
		default:
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
	if c.Shortcut(ui.Cmd, ui.KeyS) && state.Session != nil &&
		w.Actions.Save != nil {
		w.Actions.Save(false)
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyS) && state.Session != nil &&
		w.Actions.Save != nil {
		w.Actions.Save(true)
	}
}
