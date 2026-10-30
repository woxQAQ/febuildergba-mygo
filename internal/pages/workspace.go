package pages

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/eventeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/hexeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/portraits"
	recordspage "github.com/woxQAQ/febuildergba-mygo/internal/pages/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/texteditor"
)

// HeaderHeight is the desktop title-bar height exposed by the page composition.
const HeaderHeight = components.HeaderHeight

// Actions connects page intent to the owning desktop window.
type Actions struct {
	Open                  func()
	Save                  func(bool)
	Reload                func()
	Run                   func()
	ImportPNG             func(func([]byte, error))
	ExportPNG             func(graphics.Ref)
	HistoryContextChanged func()
}

type Workspace struct {
	Editor      *editor.State
	Actions     Actions
	tab         int
	records     *recordspage.Page
	hex         *hexeditor.Page
	texts       *texteditor.Page
	scripts     *eventeditor.Page
	portraits   *portraits.Page
	textHistory bool
}

func New() *Workspace {
	w := &Workspace{Editor: &editor.State{}}
	w.hex = hexeditor.New(w.Editor)
	w.texts = texteditor.New(
		w.Editor,
		func(kind records.Kind, id int) { w.records.Open(kind, id) },
	)
	assets := components.AssetActions{
		Import: func(done func([]byte, error)) {
			if w.Actions.ImportPNG != nil {
				w.Actions.ImportPNG(done)
			}
		},
		Export: func(ref graphics.Ref) {
			if w.Actions.ExportPNG != nil {
				w.Actions.ExportPNG(ref)
			}
		},
	}
	w.records = recordspage.New(
		w.Editor,
		w.openHex,
		w.openText,
		func(kind records.Kind) { w.tab = slices.Index(records.Kinds, kind) },
		assets,
	)
	w.portraits = portraits.New(w.Editor, assets, w.records.Open, w.openHex)
	w.records.SetPortraitOpener(w.OpenPortrait)
	w.scripts = eventeditor.New(
		w.Editor,
		w.openHex,
		w.openText,
		w.openDeployment,
	)
	w.scripts.SetMapChangeOpener(w.openMapChange)
	w.records.SetScriptOpener(w.OpenScript)
	return w
}

func (w *Workspace) SetSession(session *game.Session) {
	w.Editor.SetSession(session)
	w.records.Reset()
	w.hex.Reset()
	w.texts.Reset()
	w.scripts.Reset()
	w.portraits.Reset()
	w.tab = 0
}

func (w *Workspace) openHex(address, length int) {
	w.tab = len(records.Kinds) + 1
	w.hex.Open(address, length)
}

func (w *Workspace) openText(id uint16) {
	w.tab = len(records.Kinds)
	w.texts.Open(id)
}

// OpenScript navigates to a chapter's physical script entry pointer.
func (w *Workspace) OpenScript(chapter, site int) {
	w.tab = len(records.Kinds) + 2
	w.scripts.Open(chapter, site)
}

// OpenPortrait navigates to the resource table referenced by a record.
func (w *Workspace) OpenPortrait(
	id int,
) {
	w.tab = len(records.Kinds) + 3
	w.portraits.Open(id)
}

func (w *Workspace) openMapChange(chapter, id int) {
	w.records.OpenMapChange(chapter, id)
}

func (w *Workspace) openDeployment(chapter, site int) {
	w.records.OpenDeployment(chapter, site)
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
		labels := make([]string, 0, len(records.Kinds)+2)
		for _, kind := range records.Kinds {
			labels = append(labels, locales.Label(string(kind)))
		}
		labels = append(
			labels,
			locales.TextEditor(),
			locales.HexEditor(),
			locales.ScriptEditor(),
			locales.PortraitTable(),
		)
		ui.Tabs(c, &w.tab, labels...)
		if w.tab >= len(records.Kinds) {
			w.records.Deactivate()
		}
		switch w.tab {
		case len(records.Kinds):
			w.texts.View(c)
		case len(records.Kinds) + 1:
			w.hex.View(c)
		case len(records.Kinds) + 2:
			w.scripts.View(c)
		case len(records.Kinds) + 3:
			w.portraits.View(c)
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
