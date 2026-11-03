package app

import (
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

var errRunSaved = fault.Code{ID: "app.run_saved", Kind: fault.Conflict}

func (m *application) newWindow(session *game.Session) *mygo.Window {
	a := pages.New()
	if session != nil {
		a.SetSession(session)
	}
	titleBarStyle := mygo.TitleBarDefault
	if runtime.GOOS == "darwin" {
		titleBarStyle = mygo.TitleBarHidden
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:          locales.AppTitle(),
		Width:          1150,
		Height:         780,
		MinWidth:       880,
		MinHeight:      560,
		TitleBarStyle:  titleBarStyle,
		TitleBarHeight: pages.HeaderHeight,
		TrafficLightPosition: &mygo.Point{
			X: 16,
			Y: (pages.HeaderHeight - 14) / 2,
		},
		Content: ui.View(a.View),
	})
	m.windows[win] = a
	// Native menus can replace GTK widgets, so update them after the view
	// finishes building and the current native draw has returned.
	a.Actions.MenuStateChanged = func() {
		win.Update(func() {
			if m.windows[win] != a {
				return
			}
			if runtime.GOOS == "darwin" {
				m.setMenu()
			} else {
				win.SetMenu(m.windowMenu(win))
			}
		})
	}
	a.Actions.Open = func() { m.openDialog(win, a) }
	a.Actions.Save = func(as bool) { m.saveDocument(win, a, as) }
	a.Actions.Reload = func() { reloadDocument(win, a) }
	a.Actions.ImportPNG = func(done func([]byte, error)) { importPNG(win, a, done) }
	a.Actions.ExportPNG = func(ref graphics.Ref) { m.exportPNG(win, a, ref) }
	a.Actions.ImportWorldMapBackground = func(source *worldmap.Background, done func(*worldmap.BackgroundDraft, error)) {
		title := locales.WorldmapImportBackground()
		if source.Tone() == worldmap.Darkened {
			title = locales.WorldmapImportDarkBackground()
		}
		importResourcePNG(
			win,
			a,
			title,
			source.ImportPNG,
			done,
		)
	}
	a.Actions.ExportWorldMapBackground = func(source *worldmap.Background) {
		suffix := "worldmap-background"
		if source.Tone() == worldmap.Darkened {
			suffix += "-dark"
		}
		m.exportPNGContent(win, a, suffix, source.PNG)
	}
	a.Actions.ImportWorldMapRoadTileset = func(source *worldmap.RoadTileset, done func(*worldmap.RoadTilesetDraft, error)) {
		importResourcePNG(
			win,
			a,
			locales.WorldmapImportRoadTileset(),
			source.ImportPNG,
			done,
		)
	}
	a.Actions.ExportWorldMapRoadTileset = func(source *worldmap.RoadTileset) {
		m.exportPNGContent(win, a, "worldmap-road-tileset", source.PNG)
	}
	a.Actions.ImportMapTileset = func(source *maps.TilesetConfig, done func(*maps.TilesetDraft, error)) {
		importTilesetConfig(win, a, source, done)
	}
	a.Actions.ExportMapTileset = func(source *maps.TilesetConfig) {
		m.exportTilesetConfig(win, a, source)
	}
	a.Actions.ImportMapPalette = func(source *maps.Palette, section maps.PaletteSection, done func(*maps.PaletteDraft, error)) {
		importResourcePNG(
			win,
			a,
			locales.MapPaletteImport(),
			func(content []byte) (*maps.PaletteDraft, error) { return source.ImportPNG(section, content) },
			done,
		)
	}
	a.Actions.ExportMapPalette = func(source *maps.Palette, section maps.PaletteSection) {
		suffix := "map-palette-normal"
		if section == maps.FogPalette {
			suffix = "map-palette-fog"
		}
		if section == maps.AllPalettes {
			suffix = "map-palette-all"
		}
		m.exportPNGContent(
			win,
			a,
			suffix,
			func() ([]byte, error) { return source.PNG(section) },
		)
	}
	a.Actions.ImportMapGraphics = func(source *maps.Graphics, done func(*maps.GraphicsDraft, error)) {
		importResourcePNG(
			win,
			a,
			locales.MapGraphicsImport(),
			source.ImportPNG,
			done,
		)
	}
	a.Actions.ExportMapGraphics = func(source *maps.Graphics, bank int) {
		suffix := "map-graphics-first"
		if source.Slot() == maps.SecondGraphics {
			suffix = "map-graphics-second"
		}
		m.exportPNGContent(
			win,
			a,
			suffix,
			func() ([]byte, error) { return source.PNG(bank) },
		)
	}
	a.Actions.Run = func() {
		if a.Editor.Session == nil {
			return
		}
		if a.Editor.Session.Document.State().Dirty {
			a.Editor.SetErr(errRunSaved)
			return
		}
		path := a.Editor.Session.Document.State().Path
		background(
			win,
			a,
			"app.launch_emulator",
			func() error {
				if err := launchEmulator(path); err != nil {
					return err
				}
				slog.Info("emulator launched", "path", path)
				return nil
			},
			nil,
		)
	}
	win.OnClose(func(e *mygo.CloseEvent) {
		if a.Editor.Busy {
			e.PreventDefault()
			return
		}
		if a.Editor.Session == nil || !a.Editor.Session.Document.State().Dirty {
			return
		}
		e.PreventDefault()
		a.Editor.Busy = true
		question, save, discard, cancel := locales.CloseQuestion(), locales.Save(), locales.Discard(), locales.Cancel()
		doc := a.Editor.Session.Document
		go func() {
			res, err := mygo.Dialog.Message(
				mygo.MessageOptions{
					Parent:       win,
					Type:         mygo.MessageQuestion,
					Message:      question,
					Buttons:      []string{save, discard, cancel},
					CancelButton: 2,
				},
			)
			closeWindow := false
			if err == nil {
				switch res.Button {
				case 0:
					err = doc.Save()
					closeWindow = err == nil
					if err == nil {
						logROMSaved(doc.State(), false)
					}
				case 1:
					closeWindow = true
				}
			}
			err = fault.Wrap(
				err,
				"app.close_window",
				fault.Context{Path: doc.State().Path},
			)
			win.Update(func() {
				a.Editor.Busy = false
				a.Editor.Report("app.close_window", err)
				a.Editor.SetErr(err)
				if closeWindow {
					win.Destroy()
				}
			})
		}()
	})
	win.OnClosed(func() { delete(m.windows, win) })
	win.OnFocus(func() {
		m.setMenu()
		if a.Editor.Session == nil || a.Editor.Busy {
			return
		}
		doc := a.Editor.Session.Document
		revision := doc.State().Revision
		go func() {
			changed, err := doc.ExternalChanged()
			win.Update(func() {
				a.Editor.Report("app.external_change", err)
				if a.Editor.Session != nil &&
					a.Editor.Session.Document == doc &&
					doc.State().Revision == revision {
					if err != nil {
						a.Editor.SetErr(err)
					} else {
						setExternal(a, changed)
					}
				}
			})
		}()
	})
	updateTitle(win, a)
	m.setMenu()
	m.rememberROMs()
	return win
}

