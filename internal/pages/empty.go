// Package pages composes the editor's workspace and navigation.
package pages

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (w *Workspace) empty(c *ui.Context) {
	ui.Column(c.Key("empty-workspace")).
		Grow(1).
		Center().
		Padding(16).
		Children(func() {
			ui.Column(c).
				Width(440).
				MaxWidthPercent(100).
				Gap(12).
				AlignItems(ui.Start).
				Children(func() {
					ui.Text(c, locales.EmptyTitle()).FontSize(20).Bold()
					ui.Text(c, locales.EmptyDetail()).
						TextColor(c.Theme().TextMuted)
					ui.PrimaryButton(c.Key("open-rom"), locales.Open()).
						Disabled(w.Editor.Busy || w.Actions.Open == nil).
						OnClick(w.Actions.Open)
				})
		})
}
