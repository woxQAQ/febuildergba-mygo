package pages

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/eventeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/hexeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/portraits"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/quotes"
	recordspage "github.com/woxQAQ/febuildergba-mygo/internal/pages/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/shops"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/texteditor"
)

// HeaderHeight is the desktop title-bar height exposed by the page composition.
const HeaderHeight = components.HeaderHeight

// Actions connects page intent to the owning desktop window.
type Actions struct {
	Open             func()
	Save             func(bool)
	Reload           func()
	Run              func()
	ImportPNG        func(func([]byte, error))
	ExportPNG        func(graphics.Ref)
	MenuStateChanged func()
}

type Workspace struct {
	Editor    *editor.State
	Actions   Actions
	tab       int
	records   *recordspage.Page
	hex       *hexeditor.Page
	texts     *texteditor.Page
	scripts   *eventeditor.Page
	portraits *portraits.Page
	shops     *shops.Page
	quotes    *quotes.Page
	zoomLevel int
	menuState struct {
		text, draft, undo, redo bool
		zoom                    int
	}
}

func New() *Workspace {
	w := &Workspace{Editor: &editor.State{}, zoomLevel: 1}
	w.hex = hexeditor.New(w.Editor)
	w.texts = texteditor.New(
		w.Editor,
		func(kind records.Kind, id int) {
			for _, table := range w.Editor.Session.Quotes.Tables() {
				if string(table.Kind) == string(kind) {
					w.OpenQuote(table.Kind, id)
					return
				}
			}
			w.records.Open(kind, id)
		},
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
	w.shops = shops.New(w.Editor, w.records.OpenShop, w.openHex)
	w.quotes = quotes.New(w.Editor, w.records.Open, w.openText, w.openHex)
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
	w.shops.Reset()
	w.quotes.Reset()
	w.tab = 0
}

// OpenQuote navigates from a text use to its owning native table row.
func (w *Workspace) OpenQuote(kind quote.Kind, index int) {
	w.tab = len(records.Kinds) + 5
	w.quotes.Open(kind, index)
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
	c.SetZoom(float32(w.ZoomPercent()) / 100)
	w.zoomShortcuts(c)
	state := w.Editor
	state.TextFocused = false
	state.DraftHistory = nil
	components.ApplyTheme(c)
	ui.Column(c).Fill().Children(func() {
		components.Header(c, state, func() { w.zoomControl(c) })
		components.ErrorBanner(c, state)
		if state.Session == nil {
			w.empty(c)
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
			locales.ShopEntries(),
			locales.QuoteEditor(),
		)
		ui.ScrollHorizontal(c.Key("workspace-tabs")).Shrink(0).Children(func() {
			ui.Tabs(c, &w.tab, labels...).Shrink(0)
		})
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
		case len(records.Kinds) + 4:
			w.shops.View(c)
		case len(records.Kinds) + 5:
			w.quotes.View(c)
		default:
			w.records.View(c, w.tab)
		}
		components.StatusBar(c, state, w.Actions.Run)
	})
	history := state.History()
	context := struct {
		text, draft, undo, redo bool
		zoom                    int
	}{
		text: w.UsesTextHistory(), draft: state.DraftHistory != nil,
		undo: history.Undo != nil, redo: history.Redo != nil,
		zoom: w.ZoomPercent(),
	}
	if context != w.menuState {
		w.menuState = context
		if w.Actions.MenuStateChanged != nil {
			w.Actions.MenuStateChanged()
		}
	}
	if state.Busy {
		return
	}
	if !state.TextFocused {
		c.OnShortcut(ui.Cmd, ui.KeyZ, state.Undo)
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, state.Redo)
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
