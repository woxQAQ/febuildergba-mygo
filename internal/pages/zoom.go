package pages

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

var zoomLevels = [...]int{75, 100, 125, 150}

// ZoomPercent is a window display preference, independent of map magnification.
func (w *Workspace) ZoomPercent() int { return zoomLevels[w.zoomLevel] }

func (w *Workspace) CanZoomIn() bool  { return w.zoomLevel < len(zoomLevels)-1 }
func (w *Workspace) CanZoomOut() bool { return w.zoomLevel > 0 }

func (w *Workspace) ZoomIn()    { w.zoomLevel = min(w.zoomLevel+1, len(zoomLevels)-1) }
func (w *Workspace) ZoomOut()   { w.zoomLevel = max(w.zoomLevel-1, 0) }
func (w *Workspace) ResetZoom() { w.zoomLevel = 1 }

func (w *Workspace) zoomControl(c *ui.Context) {
	value := fmt.Sprintf("%d%%", w.ZoomPercent())
	ui.PopoverMenuButton(c.Key("interface-zoom"), value, func(menu *ui.Menu) {
		for index, percent := range zoomLevels {
			if menu.Item(fmt.Sprintf("%d%%", percent)).
				Radio(index == w.zoomLevel).
				Chosen() {
				w.zoomLevel = index
			}
		}
	}).
		Label(locales.InterfaceZoom()).
		Value(value).
		Tooltip(locales.InterfaceZoom())
}

func (w *Workspace) zoomShortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyEqual, w.ZoomIn)
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyEqual, w.ZoomIn)
	c.OnShortcut(ui.Cmd, ui.KeyMinus, w.ZoomOut)
	c.OnShortcut(ui.Cmd, ui.Key0, w.ResetZoom)
}
