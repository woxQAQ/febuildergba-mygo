package app

import (
	"runtime"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

func (m *application) setMenu() {
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(m.windowMenu(mygo.FocusedWindow()))
		return
	}
	for win := range m.windows {
		win.SetMenu(m.windowMenu(win))
	}
	if m.logWindow != nil {
		m.logWindow.SetMenu(m.windowMenu(m.logWindow))
	}
}

func (m *application) windowMenu(win *mygo.Window) *mygo.Menu {
	a := m.windows[win]
	action := func(label, shortcut string, run func(*pages.Workspace)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: shortcut,
			Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if a := m.windows[w]; a != nil {
					run(a)
					w.Invalidate()
				}
			},
		}
	}
	undo := action(
		locales.Undo(),
		"CmdOrCtrl+Z",
		func(a *pages.Workspace) { a.Editor.Undo() },
	)
	redo := action(
		locales.Redo(),
		"CmdOrCtrl+Shift+Z",
		func(a *pages.Workspace) { a.Editor.Redo() },
	)
	if a == nil || a.UsesTextHistory() {
		undo = &mygo.MenuItem{Label: locales.Undo(), Role: mygo.RoleUndo}
		redo = &mygo.MenuItem{Label: locales.Redo(), Role: mygo.RoleRedo}
	} else {
		history := a.Editor.History()
		undo.Disabled, redo.Disabled = history.Undo == nil, history.Redo == nil
		if a.Editor.DraftHistory != nil {
			undo.Label, redo.Label = locales.MapUndo(), locales.MapRedo()
		}
	}
	zoomIn := action(
		locales.ZoomIn(),
		"CmdOrCtrl+Plus",
		func(a *pages.Workspace) { a.ZoomIn() },
	)
	zoomOut := action(
		locales.ZoomOut(),
		"CmdOrCtrl+-",
		func(a *pages.Workspace) { a.ZoomOut() },
	)
	resetZoom := action(
		locales.ResetZoom(),
		"CmdOrCtrl+0",
		func(a *pages.Workspace) { a.ResetZoom() },
	)
	zoomIn.Disabled = a == nil || !a.CanZoomIn()
	zoomOut.Disabled = a == nil || !a.CanZoomOut()
	resetZoom.Disabled = a == nil || a.ZoomPercent() == 100
	open := &mygo.MenuItem{
		Label:       locales.Open(),
		Accelerator: "CmdOrCtrl+O",
		Click: func(_ *mygo.MenuItem, w *mygo.Window) {
			if a := m.windows[w]; a != nil {
				a.Actions.Open()
			} else {
				win := m.newWindow(nil)
				m.windows[win].Actions.Open()
			}
		},
	}
	open.Disabled = a != nil && a.Editor.Busy
	save := action(
		locales.Save(),
		"CmdOrCtrl+S",
		func(a *pages.Workspace) { a.Actions.Save(false) },
	)
	saveAs := action(
		locales.SaveAs(),
		"CmdOrCtrl+Shift+S",
		func(a *pages.Workspace) { a.Actions.Save(true) },
	)
	save.Disabled = a == nil || a.Editor.Busy || a.Editor.Session == nil
	saveAs.Disabled = save.Disabled
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.File(), Submenu: []*mygo.MenuItem{
			open,
			save,
			saveAs,
			{Role: mygo.RoleClose},
		}},
		{Label: locales.Edit(), Submenu: []*mygo.MenuItem{
			undo,
			redo,
			mygo.Separator(),
			{Role: mygo.RoleCut},
			{Role: mygo.RoleCopy},
			{Role: mygo.RolePaste},
			{Role: mygo.RoleSelectAll},
		}},
		{
			Label: locales.ViewMenu(),
			Submenu: []*mygo.MenuItem{
				zoomIn,
				zoomOut,
				resetZoom,
				mygo.Separator(),
				{
					Label:       locales.LogViewer(),
					Accelerator: "CmdOrCtrl+Shift+L",
					Click:       func(*mygo.MenuItem, *mygo.Window) { m.openLogViewer() },
				},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
}
