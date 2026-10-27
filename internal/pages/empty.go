// Package pages composes the editor's workspace and navigation.
package pages

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func empty(c *ui.Context) {
	ui.Column(c).Grow(1).Center().Gap(12).Padding(32).Children(func() {
		ui.Text(c, locales.AppTitle()).FontSize(28).Bold()
		ui.Text(c, locales.EmptyTitle()).FontSize(18)
		ui.Text(c, locales.EmptyDetail()).TextColor(c.Theme().TextMuted)
	})
}
