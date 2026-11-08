package app

import (
	"runtime"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (m *application) setMenu() {
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(m.windowMenu(mygo.FocusedWindow()))
		return
	}
	for win := range m.windows {
		win.SetMenu(m.windowMenu(win))
	}
	for _, tool := range m.hexWindows {
		tool.window.SetMenu(m.windowMenu(tool.window))
	}
	if m.logWindow != nil {
		m.logWindow.SetMenu(m.windowMenu(m.logWindow))
	}
}

func (m *application) windowMenu(win *mygo.Window) *mygo.Menu {
	a := m.workspace(win)
	state := m.windowEditor(win)
	action := func(label, shortcut string, run func(*mygo.Window)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: shortcut,
			Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if m.workspace(w) != nil {
					run(w)
					w.Invalidate()
				}
			},
		}
	}
	undo := action(
		locales.Undo(),
		"CmdOrCtrl+Z",
		func(w *mygo.Window) { m.windowEditor(w).Undo() },
	)
	redo := action(
		locales.Redo(),
		"CmdOrCtrl+Shift+Z",
		func(w *mygo.Window) { m.windowEditor(w).Redo() },
	)
	if state == nil || state.TextFocused || state.Busy {
		undo = &mygo.MenuItem{Label: locales.Undo(), Role: mygo.RoleUndo}
		redo = &mygo.MenuItem{Label: locales.Redo(), Role: mygo.RoleRedo}
	} else {
		history := state.History()
		undo.Disabled, redo.Disabled = history.Undo == nil, history.Redo == nil
		if state.DraftHistory != nil {
			undo.Label, redo.Label = locales.MapUndo(), locales.MapRedo()
		}
	}
	zoomIn := action(locales.ZoomIn(), "CmdOrCtrl+Plus", func(w *mygo.Window) {
		if workspace := m.windows[w]; workspace != nil {
			workspace.ZoomIn()
		}
	})
	zoomOut := action(locales.ZoomOut(), "CmdOrCtrl+-", func(w *mygo.Window) {
		if workspace := m.windows[w]; workspace != nil {
			workspace.ZoomOut()
		}
	})
	resetZoom := action(
		locales.ResetZoom(),
		"CmdOrCtrl+0",
		func(w *mygo.Window) {
			if workspace := m.windows[w]; workspace != nil {
				workspace.ResetZoom()
			}
		},
	)
	main := m.windows[win]
	zoomIn.Disabled = main == nil || !main.CanZoomIn()
	zoomOut.Disabled = main == nil || !main.CanZoomOut()
	resetZoom.Disabled = main == nil || main.ZoomPercent() == 100
	open := &mygo.MenuItem{
		Label:       locales.Open(),
		Accelerator: "CmdOrCtrl+O",
		Click: func(_ *mygo.MenuItem, w *mygo.Window) {
			if a := m.workspace(w); a != nil {
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
		func(w *mygo.Window) { m.workspace(w).Actions.Save(false) },
	)
	saveAs := action(
		locales.SaveAs(),
		"CmdOrCtrl+Shift+S",
		func(w *mygo.Window) { m.workspace(w).Actions.Save(true) },
	)
	save.Disabled = a == nil || a.Editor.Busy || a.Editor.Session == nil
	saveAs.Disabled = save.Disabled
	hex := action(
		locales.HexEditor(),
		"CmdOrCtrl+Shift+H",
		func(w *mygo.Window) {
			if m.windows[w] != nil {
				m.openHexEditor(w)
			} else {
				w.Show()
				w.Focus()
			}
		},
	)
	hex.Disabled = save.Disabled
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
				hex,
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
