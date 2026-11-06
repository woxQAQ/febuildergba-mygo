package chapters

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *Page) tabs(c *ui.Context, width float32) {
	labels := []string{
		locales.DeployConfiguration(),
		locales.Deployments(),
		locales.TurnEvents(),
		locales.AreaEvents(),
		locales.TalkEvents(),
		locales.Objectives(),
		locales.MapEditor(),
		locales.MapChangesEditor(),
		locales.LocationEvents(),
		locales.MapTilesetEditor(),
		locales.MapPaletteEditor(),
		locales.MapGraphicsEditor(),
	}
	follow := v.navigationMode != v.mode || v.navigationWidth != width ||
		v.navigationLocale != c.Locale()
	ui.ScrollHorizontal(c.Key("chapter-navigation")).
		Scrollbars(false).
		Width(width).
		MaxWidthPercent(100).
		Children(func() {
			tabs := ui.TabsBase(c.Key("chapter-mode"), &v.mode, len(labels))
			items := make([]ui.Element, len(labels))
			tabs.List.Label(locales.Chapters()).
				Gap(c.Theme().Space(1)).
				OnChange(func() { v.resourceEpoch++ }).
				Children(func() {
					for index, label := range labels {
						tab := tabs.Tab(index).
							Padding(c.Theme().Space(2), c.Theme().Space(3)).
							Label(label).
							BorderWidth(0, 0, 2, 0).
							BorderColor(ui.Transparent).
							TextColor(c.Theme().TextMuted)
						if index == v.mode {
							tab.Bold().
								TextColor(c.Theme().Text).
								BorderColor(c.Theme().Accent)
						} else if tab.Hovered() || tab.Pressed() {
							tab.TextColor(c.Theme().Text)
						}
						items[index] = tab
						tab.Children(func() { ui.Text(c, label).SingleLine() })
					}
				})
			if follow || v.navigationMode != v.mode {
				items[v.mode].ScrollIntoView()
			}
		})
	v.navigationMode, v.navigationWidth, v.navigationLocale = v.mode, width, c.Locale()
}
