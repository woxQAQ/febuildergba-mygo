package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

// One viewer covers the application's shared log stream across ROM windows.
func (m *application) openLogViewer() {
	if m.logWindow != nil {
		m.logWindow.Show()
		m.logWindow.Focus()
		return
	}
	viewer := pages.NewLogViewer(m.logBuffer)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     locales.LogViewer(),
		Width:     1100,
		Height:    740,
		MinWidth:  760,
		MinHeight: 480,
		Content:   ui.View(viewer.View),
	})
	m.logWindow = win
	win.OnFocus(func() { m.setMenu() })
	win.OnClosed(func() {
		if m.logWindow == win {
			m.logWindow = nil
		}
		m.setMenu()
	})
	m.setMenu()
}