func updateTitle(win *mygo.Window, a *pages.Workspace) {
	title := locales.AppTitle()
	if a.Editor.Session != nil {
		title = filepath.Base(
			a.Editor.Session.Document.State().Path,
		) + " — " + locales.AppTitle()
	}
	win.SetTitle(title)
}

func background(
	win *mygo.Window,
	a *pages.Workspace,
	operation string,
	work func() error,
	completed func(),
) {
	if a.Editor.Busy {
		return
	}
	a.Editor.Busy = true
	doc := a.Editor.Session.Document
	revision := doc.State().Revision
	go func() {
		err := fault.Wrap(
			work(),
			operation,
			fault.Context{Path: doc.State().Path},
		)
		changed, checkErr := doc.ExternalChanged()
		win.Update(
			func() {
				a.Editor.Busy = false
				a.Editor.Report(operation, err)
				if a.Editor.Session == nil ||
					a.Editor.Session.Document != doc ||
					doc.State().Revision != revision {
					a.Editor.Report("app.external_change", checkErr)
					return
				}
				a.Editor.SetErr(err)
				if checkErr != nil {
					a.Editor.Report("app.external_change", checkErr)
				} else {
					setExternal(a, changed)
				}
				updateTitle(win, a)
				if err == nil && completed != nil {
					completed()
				}
			},
		)
	}()
	win.Invalidate()
}
