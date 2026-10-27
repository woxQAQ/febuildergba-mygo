package app

import (
	"errors"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/locales"
	"github.com/woxQAQ/febuildergba-mygo/pages"
)

func (m *application) newWindow(session *game.Session) *mygo.Window {
	a := pages.New()
	if session != nil {
		a.SetSession(session)
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:          locales.AppTitle(),
		Width:          1150,
		Height:         780,
		MinWidth:       880,
		MinHeight:      560,
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: components.HeaderHeight,
		TrafficLightPosition: &mygo.Point{
			X: 16,
			Y: (components.HeaderHeight - 14) / 2,
		},
		Content: ui.View(a.View),
	})
	m.windows[win] = a
	a.Actions.HistoryContextChanged = m.setMenu
	a.Actions.Open = func() { m.openDialog(win, a) }
	a.Actions.Save = func(as bool) { saveDocument(win, a, as) }
	a.Actions.Reload = func() { reloadDocument(win, a) }
	a.Actions.ImportPNG = func(done func([]byte, error)) { importPNG(win, a, done) }
	a.Actions.ExportPNG = func(ref game.AssetRef) { m.exportPNG(win, a, ref) }
	a.Actions.Run = func() {
		if a.Editor.Session == nil {
			return
		}
		if a.Editor.Session.Document.State().Dirty {
			a.Editor.Err = errors.New("run_saved")
			return
		}
		path := a.Editor.Session.Document.State().Path
		background(win, a, func() error { return launchEmulator(path) })
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
				case 1:
					closeWindow = true
				}
			}
			win.Update(func() {
				a.Editor.Busy = false
				a.Editor.Err = err
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
		go func() {
			changed, _ := doc.ExternalChanged()
			win.Update(func() {
				if a.Editor.Session != nil && a.Editor.Session.Document == doc {
					a.Editor.External = changed
				}
			})
		}()
	})
	updateTitle(win, a)
	m.setMenu()
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

func background(win *mygo.Window, a *pages.Workspace, work func() error) {
	if a.Editor.Busy {
		return
	}
	a.Editor.Busy = true
	doc := a.Editor.Session.Document
	go func() {
		err := work()
		changed, _ := doc.ExternalChanged()
		win.Update(
			func() { a.Editor.Busy = false; a.Editor.Err = err; a.Editor.External = changed; updateTitle(win, a) },
		)
	}()
	win.Invalidate()
}
