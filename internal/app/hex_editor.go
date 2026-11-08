package app

import (
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type hexEditorWindow struct {
	window *mygo.Window
	page   *pages.HexEditor
	state  hexWindowState
}

// The shared presentation snapshot bounds redraws when either window edits
// the document or completes an operation. Focus and text drafts stay local.
type hexWindowState struct {
	session        *game.Session
	document       rom.State
	busy, external bool
	historyEpoch   uint64
	diagnostic     string
}

func (m *application) openHexEditor(owner *mygo.Window) *hexEditorWindow {
	a := m.windows[owner]
	if a == nil || a.Editor.Session == nil || a.Editor.Busy {
		return nil
	}
	if tool := m.hexWindows[owner]; tool != nil {
		tool.window.Show()
		tool.window.Focus()
		return tool
	}
	viewer := pages.NewHexEditor(a.Editor)
	viewer.Reload = a.Actions.Reload
	tool := &hexEditorWindow{page: viewer}
	win := mygo.NewWindow(mygo.WindowOptions{
		Width:     1000,
		Height:    700,
		MinWidth:  760,
		MinHeight: 480,
		Content: ui.View(func(c *ui.Context) {
			viewer.View(c)
			m.refreshHexEditor(owner)
		}),
	})
	tool.window = win
	m.hexWindows[owner] = tool
	viewer.MenuStateChanged = func() {
		win.Update(func() {
			if m.hexWindows[owner] != tool {
				return
			}
			if runtime.GOOS == "darwin" {
				m.setMenu()
			} else {
				win.SetMenu(m.windowMenu(win))
			}
		})
	}
	win.OnFocus(func() {
		m.setMenu()
		checkExternal(win, a)
	})
	win.OnClosed(func() {
		if m.hexWindows[owner] == tool {
			delete(m.hexWindows, owner)
		}
		m.setMenu()
	})
	updateHexTitle(win, a)
	m.setMenu()
	return tool
}

func updateHexTitle(win *mygo.Window, a *pages.Workspace) {
	win.SetTitle(
		filepath.Base(
			a.Editor.Session.Document.State().Path,
		) + " — " + locales.HexEditor(),
	)
}

func (m *application) refreshHexEditor(owner *mygo.Window) {
	tool := m.hexWindows[owner]
	if tool == nil {
		return
	}
	a := m.windows[owner]
	state := hexWindowState{
		session: a.Editor.Session, busy: a.Editor.Busy,
		external: a.Editor.External, historyEpoch: a.Editor.HistoryEpoch,
		document: a.Editor.Session.Document.State(),
	}
	if a.Editor.Err != nil {
		state.diagnostic = a.Editor.Err.Error()
	}
	if state == tool.state {
		return
	}
	tool.state = state
	updateHexTitle(tool.window, a)
	owner.Invalidate()
	tool.window.Invalidate()
}

// workspace resolves document actions from either the main or byte window.
func (m *application) workspace(win *mygo.Window) *pages.Workspace {
	if a := m.windows[win]; a != nil {
		return a
	}
	for owner, tool := range m.hexWindows {
		if tool.window == win {
			return m.windows[owner]
		}
	}
	return nil
}

func (m *application) windowEditor(win *mygo.Window) *editor.State {
	if a := m.windows[win]; a != nil {
		return a.Editor
	}
	for _, tool := range m.hexWindows {
		if tool.window == win {
			return tool.page.Editor
		}
	}
	return nil
}
