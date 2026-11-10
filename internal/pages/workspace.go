package pages

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/arenas"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/eventeditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/portraits"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/quotes"
	recordspage "github.com/woxQAQ/febuildergba-mygo/internal/pages/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/shops"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/supports"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/texteditor"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/worldmaps"
)

// HeaderHeight is the desktop title-bar height exposed by the page composition.
const HeaderHeight = components.HeaderHeight

// These offsets follow the record-table tabs.
const (
	textTab = iota
	scriptsTab
	portraitsTab
	shopsTab
	quotesTab
	supportsTab
	arenasTab
	worldmapTab
)

// Actions connects page intent to the owning desktop window.
type Actions struct {
	Open                      func()
	Save                      func(bool)
	Reload                    func()
	Run                       func()
	OpenHex                   func(int, int)
	ImportPNG                 func(func([]byte, error))
	ExportPNG                 func(graphics.Ref)
	ImportWorldMapBackground  func(*worldmap.Background, func(*worldmap.BackgroundDraft, error))
	ExportWorldMapBackground  func(*worldmap.Background)
	ImportWorldMapRoadTileset func(*worldmap.RoadTileset, func(*worldmap.RoadTilesetDraft, error))
	ExportWorldMapRoadTileset func(*worldmap.RoadTileset)
	ImportMapTileset          func(*maps.TilesetConfig, func(*maps.TilesetDraft, error))
	ExportMapTileset          func(*maps.TilesetConfig)
	ImportMapPalette          func(*maps.Palette, maps.PaletteSection, func(*maps.PaletteDraft, error))
	ExportMapPalette          func(*maps.Palette, maps.PaletteSection)
	ImportMapGraphics         func(*maps.Graphics, maps.GraphicsImport, func(*maps.GraphicsDraft, error))
	ExportMapGraphics         func(*maps.Graphics, int)
	ImportTextFile            func(*text.File, func(*text.Import, string, error))
	ExportTextFile            func(*text.File, []uint16)
	MenuStateChanged          func()
}

type Workspace struct {
	Editor    *editor.State
	Actions   Actions
	tab       int
	records   *recordspage.Page
	texts     *texteditor.Page
	scripts   *eventeditor.Page
	portraits *portraits.Page
	shops     *shops.Page
	quotes    *quotes.Page
	defeats   map[int]*quotes.Page
	supports  *supports.Page
	arenas    *arenas.Page
	worldmap  *worldmaps.Page
	zoomLevel int
	menuState struct {
		text, draft, undo, redo bool
		busy, session           bool
		zoom                    int
	}
}

