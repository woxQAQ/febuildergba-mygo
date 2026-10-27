package app

import (
	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/locales"
	"github.com/woxQAQ/febuildergba-mygo/pages"
)

func (m *application) setMenu() {
	action := func(label, shortcut string, run func(*pages.Workspace)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: shortcut,
			Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if a := m.windows[w]; a != nil {
					run(a)
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
	if a := m.windows[mygo.FocusedWindow()]; a == nil || a.UsesTextHistory() {
		undo = &mygo.MenuItem{Label: locales.Undo(), Role: mygo.RoleUndo}
		redo = &mygo.MenuItem{Label: locales.Redo(), Role: mygo.RoleRedo}
	}
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
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.File(), Submenu: []*mygo.MenuItem{
			open,
			action(
				locales.Save(),
				"CmdOrCtrl+S",
				func(a *pages.Workspace) { a.Actions.Save(false) },
			),
			action(
				locales.SaveAs(),
				"CmdOrCtrl+Shift+S",
				func(a *pages.Workspace) { a.Actions.Save(true) },
			),
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
		{Role: mygo.RoleWindowMenu},
	}))
}