func New() *Workspace {
	w := &Workspace{Editor: editor.New(), zoomLevel: 1}
	w.texts = texteditor.New(
		w.Editor,
		func(use text.Use) {
			kind, id := records.Kind(use.Kind), use.ID
			if use.Kind == event.TextUse {
				w.tab = len(records.Kinds) + scriptsTab
				w.scripts.OpenInstruction(
					id,
					use.Pointer,
					use.Address,
					use.Source,
				)
				return
			}
			if string(kind) == worldmap.Resource {
				w.openWorldMap(id)
				return
			}
			if string(kind) == support.TalkUse {
				w.tab = len(records.Kinds) + supportsTab
				w.supports.OpenTalk(id)
				return
			}
			for _, table := range w.Editor.Session.Quotes.Tables() {
				if string(table.Kind) == string(kind) {
					w.OpenQuote(table.Kind, id)
					return
				}
			}
			w.records.Open(kind, id)
		},
	)
	w.texts.SetFileActions(
		func(source *text.File, done func(*text.Import, string, error)) {
			if w.Actions.ImportTextFile != nil {
				w.Actions.ImportTextFile(source, done)
			}
		},
		func(source *text.File, ids []uint16) {
			if w.Actions.ExportTextFile != nil {
				w.Actions.ExportTextFile(source, ids)
			}
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
	w.records.SetMapTilesetActions(
		func(source *maps.TilesetConfig, done func(*maps.TilesetDraft, error)) {
			if w.Actions.ImportMapTileset != nil {
				w.Actions.ImportMapTileset(source, done)
			}
		},
		func(source *maps.TilesetConfig) {
			if w.Actions.ExportMapTileset != nil {
				w.Actions.ExportMapTileset(source)
			}
		},
	)
	w.records.SetMapPaletteActions(
		func(source *maps.Palette, section maps.PaletteSection, done func(*maps.PaletteDraft, error)) {
			if w.Actions.ImportMapPalette != nil {
				w.Actions.ImportMapPalette(source, section, done)
			}
		},
		func(source *maps.Palette, section maps.PaletteSection) {
			if w.Actions.ExportMapPalette != nil {
				w.Actions.ExportMapPalette(source, section)
			}
		},
	)
	w.records.SetMapGraphicsActions(
		func(source *maps.Graphics, options maps.GraphicsImport, done func(*maps.GraphicsDraft, error)) {
			if w.Actions.ImportMapGraphics != nil {
				w.Actions.ImportMapGraphics(source, options, done)
			}
		},
		func(source *maps.Graphics, bank int) {
			if w.Actions.ExportMapGraphics != nil {
				w.Actions.ExportMapGraphics(source, bank)
			}
		},
	)
	w.portraits = portraits.New(w.Editor, assets, w.records.Open)
	w.records.SetPortraitOpener(w.OpenPortrait)
	w.shops = shops.New(w.Editor, w.records.OpenShop)
	w.shops.SetNodeOpener(w.openWorldMap)
	w.worldmap = worldmaps.New(
		w.Editor,
		w.records.Open,
		w.openText,
		func(key shop.Key) {
			w.tab = len(records.Kinds) + shopsTab
			w.shops.Open(key)
		},
	)
	w.arenas = arenas.New(w.Editor, w.records.Open)
	w.quotes = quotes.New(w.Editor, w.records.Open, w.openText, w.openHex)
	w.defeats = make(map[int]*quotes.Page)
	w.records.SetDefeatView(
		func(c *ui.Context, id int, available float32) { w.defeat(id).ViewDefeat(c, available) },
	)
	w.supports = supports.New(
		w.Editor,
		w.records.OpenSupport,
		w.openText,
	)
	w.records.SetSupportTalkOpener(
		func(unit, partner int) { w.tab = len(records.Kinds) + supportsTab; w.supports.OpenPair(unit, partner) },
	)
	w.scripts = eventeditor.New(
		w.Editor,
		w.openHex,
		w.openText,
		w.openDeployment,
	)
	w.worldmap.SetScriptOpener(w.OpenScript)
	w.worldmap.SetBackgroundActions(
		func(source *worldmap.Background, done func(*worldmap.BackgroundDraft, error)) {
			if w.Actions.ImportWorldMapBackground != nil {
				w.Actions.ImportWorldMapBackground(source, done)
			}
		},
		func(source *worldmap.Background) {
			if w.Actions.ExportWorldMapBackground != nil {
				w.Actions.ExportWorldMapBackground(source)
			}
		},
	)
	w.worldmap.SetRoadTilesetActions(
		func(source *worldmap.RoadTileset, done func(*worldmap.RoadTilesetDraft, error)) {
			if w.Actions.ImportWorldMapRoadTileset != nil {
				w.Actions.ImportWorldMapRoadTileset(source, done)
			}
		},
		func(source *worldmap.RoadTileset) {
			if w.Actions.ExportWorldMapRoadTileset != nil {
				w.Actions.ExportWorldMapRoadTileset(source)
			}
		},
	)
	w.scripts.SetWorldMapOpener(func(target event.ScriptTarget, id int) {
		w.tab = len(records.Kinds) + worldmapTab
		if target == event.ScriptWorldMapRouteTarget {
			w.worldmap.OpenRoute(id)
		} else {
			w.worldmap.Open(id)
		}
	})
	w.scripts.SetMapChangeOpener(w.openMapChange)
	w.scripts.SetPortraitOpener(w.OpenPortrait)
	w.records.SetScriptOpener(w.OpenScript)
	return w
}

func (w *Workspace) SetSession(session *game.Session) {
	w.Editor.SetSession(session)
	w.records.Reset()
	w.texts.Reset()
	w.scripts.Reset()
	w.portraits.Reset()
	w.shops.Reset()
	w.quotes.Reset()
	w.defeats = make(map[int]*quotes.Page)
	w.supports.Reset()
	w.arenas.Reset()
	w.worldmap.Reset()
	w.tab = 0
}

func (w *Workspace) defeat(id int) *quotes.Page {
	page := w.defeats[id]
	if page == nil {
		page = quotes.New(w.Editor, w.records.Open, w.openText, w.openHex)
		page.ForUnit(id)
		w.defeats[id] = page
	}
	return page
}

// OpenQuote navigates from a text use to its owning native table row.
func (w *Workspace) OpenQuote(kind quote.Kind, index int) {
	if kind == quote.Death || kind == quote.DeathLyn ||
		kind == quote.DeathOther {
		draft, err := w.Editor.Session.Quotes.NewDraft(kind)
		if err == nil {
			value, readErr := draft.Value(index, quote.Unit)
			id := int(value)
			units := w.Editor.Session.Records.Table(records.Units)
			if readErr == nil && units != nil && units.Err == nil &&
				slices.ContainsFunc(
					units.Rows,
					func(row records.Row) bool { return row.ID == id },
				) {
				w.records.OpenDefeat(id)
				w.defeat(id).Open(kind, index)
				return
			}
		}
	}
	w.tab = len(records.Kinds) + quotesTab
	w.quotes.Open(kind, index)
}

func (w *Workspace) openWorldMap(id int) {
	w.tab = len(records.Kinds) + worldmapTab
	w.worldmap.Open(id)
}

func (w *Workspace) openHex(address, length int) {
	if w.Actions.OpenHex != nil {
		w.Actions.OpenHex(address, length)
	}
}

func (w *Workspace) openText(id uint16) {
	w.tab = len(records.Kinds) + textTab
	w.texts.Open(id)
}

// OpenScript navigates to a chapter's physical script entry pointer.
func (w *Workspace) OpenScript(chapter, site int) {
	w.tab = len(records.Kinds) + scriptsTab
	w.scripts.Open(chapter, site)
}

// OpenPortrait navigates to the resource table referenced by a record.
func (w *Workspace) OpenPortrait(
	id int,
) {
	w.tab = len(records.Kinds) + portraitsTab
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
	state.HistoryClaim = editor.HistoryClaimNone
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
			locales.ScriptEditor(),
			locales.PortraitTable(),
			locales.ShopEntries(),
			locales.QuoteEditor(),
			locales.SupportEditor(),
			locales.ArenaEditor(),
		)
		if state.Session.WorldMap.Available() {
			labels = append(labels, locales.Worldmap())
		}
		ui.ScrollHorizontal(c.Key("workspace-tabs")).Shrink(0).Children(func() {
			ui.Tabs(c, &w.tab, labels...).Shrink(0)
		})
		if w.tab >= len(records.Kinds) {
			w.records.Deactivate()
		}
		if w.tab != len(records.Kinds)+textTab {
			w.texts.Deactivate()
		}
		if w.tab != len(records.Kinds)+worldmapTab {
			w.worldmap.Deactivate()
		}
		switch w.tab {
		case len(records.Kinds) + textTab:
			w.texts.View(c)
		case len(records.Kinds) + scriptsTab:
			w.scripts.View(c)
		case len(records.Kinds) + portraitsTab:
			w.portraits.View(c)
		case len(records.Kinds) + shopsTab:
			w.shops.View(c)
		case len(records.Kinds) + quotesTab:
			w.quotes.View(c)
		case len(records.Kinds) + supportsTab:
			w.supports.View(c)
		case len(records.Kinds) + arenasTab:
			w.arenas.View(c)
		case len(records.Kinds) + worldmapTab:
			w.worldmap.View(c)
		default:
			w.records.View(c, w.tab)
		}
		components.StatusBar(c, state, w.Actions.Run, func(c *ui.Context) {
			if w.tab < len(records.Kinds) {
				w.records.Footer(c)
			}
		})
	})
	// Pages and popovers are built; settle history routing once so the menu
	// snapshot and the shortcuts below consume the same result.
	state.ResolveHistoryFocus(c.FocusTakesText())
	history := state.History()
	context := struct {
		text, draft, undo, redo bool
		busy, session           bool
		zoom                    int
	}{
		text: w.UsesTextHistory(), draft: state.DraftHistory != nil,
		undo: history.Undo != nil, redo: history.Redo != nil,
		busy: state.Busy, session: state.Session != nil,
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
